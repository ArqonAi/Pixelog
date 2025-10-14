package main

import (
	"fmt"
	"os"

	"github.com/ArqonAi/Pixelog/internal/converter"
	"github.com/ArqonAi/Pixelog/pkg/config"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "convert":
		handleConvert()
	case "extract":
		handleExtract()
	case "index":
		handleIndex()
	case "search":
		handleSearch()
	case "version":
		handleVersion()
	case "versions":
		handleListVersions()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Pixe CLI - Convert files to .pixe format with smart indexing

Usage:
  pixe convert <input> [options]    Convert file to .pixe format
  pixe extract <input> [options]    Extract content from .pixe file
  pixe index <input>                Build vector index for fast search
  pixe search <input> <query>       Semantic search in .pixe file
  pixe version <input> [options]    Create new version with delta encoding
  pixe versions <input>             List all versions of a .pixe file
  pixe help                         Show this help message

Convert Options:
  -o, --output <file>               Output file path (default: input.pixe)
  --encrypt                         Enable encryption
  --password <password>             Password for encryption

Extract Options:
  -o, --output <dir>                Output directory (default: ./output)
  --password <password>             Password for decryption

Index Options:
  --provider <provider>             Embedding provider (openai, mock)
  --api-key <key>                   API key for embeddings

Search Options:
  --top <N>                         Return top N results (default: 5)

Version Options:
  -m, --message <message>           Version commit message
  --author <name>                   Author name

Examples:
  # Basic conversion
  pixe convert document.txt -o doc.pixe
  
  # Smart indexing for fast search
  pixe index doc.pixe
  pixe search doc.pixe "main topics"
  
  # Version control (Git for QR codes)
  pixe version doc.pixe -m "Added new section"
  pixe versions doc.pixe
  
  # Encryption
  pixe convert secret.txt -o secret.pixe --encrypt --password mypass123
  pixe extract secret.pixe -o ./extracted --password mypass123`)
}

func handleConvert() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input file required")
		printUsage()
		os.Exit(1)
	}

	inputPath := os.Args[2]
	outputPath := ""
	password := ""
	encrypt := false

	// Parse flags
	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-o", "--output":
			if i+1 < len(os.Args) {
				outputPath = os.Args[i+1]
				i++
			}
		case "--encrypt":
			encrypt = true
		case "--password":
			if i+1 < len(os.Args) {
				password = os.Args[i+1]
				i++
			}
		}
	}

	if outputPath == "" {
		outputPath = inputPath + ".pixe"
	}

	if encrypt && password == "" {
		fmt.Fprintln(os.Stderr, "Error: --password required when using --encrypt")
		os.Exit(1)
	}

	// Initialize converter
	cfg := &config.Config{
		ChunkSize: 2900,
		FrameRate: 2.0,
		Quality:   23,
		Verbose:   true,
		TempDir:   "./temp",
		OutputDir: "./output",
	}

	conv, err := converter.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing converter: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Converting %s to %s...\n", inputPath, outputPath)

	// Convert
	err = conv.Convert(inputPath, outputPath, nil, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error converting file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Successfully created %s\n", outputPath)
}

func handleExtract() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file required")
		printUsage()
		os.Exit(1)
	}

	inputPath := os.Args[2]
	outputDir := "./output"
	password := ""

	// Parse flags
	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-o", "--output":
			if i+1 < len(os.Args) {
				outputDir = os.Args[i+1]
				i++
			}
		case "--password":
			if i+1 < len(os.Args) {
				password = os.Args[i+1]
				i++
			}
		}
	}

	// Initialize converter
	cfg := &config.Config{
		ChunkSize: 2900,
		FrameRate: 2.0,
		Quality:   23,
		Verbose:   true,
		TempDir:   "./temp",
		OutputDir: outputDir,
	}

	conv, err := converter.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing converter: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Extracting %s to %s...\n", inputPath, outputDir)

	// Extract
	err = conv.Extract(inputPath, outputDir, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Successfully extracted to %s\n", outputDir)
}
