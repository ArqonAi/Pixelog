package memory

import (
	"strings"
	"testing"
)

func TestLLMCompressor_FallbackCompress(t *testing.T) {
	compressor := NewLLMCompressor(nil) // no LLM — heuristic mode

	content := "Machine learning models require training data. The data must be cleaned and preprocessed. Feature engineering improves model accuracy. Cross-validation prevents overfitting."
	frame, err := compressor.CompressFrame("f1", content, "test.pixe")
	if err != nil {
		t.Fatalf("fallback compress failed: %v", err)
	}

	if frame.FrameID != "f1" {
		t.Errorf("expected frame ID 'f1', got %q", frame.FrameID)
	}
	if frame.Source != "test.pixe" {
		t.Errorf("expected source 'test.pixe', got %q", frame.Source)
	}
	if frame.Narrative == "" {
		t.Error("expected non-empty narrative from fallback")
	}
	if len(frame.Facts) == 0 {
		t.Error("expected at least one fact from fallback")
	}
}

func TestLLMCompressor_WithMockLLM(t *testing.T) {
	mockResponse := `<facts>
- Go is a statically typed language
- Go supports concurrency via goroutines
</facts>

<concepts>
Go, concurrency, goroutines, static typing
</concepts>

<narrative>
Go is a systems programming language that supports concurrency through goroutines and channels.
</narrative>`

	chatFn := func(prompt string) (string, error) {
		return mockResponse, nil
	}

	compressor := NewLLMCompressor(chatFn)
	frame, err := compressor.CompressFrame("f2", "some go content", "code.pixe")
	if err != nil {
		t.Fatalf("compress with mock LLM failed: %v", err)
	}

	if len(frame.Facts) != 2 {
		t.Errorf("expected 2 facts, got %d: %v", len(frame.Facts), frame.Facts)
	}
	if !strings.Contains(frame.Facts[0], "statically typed") {
		t.Errorf("unexpected first fact: %q", frame.Facts[0])
	}

	if len(frame.Concepts) < 3 {
		t.Errorf("expected at least 3 concepts, got %d: %v", len(frame.Concepts), frame.Concepts)
	}

	if !strings.Contains(frame.Narrative, "goroutines") {
		t.Errorf("narrative missing goroutines: %q", frame.Narrative)
	}
}

func TestLLMCompressor_EmptyResponse(t *testing.T) {
	chatFn := func(prompt string) (string, error) {
		return "", nil // empty response
	}

	compressor := NewLLMCompressor(chatFn)
	frame, err := compressor.CompressFrame("f3", "test content", "test.pixe")
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}

	// Should fallback to heuristic
	if frame.Narrative == "" {
		t.Error("expected non-empty narrative from fallback after empty LLM response")
	}
}

func TestLLMCompressor_CompressBatch(t *testing.T) {
	compressor := NewLLMCompressor(nil)

	items := []struct {
		FrameID    string
		Content    string
		SourceFile string
	}{
		{"f1", "First frame content about databases.", "db.pixe"},
		{"f2", "Second frame about networking protocols.", "net.pixe"},
		{"f3", "Third frame covering security concepts.", "sec.pixe"},
	}

	results, err := compressor.CompressBatch(items)
	if err != nil {
		t.Fatalf("batch compress failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 compressed frames, got %d", len(results))
	}

	for _, r := range results {
		if r.FrameID == "" {
			t.Error("empty frame ID in batch result")
		}
		if r.Narrative == "" {
			t.Error("empty narrative in batch result")
		}
	}
}

func TestParseCompressedResponse(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		wantFacts  int
		wantConcepts int
		wantNarr   bool
	}{
		{
			name: "complete response",
			response: `<facts>
- fact one
- fact two
- fact three
</facts>
<concepts>
alpha, beta, gamma
</concepts>
<narrative>
A summary of the content.
</narrative>`,
			wantFacts:    3,
			wantConcepts: 3,
			wantNarr:     true,
		},
		{
			name:         "empty response",
			response:     "",
			wantFacts:    0,
			wantConcepts: 0,
			wantNarr:     false,
		},
		{
			name:     "partial response with facts only",
			response: "<facts>\n- only one fact\n</facts>",
			wantFacts: 1,
			wantConcepts: 0,
			wantNarr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := parseCompressedResponse(tt.response)

			if len(frame.Facts) != tt.wantFacts {
				t.Errorf("expected %d facts, got %d: %v", tt.wantFacts, len(frame.Facts), frame.Facts)
			}
			if len(frame.Concepts) != tt.wantConcepts {
				t.Errorf("expected %d concepts, got %d: %v", tt.wantConcepts, len(frame.Concepts), frame.Concepts)
			}
			if tt.wantNarr && frame.Narrative == "" {
				t.Error("expected non-empty narrative")
			}
		})
	}
}

func TestExtractHeuristicConcepts(t *testing.T) {
	content := "Machine Learning and Natural Language Processing are subfields of Artificial Intelligence. Deep Neural Networks are commonly used."
	concepts := extractHeuristicConcepts(content)

	if len(concepts) < 2 {
		t.Errorf("expected at least 2 concepts, got %d: %v", len(concepts), concepts)
	}

	// Check dedup
	seen := make(map[string]bool)
	for _, c := range concepts {
		lower := strings.ToLower(c)
		if seen[lower] {
			t.Errorf("duplicate concept: %q", c)
		}
		seen[lower] = true
	}
}
