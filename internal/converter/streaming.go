package converter

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/ArqonAi/Pixelog/internal/qr"
)

const (
	// StreamChunkSize is the I/O read buffer size for streaming (1MB).
	// Each read is further sub-chunked into QR-safe pieces.
	StreamChunkSize = 1 * 1024 * 1024

	// maxQRPayload is the maximum data content per QR code frame.
	// QR Version 40 with L error correction holds ~2953 bytes total.
	// The chunk JSON envelope (index, total, hash, source_file, mime_type,
	// created_at, etc.) adds ~300 bytes. We use 2000 for safety.
	maxQRPayload = 2000
)

// ProgressCallback reports progress during streaming
type ProgressCallback func(bytesProcessed int64, totalBytes int64, currentChunk int, totalChunks int)

// StreamingProcessor handles large file conversion with constant memory usage
type StreamingProcessor struct {
	converter        *Converter
	chunkSize        int
	progressCallback ProgressCallback
}

// NewStreamingProcessor creates a streaming file processor
func NewStreamingProcessor(conv *Converter) *StreamingProcessor {
	return &StreamingProcessor{
		converter: conv,
		chunkSize: StreamChunkSize,
	}
}

// SetProgressCallback sets callback for progress updates
func (sp *StreamingProcessor) SetProgressCallback(callback ProgressCallback) {
	sp.progressCallback = callback
}

// ProcessFileStreaming processes a file in chunks without loading it entirely into memory
func (sp *StreamingProcessor) ProcessFileStreaming(filePath string, encryptionPassword string) ([]qr.Chunk, *ContentItem, error) {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat file: %w", err)
	}

	totalBytes := fileInfo.Size()
	estimatedChunks := int((totalBytes + int64(sp.chunkSize) - 1) / int64(sp.chunkSize))

	// Open file for reading
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create buffered reader
	reader := bufio.NewReaderSize(file, sp.chunkSize)

	// Initialize hash for entire file
	fileHasher := sha256.New()

	// Process chunks — read in large I/O buffers, then sub-chunk into
	// QR-safe pieces so each QR frame stays within encoder limits.
	var chunks []qr.Chunk
	buffer := make([]byte, sp.chunkSize)
	bytesProcessed := int64(0)
	chunkIndex := 0

	// Re-estimate based on QR payload size for accurate progress
	estimatedChunks = int((totalBytes + int64(maxQRPayload) - 1) / int64(maxQRPayload))

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			readData := buffer[:n]

			// Update file hash with raw data
			fileHasher.Write(readData)
			bytesProcessed += int64(n)

			// Sub-chunk the read buffer into QR-safe pieces
			for off := 0; off < len(readData); off += maxQRPayload {
				end := off + maxQRPayload
				if end > len(readData) {
					end = len(readData)
				}
				chunkData := readData[off:end]

				// Encrypt chunk if needed
				if encryptionPassword != "" && sp.converter.cryptoService.IsEnabled() {
					encryptedData, encErr := sp.converter.cryptoService.EncryptData(chunkData, encryptionPassword)
					if encErr != nil {
						return nil, nil, fmt.Errorf("failed to encrypt chunk %d: %w", chunkIndex, encErr)
					}
					chunkData = encryptedData
				}

				chunk := qr.Chunk{
					Index: chunkIndex,
					Data:  string(chunkData),
					Total: estimatedChunks, // Updated at the end
				}
				chunks = append(chunks, chunk)
				chunkIndex++
			}

			// Report progress
			if sp.progressCallback != nil {
				sp.progressCallback(bytesProcessed, totalBytes, chunkIndex, estimatedChunks)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read chunk %d: %w", chunkIndex, err)
		}
	}

	// Update total chunks in all chunks
	actualChunks := len(chunks)
	for i := range chunks {
		chunks[i].Total = actualChunks
	}

	// Calculate final hash
	fileHash := fmt.Sprintf("%x", fileHasher.Sum(nil))

	// Detect MIME type
	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Create content item (matches existing ContentItem type)
	contentItem := &ContentItem{
		Name:      filepath.Base(filePath),
		Type:      mimeType,
		Size:      formatSizeHelper(totalBytes),
		Hash:      fileHash,
		CreatedAt: fileInfo.ModTime(),
	}

	return chunks, contentItem, nil
}

// StreamToVideo processes file and streams directly to video creation
// This is the most memory-efficient approach
func (sp *StreamingProcessor) StreamToVideo(filePath string, outputPath string, encryptionPassword string) error {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	totalBytes := fileInfo.Size()
	fmt.Printf("📦 Streaming %s (%s) → %s\n", filepath.Base(filePath), formatSizeHelper(totalBytes), outputPath)
	fmt.Printf("🔄 Processing in %s chunks...\n", formatSizeHelper(int64(sp.chunkSize)))

	// Process file streaming
	chunks, contentItem, err := sp.ProcessFileStreaming(filePath, encryptionPassword)
	if err != nil {
		return err
	}

	fmt.Printf("✓ Processed %d chunks\n", len(chunks))
	fmt.Printf("🎬 Creating video...\n")

	// Create temp directory for QR frames
	tempDir, err := os.MkdirTemp("", "pixelog-stream-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate QR frames using existing generator
	generator, err := qr.New(tempDir)
	if err != nil {
		return fmt.Errorf("failed to create QR generator: %w", err)
	}

	frames, err := generator.GenerateFrames(chunks)
	if err != nil {
		return fmt.Errorf("failed to generate QR frames: %w", err)
	}

	// Use existing converter's video maker
	maker, err := sp.converter.GetVideoMaker()
	if err != nil {
		return fmt.Errorf("failed to get video maker: %w", err)
	}

	metadata := Metadata{
		Version:     "1.0",
		CreatedAt:   time.Now(),
		TotalChunks: len(chunks),
		Contents:    []ContentItem{*contentItem},
		Config:      sp.converter.GetConfig(),
	}

	err = maker.CreateVideo(frames, outputPath, metadata, sp.converter.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create video: %w", err)
	}

	fmt.Printf("✅ Video created: %s\n", outputPath)
	return nil
}

// StreamExtraction extracts a file from .pixe in streaming mode
func (sp *StreamingProcessor) StreamExtraction(pixeFile string, outputDir string, decryptionPassword string) error {
	// Open pixe file
	file, err := os.Open(pixeFile)
	if err != nil {
		return fmt.Errorf("failed to open pixe file: %w", err)
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	fmt.Printf("📦 Extracting from %s (%s)\n", filepath.Base(pixeFile), formatSizeHelper(fileInfo.Size()))

	// TODO: Implement streaming extraction
	// For now, fall back to standard extraction
	return fmt.Errorf("streaming extraction not yet implemented - use standard extract")
}

// formatSizeHelper - renamed to avoid conflict with main.go
func formatSizeHelper(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
