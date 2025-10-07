// Package converter provides functionality to convert files into .pixe format
package converter

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ArqonAi/Pixelog/pkg/crypto"
	"github.com/ArqonAi/Pixelog/pkg/qr"
)

// Converter handles file to .pixe conversion
type Converter struct {
	OutputDir     string
	QRGenerator   *qr.Generator
	CryptoService *crypto.EncryptionService
}

// ContentItem represents a file or content within a .pixe archive
type ContentItem struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Size      string    `json:"size"`
	Hash      string    `json:"hash"`
	Encrypted bool      `json:"encrypted"`
	CreatedAt time.Time `json:"created_at"`
}

// PixeMetadata contains metadata about the .pixe file
type PixeMetadata struct {
	Version   string        `json:"version"`
	CreatedAt time.Time     `json:"created_at"`
	Contents  []ContentItem `json:"contents"`
	Encrypted bool          `json:"encrypted"`
	Signature string        `json:"signature,omitempty"`
}

// ConvertOptions configures the conversion process
type ConvertOptions struct {
	OutputPath      string
	EncryptionKey   string
	CompressionMode string
	QRCodeEnabled   bool
}

// New creates a new Converter instance
func New(outputDir string) (*Converter, error) {
	if outputDir == "" {
		outputDir = "./output"
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	qrGen, err := qr.NewGenerator()
	if err != nil {
		return nil, fmt.Errorf("failed to create QR generator: %w", err)
	}

	cryptoSvc := crypto.NewEncryptionService()

	return &Converter{
		OutputDir:     outputDir,
		QRGenerator:   qrGen,
		CryptoService: cryptoSvc,
	}, nil
}

// ConvertFile converts a single file to .pixe format
func (c *Converter) ConvertFile(inputPath string, opts *ConvertOptions) (string, error) {
	if opts == nil {
		opts = &ConvertOptions{}
	}

	// Read input file
	fileInfo, err := os.Stat(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat input file: %w", err)
	}

	content, err := os.ReadFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to read input file: %w", err)
	}

	// Generate hash
	hash := sha256.Sum256(content)
	hashStr := base64.StdEncoding.EncodeToString(hash[:])

	// Encrypt if key provided
	var encrypted bool
	if opts.EncryptionKey != "" {
		encryptedContent, err := c.CryptoService.Encrypt(content, opts.EncryptionKey)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt content: %w", err)
		}
		content = encryptedContent
		encrypted = true
	}

	// Create metadata
	filename := filepath.Base(inputPath)
	mimeType := mime.TypeByExtension(filepath.Ext(inputPath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	contentItem := ContentItem{
		Name:      filename,
		Type:      mimeType,
		Size:      formatFileSize(fileInfo.Size()),
		Hash:      hashStr,
		Encrypted: encrypted,
		CreatedAt: time.Now(),
	}

	metadata := PixeMetadata{
		Version:   "1.0",
		CreatedAt: time.Now(),
		Contents:  []ContentItem{contentItem},
		Encrypted: encrypted,
	}

	// Generate output filename if not provided
	outputPath := opts.OutputPath
	if outputPath == "" {
		baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
		outputPath = filepath.Join(c.OutputDir, baseName+".pixe")
	}

	// Create .pixe file (JSON format for now)
	pixeData := map[string]interface{}{
		"metadata": metadata,
		"content":  base64.StdEncoding.EncodeToString(content),
	}

	jsonData, err := json.MarshalIndent(pixeData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal .pixe data: %w", err)
	}

	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return "", fmt.Errorf("failed to write .pixe file: %w", err)
	}

	return outputPath, nil
}

// ExtractFile extracts content from a .pixe file
func (c *Converter) ExtractFile(pixePath, extractPath, decryptionKey string) error {
	// Read .pixe file
	pixeData, err := os.ReadFile(pixePath)
	if err != nil {
		return fmt.Errorf("failed to read .pixe file: %w", err)
	}

	// Parse JSON
	var data map[string]interface{}
	if err := json.Unmarshal(pixeData, &data); err != nil {
		return fmt.Errorf("failed to parse .pixe file: %w", err)
	}

	// Extract content
	contentStr, ok := data["content"].(string)
	if !ok {
		return fmt.Errorf("invalid .pixe file format: missing content")
	}

	content, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		return fmt.Errorf("failed to decode content: %w", err)
	}

	// Check if encrypted and decrypt
	if metadata, ok := data["metadata"].(map[string]interface{}); ok {
		if encrypted, ok := metadata["encrypted"].(bool); ok && encrypted {
			if decryptionKey == "" {
				return fmt.Errorf("decryption key required for encrypted .pixe file")
			}

			decryptedContent, err := c.CryptoService.Decrypt(content, decryptionKey)
			if err != nil {
				return fmt.Errorf("failed to decrypt content: %w", err)
			}
			content = decryptedContent
		}
	}

	// Write extracted file
	if err := os.WriteFile(extractPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write extracted file: %w", err)
	}

	return nil
}

// ListContents lists the contents of a .pixe file without extracting
func (c *Converter) ListContents(pixePath string) ([]ContentItem, error) {
	pixeData, err := os.ReadFile(pixePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .pixe file: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(pixeData, &data); err != nil {
		return nil, fmt.Errorf("failed to parse .pixe file: %w", err)
	}

	metadata, ok := data["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid .pixe file format: missing metadata")
	}

	contentsData, ok := metadata["contents"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid .pixe file format: invalid contents")
	}

	var contents []ContentItem
	for _, item := range contentsData {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		contentItem := ContentItem{}
		if name, ok := itemMap["name"].(string); ok {
			contentItem.Name = name
		}
		if type_, ok := itemMap["type"].(string); ok {
			contentItem.Type = type_
		}
		if size, ok := itemMap["size"].(string); ok {
			contentItem.Size = size
		}
		if hash, ok := itemMap["hash"].(string); ok {
			contentItem.Hash = hash
		}
		if encrypted, ok := itemMap["encrypted"].(bool); ok {
			contentItem.Encrypted = encrypted
		}

		contents = append(contents, contentItem)
	}

	return contents, nil
}

// formatFileSize converts bytes to human-readable format
func formatFileSize(bytes int64) string {
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
