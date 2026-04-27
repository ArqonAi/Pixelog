package index

import "time"

// MemoryIndex stores vector embeddings and metadata for fast retrieval
type MemoryIndex struct {
	MemoryID     string                  `json:"memory_id"`
	PixeFile     string                  `json:"pixe_file"`
	TotalFrames  int                     `json:"total_frames"`
	VectorDim    int                     `json:"vector_dim"`     // 384 for minilm, 1536 for OpenAI
	Frames       []FrameIndex            `json:"frames"`
	CreatedAt    time.Time               `json:"created_at"`
	UpdatedAt    time.Time               `json:"updated_at"`
	Version      int                     `json:"version"`        // For delta encoding
}

// FrameIndex contains metadata and embedding for a single frame
type FrameIndex struct {
	FrameNumber  int       `json:"frame_number"`  // Frame index in video (for seeking)
	ChunkIndex   int       `json:"chunk_index"`   // Chunk index in file
	Hash         string    `json:"hash"`          // File hash (for grouping)
	SourceFile   string    `json:"source_file"`   // Original filename
	ContentHash  string    `json:"content_hash"`  // Hash of decoded content
	ContentLen   int       `json:"content_len"`   // Content length in bytes
	Embedding    []float32 `json:"embedding"`     // Vector embedding
	Preview      string    `json:"preview"`       // First 200 chars for debugging
}

// SearchResult represents a frame matched by vector search
type SearchResult struct {
	FrameNumber  int       `json:"frame_number"`
	Score        float32   `json:"score"`         // Cosine similarity
	SourceFile   string    `json:"source_file"`
	Preview      string    `json:"preview"`
	ChunkIndex   int       `json:"chunk_index"`
}

// DeltaVersion represents an incremental change to a memory
type DeltaVersion struct {
	Version       int                 `json:"version"`        // v2, v3, v4...
	ParentVersion int                 `json:"parent_version"` // v1, v2, v3...
	Timestamp     time.Time           `json:"timestamp"`
	DeltaFile     string              `json:"delta_file"`     // Path to delta .pixe
	Operations    []DeltaOp           `json:"operations"`     // What changed
	Message       string              `json:"message"`        // Commit message
	Author        string              `json:"author"`         // Who made the change
	FrameCount    int                 `json:"frame_count"`    // Frames in this delta
}

// DeltaOp represents a single operation in a version delta
type DeltaOp struct {
	Type          string    `json:"type"`           // "insert", "delete", "replace"
	FrameIndex    int       `json:"frame_index"`    // Which frame affected
	OldHash       string    `json:"old_hash"`       // Previous content hash
	NewHash       string    `json:"new_hash"`       // New content hash
	ChunkData     string    `json:"chunk_data"`     // The actual delta data
}

// VersionedMemory tracks all versions of a memory with delta encoding
type VersionedMemory struct {
	MemoryID      string                  `json:"memory_id"`
	BaseFile      string                  `json:"base_file"`      // Initial .pixe (v1)
	CurrentHead   int                     `json:"current_head"`   // Latest version number
	Versions      []DeltaVersion          `json:"versions"`       // All versions
	Branches      map[string]int          `json:"branches"`       // Branch name → version number
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
}
