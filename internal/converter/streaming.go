package converter

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ArqonAi/Pixelog/internal/qr"
)

const (
	// StreamChunkSize is the I/O read buffer size for streaming (1MB).
	// Each read is further sub-chunked into QR-safe pieces.
	StreamChunkSize = 1 * 1024 * 1024

	// streamRawPayload is how many raw input bytes go into one frame.
	//
	// It MUST stay a multiple of 3. Binary payloads are base64-encoded per
	// frame, and extraction concatenates the frames' data before decoding
	// once — concatenated base64 only decodes correctly when every frame
	// except the last encodes a whole number of 3-byte groups. Any other
	// size puts '=' padding mid-stream and the joined string is invalid.
	//
	// 1920 raw bytes → 2560 base64 chars, leaving room for the ~250-byte
	// JSON envelope under qrFramePayloadLimit.
	streamRawPayload = 1920
)

// ProgressCallback reports progress during streaming
type ProgressCallback func(bytesProcessed int64, totalBytes int64, currentChunk int, totalChunks int)

// splitAtRuneBoundary returns b truncated to the last complete UTF-8 rune,
// plus the incomplete trailing bytes to carry into the next frame. A complete
// b is returned unchanged with no remainder.
func splitAtRuneBoundary(b []byte) (head, tail []byte) {
	if len(b) == 0 || utf8.Valid(b) {
		return b, nil
	}
	// A rune is at most utf8.UTFMax bytes, so a complete boundary lies
	// within that distance of the end unless the input is malformed.
	for i := 1; i < utf8.UTFMax && i <= len(b); i++ {
		cut := len(b) - i
		if utf8.Valid(b[:cut]) {
			return b[:cut], b[cut:]
		}
	}
	// Malformed beyond a split point: pass it through unchanged rather than
	// silently dropping bytes.
	return b, nil
}

// framePayloads encodes raw into one or more frame payload strings.
//
// Text is stored verbatim and binary as base64, matching what the extractor
// expects for each MIME class. Normally this yields exactly one payload; a
// frame is split only when JSON escaping inflates it past the QR capacity,
// which dense punctuation or control characters in text can do. Splitting is
// safe for text because the extractor simply concatenates the strings.
//
// Binary is never split here: streamRawPayload is a multiple of 3, so each
// base64 payload is padding-free and the concatenation stays decodable.
func framePayloads(raw []byte, isText bool) []string {
	if !isText {
		return []string{base64.StdEncoding.EncodeToString(raw)}
	}

	var out []string
	pending := [][]byte{raw}
	for len(pending) > 0 {
		piece := pending[0]
		pending = pending[1:]

		if len(piece) == 0 {
			continue
		}
		if fitsFrame(piece) || len(piece) <= 256 {
			out = append(out, string(piece))
			continue
		}

		// Halve at a rune boundary and retry both halves.
		mid := len(piece) / 2
		head, tail := splitAtRuneBoundary(piece[:mid])
		if len(head) == 0 {
			out = append(out, string(piece))
			continue
		}
		rest := append(append([]byte{}, tail...), piece[mid:]...)
		pending = append([][]byte{head, rest}, pending...)
	}
	return out
}

// fitsFrame reports whether a payload leaves the marshaled chunk within the
// QR capacity, measured against the same limit the non-streaming path uses.
func fitsFrame(payload []byte) bool {
	probe := qr.Chunk{
		ID:         "00000000_000000",
		Index:      999999,
		Total:      999999,
		Data:       string(payload),
		SourceFile: "placeholder-for-sizing",
		MimeType:   "application/octet-stream",
		Hash:       strings.Repeat("0", 64),
		CreatedAt:  time.Now(),
	}
	encoded, err := json.Marshal(probe)
	return err == nil && len(encoded) <= qrFramePayloadLimit
}

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

