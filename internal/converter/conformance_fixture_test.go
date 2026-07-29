package converter

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmitConformanceFixtures writes the chunk stream this implementation
// produces for a set of fixed inputs, so other implementations can be checked
// against it rather than against a reading of this source.
//
// The browser client re-implements the chunking rules in TypeScript because a
// .pixe has to be creatable in a tab with no daemon. Two independent chunkers
// is exactly the situation that produced the original bug — capsules that one
// side wrote and the other could not read — so the rules are pinned here as
// data.
//
// Skipped unless PIXE_FIXTURE_DIR is set; it writes files, which a normal test
// run should not.
func TestEmitConformanceFixtures(t *testing.T) {
	outDir := os.Getenv("PIXE_FIXTURE_DIR")
	if outDir == "" {
		t.Skip("set PIXE_FIXTURE_DIR to emit conformance fixtures")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	conv := newTestConverter(t, dir)

	// A fixed timestamp keeps the fixture byte-stable across runs; the real
	// encoder uses time.Now(), which is irrelevant to the chunking rules.
	const fixedTime = "2026-07-29T00:00:00Z"

	cases := []struct {
		name    string
		file    string
		content []byte
	}{
		{"ascii_markdown", "notes.md", []byte(strings.Repeat("- the quick brown fox jumps over the lazy dog\n", 120))},
		{"unicode_markdown", "unicode.md", []byte(strings.Repeat("héllo → 世界 🌍 \"quoted\"\n", 200))},
		{"binary_blob", "blob.bin", func() []byte {
			b := make([]byte, 8000)
			for i := range b {
				b[i] = byte((i * 7) % 251)
			}
			return b
		}()},
		{"tiny_text", "tiny.md", []byte("# one frame\n")},
		{"json_payload", "data.json", []byte(`{"a":1,"b":"<tag>&amp;","c":[1,2,3]}`)},
	}

	type fixture struct {
		Name     string     `json:"name"`
		Filename string     `json:"filename"`
		MimeType string     `json:"mime_type"`
		Content  string     `json:"content_base64"`
		Chunks   []qrChunkJ `json:"chunks"`
	}

	var out []fixture
	for _, tc := range cases {
		src := filepath.Join(dir, tc.file)
		if err := os.WriteFile(src, tc.content, 0o644); err != nil {
			t.Fatal(err)
		}
		chunks, item, err := conv.processFile(src, "")
		if err != nil {
			t.Fatalf("%s: processFile: %v", tc.name, err)
		}

		normalized := make([]qrChunkJ, 0, len(chunks))
		for _, c := range chunks {
			normalized = append(normalized, qrChunkJ{
				ID:         c.ID,
				Index:      c.Index,
				Total:      c.Total,
				Data:       c.Data,
				SourceFile: c.SourceFile,
				MimeType:   c.MimeType,
				Hash:       c.Hash,
				Encrypted:  c.Encrypted,
				CreatedAt:  fixedTime,
			})
		}

		out = append(out, fixture{
			Name:     tc.name,
			Filename: tc.file,
			MimeType: item.Type,
			Content:  encodeBase64(tc.content),
			Chunks:   normalized,
		})
		t.Logf("%-18s %6d bytes -> %2d chunks (%s)", tc.name, len(tc.content), len(chunks), item.Type)
	}

	blob, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(outDir, "go-chunks.json")
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s (%d bytes)", path, len(blob))
}

func encodeBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// qrChunkJ mirrors qr.Chunk with a string timestamp so fixtures are stable.
type qrChunkJ struct {
	ID         string `json:"id"`
	Index      int    `json:"index"`
	Total      int    `json:"total"`
	Data       string `json:"data"`
	SourceFile string `json:"source_file"`
	MimeType   string `json:"mime_type"`
	Hash       string `json:"hash"`
	Encrypted  bool   `json:"encrypted"`
	CreatedAt  string `json:"created_at"`
}
