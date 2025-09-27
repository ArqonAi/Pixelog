package qr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/skip2/go-qrcode"
)

type Generator struct {
	outputDir string
}

type Chunk struct {
	ID         string    `json:"id"`
	Index      int       `json:"index"`
	Total      int       `json:"total"`
	Data       string    `json:"data"`
	SourceFile string    `json:"source_file"`
	MimeType   string    `json:"mime_type"`
	Hash       string    `json:"hash"`
	CreatedAt  time.Time `json:"created_at"`
}

func New(outputDir string) (*Generator, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &Generator{
		outputDir: outputDir,
	}, nil
}

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

		err = qrcode.WriteFile(string(chunkData), qrcode.Medium, 512, framePath)
		if err != nil {
			return nil, fmt.Errorf("failed to generate QR code for chunk %d: %w", i, err)
		}

		framePaths = append(framePaths, framePath)
	}

	return framePaths, nil
}

func (g *Generator) GenerateFrame(chunk Chunk, frameNumber int) (string, error) {
	// Serialize chunk to JSON
	chunkData, err := json.Marshal(chunk)
	if err != nil {
		return "", fmt.Errorf("failed to serialize chunk: %w", err)
	}

	// Generate QR code
	framePath := filepath.Join(g.outputDir, fmt.Sprintf("frame_%05d.png", frameNumber))

	err = qrcode.WriteFile(string(chunkData), qrcode.Medium, 512, framePath)
	if err != nil {
		return "", fmt.Errorf("failed to generate QR code: %w", err)
	}

	return framePath, nil
}

func DecodeFrame(imagePath string) (*Chunk, error) {
	// This would require a QR code reader library
	// For now, we'll implement a placeholder
	return nil, fmt.Errorf("QR decode not implemented yet")
}
