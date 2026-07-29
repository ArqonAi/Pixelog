package converter

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArqonAi/Pixelog/pkg/config"
)

func newTestConverter(t *testing.T, dir string) *Converter {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH")
	}
	conv, err := New(&config.Config{
		ChunkSize: 2900,
		FrameRate: 2.0,
		Quality:   23,
		TempDir:   filepath.Join(dir, "temp"),
		OutputDir: filepath.Join(dir, "output"),
	})
	if err != nil {
		t.Skipf("converter unavailable: %v", err)
	}
	return conv
}

// TestStreamRoundTrip is the regression test for --stream producing capsules
// that could not be read back.
//
// ProcessFileStreaming used to emit chunks carrying only Index, Total and Data
// — no SourceFile, MimeType or Hash — and stored raw bytes where the extractor
// expects base64 for non-text. Extraction groups by Hash and switches on
// MimeType, so every streamed capsule failed with
// `failed to decode base64 data for : illegal base64 data at input byte 0`,
// the empty name being the unset SourceFile.
//
// `pixe verify` reported those same files as "File integrity: GOOD" because it
// only checks that frames decode as QR codes and never reassembles, so this
// test asserts on the extracted bytes instead.
func TestStreamRoundTrip(t *testing.T) {
	// Text and binary take different encoding paths, and the mixed/unicode
	// cases exercise JSON escaping and rune-boundary splitting.
	var markdown strings.Builder
	markdown.WriteString("# Capsule\n\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&markdown, "- fact %d: the quick brown fox jumps over the lazy dog number %d\n", i, i)
	}

	binary := make([]byte, 40000)
	if _, err := rand.Read(binary); err != nil {
		t.Fatal(err)
	}

	unicode := strings.Repeat("héllo → 世界 🌍 \"quoted\"\t{braces}\n", 700)

	cases := []struct {
		name    string
		file    string
		content []byte
	}{
		{"markdown", "capsule.md", []byte(markdown.String())},
		{"binary", "blob.bin", binary},
		{"unicode_text", "unicode.txt", []byte(unicode)},
		{"tiny", "tiny.md", []byte("# just one frame\n")},
		{"empty", "empty.md", []byte{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			conv := newTestConverter(t, dir)

			src := filepath.Join(dir, tc.file)
			if err := os.WriteFile(src, tc.content, 0o644); err != nil {
				t.Fatal(err)
			}

			out := filepath.Join(dir, "capsule.pixe")
			streamer := NewStreamingProcessor(conv)
			if err := streamer.StreamToVideo(src, out, ""); err != nil {
				if len(tc.content) == 0 {
					t.Skipf("empty input rejected by streaming: %v", err)
				}
				t.Fatalf("StreamToVideo: %v", err)
			}

			extractDir := filepath.Join(dir, "extracted")
			if err := os.MkdirAll(extractDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := conv.Extract(out, extractDir); err != nil {
				t.Fatalf("a capsule written by --stream cannot be extracted: %v", err)
			}

			got, err := os.ReadFile(filepath.Join(extractDir, tc.file))
			if err != nil {
				entries, _ := os.ReadDir(extractDir)
				var names []string
				for _, e := range entries {
					names = append(names, e.Name())
				}
				t.Fatalf("expected %s in the output; extractDir holds %v (err %v)", tc.file, names, err)
			}

			if !bytes.Equal(got, tc.content) {
				t.Errorf("round trip is not byte-exact: wrote %d bytes, recovered %d",
					len(tc.content), len(got))
			}
		})
	}
}

// TestConvertRoundTrip covers the non-streaming path over the same inputs.
//
// Both paths feed the same extractor, so both must be byte-exact. The default
// path corrupted multibyte text: createChunks sliced the raw UTF-8 payload at
// a fixed byte offset, and json.Marshal turned each split rune into U+FFFD,
// so a 23400-byte mixed-script file came back as 23428 bytes. It went
// unnoticed because ASCII and base64 payloads never split a rune.
func TestConvertRoundTrip(t *testing.T) {
	var markdown strings.Builder
	markdown.WriteString("# Capsule\n\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&markdown, "- fact %d: the quick brown fox jumps over the lazy dog number %d\n", i, i)
	}

	binary := make([]byte, 40000)
	if _, err := rand.Read(binary); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		file    string
		content []byte
	}{
		{"markdown", "capsule.md", []byte(markdown.String())},
		{"binary", "blob.bin", binary},
		{"unicode_text", "unicode.txt", []byte(strings.Repeat("héllo → 世界 🌍 \"quoted\"\t{braces}\n", 700))},
		{"emoji_dense", "emoji.txt", []byte(strings.Repeat("🌍🌎🌏", 2000))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			conv := newTestConverter(t, dir)

			src := filepath.Join(dir, tc.file)
			if err := os.WriteFile(src, tc.content, 0o644); err != nil {
				t.Fatal(err)
			}

			out := filepath.Join(dir, "capsule.pixe")
			if err := conv.Convert(src, out, nil); err != nil {
				t.Fatalf("Convert: %v", err)
			}

			extractDir := filepath.Join(dir, "extracted")
			if err := os.MkdirAll(extractDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := conv.Extract(out, extractDir); err != nil {
				t.Fatalf("Extract: %v", err)
			}

			got, err := os.ReadFile(filepath.Join(extractDir, tc.file))
			if err != nil {
				t.Fatalf("reading extracted %s: %v", tc.file, err)
			}
			if !bytes.Equal(got, tc.content) {
				t.Errorf("round trip is not byte-exact: wrote %d bytes, recovered %d",
					len(tc.content), len(got))
			}
		})
	}
}

