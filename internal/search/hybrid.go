package search

import (
	"context"
	"sort"
	"sync"
)

// HybridSearchConfig configures the weighting for hybrid search fusion.
type HybridSearchConfig struct {
	BM25Weight   float64 // weight for keyword search signal (default 0.4)
	VectorWeight float64 // weight for semantic search signal (default 0.6)
	RRFConstant  float64 // reciprocal rank fusion constant k (default 60)
}

// DefaultHybridConfig returns production-tuned defaults matching agentmemory benchmarks.
func DefaultHybridConfig() HybridSearchConfig {
	return HybridSearchConfig{
		BM25Weight:   0.4,
		VectorWeight: 0.6,
		RRFConstant:  60,
	}
}

// HybridResult contains a document scored by the hybrid search pipeline.
type HybridResult struct {
	DocID         string  `json:"doc_id"`
	CombinedScore float64 `json:"combined_score"`
	BM25Score     float64 `json:"bm25_score"`
	VectorScore   float64 `json:"vector_score"`
	BM25Rank      int     `json:"bm25_rank"`
	VectorRank    int     `json:"vector_rank"`
}

// HybridSearcher combines BM25 keyword search with vector semantic search
// using Reciprocal Rank Fusion (RRF).
type HybridSearcher struct {
	mu     sync.RWMutex
	bm25   *BM25Index
	vector *InMemoryVectorStore
	embed  EmbeddingProvider
	config HybridSearchConfig
}

// NewHybridSearcher creates a hybrid searcher backed by both BM25 and vector indexes.
func NewHybridSearcher(embed EmbeddingProvider, config HybridSearchConfig) *HybridSearcher {
	return &HybridSearcher{
		bm25:   NewBM25Index(),
		vector: NewInMemoryVectorStore(),
		embed:  embed,
		config: config,
	}
}

// IndexDocument adds a document to both the BM25 and vector indexes.
func (h *HybridSearcher) IndexDocument(ctx context.Context, doc *Document) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// BM25 index — no API call needed
	h.bm25.AddDocument(doc.ID, doc.Content)

	// Vector index — generate embedding if not present
	if len(doc.Embedding) == 0 && h.embed != nil {
		embedding, err := h.embed.GenerateEmbedding(ctx, doc.Content)
		if err != nil {
			return err
		}
		doc.Embedding = embedding
	}

	_ = h.vector.AddDocument(ctx, doc)
	return nil
}

// RemoveDocument removes from both indexes.
func (h *HybridSearcher) RemoveDocument(ctx context.Context, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.bm25.RemoveDocument(id)
	_ = h.vector.DeleteDocument(ctx, id)
}

// Search performs hybrid BM25 + vector search with RRF fusion.
func (h *HybridSearcher) Search(ctx context.Context, query string, limit int) ([]HybridResult, error) {
	if limit <= 0 {
		limit = 10
	}

	// Fetch more candidates from each stream than we need for better fusion
	candidateLimit := limit * 3
	if candidateLimit < 20 {
		candidateLimit = 20
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	// BM25 keyword search — free, no API call
	bm25Results := h.bm25.Search(query, candidateLimit)

	// Vector semantic search — requires embedding the query
	var vectorResults []*SearchResult
	if h.embed != nil {
		queryEmbedding, err := h.embed.GenerateEmbedding(ctx, query)
		if err != nil {
			// Degrade gracefully: return BM25-only results
			return h.bm25OnlyResults(bm25Results, limit), nil
		}
		vectorResults, _ = h.vector.Search(ctx, queryEmbedding, candidateLimit, 0.0)
	}

	// Build rank maps
	bm25Ranks := make(map[string]int, len(bm25Results))
	bm25Scores := make(map[string]float64, len(bm25Results))
	for i, r := range bm25Results {
		bm25Ranks[r.DocID] = i + 1
		bm25Scores[r.DocID] = r.Score
	}

	vectorRanks := make(map[string]int, len(vectorResults))
	vectorScores := make(map[string]float64, len(vectorResults))
	for i, r := range vectorResults {
		vectorRanks[r.Document.ID] = i + 1
		vectorScores[r.Document.ID] = float64(r.Score)
	}

	// Collect all unique doc IDs
	allIDs := make(map[string]struct{})
	for _, r := range bm25Results {
		allIDs[r.DocID] = struct{}{}
	}
	for _, r := range vectorResults {
		allIDs[r.Document.ID] = struct{}{}
	}

	// RRF fusion
	k := h.config.RRFConstant
	results := make([]HybridResult, 0, len(allIDs))

	for id := range allIDs {
		var rrfBM25, rrfVector float64

		if rank, ok := bm25Ranks[id]; ok {
			rrfBM25 = 1.0 / (k + float64(rank))
		}
		if rank, ok := vectorRanks[id]; ok {
			rrfVector = 1.0 / (k + float64(rank))
		}

		combined := h.config.BM25Weight*rrfBM25 + h.config.VectorWeight*rrfVector

		br, _ := bm25Ranks[id]
		vr, _ := vectorRanks[id]

		results = append(results, HybridResult{
			DocID:         id,
			CombinedScore: combined,
			BM25Score:     bm25Scores[id],
			VectorScore:   vectorScores[id],
			BM25Rank:      br,
			VectorRank:    vr,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CombinedScore > results[j].CombinedScore
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// BM25Only returns BM25 keyword results without vector search (for offline/no-API scenarios).
func (h *HybridSearcher) BM25Only(query string, limit int) []BM25Result {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.bm25.Search(query, limit)
}

// Stats returns current index statistics.
func (h *HybridSearcher) Stats() (bm25Count, vectorCount int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.bm25.DocCount(), h.vector.documentCount()
}

func (h *HybridSearcher) bm25OnlyResults(bm25Results []BM25Result, limit int) []HybridResult {
	results := make([]HybridResult, 0, len(bm25Results))
	for i, r := range bm25Results {
		if i >= limit {
			break
		}
		results = append(results, HybridResult{
			DocID:         r.DocID,
			CombinedScore: r.Score,
			BM25Score:     r.Score,
			VectorScore:   0,
			BM25Rank:      i + 1,
			VectorRank:    0,
		})
	}
	return results
}
