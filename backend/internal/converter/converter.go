package converter

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ArqonAi/Pixelog/backend/internal/crypto"
	"github.com/ArqonAi/Pixelog/backend/internal/qr"
	"github.com/ArqonAi/Pixelog/backend/internal/video"
	"github.com/ArqonAi/Pixelog/backend/pkg/config"
)

type Converter struct {
	config        *config.Config
	qrGenerator   *qr.Generator
	videoMaker    *video.Maker
	cryptoService *crypto.EncryptionService
	mu            sync.RWMutex
	jobs          map[string]*Job
}

type Job struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	Progress   int       `json:"progress"`
	Stage      string    `json:"stage"`
	Error      string    `json:"error,omitempty"`
	InputFile  string    `json:"input_file"`
	OutputFile string    `json:"output_file"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Progress struct {
	Stage      string `json:"stage"`
	Percentage int    `json:"percentage"`
	Message    string `json:"message"`
}

type ContentItem struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Size      string    `json:"size"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
}

type Metadata struct {
	Version     string         `json:"version"`
	CreatedAt   time.Time      `json:"created_at"`
	TotalChunks int            `json:"total_chunks"`
	Contents    []ContentItem  `json:"contents"`
	Config      *config.Config `json:"config"`
}

func New(cfg *config.Config) (*Converter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	qrGen, err := qr.New(cfg.TempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create QR generator: %w", err)
	}

	videoMaker, err := video.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create video maker: %w", err)
	}

	// Initialize crypto service
	cryptoService := crypto.NewEncryptionService(cfg.EncryptionEnabled)

	return &Converter{
		config:        cfg,
		qrGenerator:   qrGen,
		videoMaker:    videoMaker,
		cryptoService: cryptoService,
		jobs:          make(map[string]*Job),
	}, nil
}

func (c *Converter) Convert(inputPath, outputPath string, progressChan chan<- Progress, encryptionPassword ...string) error {
	jobID := generateJobID()

	job := &Job{
		ID:         jobID,
		Status:     "starting",
		Progress:   0,
		Stage:      "Initializing",
		InputFile:  inputPath,
		OutputFile: outputPath,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	c.setJob(jobID, job)
	// Don't immediately remove job - keep it for status checking
	// Remove after 10 minutes to prevent memory leaks
	go func() {
		time.Sleep(10 * time.Minute)
		c.removeJob(jobID)
	}()

	// Extract encryption password if provided
	var password string
	if len(encryptionPassword) > 0 {
		password = encryptionPassword[0]
	}

	updateProgress := func(stage string, progress int, message string) {
		job.Stage = stage
		job.Progress = progress
		job.UpdatedAt = time.Now()
		c.setJob(jobID, job)

		if progressChan != nil {
			select {
			case progressChan <- Progress{Stage: stage, Percentage: progress, Message: message}:
			default:
			}
		}
	}

	updateProgress("Analyzing input", 10, "Scanning files...")

	// Analyze input
	files, err := c.analyzeInput(inputPath)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		c.setJob(jobID, job)
		return fmt.Errorf("failed to analyze input: %w", err)
	}

	updateProgress("Processing files", 25, fmt.Sprintf("Found %d files", len(files)))

	// Process all files and create chunks
	var allChunks []qr.Chunk
	var contents []ContentItem

	for i, file := range files {
		chunks, item, err := c.processFile(file, password)
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
			c.setJob(jobID, job)
			return fmt.Errorf("failed to process file %s: %w", file, err)
		}

		allChunks = append(allChunks, chunks...)
		contents = append(contents, *item)

		progress := 25 + (i+1)*30/len(files)
		updateProgress("Processing files", progress, fmt.Sprintf("Processed %s", filepath.Base(file)))
	}

	updateProgress("Generating QR codes", 60, fmt.Sprintf("Creating %d QR frames", len(allChunks)))

	// Generate QR codes
	framePaths, err := c.qrGenerator.GenerateFrames(allChunks)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		c.setJob(jobID, job)
		return fmt.Errorf("failed to generate QR frames: %w", err)
	}

	updateProgress("Creating video", 80, "Assembling video file...")

	// Create metadata
	metadata := &Metadata{
		Version:     "1.0.0",
		CreatedAt:   time.Now(),
		TotalChunks: len(allChunks),
		Contents:    contents,
		Config:      c.config,
	}

	// Create video with metadata
	err = c.videoMaker.CreateVideo(framePaths, outputPath, metadata, c.config)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		c.setJob(jobID, job)
		return fmt.Errorf("failed to create video: %w", err)
	}

	updateProgress("Finalizing", 95, "Cleaning up temporary files...")

	// Cleanup
	for _, framePath := range framePaths {
		os.Remove(framePath)
	}

	updateProgress("Complete", 100, "Successfully created .pixe file!")

	job.Status = "completed"
	job.Progress = 100
	job.UpdatedAt = time.Now()
	c.setJob(jobID, job)

	return nil
}

