package config

import (
	"testing"
)

func TestConfigValidation(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
}

func TestInvalidChunkSize(t *testing.T) {
	cfg := Default()
	cfg.ChunkSize = 5000 // Too large
	if err := cfg.Validate(); err == nil {
		t.Error("Should reject chunk size > 4000")
	}
}
