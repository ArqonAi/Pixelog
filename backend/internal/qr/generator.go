package qr

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/makiuchi-d/gozxing"
	qrReader "github.com/makiuchi-d/gozxing/qrcode"
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
	// Use gozxing for decoding (matches the API handler)
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	// Decode image
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Create bitmap and decode QR
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return nil, fmt.Errorf("failed to create bitmap: %w", err)
	}

	reader := qrReader.NewQRCodeReader()
	result, err := reader.Decode(bmp, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode QR code: %w", err)
	}

	// Parse chunk data
	var chunk Chunk
	if err := json.Unmarshal([]byte(result.GetText()), &chunk); err != nil {
		return nil, fmt.Errorf("failed to parse chunk JSON: %w", err)
	}

	return &chunk, nil
}