func (c *Converter) Extract(pixeFilePath, outputDir string, decryptionPassword ...string) error {
	// Get the password if provided
	var password string
	if len(decryptionPassword) > 0 {
		password = decryptionPassword[0]
	}

	fmt.Printf("DEBUG: Starting extraction from %s\n", pixeFilePath)
	
	// Use the video maker to extract data - this will create the files
	err := c.videoMaker.ExtractData(pixeFilePath, outputDir)
	if err != nil {
		return fmt.Errorf("failed to extract data from video: %w", err)
	}

	fmt.Printf("DEBUG: Video extraction completed, now processing for decryption if needed\n")
	
	// If no password provided, we're done
	if password == "" {
		return nil
	}

	// If password is provided, we need to decrypt the extracted files
	// This is a simplified approach - in reality we'd need to read the chunk metadata
	// to determine which files are encrypted, but for now let's try to decrypt all files
	extractedFiles, err := filepath.Glob(filepath.Join(outputDir, "*"))
	if err != nil {
		return fmt.Errorf("failed to list extracted files: %w", err)
	}

	for _, filePath := range extractedFiles {
		// Try to decrypt the file
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip files we can't read
		}

		// Attempt decryption
		decryptedData, err := c.cryptoService.DecryptData(data, password)
		if err != nil {
			// If decryption fails, the file might not be encrypted, so leave it as is
			fmt.Printf("DEBUG: File %s is not encrypted or decryption failed: %v\n", filepath.Base(filePath), err)
			continue
		}

		// Write the decrypted data back
		err = os.WriteFile(filePath, decryptedData, 0644)
		if err != nil {
			fmt.Printf("ERROR: Failed to write decrypted file %s: %v\n", filePath, err)
			continue
		}

		fmt.Printf("DEBUG: Successfully decrypted file %s\n", filepath.Base(filePath))
	}

	return nil
}

func (c *Converter) ListContents(inputPath string) ([]ContentItem, error) {
	metadata, err := c.videoMaker.ExtractMetadata(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}

	// Convert video.ContentItem to converter.ContentItem
	var contents []ContentItem
	for _, item := range metadata.Contents {
		contents = append(contents, ContentItem{
			Name:      item.Name,
			Type:      item.Type,
			Size:      item.Size,
			Hash:      item.Hash,
			CreatedAt: time.Now(), // Parse from string if needed
		})
	}

	return contents, nil
}

func (c *Converter) analyzeInput(inputPath string) ([]string, error) {
	var files []string

	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		err = filepath.Walk(inputPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				files = append(files, path)
			}
			return nil
		})
	} else {
		files = []string{inputPath}
	}

	return files, err
}

func (c *Converter) processFile(filePath string, encryptionPassword string) ([]qr.Chunk, *ContentItem, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, err
	}

	// Encrypt data if password is provided
	originalData := data
	if encryptionPassword != "" && c.cryptoService.IsEnabled() {
		fmt.Printf("DEBUG: Encrypting file %s with AES-256-GCM\n", filepath.Base(filePath))
		encryptedData, err := c.cryptoService.EncryptData(data, encryptionPassword)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to encrypt data: %w", err)
		}
		data = encryptedData
		fmt.Printf("DEBUG: Encryption successful - size changed from %d to %d bytes\n", len(originalData), len(data))
	}

	// Calculate hash (of encrypted data if encrypted)
	hasher := sha256.New()
	hasher.Write(data)
	hash := fmt.Sprintf("%x", hasher.Sum(nil))

	// Detect MIME type
	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Create content item
	item := &ContentItem{
		Name:      filepath.Base(filePath),
		Type:      mimeType,
		Size:      formatSize(int64(len(data))),
		Hash:      hash,
		CreatedAt: time.Now(),
	}

	// Encode data
	var encodedData string
	if strings.HasPrefix(mimeType, "text/") {
		encodedData = string(data)
	} else {
		encodedData = base64.StdEncoding.EncodeToString(data)
	}

	// Create chunks
	isEncrypted := encryptionPassword != "" && c.cryptoService.IsEnabled()
	chunks := c.createChunks(encodedData, filePath, mimeType, hash, isEncrypted)

	return chunks, item, nil
}

func (c *Converter) createChunks(data, filePath, mimeType, hash string, encrypted bool) []qr.Chunk {
	var chunks []qr.Chunk
	chunkSize := c.config.ChunkSize - 200 // Leave room for metadata

	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := qr.Chunk{
			ID:         fmt.Sprintf("%s_%d", hash[:8], len(chunks)),
			Index:      len(chunks),
			Total:      (len(data) + chunkSize - 1) / chunkSize,
			Data:       data[i:end],
			SourceFile: filepath.Base(filePath),
			MimeType:   mimeType,
			Hash:       hash,
			Encrypted:  encrypted,
			CreatedAt:  time.Now(),
		}

		chunks = append(chunks, chunk)
	}

	// Update total count
	for i := range chunks {
		chunks[i].Total = len(chunks)
	}

	return chunks
}

func (c *Converter) setJob(id string, job *Job) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.jobs[id] = job
}

func (c *Converter) GetJob(id string) (*Job, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	job, exists := c.jobs[id]
	return job, exists
}

func (c *Converter) removeJob(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.jobs, id)
}

func (c *Converter) Cleanup() {
	c.config.Cleanup()
}

func (c *Converter) GetOutputDir() string {
	return c.config.OutputDir
}

func generateJobID() string {
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}


func formatSize(bytes int64) string {
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
