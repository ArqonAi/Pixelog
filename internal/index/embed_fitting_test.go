package index

import (
	"fmt"
	"strings"
	"testing"
)

// Rejects input over limit bytes the way vLLM does for a 512-token model.
type limitEmbedder struct {
	limit int
	calls int
}

func (l *limitEmbedder) Embed(text string) ([]float32, error) {
	l.calls++
	if len(text) > l.limit {
		return nil, fmt.Errorf(`endpoint returned 400: {"error":{"message":"This model's maximum context length is 512 tokens. However, your prompt contains at least 513 input tokens"}}`)
	}
	return []float32{float32(len(text))}, nil
}

func TestEmbedFittingHalvesRejectedChunks(t *testing.T) {
	// A dense JSON frame: the char budget let 2,250 chars through to a
	// model that holds ~600 of them.
	text := strings.Repeat(`{"k":"v","n":1},`, 140) // 2,240 chars
	e := &limitEmbedder{limit: 600}
	chunks, vecs, err := embedFitting(e, []string{text})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(chunks, "") != text {
		t.Fatal("every byte must stay indexed")
	}
	for i, c := range chunks {
		if len(c) > 600 || vecs[i] == nil {
			t.Fatalf("chunk %d: %d chars, vec %v", i, len(c), vecs[i])
		}
	}
	// Other failures are reported, not split.
	if _, vecs, err := embedFitting(&failingEmbedder{}, []string{"hello"}); err == nil || vecs[0] != nil {
		t.Fatalf("err=%v vecs=%v", err, vecs)
	}
}

type failingEmbedder struct{}

func (failingEmbedder) Embed(string) ([]float32, error) { return nil, fmt.Errorf("connection refused") }
