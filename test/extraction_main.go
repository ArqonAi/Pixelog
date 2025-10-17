package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArqonAi/Pixelog/backend/internal/converter"
	"github.com/ArqonAi/Pixelog/backend/pkg/config"
)

func TestPixeExtractionCycle(t *testing.T) {
	fmt.Println("🧪 Testing Complete .pixe Extraction Cycle")
	fmt.Println("==========================================")

	// Create temporary directories
	tempDir, err := os.MkdirTemp("", "pixelog-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outputDir := filepath.Join(tempDir, "output")
	extractDir := filepath.Join(tempDir, "extracted")
	
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract directory: %v", err)
	}

	// Create test configuration
	cfg := &config.Config{
		ChunkSize:   2800,
		Quality:     23,
		FrameRate:   0.5,
		Verbose:     true,
		TempDir:     tempDir,
		OutputDir:   outputDir,
	}

	// Initialize converter
	conv, err := converter.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create converter: %v", err)
	}
	defer conv.Cleanup()

	fmt.Println("✅ Test environment initialized")

	// Step 1: Create test input files
	testFiles := map[string]string{
		"test.txt": "Hello, this is a test file for Pixelog extraction!\nThis content should be perfectly reconstructed.",
		"data.json": `{
			"name": "Pixelog Test",
			"version": "1.0.0",
			"features": ["extraction", "qr_codes", "video_encoding"]
		}`,
	}

	var inputPaths []string
	for filename, content := range testFiles {
		inputPath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
		inputPaths = append(inputPaths, inputPath)
		fmt.Printf("✅ Created test file: %s (%d bytes)\n", filename, len(content))
	}

	// Step 2: Convert files to .pixe
	fmt.Println("\n🔄 Converting files to .pixe format...")
	
	pixePath := filepath.Join(outputDir, "test.pixe")
	progressChan := make(chan converter.Progress, 10)
	
	// Run conversion in goroutine to handle progress
	conversionDone := make(chan error, 1)
	go func() {
		conversionDone <- conv.Convert(tempDir, pixePath, progressChan)
	}()

	// Monitor progress
	for {
		select {
		case progress := <-progressChan:
			fmt.Printf("  Progress: %s - %d%% - %s\n", progress.Stage, progress.Percentage, progress.Message)
		case err := <-conversionDone:
			if err != nil {
				t.Fatalf("Conversion failed: %v", err)
			}
			fmt.Println("✅ Conversion completed")
			goto conversionComplete
		}
	}

conversionComplete:
	// Verify .pixe file was created
	if _, err := os.Stat(pixePath); os.IsNotExist(err) {
		t.Fatalf(".pixe file was not created at %s", pixePath)
	}

	stat, _ := os.Stat(pixePath)
	fmt.Printf("✅ Created .pixe file: %s (%.2f KB)\n", filepath.Base(pixePath), float64(stat.Size())/1024)

	// Step 3: Test extraction
	fmt.Println("\n🔍 Testing extraction functionality...")
	
	err = conv.Extract(pixePath, extractDir)
	if err != nil {
		t.Fatalf("Extraction failed: %v", err)
	}
	
	fmt.Println("✅ Extraction completed")

	// Step 4: Verify extracted files
	fmt.Println("\n✅ Verifying extracted files...")
	
	extractedFiles, err := os.ReadDir(extractDir)
	if err != nil {
		t.Fatalf("Failed to read extracted directory: %v", err)
	}

	if len(extractedFiles) == 0 {
		t.Fatalf("No files were extracted")
	}

	fmt.Printf("Found %d extracted files:\n", len(extractedFiles))
	
	// Verify each extracted file matches original
	for _, file := range extractedFiles {
		if file.IsDir() {
			continue
		}
		
		filename := file.Name()
		fmt.Printf("  Checking: %s\n", filename)
		
		// Read extracted content
		extractedPath := filepath.Join(extractDir, filename)
		extractedContent, err := os.ReadFile(extractedPath)
		if err != nil {
			t.Fatalf("Failed to read extracted file %s: %v", filename, err)
		}

		// Compare with original
		originalContent, exists := testFiles[filename]
		if !exists {
			t.Fatalf("Unexpected file extracted: %s", filename)
		}

		if string(extractedContent) != originalContent {
			t.Fatalf("Content mismatch for %s:\nOriginal: %q\nExtracted: %q", 
				filename, originalContent, string(extractedContent))
		}

		fmt.Printf("    ✅ Content matches original (%d bytes)\n", len(extractedContent))
	}

	// Step 5: Test ListContents functionality
	fmt.Println("\n📋 Testing ListContents functionality...")
	
	contents, err := conv.ListContents(pixePath)
	if err != nil {
		t.Fatalf("ListContents failed: %v", err)
	}

	fmt.Printf("Listed %d content items:\n", len(contents))
	for _, item := range contents {
		fmt.Printf("  - %s (%s, %s)\n", item.Name, item.Type, item.Size)
	}

	// Final verification
	fmt.Println("\n🎉 ALL TESTS PASSED!")
	fmt.Println("✅ .pixe creation works")
	fmt.Println("✅ .pixe extraction works")
	fmt.Println("✅ File content integrity preserved")
	fmt.Println("✅ ListContents functionality works")
}

// Helper function to run individual test components
func TestIndividualComponents(t *testing.T) {
	fmt.Println("\n🔧 Testing Individual Components")
	fmt.Println("=================================")

	// Test 1: QR Generation and Reading compatibility
	fmt.Println("Testing QR generation/reading compatibility...")
	
	// This would test if our QR generator output is compatible with our QR reader
	// For now, we'll skip this as it requires more complex setup
	
	fmt.Println("⏭️  Skipping QR compatibility test (requires video frames)")

	// Test 2: FFmpeg availability
	fmt.Println("Testing FFmpeg availability...")
	
	// Test if ffmpeg and ffprobe are available
	if err := testFFmpegAvailability(); err != nil {
		t.Fatalf("FFmpeg test failed: %v", err)
	}
	
	fmt.Println("✅ FFmpeg and ffprobe are available")
}

func testFFmpegAvailability() error {
	// Test ffmpeg
	if err := exec.Command("ffmpeg", "-version").Run(); err != nil {
		return fmt.Errorf("ffmpeg not available: %w", err)
	}

	// Test ffprobe  
	if err := exec.Command("ffprobe", "-version").Run(); err != nil {
		return fmt.Errorf("ffprobe not available: %w", err)
	}

	return nil
}

// Main function for running tests manually
func main() {
	fmt.Println("🧪 Pixelog Extraction Testing Suite")
	fmt.Println("====================================")

	// Run the main extraction test
	t := &testing.T{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("❌ Test panicked: %v\n", r)
				os.Exit(1)
			}
		}()
		
		TestPixeExtractionCycle(t)
		
		if t.Failed() {
			fmt.Println("❌ Tests FAILED")
			os.Exit(1)
		}
	}()

	// Run component tests
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("❌ Component test panicked: %v\n", r)
				os.Exit(1)
			}
		}()
		
		TestIndividualComponents(t)
		
		if t.Failed() {
			fmt.Println("❌ Component tests FAILED") 
			os.Exit(1)
		}
	}()

	fmt.Println("\n🎉 ALL TESTS COMPLETED SUCCESSFULLY!")
	fmt.Println("Pixelog extraction functionality is working correctly! 🚀")
}
