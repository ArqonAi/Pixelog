package index

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

// The bug this guards: a 512-token embedder was handed ~2000 tokens
// because the cap was a fixed 8000 bytes, so every oversized frame
// returned HTTP 400 and the whole index came out empty.
func TestSplitForEmbedding_EveryChunkWithinLimit(t *testing.T) {
	limit := 1530 // (512-2)*3, the real budget for a 512-token model
	text := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 400)

	chunks := splitForEmbedding(text, limit)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > limit {
			t.Errorf("chunk %d is %d bytes, over the %d limit", i, len(c), limit)
		}
	}
}

// Splitting exists so nothing is dropped. Truncation would lose the tail
// silently, which is worse than a slow index.
func TestSplitForEmbedding_PreservesAllContent(t *testing.T) {
	text := strings.Repeat("alpha beta gamma delta epsilon. ", 200)
	chunks := splitForEmbedding(text, 500)

	var sb strings.Builder
	for _, c := range chunks {
		sb.WriteString(c)
	}
	// Whitespace at break points is consumed; compare on non-space runes.
	strip := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if r != ' ' && r != '\n' && r != '\t' && r != '\r' {
				b.WriteRune(r)
			}
		}
		return b.String()
	}
	if got, want := strip(sb.String()), strip(text); got != want {
		t.Errorf("content lost: reassembled %d chars, original %d", len(got), len(want))
	}
}

func TestSplitForEmbedding_ShortTextUnchanged(t *testing.T) {
	chunks := splitForEmbedding("short", 1000)
	if len(chunks) != 1 || chunks[0] != "short" {
		t.Errorf("got %q", chunks)
	}
}

func TestSplitForEmbedding_PrefersParagraphBoundary(t *testing.T) {
	a := strings.Repeat("a", 600)
	b := strings.Repeat("b", 600)
	chunks := splitForEmbedding(a+"\n\n"+b, 1000)

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != a {
		t.Errorf("first chunk should end at the blank line (got %d bytes)", len(chunks[0]))
	}
	if chunks[1] != b {
		t.Errorf("second chunk should start after the blank line (got %d bytes)", len(chunks[1]))
	}
}

func TestSplitForEmbedding_PrefersSentenceBoundary(t *testing.T) {
	first := strings.Repeat("x", 700) + "."
	text := first + " " + strings.Repeat("y", 500)
	chunks := splitForEmbedding(text, 1000)

	if len(chunks) < 2 {
		t.Fatalf("expected a split, got %d chunk(s)", len(chunks))
	}
	if !strings.HasSuffix(chunks[0], ".") {
		t.Errorf("first chunk should end at the sentence terminator, got %q...", chunks[0][max(0, len(chunks[0])-20):])
	}
}

// Minified JSON and base64 blobs have no whitespace at all. The splitter
// must still make progress rather than loop or emit an oversized chunk.
func TestSplitForEmbedding_NoBoundaryStillSplits(t *testing.T) {
	text := strings.Repeat("Zm9vYmFy", 500) // 4000 bytes, no spaces
	chunks := splitForEmbedding(text, 900)

	if len(chunks) < 4 {
		t.Fatalf("expected several chunks, got %d", len(chunks))
	}
	total := 0
	for i, c := range chunks {
		if len(c) > 900 {
			t.Errorf("chunk %d is %d bytes, over limit", i, len(c))
		}
		total += len(c)
	}
	if total != len(text) {
		t.Errorf("reassembled %d bytes, want %d", total, len(text))
	}
}

// A chunk cut through a multi-byte character is invalid UTF-8 and the
// endpoint rejects the request as malformed JSON.
func TestSplitForEmbedding_KeepsValidUTF8(t *testing.T) {
	text := strings.Repeat("日本語のテキストです", 200)
	for _, c := range splitForEmbedding(text, 700) {
		if !utf8.ValidString(c) {
			t.Fatalf("chunk is not valid UTF-8: %q", c[:20])
		}
	}
}

func TestSplitForEmbedding_ZeroLimitDoesNotLoop(t *testing.T) {
	chunks := splitForEmbedding("some text", 0)
	if len(chunks) != 1 {
		t.Errorf("limit 0 should yield the text unchanged, got %d chunks", len(chunks))
	}
}

func TestTruncateRunes_DoesNotSplitCharacters(t *testing.T) {
	s := "日本語"                                  // 9 bytes, 3 runes
	if got := truncateRunes(s, 4); got != "日" { // 4 lands mid-rune
		t.Errorf("truncateRunes(%q, 4) = %q, want %q", s, got, "日")
	}
	if got := truncateRunes(s, 100); got != s {
		t.Errorf("under-limit input should be unchanged, got %q", got)
	}
}

// The limit must come from the endpoint, not a constant. vLLM, TGI and
// llama.cpp all advertise max_model_len on /models.
func TestMaxInputChars_UsesAdvertisedModelLen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "bge-base-en-v1.5", "max_model_len": 512},
			},
		})
	}))
	defer srv.Close()

	e := &SimpleEmbedder{
		spec:   EndpointSpec{BaseURL: srv.URL},
		model:  "bge-base-en-v1.5",
		client: srv.Client(),
	}
	want := (512 - reservedTokens) * charsPerToken
	if got := e.MaxInputChars(); got != want {
		t.Errorf("MaxInputChars() = %d, want %d", got, want)
	}
	if got := e.MaxInputChars(); got != want {
		t.Errorf("second call = %d, want the cached %d", got, want)
	}
}

// A multi-model server may list several limits; the one in use wins.
func TestMaxInputChars_MatchesConfiguredModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "big-model", "max_model_len": 8192},
				{"id": "small-embedder", "max_model_len": 256},
			},
		})
	}))
	defer srv.Close()

	e := &SimpleEmbedder{
		spec:   EndpointSpec{BaseURL: srv.URL},
		model:  "small-embedder",
		client: srv.Client(),
	}
	want := (256 - reservedTokens) * charsPerToken
	if got := e.MaxInputChars(); got != want {
		t.Errorf("MaxInputChars() = %d, want %d for the configured model", got, want)
	}
}

// Hosted APIs generally don't advertise a limit; fall to the safe cap.
func TestMaxInputChars_NoAdvertisedLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": "text-embedding-3-small"}},
		})
	}))
	defer srv.Close()

	e := &SimpleEmbedder{
		spec:   EndpointSpec{BaseURL: srv.URL},
		model:  "text-embedding-3-small",
		client: srv.Client(),
	}
	if got := e.MaxInputChars(); got != absoluteCharCap {
		t.Errorf("MaxInputChars() = %d, want the %d cap", got, absoluteCharCap)
	}
}

func TestMaxInputChars_UnreachableEndpoint(t *testing.T) {
	e := &SimpleEmbedder{
		spec:   EndpointSpec{BaseURL: "http://127.0.0.1:1"},
		model:  "whatever",
		client: &http.Client{},
	}
	if got := e.MaxInputChars(); got != absoluteCharCap {
		t.Errorf("MaxInputChars() = %d, want the %d cap", got, absoluteCharCap)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
