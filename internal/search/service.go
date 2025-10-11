package search

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"

	"github.com/ArqonAi/Pixelog/pkg/config"
)

type SearchService struct {
	vectorStore VectorStore
	embeddings  EmbeddingProvider
	extractor   *MultiExtractor
	config      *config.Config
}

type IndexRequest struct {
	ID       string            `json:"id"`
	Content  string            `json:"content,omitempty"`
	Reader   io.Reader         `json:"-"`
	Filename string            `json:"filename"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type SearchRequest struct {
	Query     string  `json:"query"`
	Limit     int     `json:"limit,omitempty"`
	Threshold float32 `json:"threshold,omitempty"`
	Filters   map[string]interface{} `json:"filters,omitempty"`
}

func NewSearchService(cfg *config.Config) (*SearchService, error) {
	// Initialize embedding provider
	embeddings, err := NewEmbeddingProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding provider: %w", err)
	}

	// Initialize vector store (in-memory for now)
	vectorStore := NewInMemoryVectorStore()

	// Initialize text extractor
	extractor := NewMultiExtractor()

	return &SearchService{
		vectorStore: vectorStore,
		embeddings:  embeddings,
		extractor:   extractor,
		config:      cfg,
	}, nil
}

func (s *SearchService) IndexFile(ctx context.Context, req *IndexRequest) error {
	var content string
	var err error

	if req.Content != "" {
		content = req.Content
	} else if req.Reader != nil {
		content, err = s.extractor.Extract(req.Reader, req.Filename)
		if err != nil {
			return fmt.Errorf("failed to extract text from file: %w", err)
		}
	} else {
		return fmt.Errorf("either Content or Reader must be provided")
	}

	// Clean and prepare content
	content = s.preprocessText(content)
	if len(content) == 0 {
		return fmt.Errorf("no extractable text content found")
	}

	// Generate embedding
	embedding, err := s.embeddings.GenerateEmbedding(ctx, content)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Create document
	doc := &Document{
		ID:        req.ID,
		Content:   content,
		Embedding: embedding,
		Metadata:  req.Metadata,
	}

	// Ensure metadata exists
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]interface{})
	}

	// Add file metadata
	doc.Metadata["filename"] = req.Filename
	doc.Metadata["content_length"] = len(content)
	doc.Metadata["embedding_model"] = "text-embedding-3-small" // TODO: Make dynamic

	// Store document
	if err := s.vectorStore.AddDocument(ctx, doc); err != nil {
		return fmt.Errorf("failed to store document: %w", err)
	}

	return nil
}

func (s *SearchService) Search(ctx context.Context, req *SearchRequest) ([]*SearchResult, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Threshold <= 0 {
		req.Threshold = 0.7 // 70% similarity threshold
	}

	// Generate query embedding
	queryEmbedding, err := s.embeddings.GenerateEmbedding(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search vector store
	results, err := s.vectorStore.Search(ctx, queryEmbedding, req.Limit, req.Threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to search vector store: %w", err)
	}

	// TODO: Apply additional filters if provided
	results = s.applyFilters(results, req.Filters)

	return results, nil
}

func (s *SearchService) GetSimilarDocuments(ctx context.Context, documentID string, limit int) ([]*SearchResult, error) {
	// Get the source document
	doc, err := s.vectorStore.GetDocument(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source document: %w", err)
	}

	// Search using the document's embedding
	results, err := s.vectorStore.Search(ctx, doc.Embedding, limit+1, 0.5) // +1 to exclude self
	if err != nil {
		return nil, fmt.Errorf("failed to find similar documents: %w", err)
	}

	// Remove the source document from results
	filtered := make([]*SearchResult, 0, len(results))
	for _, result := range results {
		if result.Document.ID != documentID {
			filtered = append(filtered, result)
		}
	}

	// Limit results
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

func (s *SearchService) ListDocuments(ctx context.Context, limit, offset int) ([]*Document, error) {
	return s.vectorStore.ListDocuments(ctx, limit, offset)
}

func (s *SearchService) DeleteDocument(ctx context.Context, id string) error {
	return s.vectorStore.DeleteDocument(ctx, id)
}

func (s *SearchService) IsFileSupported(filename string) bool {
	return s.extractor.IsSupported(filename)
}

// Helper functions
func (s *SearchService) preprocessText(text string) string {
	// Basic text cleaning
	text = strings.TrimSpace(text)
	
	// Remove excessive whitespace
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 0 {
			cleanLines = append(cleanLines, line)
		}
	}
	
	return strings.Join(cleanLines, "\n")
}

func (s *SearchService) applyFilters(results []*SearchResult, filters map[string]interface{}) []*SearchResult {
	if len(filters) == 0 {
		return results
	}

	var filtered []*SearchResult
	for _, result := range results {
		if s.matchesFilters(result.Document, filters) {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

func (s *SearchService) matchesFilters(doc *Document, filters map[string]interface{}) bool {
	// TODO: Implement sophisticated filtering logic
	// For now, simple string matching on metadata
	for key, value := range filters {
		if metaValue, exists := doc.Metadata[key]; exists {
			if fmt.Sprintf("%v", metaValue) != fmt.Sprintf("%v", value) {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

func generateDocumentID(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	return fmt.Sprintf("%x", hasher.Sum(nil))[:16]
}