// TestEncryptRoundTrip pins that an encrypted capsule is actually encrypted
// and comes back with the password.
//
// The CLI built its config without EncryptionEnabled, and the encryption
// service returns its input untouched when disabled. So `convert --encrypt
// --password x` validated the password, reported success, and wrote the file
// in plaintext — the one failure mode where a silent no-op is a security
// problem rather than a corruption one.
func TestEncryptRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH")
	}
	conv, err := New(&config.Config{
		ChunkSize:         2900,
		FrameRate:         2.0,
		Quality:           23,
		TempDir:           filepath.Join(dir, "temp"),
		OutputDir:         filepath.Join(dir, "output"),
		EncryptionEnabled: true,
	})
	if err != nil {
		t.Skipf("converter unavailable: %v", err)
	}

	secret := []byte(strings.Repeat("the launch code is hunter2\n", 200))
	src := filepath.Join(dir, "secret.md")
	if err := os.WriteFile(src, secret, 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "secret.pixe")
	if err := conv.Convert(src, out, nil, "correct horse battery staple"); err != nil {
		t.Fatalf("Convert: %v", err)
	}

	// Extracting without the password must not yield the plaintext.
	plainDir := filepath.Join(dir, "no-password")
	if err := os.MkdirAll(plainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := conv.Extract(out, plainDir); err == nil {
		if got, err := os.ReadFile(filepath.Join(plainDir, "secret.md")); err == nil {
			if bytes.Equal(got, secret) {
				t.Error("capsule was written in plaintext: extraction without the password returned the secret")
			}
		}
	}

	// With the password it must come back byte-exact.
	okDir := filepath.Join(dir, "with-password")
	if err := os.MkdirAll(okDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := conv.Extract(out, okDir, "correct horse battery staple"); err != nil {
		t.Fatalf("Extract with password: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(okDir, "secret.md"))
	if err != nil {
		t.Fatalf("reading decrypted output: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Errorf("decrypted round trip is not byte-exact: %d vs %d bytes", len(got), len(secret))
	}
}

// TestStreamRefusesEncryption pins that --stream and --encrypt cannot be
// combined silently.
//
// Streaming sealed each frame with its own AES-GCM nonce and tag, but Extract
// decrypts the reassembled file in a single pass, so such a capsule can never
// be recovered. Writing one is worse than refusing.
func TestStreamRefusesEncryption(t *testing.T) {
	dir := t.TempDir()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH")
	}
	conv, err := New(&config.Config{
		ChunkSize:         2900,
		FrameRate:         2.0,
		Quality:           23,
		TempDir:           filepath.Join(dir, "temp"),
		OutputDir:         filepath.Join(dir, "output"),
		EncryptionEnabled: true,
	})
	if err != nil {
		t.Skipf("converter unavailable: %v", err)
	}

	src := filepath.Join(dir, "secret.md")
	if err := os.WriteFile(src, []byte(strings.Repeat("classified\n", 500)), 0o644); err != nil {
		t.Fatal(err)
	}

	streamer := NewStreamingProcessor(conv)
	err = streamer.StreamToVideo(src, filepath.Join(dir, "secret.pixe"), "hunter2")
	if err == nil {
		t.Fatal("streaming accepted a password; per-frame encryption cannot be reassembled by Extract")
	}
	if !strings.Contains(err.Error(), "cannot encrypt") {
		t.Errorf("error should explain the incompatibility, got: %v", err)
	}
}
