package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ArqonAi/Pixelog/internal/search"
)

// Embedder is the minimal embedding interface required by CategoryStore.
// It is satisfied by search.EmbeddingProvider, but kept narrow for testability.
type Embedder interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

// CategoryStore manages per-category vector subspaces. Each MemoryCategory
// gets its own search.VectorStore so retrieval can be scoped or weighted
// independently per category.
type CategoryStore struct {
	stores   map[MemoryCategory]search.VectorStore
	embedder Embedder
	mu       sync.RWMutex
}

// CategoryStoreFactory returns a fresh per-category VectorStore.
// Callers can supply a persistent backend; default is in-memory.
type CategoryStoreFactory func(category MemoryCategory) search.VectorStore

// DefaultCategoryStoreFactory returns an in-memory VectorStore per category.
func DefaultCategoryStoreFactory(_ MemoryCategory) search.VectorStore {
	return search.NewInMemoryVectorStore()
}

// NewCategoryStore creates a category-partitioned vector store.
// If factory is nil, DefaultCategoryStoreFactory is used.
func NewCategoryStore(embedder Embedder, factory CategoryStoreFactory) *CategoryStore {
	if factory == nil {
		factory = DefaultCategoryStoreFactory
	}
	cs := &CategoryStore{
		stores:   make(map[MemoryCategory]search.VectorStore),
		embedder: embedder,
	}
	for _, cat := range AllCategories() {
		cs.stores[cat] = factory(cat)
	}
	return cs
}

// Store adds a typed memory to the appropriate category subspace.
func (cs *CategoryStore) Store(ctx context.Context, mem TypedMemory) error {
	if cs.embedder == nil {
		return fmt.Errorf("category store: no embedder configured")
	}

	cs.mu.RLock()
	store, ok := cs.stores[mem.Category]
	cs.mu.RUnlock()
	if !ok {
		return fmt.Errorf("category store: unknown category %q", mem.Category)
	}

	content := fmt.Sprintf("%s: %s", mem.Key, mem.Value)
	embedding, err := cs.embedder.GenerateEmbedding(ctx, content)
	if err != nil {
		return fmt.Errorf("category store: embed: %w", err)
	}

	ts := mem.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	doc := &search.Document{
		ID:        mem.ID,
		Content:   content,
		Embedding: embedding,
		Metadata: map[string]interface{}{
			"category":   string(mem.Category),
			"key":        mem.Key,
			"value":      mem.Value,
			"source":     mem.Source,
			"confidence": mem.Confidence,
		},
		CreatedAt: ts,
		UpdatedAt: ts,
	}

	return store.AddDocument(ctx, doc)
}

// Search queries one or more categories with weighted scoring.
// If categories is empty, all categories are searched.
func (cs *CategoryStore) Search(ctx context.Context, query string, categories []MemoryCategory, limit int) ([]TypedSearchResult, error) {
	if cs.embedder == nil {
		return nil, fmt.Errorf("category store: no embedder configured")
	}

	embedding, err := cs.embedder.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("category store: embed query: %w", err)
	}

	if limit <= 0 {
		limit = 5
	}
	if len(categories) == 0 {
		categories = AllCategories()
	}

	var allResults []TypedSearchResult
	for _, cat := range categories {
		cs.mu.RLock()
		store, ok := cs.stores[cat]
		cs.mu.RUnlock()
		if !ok {
			continue
		}

		results, err := store.Search(ctx, embedding, limit, 0.0)
		if err != nil {
			continue
		}

		weight := cat.Weight()
		for _, r := range results {
			allResults = append(allResults, TypedSearchResult{
				Document: r.Document,
				Score:    float64(r.Score) * weight,
				Category: cat,
			})
		}
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	if len(allResults) > limit {
		allResults = allResults[:limit]
	}
	return allResults, nil
}

// SearchCategory queries a single category.
func (cs *CategoryStore) SearchCategory(ctx context.Context, query string, category MemoryCategory, limit int) ([]TypedSearchResult, error) {
	return cs.Search(ctx, query, []MemoryCategory{category}, limit)
}

// Delete removes a typed memory from its category subspace.
func (cs *CategoryStore) Delete(ctx context.Context, category MemoryCategory, id string) error {
	cs.mu.RLock()
	store, ok := cs.stores[category]
	cs.mu.RUnlock()
	if !ok {
		return fmt.Errorf("category store: unknown category %q", category)
	}
	return store.DeleteDocument(ctx, id)
}

// Stats returns per-category counts plus a total.
func (cs *CategoryStore) Stats(ctx context.Context) map[string]int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	stats := make(map[string]int)
	total := 0
	for cat, store := range cs.stores {
		docs, err := store.ListDocuments(ctx, 0, 0)
		if err != nil {
			continue
		}
		stats[string(cat)] = len(docs)
		total += len(docs)
	}
	stats["total"] = total
	return stats
}

// TypedSearchResult is a category-tagged search hit.
type TypedSearchResult struct {
	Document *search.Document `json:"document"`
	Score    float64          `json:"score"`
	Category MemoryCategory   `json:"category"`
}
