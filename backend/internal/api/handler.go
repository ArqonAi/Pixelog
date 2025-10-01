package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/ArqonAi/Pixelog/backend/internal/converter"
	"github.com/ArqonAi/Pixelog/backend/internal/crypto"
	"github.com/ArqonAi/Pixelog/backend/internal/search"
	"github.com/ArqonAi/Pixelog/backend/internal/storage"
)

type Handler struct {
	converter  *converter.Converter
	upgrader   *websocket.Upgrader
	search     *search.SearchService
	encryption *crypto.EncryptionService
	cloud      *storage.CloudService
}

type ConvertRequest struct {
	Quality   int     `json:"quality" form:"quality"`
	FrameRate float64 `json:"framerate" form:"framerate"`
	ChunkSize int     `json:"chunksize" form:"chunksize"`
}

type ConvertResponse struct {
	JobID     string `json:"job_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	OutputURL string `json:"output_url,omitempty"`
}

type PixeFile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Size      string    `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Path      string    `json:"path"`
}

func NewHandler(conv *converter.Converter, upgrader *websocket.Upgrader, searchSvc *search.SearchService, encSvc *crypto.EncryptionService, cloudSvc *storage.CloudService) *Handler {
	return &Handler{
		converter:  conv,
		upgrader:   upgrader,
		search:     searchSvc,
		encryption: encSvc,
		cloud:      cloudSvc,
	}
}

// Search endpoints
func (h *Handler) Search(c *gin.Context) {
	if h.search == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Search service not available. Set OPENAI_API_KEY environment variable.",
		})
		return
	}

	var req search.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results, err := h.search.Search(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
	})
}

func (h *Handler) GetSimilar(c *gin.Context) {
	if h.search == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Search service not available",
		})
		return
	}

	documentID := c.Param("id")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "document ID required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "5")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 5
	}

	results, err := h.search.GetSimilarDocuments(c.Request.Context(), documentID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
	})
}

func (h *Handler) ListDocuments(c *gin.Context) {
	if h.search == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Search service not available",
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}

	documents, err := h.search.ListDocuments(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": documents,
		"count":     len(documents),
	})
}

func (h *Handler) ConvertFile(c *gin.Context) {
	// Check file size limits
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 100<<20) // 100MB limit

	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files provided"})
		return
	}

	// Parse options
	var req ConvertRequest
	if err := c.ShouldBind(&req); err != nil {
		// Use defaults
		req.Quality = 23
		req.FrameRate = 0.5
		req.ChunkSize = 2800
	}

	// Create temporary directory for uploaded files
	tempDir, err := os.MkdirTemp("", "pixelog-upload-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp directory"})
		return
	}

	// Save uploaded files
	var uploadedFiles []string
	for _, file := range files {
		dst := filepath.Join(tempDir, file.Filename)
		if err := c.SaveUploadedFile(file, dst); err != nil {
			os.RemoveAll(tempDir)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save file %s", file.Filename)})
			return
		}
		uploadedFiles = append(uploadedFiles, dst)
	}

	// Generate output filename
	timestamp := time.Now().Format("20060102_150405")
	outputName := fmt.Sprintf("pixelog_%s.pixe", timestamp)
	outputPath := filepath.Join("./output", outputName)

	// Ensure output directory exists
	os.MkdirAll("./output", 0755)

	// Start conversion asynchronously
	jobID := generateJobID()

	go func() {
		defer os.RemoveAll(tempDir)

		var inputPath string
		if len(uploadedFiles) == 1 {
			inputPath = uploadedFiles[0]
		} else {
			inputPath = tempDir // Directory with multiple files
		}

		progressChan := make(chan converter.Progress, 10)
		defer close(progressChan)

		// Index files for search if search service is available
		if h.search != nil {
			for _, filePath := range uploadedFiles {
				file, err := os.Open(filePath)
				if err != nil {
					continue // Skip files that can't be opened
				}

				// Check if file is supported for text extraction
				if h.search.IsFileSupported(filepath.Base(filePath)) {
					_, err = file.Seek(0, 0) // Reset file pointer
					if err != nil {
						file.Close()
						continue
					}

					// Index the file
					indexReq := &search.IndexRequest{
						ID:       fmt.Sprintf("%s_%s", jobID, filepath.Base(filePath)),
						Reader:   file,
						Filename: filepath.Base(filePath),
						Metadata: map[string]interface{}{
							"job_id":     jobID,
							"file_path":  filePath,
							"indexed_at": time.Now(),
						},
					}

					if err := h.search.IndexFile(c.Request.Context(), indexReq); err != nil {
						// Log error but don't fail conversion
						fmt.Printf("Failed to index file %s: %v\n", filePath, err)
					}
				}
				file.Close()
			}
		}

		if err := h.converter.Convert(inputPath, outputPath, progressChan); err != nil {
			// Handle error (could be logged or stored for retrieval)
			return
		}
	}()

	c.JSON(http.StatusAccepted, ConvertResponse{
		JobID:   jobID,
		Status:  "processing",
		Message: "Conversion started successfully",
	})
}