// ProcessFileStreaming processes a file in chunks without loading it entirely into memory.
//
// Frames produced here must be indistinguishable from the ones processFile
// produces, because both are read back by the same extractor. It previously
// filled in only Index, Total and Data, leaving SourceFile, MimeType and Hash
// empty and storing raw bytes where the extractor expects base64. Extraction
// groups frames by Hash, so every streamed frame landed in one ""-keyed
// bucket; an empty MimeType is not "text/"-prefixed, so the extractor took
// the binary path and base64-decoded data that was never encoded, failing
// with `failed to decode base64 data for : illegal base64 data at input
// byte 0` — the empty name in that message being the unset SourceFile. Even
// had it decoded, the output path was filepath.Join(outputDir, "") — the
// directory itself.
//
// Every capsule written with --stream was therefore unreadable, while
// `pixe verify` still called it intact, because verification only checks that
// frames decode as QR codes and never attempts reassembly.
func (sp *StreamingProcessor) ProcessFileStreaming(filePath string, encryptionPassword string) ([]qr.Chunk, *ContentItem, error) {
	// Per-frame AES-GCM cannot be reassembled by the extractor: it decrypts
	// the whole rejoined file in one pass, so a sequence of independently
	// sealed frames (each with its own nonce and tag) can never be recovered.
	// Refuse rather than write another capsule that cannot be read back.
	if encryptionPassword != "" && sp.converter.cryptoService.IsEnabled() {
		return nil, nil, fmt.Errorf("streaming mode cannot encrypt: the extractor decrypts the reassembled file in a single pass, " +
			"which per-frame encryption cannot satisfy — convert without --stream to use --encrypt")
	}

	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat file: %w", err)
	}

	totalBytes := fileInfo.Size()

	// Detect MIME type up front: it decides the payload encoding, and the
	// extractor uses the same test to decide how to decode.
	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	isText := strings.HasPrefix(mimeType, "text/")

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

	estimatedChunks := int((totalBytes + int64(streamRawPayload) - 1) / int64(streamRawPayload))
	if estimatedChunks == 0 {
		estimatedChunks = 1
	}

	sourceName := filepath.Base(filePath)
	createdAt := fileInfo.ModTime()

	var chunks []qr.Chunk
	buffer := make([]byte, streamRawPayload)
	bytesProcessed := int64(0)

	// Text payloads are stored as-is, so a frame boundary must not fall
	// inside a multi-byte rune: json.Marshal replaces invalid UTF-8 with
	// U+FFFD, which would silently corrupt the byte either side of every
	// boundary. Hold the incomplete tail back for the next frame.
	var carry []byte

	for {
		n, readErr := io.ReadFull(reader, buffer)
		if n > 0 {
			fileHasher.Write(buffer[:n])
			bytesProcessed += int64(n)
		}

		atEOF := readErr == io.EOF || readErr == io.ErrUnexpectedEOF
		if readErr != nil && !atEOF {
			return nil, nil, fmt.Errorf("failed to read at offset %d: %w", bytesProcessed, readErr)
		}

		raw := make([]byte, 0, len(carry)+n)
		raw = append(raw, carry...)
		raw = append(raw, buffer[:n]...)
		carry = nil

		if isText && !atEOF {
			var held []byte
			raw, held = splitAtRuneBoundary(raw)
			carry = held
		}

		if len(raw) > 0 {
			for _, payload := range framePayloads(raw, isText) {
				chunks = append(chunks, qr.Chunk{
					Index:      len(chunks),
					Data:       payload,
					SourceFile: sourceName,
					MimeType:   mimeType,
					CreatedAt:  createdAt,
				})
			}
		}

		if sp.progressCallback != nil {
			sp.progressCallback(bytesProcessed, totalBytes, len(chunks), estimatedChunks)
		}

		if atEOF {
			break
		}
	}

	// A trailing incomplete rune means truncated input; emit it rather than
	// dropping bytes, so the extracted file still matches the source.
	if len(carry) > 0 {
		for _, payload := range framePayloads(carry, isText) {
			chunks = append(chunks, qr.Chunk{
				Index:      len(chunks),
				Data:       payload,
				SourceFile: sourceName,
				MimeType:   mimeType,
				CreatedAt:  createdAt,
			})
		}
	}

	// Calculate final hash
	fileHash := fmt.Sprintf("%x", fileHasher.Sum(nil))

	// Total, Hash and ID are only knowable once the whole file is read.
	// Hash doubles as the extractor's grouping key, so it must be set on
	// every frame.
	actualChunks := len(chunks)
	for i := range chunks {
		chunks[i].Total = actualChunks
		chunks[i].Hash = fileHash
		chunks[i].ID = fmt.Sprintf("%s_%d", fileHash[:8], i)
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
