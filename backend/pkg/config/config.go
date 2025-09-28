package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the application configuration
type Config struct {
	ChunkSize           int     `json:"chunk_size"`
	Quality             int     `json:"quality"`
	FrameRate           float64 `json:"frame_rate"`
	Verbose             bool    `json:"verbose"`
	TempDir             string  `json:"temp_dir"`
	OutputDir           string  `json:"output_dir"`
	
	// AI Provider Configuration
	EmbeddingProvider   string  `json:"embedding_provider"`
	OpenAIAPIKey        string  `json:"openai_api_key"`
	OpenRouterAPIKey    string  `json:"openrouter_api_key"`
	OpenRouterModel     string  `json:"openrouter_model"`
	GoogleAPIKey        string  `json:"google_api_key"`
	XAIAPIKey           string  `json:"xai_api_key"`
	OllamaBaseURL       string  `json:"ollama_base_url"`
	OllamaModel         string  `json:"ollama_model"`
	SearchEnabled       bool    `json:"search_enabled"`
	
	// Encryption Configuration
	EncryptionEnabled   bool    `json:"encryption_enabled"`
	DefaultPassword     string  `json:"default_password"`
	
	// Cloud Storage Configuration
	CloudEnabled        bool    `json:"cloud_enabled"`
	CloudProvider       string  `json:"cloud_provider"`
	
	// AWS S3
	S3Bucket           string  `json:"s3_bucket"`
	S3Region           string  `json:"s3_region"`
	S3AccessKey        string  `json:"s3_access_key"`
	S3SecretKey        string  `json:"s3_secret_key"`
	S3Endpoint         string  `json:"s3_endpoint"`
	
	// Google Cloud Storage
	GCSBucket          string  `json:"gcs_bucket"`
	GCSProjectID       string  `json:"gcs_project_id"`
	GCSCredentials     string  `json:"gcs_credentials"`
	
	// Azure Blob Storage
	AzureAccount       string  `json:"azure_account"`
	AzureContainer     string  `json:"azure_container"`
	AzureKey           string  `json:"azure_key"`
}

// Default returns a default configuration
func Default() *Config {
	tempDir, _ := os.MkdirTemp("", "pixelog-*")
	homeDir, _ := os.UserHomeDir()
	outputDir := filepath.Join(homeDir, "Documents", "Pixelog")

	return &Config{
		ChunkSize:         2800,
		Quality:           23,
		FrameRate:         0.5,
		Verbose:           false,
		TempDir:           tempDir,
		OutputDir:         outputDir,
		
		// AI Provider Configuration
		EmbeddingProvider: getEnvOrDefault("EMBEDDING_PROVIDER", "auto"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		OpenRouterAPIKey:  os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:   getEnvOrDefault("OPENROUTER_MODEL", "text-embedding-3-small"),
		GoogleAPIKey:      os.Getenv("GOOGLE_API_KEY"),
		XAIAPIKey:         os.Getenv("XAI_API_KEY"),
		OllamaBaseURL:     getEnvOrDefault("OLLAMA_BASE_URL", "http://localhost:11434"),
		OllamaModel:       getEnvOrDefault("OLLAMA_MODEL", "nomic-embed-text"),
		SearchEnabled:     hasAnyEmbeddingProvider(),
		
		// Encryption Configuration
		EncryptionEnabled: getBoolEnv("ENCRYPTION_ENABLED"),
		DefaultPassword:   os.Getenv("DEFAULT_PASSWORD"),
		
		// Cloud Storage Configuration
		CloudEnabled:      hasCloudProvider(),
		CloudProvider:     getEnvOrDefault("CLOUD_PROVIDER", "s3"),
		
		// AWS S3
		S3Bucket:          os.Getenv("S3_BUCKET"),
		S3Region:          getEnvOrDefault("S3_REGION", "us-east-1"),
		S3AccessKey:       os.Getenv("AWS_ACCESS_KEY_ID"),
		S3SecretKey:       os.Getenv("AWS_SECRET_ACCESS_KEY"),
		S3Endpoint:        os.Getenv("S3_ENDPOINT"),
		
		// Google Cloud Storage
		GCSBucket:         os.Getenv("GCS_BUCKET"),
		GCSProjectID:      os.Getenv("GOOGLE_PROJECT_ID"),
		GCSCredentials:    os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		
		// Azure Blob Storage
		AzureAccount:      os.Getenv("AZURE_STORAGE_ACCOUNT"),
		AzureContainer:    getEnvOrDefault("AZURE_CONTAINER", "pixelog"),
		AzureKey:          os.Getenv("AZURE_STORAGE_KEY"),
	}
}

// Validate ensures the configuration is valid
func (c *Config) Validate() error {
	if c.ChunkSize <= 0 || c.ChunkSize > 4000 {
		return fmt.Errorf("chunk size must be between 1 and 4000 bytes")
	}

	if c.Quality < 0 || c.Quality > 51 {
		return fmt.Errorf("quality must be between 0 and 51")
	}

	if c.FrameRate <= 0 || c.FrameRate > 60 {
		return fmt.Errorf("frame rate must be between 0.1 and 60 FPS")
	}

	if c.TempDir == "" {
		tempDir, err := os.MkdirTemp("", "pixelog-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		c.TempDir = tempDir
	}

	if c.OutputDir == "" {
		c.OutputDir = "./output"
	}

	// Ensure directories exist
	if err := os.MkdirAll(c.TempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	if err := os.MkdirAll(c.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	return nil
}

// Cleanup removes temporary directories
func (c *Config) Cleanup() {
	if c.TempDir != "" {
		os.RemoveAll(c.TempDir)
	}
}

// Helper functions
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func hasAnyEmbeddingProvider() bool {
	return os.Getenv("OPENAI_API_KEY") != "" ||
		os.Getenv("OPENROUTER_API_KEY") != "" ||
		os.Getenv("GOOGLE_API_KEY") != "" ||
		os.Getenv("XAI_API_KEY") != "" ||
		checkOllamaAvailability()
}

func checkOllamaAvailability() bool {
	// Simple check - could be enhanced with actual HTTP request
	return true // Assume ollama is available for now
}

func getBoolEnv(key string) bool {
	value := strings.ToLower(os.Getenv(key))
	return value == "true" || value == "1" || value == "yes"
}

func hasCloudProvider() bool {
	return os.Getenv("S3_BUCKET") != "" ||
		os.Getenv("GCS_BUCKET") != "" ||
		os.Getenv("AZURE_STORAGE_ACCOUNT") != ""
}
