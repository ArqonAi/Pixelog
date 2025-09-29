package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Development server without converter dependencies
	// This allows frontend development without needing FFmpeg
	
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

		// Mock file endpoints
		api.GET("/files", func(c *gin.Context) {
			c.JSON(http.StatusOK, []gin.H{
				{
					"id":         "sample1",
					"name":       "sample1.pixe",
					"size":       "2.4 MB",
					"created_at": "2025-09-28T16:33:00Z",
					"path":       "./output/sample1.pixe",
				},
				{
					"id":         "sample2",
					"name":       "sample2.pixe", 
					"size":       "1.8 MB",
					"created_at": "2025-09-28T16:30:00Z",
					"path":       "./output/sample2.pixe",
				},
			})
		})

		// Mock conversion endpoint
		api.POST("/convert", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"job_id":  "dev_job_123",
				"status":  "completed",
				"message": "Mock conversion completed (development mode)",
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

		// Real delete endpoint - actually deletes .pixe files
		api.DELETE("/files/:id", func(c *gin.Context) {
			fileID := c.Param("id")
			filePath := filepath.Join("./output", fileID+".pixe")

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
		})

		// Mock search endpoints
		search := api.Group("/search")
		{
			search.POST("/query", func(c *gin.Context) {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"error": "Search not available in development mode",
				})
			})

			search.GET("/documents", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"documents": []gin.H{},
					"message":   "No documents in development mode",
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
