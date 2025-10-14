package index

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
	
	"github.com/ArqonAi/Pixelog/internal/video"
)

type Indexer struct {
	indexDir     string
	embedder     Embedder
	videoMaker   *video.Maker
}

// Embedder interface for generating vector embeddings
type Embedder interface {
	Embed(text string) ([]float32, error)
	Dim() int
}

func NewIndexer(indexDir string, embedder Embedder) (*Indexer, error) {
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create index directory: %w", err)
	}
	
	maker, err := video.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create video maker: %w", err)
	}
	
	return &Indexer{
		indexDir:   indexDir,
		embedder:   embedder,
		videoMaker: maker,
	}, nil
}

// BuildIndex creates a vector index for a .pixe file
func (idx *Indexer) BuildIndex(memoryID, pixeFile string) (*MemoryIndex, error) {
	fmt.Printf("Building index for %s...\n", memoryID)
	
	// Extract all frames (one-time cost)
	tempDir, err := os.MkdirTemp("", "pixelog-index-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)
	
	err = idx.videoMaker.ExtractData(pixeFile, tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to extract frames: %w", err)
	}
	
	// Read all extracted files and build frame index
	extractedFiles, err := filepath.Glob(filepath.Join(tempDir, "*"))
	if err != nil {
		return nil, err
	}
	
	index := &MemoryIndex{
		MemoryID:    memoryID,
		PixeFile:    pixeFile,
		VectorDim:   idx.embedder.Dim(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Version:     1,
	}
	
	frameNum := 0
	for _, file := range extractedFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		
		text := string(content)
		if len(text) == 0 {
			continue
		}
		
		// Generate embedding
		embedding, err := idx.embedder.Embed(text)
		if err != nil {
			fmt.Printf("Warning: failed to embed frame %d: %v\n", frameNum, err)
			continue
		}
		
		// Create frame index entry
		frameIdx := FrameIndex{
			FrameNumber:  frameNum,
			ChunkIndex:   frameNum, // TODO: Get from chunk metadata
			SourceFile:   filepath.Base(file),
			ContentHash:  fmt.Sprintf("%x", sha256.Sum256(content)),
			ContentLen:   len(content),
			Embedding:    embedding,
			Preview:      truncate(text, 200),
		}
		
		index.Frames = append(index.Frames, frameIdx)
		index.TotalFrames++
		frameNum++
	}
	
	// Save index to disk
	if err := idx.SaveIndex(index); err != nil {
		return nil, fmt.Errorf("failed to save index: %w", err)
	}
	
	fmt.Printf("Index built: %d frames, %d dimensions\n", index.TotalFrames, index.VectorDim)
	return index, nil
}

// Search performs vector similarity search
func (idx *Indexer) Search(index *MemoryIndex, query string, topK int) ([]SearchResult, error) {
	// Embed the query
	queryEmbed, err := idx.embedder.Embed(query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}
	
	// Calculate cosine similarity for all frames
	type scored struct {
		frame FrameIndex
		score float32
	}
	
	scores := make([]scored, 0, len(index.Frames))
	for _, frame := range index.Frames {
		similarity := cosineSimilarity(queryEmbed, frame.Embedding)
		scores = append(scores, scored{frame: frame, score: similarity})
	}
	
	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})
	
	// Return top K results
	if topK > len(scores) {
		topK = len(scores)
	}
	
	results := make([]SearchResult, topK)
	for i := 0; i < topK; i++ {
		results[i] = SearchResult{
			FrameNumber: scores[i].frame.FrameNumber,
			Score:       scores[i].score,
			SourceFile:  scores[i].frame.SourceFile,
			Preview:     scores[i].frame.Preview,
			ChunkIndex:  scores[i].frame.ChunkIndex,
		}
	}
	
	return results, nil
}

// LoadIndex loads an index from disk
func (idx *Indexer) LoadIndex(memoryID string) (*MemoryIndex, error) {
	indexPath := filepath.Join(idx.indexDir, memoryID+".index")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("index not found: %w", err)
	}
	
	var index MemoryIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse index: %w", err)
	}
	
	return &index, nil
}

// SaveIndex saves an index to disk
func (idx *Indexer) SaveIndex(index *MemoryIndex) error {
	indexPath := filepath.Join(idx.indexDir, index.MemoryID+".index")
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}
	
	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}
	
	return nil
}

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	
	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	
	if normA == 0 || normB == 0 {
		return 0
	}
	
	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
