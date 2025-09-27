package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/pixelog/pixelog-go-v1/backend/internal/converter"
)

type Handler struct {
	converter *converter.Converter
	upgrader  *websocket.Upgrader
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

func NewHandler(conv *converter.Converter, upgrader *websocket.Upgrader) *Handler {
	return &Handler{
		converter: conv,
		upgrader:  upgrader,
	}
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
	filePath := filepath.Join("./output", fileID+".pixe")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.File(filePath)
}

func (h *Handler) DeletePixeFile(c *gin.Context) {
	fileID := c.Param("id")
	filePath := filepath.Join("./output", fileID+".pixe")

	if err := os.Remove(filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}

func (h *Handler) SearchContent(c *gin.Context) {
	// Implementation for content search
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Search not yet implemented"})
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
