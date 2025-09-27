package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	ChunkSize int     `json:"chunk_size"`
	Quality   int     `json:"quality"`
	FrameRate float64 `json:"frame_rate"`
	Verbose   bool    `json:"verbose"`
	TempDir   string  `json:"temp_dir"`
	OutputDir string  `json:"output_dir"`
}

// Default returns a default configuration
func Default() *Config {
	tempDir, _ := os.MkdirTemp("", "pixelog-*")
	homeDir, _ := os.UserHomeDir()
	outputDir := filepath.Join(homeDir, "Documents", "Pixelog")

	return &Config{
		ChunkSize: 2800,
		Quality:   23,
		FrameRate: 0.5,
		Verbose:   false,
		TempDir:   tempDir,
		OutputDir: outputDir,
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