func (h *Handler) GetProgress(c *gin.Context) {
	jobID := c.Param("id")

	job, exists := h.converter.GetJob(jobID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

func (h *Handler) WebSocketHandler(c *gin.Context) {
	jobID := c.Param("id")

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Send progress updates via WebSocket
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			job, exists := h.converter.GetJob(jobID)
			if !exists {
				conn.WriteJSON(gin.H{"error": "Job not found"})
				return
			}

			if err := conn.WriteJSON(job); err != nil {
				return
			}

			if job.Status == "completed" || job.Status == "failed" {
				return
			}
		}
	}
}

// ExtractPixeFile extracts content from a .pixe file
func (h *Handler) ExtractPixeFile(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filename required"})
		return
	}

	// Construct full path
	pixePath := filepath.Join(h.converter.GetOutputDir(), filename)
	if !strings.HasSuffix(pixePath, ".pixe") {
		pixePath += ".pixe"
	}

	// Check if file exists
	if _, err := os.Stat(pixePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Create temporary output directory
	outputDir, err := os.MkdirTemp("", "pixelog-extract-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp directory"})
		return
	}
	defer os.RemoveAll(outputDir)

	// Extract the data
	err = h.converter.Extract(pixePath, outputDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Extraction failed: %v", err)})
		return
	}

	// List extracted files
	extractedFiles, err := filepath.Glob(filepath.Join(outputDir, "*"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list extracted files"})
		return
	}

	// Return list of extracted files
	var fileList []string
	for _, file := range extractedFiles {
		fileList = append(fileList, filepath.Base(file))
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Extraction completed",
		"extracted_files": fileList,
		"output_dir":      outputDir,
	})
}

// ListPixeContents lists the contents of a .pixe file without extracting
func (h *Handler) ListPixeContents(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filename required"})
		return
	}

	// Construct full path
	pixePath := filepath.Join(h.converter.GetOutputDir(), filename)
	if !strings.HasSuffix(pixePath, ".pixe") {
		pixePath += ".pixe"
	}

	// Check if file exists
	if _, err := os.Stat(pixePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// List contents
	contents, err := h.converter.ListContents(pixePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list contents: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"contents": contents})
}

func (h *Handler) ListPixeFiles(c *gin.Context) {
	outputDir := "./output"

	files, err := os.ReadDir(outputDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read output directory"})
		return
	}

	var pixeFiles []PixeFile
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".pixe" {
			info, err := file.Info()
			if err != nil {
				continue
			}

			pixeFile := PixeFile{
				ID:        strings.TrimSuffix(file.Name(), ".pixe"),
				Name:      file.Name(),
				Size:      formatFileSize(info.Size()),
				CreatedAt: info.ModTime(),
				Path:      filepath.Join(outputDir, file.Name()),
			}
			pixeFiles = append(pixeFiles, pixeFile)
		}
	}

	c.JSON(http.StatusOK, pixeFiles)
}

func (h *Handler) ExtractFile(c *gin.Context) {
	// Implementation for file extraction
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Extraction not yet implemented"})
}

func (h *Handler) DownloadFile(c *gin.Context) {
	fileID := c.Param("id")
	
	// Use the converter's output directory
	outputDir := h.converter.GetOutputDir()
	filePath := filepath.Join(outputDir, fileID+".pixe")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Set proper headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+fileID+".pixe")
	c.Header("Content-Type", "application/octet-stream")
	
	c.File(filePath)
}

func (h *Handler) DeletePixeFile(c *gin.Context) {
	fileID := c.Param("id")
	
	// Use the converter's output directory
	outputDir := h.converter.GetOutputDir()
	filePath := filepath.Join(outputDir, fileID+".pixe")

	// Check if file exists first
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File deleted successfully",
		"file_id": fileID,
	})
}

func (h *Handler) SearchContent(c *gin.Context) {
	// Implementation for content search
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Search not yet implemented"})
}

// LLM Memory Processing
type ProcessMemoryRequest struct {
	FileIDs       []string `json:"file_ids"`
	FileNames     []string `json:"file_names"`
	DecryptionKey string   `json:"decryption_key,omitempty"`
}

