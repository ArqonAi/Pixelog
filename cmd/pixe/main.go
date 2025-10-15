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
	case "chat":
		handleChat()
	case "query":
		handleQuery()
	case "diff":
		handleDiff()
	case "info":
		handleInfo()
	case "verify":
		handleVerify()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func printUsage() {
	fmt.Println(`Pixe CLI - Convert files to .pixe format with smart indexing

Basic Commands:
  pixe convert <input> [options]    Convert file to .pixe format
    --stream                        Use streaming mode for large files (constant memory)
  pixe extract <input> [options]    Extract content from .pixe file
  pixe info <input>                 Show detailed file information
  pixe verify <input>               Verify file integrity

Smart Indexing:
  pixe index <input>                Build vector index for fast search
  pixe search <input> <query>       Semantic search in .pixe file
  pixe chat <input> [options]       Interactive LLM chat with memory

Version Control (Git for QR codes):
  pixe version <input> [options]    Create new version with delta encoding
  pixe versions <input>             List all versions of a .pixe file
  pixe query <input> <v> <query>    Time-travel query at specific version
  pixe diff <input> <from> <to>     Show differences between versions

Help:
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
  # Basic usage
  pixe convert document.txt -o doc.pixe
  pixe info doc.pixe
  pixe verify doc.pixe
  
  # Smart indexing and search
  pixe index doc.pixe
  pixe search doc.pixe "main topics" --top 5
  
  # Interactive LLM chat
  pixe chat doc.pixe --api-key sk-xxx
  export OPENROUTER_API_KEY=sk-xxx
  pixe chat doc.pixe --provider openrouter --model deepseek/deepseek-chat
  
  # Version control (Git for QR codes)
  pixe version doc.pixe -m "Added new section" --author "user"
  pixe versions doc.pixe
  pixe diff doc.pixe 1 2
  pixe query doc.pixe 1 "what was in version 1?"
  
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
	useStreaming := false

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
		case "--stream":
			useStreaming = true
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

	// Check file size to auto-enable streaming
	fileInfo, err := os.Stat(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Auto-enable streaming for files > 100MB
	if !useStreaming && fileInfo.Size() > 100*1024*1024 {
		fmt.Printf("🔄 File size %s detected - auto-enabling streaming mode\n", formatSize(fileInfo.Size()))
		useStreaming = true
	}

	if useStreaming {
		fmt.Printf("📦 Converting %s to %s (streaming mode)...\n", inputPath, outputPath)
		
		// Use streaming processor
		streamer := converter.NewStreamingProcessor(conv)
		
		// Set progress callback
		streamer.SetProgressCallback(func(bytesProcessed, totalBytes int64, currentChunk, totalChunks int) {
			percent := float64(bytesProcessed) / float64(totalBytes) * 100
			fmt.Printf("\r🔄 Progress: %.1f%% (%s / %s) - Chunk %d/%d", 
				percent, formatSize(bytesProcessed), formatSize(totalBytes), currentChunk, totalChunks)
		})
		
		err = streamer.StreamToVideo(inputPath, outputPath, password)
		fmt.Println() // New line after progress
	} else {
		fmt.Printf("Converting %s to %s...\n", inputPath, outputPath)
		
		// Standard conversion
		err = conv.Convert(inputPath, outputPath, nil, password)
	}
	
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
