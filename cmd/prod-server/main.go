package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/ArqonAi/Pixelog/internal/converter"
	"github.com/ArqonAi/Pixelog/pkg/config"
)

type FileInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Path      string    `json:"path"`
	Type      string    `json:"type"`
}

var uploadedFiles []FileInfo

func main() {
	// Create output directory
	os.MkdirAll("./output", 0755)
	
	// Initialize services
	cfg := &config.Config{
		ChunkSize:  2900,
		FrameRate:  2.0,
		Quality:    23,
		Verbose:    false,
		TempDir:    "./temp",
		OutputDir:  "./output",
	}
	
	conv, err := converter.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize converter: %v", err)
	}
	
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":            "ok",
				"mode":              "production",
				"search_enabled":    false,
				"encryption_enabled": true,
				"cloud_enabled":     false,
				"video_enabled":     true,
			})
		})

		api.GET("/files", func(c *gin.Context) {
			c.JSON(http.StatusOK, uploadedFiles)
		})

		api.POST("/convert", func(c *gin.Context) {
			form, err := c.MultipartForm()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
				return
			}

			files := form.File["files"]
			if len(files) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "No files provided"})
				return
			}

			var processedFiles []FileInfo
			for _, file := range files {
				// Save uploaded file temporarily
				tempPath := filepath.Join("./output", "temp_"+file.Filename)
				if err := c.SaveUploadedFile(file, tempPath); err != nil {
					continue
				}

				// Convert to .pixe
				outputName := strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename)) + ".pixe"
				outputPath := filepath.Join("./output", outputName)
				
				password := c.PostForm("password")
				err := conv.Convert(tempPath, outputPath, nil, password)
				os.Remove(tempPath)
				
				if err != nil {
					log.Printf("Conversion error: %v", err)
					continue
				}

				stat, _ := os.Stat(outputPath)
				fileInfo := FileInfo{
					ID:        strings.TrimSuffix(outputName, ".pixe"),
					Name:      outputName,
					Size:      stat.Size(),
					CreatedAt: time.Now(),
					Path:      outputPath,
					Type:      "pixe",
				}
				
				processedFiles = append(processedFiles, fileInfo)
				uploadedFiles = append(uploadedFiles, fileInfo)
			}

			c.JSON(http.StatusOK, gin.H{
				"job_id":          fmt.Sprintf("job_%d", time.Now().Unix()),
				"status":          "completed",
				"message":         fmt.Sprintf("Successfully processed %d files", len(processedFiles)),
				"processed_files": processedFiles,
			})
		})

		api.GET("/files/:id", func(c *gin.Context) {
			fileID := c.Param("id")
			filePath := filepath.Join("./output", fileID+".pixe")

			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
				return
			}

			c.Header("Content-Disposition", "attachment; filename="+fileID+".pixe")
			c.Header("Content-Type", "application/octet-stream")
			c.File(filePath)
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Production Pixelog Server on port %s", port)
	log.Printf("🎥 Video creation ENABLED with FFmpeg")
	log.Printf("🔒 Encryption ENABLED")
	log.Printf("❤️  Health: http://localhost:%s/api/health", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
