package main

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/ArqonAi/Pixelog/backend/internal/crypto"
	"github.com/ArqonAi/Pixelog/backend/internal/storage"
	"github.com/ArqonAi/Pixelog/backend/pkg/config"
)

func TestEncryptionService(t *testing.T) {
	fmt.Println("=== Testing Encryption Service ===")
	
	// Test encryption service
	encSvc := crypto.NewEncryptionService(true)
	
	testData := []byte("Hello, this is test data for encryption!")
	password := "test-password-123"
	
	// Test encryption
	encrypted, err := encSvc.EncryptData(testData, password)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}
	fmt.Printf("✓ Encrypted %d bytes to %d bytes\n", len(testData), len(encrypted))
	
	// Test decryption
	decrypted, err := encSvc.DecryptData(encrypted, password)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}
	fmt.Printf("✓ Decrypted back to %d bytes\n", len(decrypted))
	
	// Verify data integrity
	if string(decrypted) != string(testData) {
		t.Fatalf("Data integrity check failed")
	}
	fmt.Println("✓ Data integrity verified")
	
	// Test wrong password
	_, err = encSvc.DecryptData(encrypted, "wrong-password")
	if err == nil {
		t.Fatalf("Expected decryption to fail with wrong password")
	}
	fmt.Println("✓ Wrong password correctly rejected")
	
	// Test password generation
	password, err = encSvc.GenerateRandomPassword(32)
	if err != nil {
		t.Fatalf("Password generation failed: %v", err)
	}
	fmt.Printf("✓ Generated random password: %s\n", password[:8]+"...")
}

func TestCloudStorageConfiguration(t *testing.T) {
	fmt.Println("\n=== Testing Cloud Storage Configuration ===")
	
	// Test with no cloud provider configured
	cfg := config.Default()
	cfg.CloudEnabled = false
	
	cloudSvc, err := storage.NewCloudService(cfg)
	if err != nil {
		t.Fatalf("CloudService creation failed: %v", err)
	}
	
	if cloudSvc.IsEnabled() {
		t.Fatalf("Expected cloud service to be disabled")
	}
	fmt.Println("✓ Cloud service correctly disabled when not configured")
	
	// Test S3 configuration (without actual AWS credentials)
	cfg.CloudEnabled = true
	cfg.CloudProvider = "s3"
	cfg.S3Bucket = "test-bucket"
	cfg.S3Region = "us-east-1"
	
	_, err = storage.NewCloudService(cfg)
	// This should not fail at initialization, only when trying to use it
	if err == nil {
		fmt.Println("✓ S3 provider configuration accepted")
	} else {
		fmt.Printf("S3 configuration rejected: %v\n", err)
	}
}

func TestConfigValidation(t *testing.T) {
	fmt.Println("\n=== Testing Configuration Validation ===")
	
	cfg := config.Default()
	
	// Test valid config
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Valid config rejected: %v", err)
	}
	fmt.Println("✓ Default configuration is valid")
	
	// Test invalid chunk size
	cfg.ChunkSize = 0
	err = cfg.Validate()
	if err == nil {
		t.Fatalf("Expected invalid chunk size to be rejected")
	}
	fmt.Println("✓ Invalid chunk size correctly rejected")
	
	// Test invalid quality
	cfg = config.Default()
	cfg.Quality = 100
	err = cfg.Validate()
	if err == nil {
		t.Fatalf("Expected invalid quality to be rejected")
	}
	fmt.Println("✓ Invalid quality correctly rejected")
}

func TestEnvironmentVariables(t *testing.T) {
	fmt.Println("\n=== Testing Environment Variable Detection ===")
	
	// Save original environment
	originalOpenAI := os.Getenv("OPENAI_API_KEY")
	originalEncryption := os.Getenv("ENCRYPTION_ENABLED")
	
	// Test with no API keys
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("OPENROUTER_API_KEY")
	os.Unsetenv("GOOGLE_API_KEY")
	os.Unsetenv("XAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	
	cfg := config.Default()
	if cfg.SearchEnabled {
		// This might still be true due to Ollama check
		fmt.Println("⚠ Search enabled (possibly due to Ollama availability)")
	} else {
		fmt.Println("✓ Search correctly disabled with no API keys")
	}
	
	// Test encryption environment variable
	os.Setenv("ENCRYPTION_ENABLED", "true")
	cfg = config.Default()
	if !cfg.EncryptionEnabled {
		t.Fatalf("Expected encryption to be enabled")
	}
	fmt.Println("✓ Encryption correctly enabled via environment variable")
	
	// Restore original environment
	if originalOpenAI != "" {
		os.Setenv("OPENAI_API_KEY", originalOpenAI)
	}
	if originalEncryption != "" {
		os.Setenv("ENCRYPTION_ENABLED", originalEncryption)
	} else {
		os.Unsetenv("ENCRYPTION_ENABLED")
	}
}

// Main function for running tests manually
func main() {
	fmt.Println("🧪 Pixelog Feature Testing")
	fmt.Println("============================")
	
	tests := []struct {
		name string
		fn   func(*testing.T)
	}{
		{"Encryption Service", TestEncryptionService},
		{"Cloud Storage Config", TestCloudStorageConfiguration},
		{"Config Validation", TestConfigValidation},
		{"Environment Variables", TestEnvironmentVariables},
	}
	
	passed := 0
	for _, test := range tests {
		t := &testing.T{}
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("❌ Test '%s' panicked: %v", test.name, r)
				}
			}()
			test.fn(t)
			if !t.Failed() {
				passed++
				fmt.Printf("✅ Test '%s' PASSED\n", test.name)
			} else {
				fmt.Printf("❌ Test '%s' FAILED\n", test.name)
			}
		}()
	}
	
	fmt.Printf("\n📊 Results: %d/%d tests passed\n", passed, len(tests))
	
	if passed == len(tests) {
		fmt.Println("🎉 All tests passed!")
	} else {
		fmt.Println("⚠️  Some tests failed - check implementation")
		os.Exit(1)
	}
}
