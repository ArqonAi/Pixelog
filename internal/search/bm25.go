package search

import (
	"math"
	"strings"
	"sync"
	"unicode"
)

// BM25Index implements Okapi BM25 ranking for keyword-based retrieval.
// No external dependencies — pure Go implementation.
type BM25Index struct {
	mu sync.RWMutex

	// Tuning parameters
	k1 float64 // term-frequency saturation (default 1.2)
	b  float64 // length normalization (default 0.75)

	// Corpus statistics
	docs     []bm25Doc          // ordered document store
	df       map[string]int     // document frequency per term
	avgDL    float64            // average document length in tokens
	totalDocs int
}

type bm25Doc struct {
	ID     string
	Tokens []string
	TF     map[string]int // term → count within this doc
	Length int
}

// BM25Result is a single scored document from keyword search.
type BM25Result struct {
	DocID string
	Score float64
}

// NewBM25Index creates a BM25 index with standard parameters.
func NewBM25Index() *BM25Index {
	return &BM25Index{
		k1: 1.2,
		b:  0.75,
		df: make(map[string]int),
	}
}

// AddDocument indexes a document's text content.
func (idx *BM25Index) AddDocument(id, content string) {
	tokens := tokenize(content)
	tf := make(map[string]int, len(tokens))
	for _, t := range tokens {
		tf[t]++
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	doc := bm25Doc{
		ID:     id,
		Tokens: tokens,
		TF:     tf,
		Length: len(tokens),
	}
	idx.docs = append(idx.docs, doc)

	// Update document frequency for each unique term
	seen := make(map[string]struct{}, len(tf))
	for term := range tf {
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		idx.df[term]++
	}

	idx.totalDocs = len(idx.docs)
	idx.recomputeAvgDL()
}

// RemoveDocument removes a document by ID.
func (idx *BM25Index) RemoveDocument(id string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for i, doc := range idx.docs {
		if doc.ID == id {
			// Decrement DF for each unique term
			for term := range doc.TF {
				if idx.df[term] > 0 {
					idx.df[term]--
				}
				if idx.df[term] == 0 {
					delete(idx.df, term)
				}
			}
			idx.docs = append(idx.docs[:i], idx.docs[i+1:]...)
			idx.totalDocs = len(idx.docs)
			idx.recomputeAvgDL()
			return
		}
	}
}

// Search returns scored results for a query string, ordered by BM25 score descending.
func (idx *BM25Index) Search(query string, limit int) []BM25Result {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.totalDocs == 0 {
		return nil
	}

	type scored struct {
		id    string
		score float64
	}

	results := make([]scored, 0, idx.totalDocs)
	for _, doc := range idx.docs {
		score := 0.0
		for _, term := range queryTokens {
			score += idx.termScore(term, doc)
		}
		if score > 0 {
			results = append(results, scored{id: doc.ID, score: score})
		}
	}

	// Sort descending by score (insertion sort — fine for typical result sizes)
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].score > results[j-1].score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	out := make([]BM25Result, len(results))
	for i, r := range results {
		out[i] = BM25Result{DocID: r.id, Score: r.score}
	}
	return out
}

// DocCount returns the number of indexed documents.
func (idx *BM25Index) DocCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.totalDocs
}

// termScore computes the BM25 score contribution for a single term in a single document.
func (idx *BM25Index) termScore(term string, doc bm25Doc) float64 {
	tf, exists := doc.TF[term]
	if !exists {
		return 0
	}

	dfVal := idx.df[term]
	if dfVal == 0 {
		return 0
	}

	// IDF with smoothing to avoid negatives
	n := float64(idx.totalDocs)
	idf := math.Log(1 + (n-float64(dfVal)+0.5)/(float64(dfVal)+0.5))

	// BM25 TF normalization
	tfNorm := (float64(tf) * (idx.k1 + 1)) /
		(float64(tf) + idx.k1*(1-idx.b+idx.b*(float64(doc.Length)/idx.avgDL)))

	return idf * tfNorm
}

func (idx *BM25Index) recomputeAvgDL() {
	if idx.totalDocs == 0 {
		idx.avgDL = 0
		return
	}
	total := 0
	for _, doc := range idx.docs {
		total += doc.Length
	}
	idx.avgDL = float64(total) / float64(idx.totalDocs)
}

// tokenize splits text into lowercase alphanumeric tokens.
func tokenize(text string) []string {
	lower := strings.ToLower(text)
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	// Filter stop words for better signal
	filtered := make([]string, 0, len(words))
	for _, w := range words {
		if len(w) < 2 {
			continue
		}
		if _, stop := stopWords[w]; stop {
			continue
		}
		filtered = append(filtered, w)
	}
	return filtered
}

var stopWords = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "but": {},
	"in": {}, "on": {}, "at": {}, "to": {}, "for": {}, "of": {},
	"with": {}, "by": {}, "from": {}, "is": {}, "it": {}, "as": {},
	"be": {}, "was": {}, "are": {}, "were": {}, "been": {}, "being": {},
	"have": {}, "has": {}, "had": {}, "do": {}, "does": {}, "did": {},
	"will": {}, "would": {}, "could": {}, "should": {}, "may": {},
	"might": {}, "shall": {}, "can": {}, "this": {}, "that": {},
	"these": {}, "those": {}, "not": {}, "no": {}, "if": {}, "then": {},
	"than": {}, "so": {}, "up": {}, "out": {}, "just": {}, "about": {},
}
