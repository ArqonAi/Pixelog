package search

import (
	"fmt"
	"testing"
)

func TestBM25Index_AddAndSearch(t *testing.T) {
	idx := NewBM25Index()

	docs := []struct {
		id      string
		content string
	}{
		{"doc1", "the quick brown fox jumps over the lazy dog"},
		{"doc2", "a fast brown fox leaps across the sleeping dog"},
		{"doc3", "machine learning and artificial intelligence in production systems"},
		{"doc4", "quantum computing breakthrough in error correction"},
		{"doc5", "the brown dog sleeps by the fence"},
	}

	for _, d := range docs {
		idx.AddDocument(d.id, d.content)
	}

	if idx.DocCount() != 5 {
		t.Fatalf("expected 5 docs, got %d", idx.DocCount())
	}

	tests := []struct {
		name      string
		query     string
		limit     int
		wantFirst string
		wantMin   int
	}{
		{
			name:    "fox query returns fox docs",
			query:   "brown fox",
			limit:   3,
			wantMin: 2,
		},
		{
			name:      "machine learning returns ml doc",
			query:     "machine learning artificial intelligence",
			limit:     3,
			wantFirst: "doc3",
			wantMin:   1,
		},
		{
			name:    "quantum returns quantum doc",
			query:   "quantum computing",
			limit:   1,
			wantFirst: "doc4",
			wantMin: 1,
		},
		{
			name:    "no results for unrelated query",
			query:   "basketball championships",
			limit:   5,
			wantMin: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := idx.Search(tt.query, tt.limit)

			if len(results) < tt.wantMin {
				t.Errorf("expected at least %d results, got %d", tt.wantMin, len(results))
			}

			if tt.wantFirst != "" && len(results) > 0 && results[0].DocID != tt.wantFirst {
				t.Errorf("expected first result %q, got %q", tt.wantFirst, results[0].DocID)
			}

			// Verify scores are descending
			for i := 1; i < len(results); i++ {
				if results[i].Score > results[i-1].Score {
					t.Errorf("results not sorted: score[%d]=%.4f > score[%d]=%.4f",
						i, results[i].Score, i-1, results[i-1].Score)
				}
			}
		})
	}
}

func TestBM25Index_RemoveDocument(t *testing.T) {
	idx := NewBM25Index()
	idx.AddDocument("a", "hello world test")
	idx.AddDocument("b", "goodbye world test")

	if idx.DocCount() != 2 {
		t.Fatalf("expected 2 docs, got %d", idx.DocCount())
	}

	idx.RemoveDocument("a")

	if idx.DocCount() != 1 {
		t.Fatalf("expected 1 doc after removal, got %d", idx.DocCount())
	}

	results := idx.Search("hello", 5)
	if len(results) != 0 {
		t.Errorf("expected no results for removed doc, got %d", len(results))
	}

	results = idx.Search("goodbye", 5)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestBM25Index_EmptyQueries(t *testing.T) {
	idx := NewBM25Index()

	results := idx.Search("anything", 5)
	if results != nil {
		t.Errorf("expected nil results from empty index, got %v", results)
	}

	idx.AddDocument("d1", "some content here")

	results = idx.Search("", 5)
	if results != nil {
		t.Errorf("expected nil results for empty query, got %v", results)
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		wantLen  int
		contains string
	}{
		{"Hello World", 2, "hello"},
		{"the and or but", 0, ""}, // all stop words
		{"machine-learning API", 3, "machine"},
		{"a b c", 0, ""},  // single-char tokens filtered
		{"", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := tokenize(tt.input)
			if len(tokens) != tt.wantLen {
				t.Errorf("tokenize(%q) = %d tokens, want %d (tokens: %v)", tt.input, len(tokens), tt.wantLen, tokens)
			}
			if tt.contains != "" {
				found := false
				for _, tok := range tokens {
					if tok == tt.contains {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected token %q in %v", tt.contains, tokens)
				}
			}
		})
	}
}

func TestBM25Index_Concurrent(t *testing.T) {
	idx := NewBM25Index()

	// Concurrent writes
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			idx.AddDocument(fmt.Sprintf("doc%d", n), fmt.Sprintf("content number %d with words", n))
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}

	if idx.DocCount() != 50 {
		t.Errorf("expected 50 docs after concurrent adds, got %d", idx.DocCount())
	}

	// Concurrent reads
	for i := 0; i < 20; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			idx.Search("content words", 5)
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}
