// Package main provides a CLI tool for .pixe file operations
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ArqonAi/pixe-core/pkg/converter"
)

func main() {
	var (
		input     = flag.String("input", "", "Input file path")
		output    = flag.String("output", "", "Output file path (optional)")
		extract   = flag.String("extract", "", "Extract .pixe file to specified path")
		list      = flag.String("list", "", "List contents of .pixe file")
		encrypt   = flag.String("encrypt", "", "Encryption key (optional)")
		decrypt   = flag.String("decrypt", "", "Decryption key for extraction")
		help      = flag.Bool("help", false, "Show help")
	)

	flag.Parse()

	if *help || len(os.Args) == 1 {
		printUsage()
		return
	}

	// Create converter
	conv, err := converter.New("./output")
	if err != nil {
		log.Fatalf("Failed to create converter: %v", err)
	}

	// Handle list operation
	if *list != "" {
		if err := listContents(conv, *list); err != nil {
			log.Fatalf("Failed to list contents: %v", err)
		}
		return
	}

	// Handle extract operation
	if *extract != "" && *input != "" {
		if err := extractFile(conv, *input, *extract, *decrypt); err != nil {
			log.Fatalf("Failed to extract file: %v", err)
		}
		fmt.Printf("Successfully extracted to: %s\n", *extract)
		return
	}

	// Handle convert operation
	if *input != "" {
		if err := convertFile(conv, *input, *output, *encrypt); err != nil {
			log.Fatalf("Failed to convert file: %v", err)
		}
		return
	}

	fmt.Println("Error: No valid operation specified")
	printUsage()
}

func printUsage() {
	fmt.Println("pixe - A tool for creating and managing .pixe files")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  Convert file to .pixe:")
	fmt.Println("    pixe -input file.txt [-output file.pixe] [-encrypt password]")
	fmt.Println()
	fmt.Println("  Extract .pixe file:")
	fmt.Println("    pixe -input file.pixe -extract output.txt [-decrypt password]")
	fmt.Println()
	fmt.Println("  List .pixe contents:")
	fmt.Println("    pixe -list file.pixe")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("  -input string     Input file path")
	fmt.Println("  -output string    Output file path (optional)")
	fmt.Println("  -extract string   Extract .pixe file to specified path")
	fmt.Println("  -list string      List contents of .pixe file")
	fmt.Println("  -encrypt string   Encryption key (optional)")
	fmt.Println("  -decrypt string   Decryption key for extraction")
	fmt.Println("  -help            Show this help message")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Convert a text file to .pixe")
	fmt.Println("  pixe -input document.txt")
	fmt.Println()
	fmt.Println("  # Convert with encryption")
	fmt.Println("  pixe -input document.txt -encrypt mypassword")
	fmt.Println()
	fmt.Println("  # Extract encrypted .pixe file")
	fmt.Println("  pixe -input document.pixe -extract document.txt -decrypt mypassword")
	fmt.Println()
	fmt.Println("  # List contents of .pixe file")
	fmt.Println("  pixe -list document.pixe")
}

func convertFile(conv *converter.Converter, input, output, encryptKey string) error {
	opts := &converter.ConvertOptions{
		OutputPath:    output,
		EncryptionKey: encryptKey,
	}

	outputPath, err := conv.ConvertFile(input, opts)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully converted '%s' to '%s'\n", input, outputPath)
	
	if encryptKey != "" {
		fmt.Println("File encrypted with provided key")
	}

	return nil
}

func extractFile(conv *converter.Converter, input, extract, decryptKey string) error {
	return conv.ExtractFile(input, extract, decryptKey)
}

func listContents(conv *converter.Converter, pixePath string) error {
	contents, err := conv.ListContents(pixePath)
	if err != nil {
		return err
	}

	fmt.Printf("Contents of %s:\n", filepath.Base(pixePath))
	fmt.Println()
	fmt.Printf("%-30s %-20s %-12s %-10s %-10s\n", "Name", "Type", "Size", "Encrypted", "Hash")
	fmt.Println(string(make([]byte, 90, 90))) // Separator line

	for _, item := range contents {
		encrypted := "No"
		if item.Encrypted {
			encrypted = "Yes"
		}
		
		hash := item.Hash
		if len(hash) > 8 {
			hash = hash[:8] + "..."
		}

		fmt.Printf("%-30s %-20s %-12s %-10s %-10s\n", 
			item.Name, item.Type, item.Size, encrypted, hash)
	}

	fmt.Printf("\nTotal items: %d\n", len(contents))
	return nil
}
