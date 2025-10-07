// Package qr provides QR code generation and decoding for .pixe files
package qr

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	qrgen "github.com/skip2/go-qrcode"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"
)

// Generator handles QR code generation for .pixe files
type Generator struct {
	outputDir string
}

// Chunk represents a data chunk that can be encoded in a QR code
type Chunk struct {
	ID         string    `json:"id"`
	Index      int       `json:"index"`
	Total      int       `json:"total"`
	Data       string    `json:"data"`
	SourceFile string    `json:"source_file"`
	MimeType   string    `json:"mime_type"`
	Hash       string    `json:"hash"`
	Encrypted  bool      `json:"encrypted"`
	CreatedAt  time.Time `json:"created_at"`
}

// NewGenerator creates a new QR code generator
func NewGenerator() (*Generator, error) {
	return &Generator{
		outputDir: "./output",
	}, nil
}

// GenerateQRCode generates a QR code for the given data
func (g *Generator) GenerateQRCode(data string, outputPath string) error {
	return qrgen.WriteFile(data, qrgen.Medium, 512, outputPath)
}

// GenerateFrame generates a single QR code frame for a chunk
func (g *Generator) GenerateFrame(chunk Chunk, frameNumber int) (string, error) {
	// Serialize chunk to JSON
	chunkData, err := json.Marshal(chunk)
	if err != nil {
		return "", fmt.Errorf("failed to serialize chunk: %w", err)
	}

	// Generate QR code
	framePath := filepath.Join(g.outputDir, fmt.Sprintf("frame_%05d.png", frameNumber))

	err = qrgen.WriteFile(string(chunkData), qrgen.Medium, 512, framePath)
	if err != nil {
		return "", fmt.Errorf("failed to generate QR code: %w", err)
	}

	return framePath, nil
}

// GenerateFrames generates multiple QR code frames for a set of chunks
func (g *Generator) GenerateFrames(chunks []Chunk) ([]string, error) {
	var framePaths []string

	for i, chunk := range chunks {
		// Serialize chunk to JSON
		chunkData, err := json.Marshal(chunk)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize chunk %d: %w", i, err)
		}

		// Generate QR code
		framePath := filepath.Join(g.outputDir, fmt.Sprintf("frame_%05d.png", i))

		err = qrgen.WriteFile(string(chunkData), qrgen.Medium, 512, framePath)
		if err != nil {
			return nil, fmt.Errorf("failed to generate QR code for chunk %d: %w", i, err)
		}

		framePaths = append(framePaths, framePath)
	}

	return framePaths, nil
}

// DecodeQRCode decodes a QR code from an image file
func DecodeQRCode(imagePath string) (string, error) {
	// Open image file
	file, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	// Decode image based on extension
	var img image.Image
	ext := filepath.Ext(imagePath)
	switch ext {
	case ".bmp":
		img, err = bmp.Decode(file)
	case ".webp":
		img, err = webp.Decode(file)
	case ".tiff", ".tif":
		img, err = tiff.Decode(file)
	default:
		img, _, err = image.Decode(file)
	}
	
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	// Create binary bitmap from image
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", fmt.Errorf("failed to create binary bitmap: %w", err)
	}

	// Create QR code reader
	qrReader := qrcode.NewQRCodeReader()
	
	// Decode QR code
	result, err := qrReader.Decode(bmp, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decode QR code: %w", err)
	}

	return result.GetText(), nil
}

// DecodeFrame decodes a QR code frame and returns the chunk data
func DecodeFrame(imagePath string) (*Chunk, error) {
	// Decode QR code
	qrData, err := DecodeQRCode(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to decode QR code: %w", err)
	}

	// Parse JSON data
	var chunk Chunk
	if err := json.Unmarshal([]byte(qrData), &chunk); err != nil {
		return nil, fmt.Errorf("failed to parse chunk JSON: %w", err)
	}

	return &chunk, nil
}
