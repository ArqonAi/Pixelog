package index

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ArqonAi/Pixelog/internal/video"
)

type Indexer struct {
	indexDir   string
	embedder   Embedder
	videoMaker *video.Maker
}

// Embedder interface for generating vector embeddings
type Embedder interface {
	Embed(text string) ([]float32, error)
	Dim() int
	// MaxInputChars is the largest input the endpoint accepts.
	// Callers split to this size; an embedder that cannot determine a
	// limit returns its safe ceiling rather than zero.
	MaxInputChars() int
}

// NewIndexer creates an indexer
// embedder can be nil for loading/searching existing indexes
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
	if idx.embedder == nil {
		return nil, fmt.Errorf("embedder required for building index - provide API key")
	}

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
		MemoryID:  memoryID,
		PixeFile:  pixeFile,
		VectorDim: idx.embedder.Dim(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}

	frameNum := 0
	attempted := 0
	var firstEmbedErr error
	for _, file := range extractedFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		text := string(content)
		if len(text) == 0 {
			continue
		}

		// Split to what the endpoint accepts rather than handing it
		// the whole frame. Embedders have small contexts — a 512-token
		// sentence encoder is typical — and an oversized input is
		// rejected outright, so a single long frame used to take the
		// entire index down with it. Chunking keeps every byte
		// searchable instead of truncating the tail away.
		chunks := splitForEmbedding(text, idx.embedder.MaxInputChars())
		// The character budget is an estimate; dense text (JSON, code)
		// runs more tokens per character. A chunk the endpoint rejects
		// as too long is halved and retried until it fits.
		chunks, embeddings, err := embedFitting(idx.embedder, chunks)
		if err != nil {
			if firstEmbedErr == nil {
				firstEmbedErr = err
			}
			fmt.Printf("Warning: failed to embed part of frame %d: %v\n", frameNum, err)
		}
		for chunkIdx, chunk := range chunks {
			attempted++
			embedding := embeddings[chunkIdx]
			if embedding == nil {
				continue
			}

			frameIdx := FrameIndex{
				FrameNumber: frameNum,
				ChunkIndex:  chunkIdx,
				SourceFile:  filepath.Base(file),
				ContentHash: fmt.Sprintf("%x", sha256.Sum256([]byte(chunk))),
				ContentLen:  len(chunk),
				Embedding:   embedding,
				Preview:     truncate(chunk, 200),
			}

			index.Frames = append(index.Frames, frameIdx)
			index.TotalFrames++
		}
		frameNum++
	}

	// Every frame failed to embed — the endpoint is down, the key is
	// dead, or the model cannot embed. A silent 0-frame index would make
	// search return nothing forever; surface the failure so callers can
	// fall back to another provider.
	if attempted > 0 && index.TotalFrames == 0 {
		return nil, fmt.Errorf("all %d frames failed to embed: %w", attempted, firstEmbedErr)
	}

	// The embedder discovers its true dimension on first successful call
	// (local servers vary); record it after embedding, not before.
	index.VectorDim = idx.embedder.Dim()

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

// embedder is what embedFitting needs from SimpleEmbedder.
type chunkEmbedder interface {
	Embed(text string) ([]float32, error)
}

// minEmbedChunk is the smallest piece embedFitting will split down to.
const minEmbedChunk = 64

// embedFitting embeds each chunk, halving any the endpoint rejects for
// length (at a rune boundary) and retrying, so every byte stays
// searchable. It returns the final chunks with their embeddings (nil for
// a chunk that failed for another reason) and the first such error.
func embedFitting(e chunkEmbedder, chunks []string) ([]string, [][]float32, error) {
	var outChunks []string
	var outVecs [][]float32
	var firstErr error
	queue := append([]string(nil), chunks...)
	for len(queue) > 0 {
		chunk := queue[0]
		queue = queue[1:]
		vec, err := e.Embed(chunk)
		if err != nil && isInputTooLong(err) && len(chunk) > minEmbedChunk {
			mid := len(chunk) / 2
			for mid > 0 && !utf8.RuneStart(chunk[mid]) {
				mid--
			}
			queue = append([]string{chunk[:mid], chunk[mid:]}, queue...)
			continue
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
		outChunks = append(outChunks, chunk)
		outVecs = append(outVecs, vec)
	}
	return outChunks, outVecs, firstErr
}

// isInputTooLong recognises an endpoint refusing input over its context
// (vLLM, OpenAI, TEI, llama.cpp wording).
func isInputTooLong(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, s := range []string{"maximum context length", "too long", "too many tokens", "exceeds the maximum", "input length", "context window"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}
