package search

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

type Document struct {
	ID          string                 `json:"id"`
	Content     string                 `json:"content"`
	Embedding   []float32              `json:"embedding"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type SearchResult struct {
	Document   *Document `json:"document"`
	Similarity float32   `json:"similarity"`
	Score      float32   `json:"score"`
}

type VectorStore interface {
	AddDocument(ctx context.Context, doc *Document) error
	Search(ctx context.Context, queryEmbedding []float32, limit int, threshold float32) ([]*SearchResult, error)
	GetDocument(ctx context.Context, id string) (*Document, error)
	DeleteDocument(ctx context.Context, id string) error
	ListDocuments(ctx context.Context, limit, offset int) ([]*Document, error)
}

// InMemoryVectorStore - Simple in-memory vector store for development
type InMemoryVectorStore struct {
	documents map[string]*Document
	mu        sync.RWMutex
}

func NewInMemoryVectorStore() *InMemoryVectorStore {
	return &InMemoryVectorStore{
		documents: make(map[string]*Document),
	}
}

func (vs *InMemoryVectorStore) AddDocument(ctx context.Context, doc *Document) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	
	if doc.ID == "" {
		return fmt.Errorf("document ID cannot be empty")
	}
	
	if len(doc.Embedding) == 0 {
		return fmt.Errorf("document embedding cannot be empty")
	}
	
	doc.UpdatedAt = time.Now()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = doc.UpdatedAt
	}
	
	vs.documents[doc.ID] = doc
	return nil
}

func (vs *InMemoryVectorStore) Search(ctx context.Context, queryEmbedding []float32, limit int, threshold float32) ([]*SearchResult, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("query embedding cannot be empty")
	}
	
	var results []*SearchResult
	
	for _, doc := range vs.documents {
		if len(doc.Embedding) != len(queryEmbedding) {
			continue // Skip documents with different embedding dimensions
		}
		
		similarity := cosineSimilarity(queryEmbedding, doc.Embedding)
		
		if similarity >= threshold {
			results = append(results, &SearchResult{
				Document:   doc,
				Similarity: similarity,
				Score:      similarity,
			})
		}
	}
	
	// Sort by similarity (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	
	// Limit results
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	
	return results, nil
}

func (vs *InMemoryVectorStore) GetDocument(ctx context.Context, id string) (*Document, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	
	doc, exists := vs.documents[id]
	if !exists {
		return nil, fmt.Errorf("document not found: %s", id)
	}
	
	return doc, nil
}

func (vs *InMemoryVectorStore) DeleteDocument(ctx context.Context, id string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	
	if _, exists := vs.documents[id]; !exists {
		return fmt.Errorf("document not found: %s", id)
	}
	
	delete(vs.documents, id)
	return nil
}

func (vs *InMemoryVectorStore) ListDocuments(ctx context.Context, limit, offset int) ([]*Document, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	
	var docs []*Document
	i := 0
	
	for _, doc := range vs.documents {
		if i < offset {
			i++
			continue
		}
		
		if limit > 0 && len(docs) >= limit {
			break
		}
		
		docs = append(docs, doc)
		i++
	}
	
	// Sort by creation time (newest first)
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].CreatedAt.After(docs[j].CreatedAt)
	})
	
	return docs, nil
}

// Utility functions
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	
	var dotProduct, normA, normB float64
	
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	
	if normA == 0 || normB == 0 {
		return 0
	}
	
	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

func normalizeVector(vec []float32) []float32 {
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	
	norm = math.Sqrt(norm)
	if norm == 0 {
		return vec
	}
	
	normalized := make([]float32, len(vec))
	for i, v := range vec {
		normalized[i] = float32(float64(v) / norm)
	}
	
	return normalized
}
