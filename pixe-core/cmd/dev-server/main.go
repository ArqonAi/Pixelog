package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// File storage structure
type FileInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Path      string    `json:"path"`
	Type      string    `json:"type"`
}

// In-memory file storage for development
var uploadedFiles []FileInfo
var fileCounter int

func main() {
	// Create output directory if it doesn't exist
	os.MkdirAll("./output", 0755)
	
	r := gin.Default()

	// Enable CORS for frontend development
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Mock API endpoints for frontend development
	api := r.Group("/api")
	{
		// Health check
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":            "ok",
				"mode":              "development",
				"search_enabled":    false,
				"encryption_enabled": false,
				"cloud_enabled":     false,
				"search_status":     "disabled",
				"cloud_status":      "disabled",
				"encryption_status": "disabled",
				"message":           "Development server - limited functionality",
			})
		})

		// Real file endpoints
		api.GET("/files", func(c *gin.Context) {
			c.JSON(http.StatusOK, uploadedFiles)
		})

		// Real file conversion endpoint
		api.POST("/convert", func(c *gin.Context) {
			form, err := c.MultipartForm()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
				return
			}

			files := form.File["files"]
			if len(files) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "No files provided"})
				return
			}

			var processedFiles []FileInfo
			for _, file := range files {
				fileCounter++
				fileID := fmt.Sprintf("file_%d", fileCounter)
				
				// Create output filename
				originalName := file.Filename
				extension := filepath.Ext(originalName)
				baseName := strings.TrimSuffix(originalName, extension)
				outputName := fmt.Sprintf("%s.pixe", baseName)
				outputPath := filepath.Join("./output", outputName)

				// Save uploaded file
				src, err := file.Open()
				if err != nil {
					continue
				}
				defer src.Close()

				dst, err := os.Create(outputPath)
				if err != nil {
					continue
				}
				defer dst.Close()

				// Copy file content (simulating conversion)
				io.Copy(dst, src)

				// Create file info
				fileInfo := FileInfo{
					ID:        fileID,
					Name:      outputName,
					Size:      file.Size,
					CreatedAt: time.Now(),
					Path:      outputPath,
					Type:      "pixe",
				}

				processedFiles = append(processedFiles, fileInfo)
				uploadedFiles = append(uploadedFiles, fileInfo)
			}

			jobID := fmt.Sprintf("job_%d", time.Now().Unix())
			c.JSON(http.StatusOK, gin.H{
				"job_id":        jobID,
				"status":        "completed",
				"message":       fmt.Sprintf("Successfully processed %d files", len(processedFiles)),
				"processed_files": processedFiles,
			})
		})

		// Mock extraction endpoint
		api.POST("/extract/:filename", func(c *gin.Context) {
			filename := c.Param("filename")
			c.JSON(http.StatusOK, gin.H{
				"message":         "Mock extraction completed",
				"extracted_files": []string{"document.txt", "data.json"},
				"output_dir":      "/tmp/mock-extraction",
				"note":            "Development mode - no actual extraction performed",
				"filename":        filename,
			})
		})

		// Mock contents endpoint
		api.GET("/contents/:filename", func(c *gin.Context) {
			filename := c.Param("filename")
			c.JSON(http.StatusOK, gin.H{
				"contents": []gin.H{
					{
						"name":       "document.txt",
						"type":       "text/plain", 
						"size":       "1.2 KB",
						"hash":       "abc123",
						"created_at": "2025-09-28T16:30:00Z",
					},
					{
						"name":       "data.json",
						"type":       "application/json",
						"size":       "856 B", 
						"hash":       "def456",
						"created_at": "2025-09-28T16:30:00Z",
					},
				},
				"filename": filename,
			})
		})

		// Real download endpoint - serves actual .pixe files
		api.GET("/files/:id", func(c *gin.Context) {
			fileID := c.Param("id")
			filePath := filepath.Join("./output", fileID+".pixe")

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
		})

		// Real delete endpoint - deletes actual files
		api.DELETE("/files/:id", func(c *gin.Context) {
			fileID := c.Param("id")
			
			// Find file in our list
			var fileToDelete *FileInfo
			var fileIndex int
			for i, file := range uploadedFiles {
				if file.ID == fileID {
					fileToDelete = &file
					fileIndex = i
					break
				}
			}

			if fileToDelete == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
				return
			}

			// Delete the actual file
			if err := os.Remove(fileToDelete.Path); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
				return
			}

			// Remove from our list
			uploadedFiles = append(uploadedFiles[:fileIndex], uploadedFiles[fileIndex+1:]...)

			c.JSON(http.StatusOK, gin.H{
				"message": "File deleted successfully",
				"file_id": fileID,
			})
		})

		// Mock search endpoints
		api.POST("/search/query", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"results": []gin.H{},
				"message": "Search disabled in development mode",
			})
		})

		api.GET("/search/documents", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"documents": []gin.H{},
				"message":   "Search disabled in development mode",
			})
		})

		// LLM Integration endpoints
		llm := api.Group("/llm")
		{
			// Process .pixe files for LLM consumption
			llm.POST("/memories", func(c *gin.Context) {
				// Mock processing multiple .pixe files
				form, _ := c.MultipartForm()
				files := form.File["files"]
				
				memories := make([]gin.H, 0)
				for i, file := range files {
					memories = append(memories, gin.H{
						"id":        fmt.Sprintf("mem_%d", i),
						"filename":  file.Filename,
						"chunks":    1000 + (i * 500), // Mock chunk count
						"size":      file.Size,
						"status":    "ready",
						"encrypted": c.PostForm("decryption_key") != "",
					})
				}

				c.JSON(http.StatusOK, gin.H{
					"memories": memories,
					"message":  "Mock processing completed - development mode",
				})
			})

			// Chat with processed memories  
			llm.POST("/chat", func(c *gin.Context) {
				var req struct {
					Query     string   `json:"query"`
					MemoryIDs []string `json:"memory_ids"`
					Provider  string   `json:"provider"`
					Model     string   `json:"model"`
				}
				
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}

				// Mock AI response with frame references
				response := gin.H{
					"content": fmt.Sprintf("Based on your .pixe memories, I found relevant information about '%s'. This content was located through frame-level analysis of QR-encoded text chunks.", req.Query),
					"sources": []gin.H{
						{"filename": "document.pdf", "frame_number": 42, "relevance": 0.94},
						{"filename": "notes.txt", "frame_number": 15, "relevance": 0.87},
					},
					"model":       req.Model,
					"provider":    req.Provider,
					"memory_ids":  req.MemoryIDs,
					"mode":        "development",
				}

				c.JSON(http.StatusOK, response)
			})

			// Search through processed memories
			llm.GET("/search", func(c *gin.Context) {
				query := c.Query("q")
				limit := c.DefaultQuery("limit", "10")
				memoryID := c.Query("memory_id")

				results := []gin.H{
					{
						"content":      fmt.Sprintf("Mock search result for '%s'", query),
						"filename":     "document.pdf", 
						"frame_number": 23,
						"relevance":    0.92,
						"chunk_id":     "chunk_123",
					},
					{
						"content":      "Another relevant text chunk found",
						"filename":     "notes.txt",
						"frame_number": 8, 
						"relevance":    0.78,
						"chunk_id":     "chunk_456",
					},
				}

				c.JSON(http.StatusOK, gin.H{
					"query":     query,
					"results":   results,
					"limit":     limit,
					"memory_id": memoryID,
					"total":     len(results),
				})
			})
		}
	}

	// Serve static files (this would normally serve the built frontend)
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Pixelog Development API Server",
			"version": "1.0.0-dev",
			"frontend": "http://localhost:5173",
			"endpoints": gin.H{
				"health":   "/api/health",
				"files":    "/api/files",
				"convert":  "/api/convert", 
				"extract":  "/api/extract/:filename",
				"contents": "/api/contents/:filename",
			},
		})
	})

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Starting Pixelog Development Server on port %s", port)
	log.Printf("📱 Frontend running at: http://localhost:5173")
	log.Printf("🔧 Backend API at: http://localhost:%s", port)
	log.Printf("❤️  Health check: http://localhost:%s/api/health", port)
	log.Println("⚠️  Development mode - limited functionality (no FFmpeg required)")

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start development server: %v", err)
	}
}
