package main

import (
	"log"
	"os"

	"github.com/ArqonAi/pixe-core/pkg/converter"
)

func main() {
	// Create a test file
	testContent := []byte("Hello, this is a test file for pixe-core!")
	testFile := "test.txt"
	
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		log.Fatal("Failed to create test file:", err)
	}
	defer os.Remove(testFile)

	// Create converter
	conv, err := converter.New("./output")
	if err != nil {
		log.Fatal("Failed to create converter:", err)
	}

	// Example 1: Basic conversion
	log.Println("=== Basic Conversion ===")
	outputPath1, err := conv.ConvertFile(testFile, nil)
	if err != nil {
		log.Fatal("Failed to convert file:", err)
	}
	log.Printf("Created: %s", outputPath1)

	// Example 2: Conversion with encryption
	log.Println("\n=== Encrypted Conversion ===")
	opts := &converter.ConvertOptions{
		EncryptionKey: "mySecretPassword123",
		OutputPath:    "./output/encrypted_test.pixe",
	}

	outputPath2, err := conv.ConvertFile(testFile, opts)
	if err != nil {
		log.Fatal("Failed to convert with encryption:", err)
	}
	log.Printf("Created encrypted file: %s", outputPath2)

	// Example 3: List contents
	log.Println("\n=== List Contents ===")
	contents1, err := conv.ListContents(outputPath1)
	if err != nil {
		log.Fatal("Failed to list contents:", err)
	}

	log.Printf("Unencrypted file contents:")
	for _, item := range contents1 {
		log.Printf("  - %s (%s, %s, encrypted: %v)", 
			item.Name, item.Type, item.Size, item.Encrypted)
	}

	contents2, err := conv.ListContents(outputPath2)
	if err != nil {
		log.Fatal("Failed to list encrypted contents:", err)
	}

	log.Printf("Encrypted file contents:")
	for _, item := range contents2 {
		log.Printf("  - %s (%s, %s, encrypted: %v)", 
			item.Name, item.Type, item.Size, item.Encrypted)
	}

	// Example 4: Extract unencrypted file
	log.Println("\n=== Extract Unencrypted ===")
	extractPath1 := "./output/extracted_test.txt"
	if err := conv.ExtractFile(outputPath1, extractPath1, ""); err != nil {
		log.Fatal("Failed to extract file:", err)
	}
	log.Printf("Extracted to: %s", extractPath1)

	// Verify content
	extractedContent, err := os.ReadFile(extractPath1)
	if err != nil {
		log.Fatal("Failed to read extracted file:", err)
	}
	log.Printf("Extracted content: %s", string(extractedContent))

	// Example 5: Extract encrypted file
	log.Println("\n=== Extract Encrypted ===")
	extractPath2 := "./output/extracted_encrypted_test.txt"
	if err := conv.ExtractFile(outputPath2, extractPath2, "mySecretPassword123"); err != nil {
		log.Fatal("Failed to extract encrypted file:", err)
	}
	log.Printf("Extracted encrypted file to: %s", extractPath2)

	// Verify content
	extractedEncryptedContent, err := os.ReadFile(extractPath2)
	if err != nil {
		log.Fatal("Failed to read extracted encrypted file:", err)
	}
	log.Printf("Extracted encrypted content: %s", string(extractedEncryptedContent))

	log.Println("\n=== All examples completed successfully! ===")
}
