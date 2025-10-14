package qr

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
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
	Encrypted  bool      `json:"encrypted"`
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

		// Generate QR code with gozxing
		framePath := filepath.Join(g.outputDir, fmt.Sprintf("frame_%05d.png", i))
		
		writer := qrcode.NewQRCodeWriter()
		hints := make(map[gozxing.EncodeHintType]interface{})
		hints[gozxing.EncodeHintType_ERROR_CORRECTION] = "M"
		bitMatrix, err := writer.Encode(string(chunkData), gozxing.BarcodeFormat_QR_CODE, 512, 512, hints)
		if err != nil {
			return nil, fmt.Errorf("failed to encode QR code for chunk %d: %w", i, err)
		}
		
		file, err := os.Create(framePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create frame file: %w", err)
		}
		defer file.Close()
		
		err = png.Encode(file, bitMatrix)
		if err != nil {
			return nil, fmt.Errorf("failed to save QR image for chunk %d: %w", i, err)
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

	// Generate QR code with gozxing
	framePath := filepath.Join(g.outputDir, fmt.Sprintf("frame_%05d.png", frameNumber))
	
	writer := qrcode.NewQRCodeWriter()
	hints := make(map[gozxing.EncodeHintType]interface{})
	hints[gozxing.EncodeHintType_ERROR_CORRECTION] = "M"
	bitMatrix, err := writer.Encode(string(chunkData), gozxing.BarcodeFormat_QR_CODE, 512, 512, hints)
	if err != nil {
		return "", fmt.Errorf("failed to encode QR code: %w", err)
	}
	
	file, err := os.Create(framePath)
	if err != nil {
		return "", fmt.Errorf("failed to create frame file: %w", err)
	}
	defer file.Close()
	
	err = png.Encode(file, bitMatrix)
	if err != nil {
		return "", fmt.Errorf("failed to save QR image: %w", err)
	}

	return framePath, nil
}

func DecodeFrame(imagePath string) (*Chunk, error) {
	// Use gozxing for pure Go QR decoding
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return nil, fmt.Errorf("failed to create bitmap: %w", err)
	}

	reader := qrcode.NewQRCodeReader()
	hints := make(map[gozxing.DecodeHintType]interface{})
	hints[gozxing.DecodeHintType_PURE_BARCODE] = true
	result, err := reader.Decode(bmp, hints)
	if err != nil {
		return nil, fmt.Errorf("failed to decode QR code: %w", err)
	}

	var chunk Chunk
	if err := json.Unmarshal([]byte(result.GetText()), &chunk); err != nil {
		return nil, fmt.Errorf("failed to parse QR data: %w", err)
	}

	return &chunk, nil
}
