package qr

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	// Use OpenCV via Python script for reliable QR decoding (like memvid)
	pythonScript := `
import cv2
import sys
import json

try:
    # Load image
    img = cv2.imread(sys.argv[1])
    if img is None:
        print("ERROR: Could not load image", file=sys.stderr)
        sys.exit(1)
    
    # Create QR detector
    qcd = cv2.QRCodeDetector()
    
    # Detect and decode QR codes
    retval, decoded_info, points, straight_qrcode = qcd.detectAndDecodeMulti(img)
    
    # Check if any QR codes were found
    if not retval or not decoded_info:
        print("ERROR: No QR codes found", file=sys.stderr)
        sys.exit(1)
    
    # Find the first non-empty decoded QR code
    for data in decoded_info:
        if data.strip():
            print(data)
            sys.exit(0)
    
    print("ERROR: All QR codes were empty", file=sys.stderr)
    sys.exit(1)
    
except Exception as e:
    print(f"ERROR: {e}", file=sys.stderr)
    sys.exit(1)
`

	// Write Python script to temp file
	tempScript := "/tmp/qr_decode.py"
	if err := os.WriteFile(tempScript, []byte(pythonScript), 0644); err != nil {
		return nil, fmt.Errorf("failed to write Python script: %w", err)
	}
	defer os.Remove(tempScript)

	// Execute Python script with OpenCV
	cmd := exec.Command("python3", tempScript, imagePath)
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		return nil, fmt.Errorf("failed to decode QR with OpenCV: %v, output: %s", err, string(output))
	}

	// Parse the JSON output
	qrData := strings.TrimSpace(string(output))
	if qrData == "" {
		return nil, fmt.Errorf("empty QR data returned")
	}

	var chunk Chunk
	if err := json.Unmarshal([]byte(qrData), &chunk); err != nil {
		return nil, fmt.Errorf("failed to parse chunk JSON: %w, data: %s", err, qrData)
	}

	return &chunk, nil
}