type ProcessedMemory struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Chunks   int    `json:"chunks"`
	Size     int64  `json:"size"`
	Status   string `json:"status"`
	Encrypted bool  `json:"encrypted"`
}

type ProcessMemoryResponse struct {
	Memories []ProcessedMemory `json:"memories"`
	Message  string            `json:"message"`
}

func (h *Handler) ProcessLLMMemories(c *gin.Context) {
	var req ProcessMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.FileIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files provided"})
		return
	}

	var memories []ProcessedMemory
	outputDir := h.converter.GetOutputDir()
	if outputDir == "" {
		outputDir = "./output" // Fallback to default
	}

	for i, fileID := range req.FileIDs {
		var filePath string
		var fileInfo os.FileInfo
		var err error
		
		// Try converter output directory first
		filePath = filepath.Join(outputDir, fileID+".pixe")
		fileInfo, err = os.Stat(filePath)
		
		// If not found, try relative ./output directory
		if err != nil {
			filePath = filepath.Join("./output", fileID+".pixe")
			fileInfo, err = os.Stat(filePath)
		}
		
		// If still not found, try absolute output path
		if err != nil {
			filePath = filepath.Join("output", fileID+".pixe")
			fileInfo, err = os.Stat(filePath)
		}
		
		if err != nil {
			fmt.Printf("File %s not found in any location\n", fileID)
			continue // Skip non-existent files
		}
		
		fmt.Printf("Found file at: %s\n", filePath)

		// Extract content from .pixe file using converter
		chunks := int(fileInfo.Size() / 2800) // Estimate chunks based on file size
		if chunks < 1 {
			chunks = 1
		}

		filename := fileID + ".pixe"
		if i < len(req.FileNames) {
			filename = req.FileNames[i]
		}

		memory := ProcessedMemory{
			ID:        fileID,
			Filename:  filename,
			Chunks:    chunks,
			Size:      fileInfo.Size(),
			Status:    "ready",
			Encrypted: req.DecryptionKey != "",
		}

		memories = append(memories, memory)
	}

	response := ProcessMemoryResponse{
		Memories: memories,
		Message:  fmt.Sprintf("Successfully processed %d files for LLM memory", len(memories)),
	}

	c.JSON(http.StatusOK, response)
}

// LLM Chat endpoint
type ChatRequest struct {
	Query     string   `json:"query"`
	MemoryIDs []string `json:"memory_ids"`
	Provider  string   `json:"provider"`
	Model     string   `json:"model"`
	APIKey    string   `json:"api_key"`
}

type ChatResponse struct {
	Content   string                 `json:"content"`
	Sources   []map[string]interface{} `json:"sources"`
	Model     string                 `json:"model"`
	Provider  string                 `json:"provider"`
	MemoryIDs []string               `json:"memory_ids"`
}

func (h *Handler) LLMChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API key is required"})
		return
	}

	// Extract content from .pixe files
	var extractedContent []string
	outputDir := h.converter.GetOutputDir()
	if outputDir == "" {
		outputDir = "./output"
	}

	for _, memoryID := range req.MemoryIDs {
		var filePath string
		
		// Try multiple paths to find the file
		filePath = filepath.Join(outputDir, memoryID+".pixe")
		if _, err := os.Stat(filePath); err != nil {
			filePath = filepath.Join("./output", memoryID+".pixe")
			if _, err := os.Stat(filePath); err != nil {
				filePath = filepath.Join("output", memoryID+".pixe")
				if _, err := os.Stat(filePath); err != nil {
					continue // Skip if file not found
				}
			}
		}

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", filePath, err)
			continue
		}

		// Convert to string and add to extracted content
		fileContent := string(content)
		if len(fileContent) > 0 {
			extractedContent = append(extractedContent, fmt.Sprintf("File %s:\n%s", memoryID, fileContent))
		}
	}

	if len(extractedContent) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No content found in specified memory files"})
		return
	}

	// Prepare context for LLM
	context := strings.Join(extractedContent, "\n\n---\n\n")
	prompt := fmt.Sprintf("Context from memory files:\n%s\n\nUser question: %s", context, req.Query)

	// Make API call to LLM provider
	allmResponse, err := h.callLLMProvider(req.Provider, req.Model, req.APIKey, prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("LLM API error: %v", err)})
		return
	}

	response := ChatResponse{
		Content:   allmResponse,
		Sources:   []map[string]interface{}{}, // TODO: Add source references
		Model:     req.Model,
		Provider:  req.Provider,
		MemoryIDs: req.MemoryIDs,
	}

	c.JSON(http.StatusOK, response)
}

// LLM Search endpoint
func (h *Handler) LLMSearch(c *gin.Context) {
	query := c.Query("q")
	limit := c.DefaultQuery("limit", "10")
	memoryID := c.Query("memory_id")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	// Mock search results for now - in full implementation would search through processed memories
	results := []map[string]interface{}{
		{
			"content":      fmt.Sprintf("Search result for '%s'", query),
			"filename":     "example.pixe",
			"frame_number": 23,
			"relevance":    0.92,
			"chunk_id":     "chunk_123",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"query":     query,
		"results":   results,
		"limit":     limit,
		"memory_id": memoryID,
		"total":     len(results),
	})
}

func generateJobID() string {
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}

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

// Real LLM Provider Integration
func (h *Handler) callLLMProvider(provider, model, apiKey, prompt string) (string, error) {
	switch provider {
	case "openai":
		return h.callOpenAI(model, apiKey, prompt)
	case "openrouter":
		return h.callOpenRouter(model, apiKey, prompt)
	case "google":
		return h.callGoogleAI(model, apiKey, prompt)
	case "anthropic":
		return h.callAnthropic(model, apiKey, prompt)
	case "ollama":
		return h.callOllama(model, prompt)
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}
}

func (h *Handler) callOpenAI(model, apiKey, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": 2000,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("OpenAI API error: %s", string(body))
	}

	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return "", fmt.Errorf("invalid response format")
	}

	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid choice format")
	}

	message, ok := firstChoice["message"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid message format")
	}

	content, ok := message["content"].(string)
	if !ok {
		return "", fmt.Errorf("invalid content format")
	}

	return content, nil
}

func (h *Handler) callOpenRouter(model, apiKey, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": 2000,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	// OpenRouter requires these headers - try wildcard approach
	req.Header.Set("HTTP-Referer", "https://localhost")
	req.Header.Set("X-Title", "Pixelog")
	// Alternative: some keys work without strict referrer checking
	req.Header.Set("User-Agent", "Pixelog/1.0")
	
	// Debug the actual API key format (first 10 chars only)
	if len(apiKey) > 10 {
		fmt.Printf("OpenRouter API key starts with: %s...\n", apiKey[:10])
	} else {
		fmt.Printf("OpenRouter API key length: %d\n", len(apiKey))
	}
	
	// Debug: Print the request details (without API key)
	fmt.Printf("OpenRouter request - Model: %s, Headers: %v\n", model, map[string]string{
		"Content-Type": "application/json",
		"HTTP-Referer": "http://localhost:3000",
		"X-Title": "Pixelog",
		"Authorization": "Bearer [REDACTED]",
	})

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("OpenRouter API error: %s", string(body))
	}

	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return "", fmt.Errorf("invalid response format")
	}

	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid choice format")
	}

	message, ok := firstChoice["message"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid message format")
	}

	content, ok := message["content"].(string)
	if !ok {
		return "", fmt.Errorf("invalid content format")
	}

	return content, nil
}

func (h *Handler) callGoogleAI(model, apiKey, prompt string) (string, error) {
	// Google AI (Gemini) API implementation
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)
	
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": 2000,
			"temperature": 0.7,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Google AI API error: %s", string(body))
	}

	candidates, ok := response["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return "", fmt.Errorf("invalid response format")
	}

	firstCandidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid candidate format")
	}

	content, ok := firstCandidate["content"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid content format")
	}

	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return "", fmt.Errorf("invalid parts format")
	}

	firstPart, ok := parts[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid part format")
	}

	text, ok := firstPart["text"].(string)
	if !ok {
		return "", fmt.Errorf("invalid text format")
	}

	return text, nil
}

func (h *Handler) callAnthropic(model, apiKey, prompt string) (string, error) {
	// Anthropic (Claude) API implementation
	reqBody := map[string]interface{}{
		"model": model,
		"max_tokens": 2000,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Anthropic API error: %s", string(body))
	}

	content, ok := response["content"].([]interface{})
	if !ok || len(content) == 0 {
		return "", fmt.Errorf("invalid response format")
	}

	firstContent, ok := content[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid content format")
	}

	text, ok := firstContent["text"].(string)
	if !ok {
		return "", fmt.Errorf("invalid text format")
	}

	return text, nil
}

func (h *Handler) callOllama(model, prompt string) (string, error) {
	// Local Ollama API implementation
	reqBody := map[string]interface{}{
		"model": model,
		"prompt": prompt,
		"stream": false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "http://localhost:11434/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second} // Longer timeout for local inference
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ollama connection failed - is Ollama running on localhost:11434? Error: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Ollama API error: %s", string(body))
	}

	responseText, ok := response["response"].(string)
	if !ok {
		return "", fmt.Errorf("invalid response format")
	}

	return responseText, nil
}
