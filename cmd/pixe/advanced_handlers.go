package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ArqonAi/Pixelog/internal/index"
	"github.com/ArqonAi/Pixelog/internal/llm"
	"github.com/ArqonAi/Pixelog/internal/video"
)

// handleChat - Interactive LLM chat using .pixe files as memory
func handleChat() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file required")
		fmt.Println("Usage: pixe chat <input.pixe> [--provider openrouter] [--model MODEL] [--api-key KEY]")
		os.Exit(1)
	}

	inputPath := os.Args[2]
	provider := "openrouter"
	model := "deepseek/deepseek-chat"
	apiKey := ""

	// Parse flags
	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--provider":
			if i+1 < len(os.Args) {
				provider = os.Args[i+1]
				i++
			}
		case "--model":
			if i+1 < len(os.Args) {
				model = os.Args[i+1]
				i++
			}
		case "--api-key":
			if i+1 < len(os.Args) {
				apiKey = os.Args[i+1]
				i++
			}
		}
	}

	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
	}

	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key required")
		fmt.Fprintln(os.Stderr, "Provide via --api-key flag or OPENROUTER_API_KEY/OPENAI_API_KEY env var")
		os.Exit(1)
	}

	fmt.Printf("🤖 Pixe Chat - Using %s with %s\n", provider, model)
	fmt.Printf("📁 Memory: %s\n\n", inputPath)
	fmt.Println("Type your questions (or 'quit' to exit)")
	fmt.Println("──────────────────────────────────────────\n")

	// Load or build index
	embedder := index.NewMockEmbedder()
	indexer, err := index.NewIndexer("./indexes", embedder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Check if index exists, build if not
	memoryID := inputPath
	idx, err := indexer.LoadIndex(memoryID)
	if err != nil {
		fmt.Println("Building index first (one-time operation)...")
		idx, err = indexer.BuildIndex(memoryID, inputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error building index: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Index built\n")
	}

	// Interactive chat loop
	scanner := bufio.NewScanner(os.Stdin)
	maker, _ := video.New()

	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}

		query := strings.TrimSpace(scanner.Text())
		if query == "" {
			continue
		}
		if query == "quit" || query == "exit" {
			fmt.Println("Goodbye!")
			break
		}

		// Search for relevant context
		results, err := indexer.Search(idx, query, 3)
		if err != nil {
			fmt.Printf("Search error: %v\n", err)
			continue
		}

		// Extract relevant frames
		var contexts []string
		for _, res := range results {
			chunk, err := maker.ExtractSingleFrame(inputPath, res.FrameNumber)
			if err == nil && chunk != nil {
				// Decompress if needed
				data := decompressIfNeeded(chunk.Data)
				contexts = append(contexts, data)
			}
		}

		if len(contexts) == 0 {
			fmt.Println("Assistant: No relevant context found.")
			continue
		}

		// Build prompt
		contextStr := strings.Join(contexts, "\n\n---\n\n")
		prompt := fmt.Sprintf("Use the following context to answer the question.\n\nContext:\n%s\n\nQuestion: %s\n\nAnswer:", contextStr, query)

		// Call LLM API
		fmt.Printf("\n🤔 Thinking...")
		client := llm.NewClient(provider, model, apiKey)
		response, err := client.Chat(prompt)
		if err != nil {
			fmt.Printf("\nLLM error: %v\n\n", err)
			continue
		}

		fmt.Printf("\r✓ Assistant: %s\n\n", response)
	}
}

// handleQuery - Query at specific version (time-travel)
func handleQuery() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Error: input file, version, and query required")
		fmt.Println("Usage: pixe query <input.pixe> <version> <query>")
		os.Exit(1)
	}

	inputPath := os.Args[2]
	versionStr := os.Args[3]
	query := os.Args[4]

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid version number\n")
		os.Exit(1)
	}

	fmt.Printf("🕰️  Time-travel query at version %d\n", version)
	fmt.Printf("File: %s\n", inputPath)
	fmt.Printf("Query: %s\n\n", query)

	// Load version history
	embedder := index.NewMockEmbedder()
	indexer, _ := index.NewIndexer("./indexes", embedder)
	deltaManager, err := index.NewDeltaManager("./deltas", indexer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Reconstruct version
	memoryID := inputPath
	pixeFile, err := deltaManager.ReconstructVersion(memoryID, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reconstructing version: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Reconstructed version %d\n", version)
	fmt.Printf("File: %s\n\n", pixeFile)

	// Now search in that version
	fmt.Println("Searching in historical version...")
	// TODO: Load index for that version and search
}

// handleDiff - Show differences between versions
func handleDiff() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Error: input file and version range required")
		fmt.Println("Usage: pixe diff <input.pixe> <from-version> <to-version>")
		os.Exit(1)
	}

	inputPath := os.Args[2]
	fromStr := os.Args[3]
	toStr := os.Args[4]

	fromVer, _ := strconv.Atoi(fromStr)
	toVer, _ := strconv.Atoi(toStr)

	fmt.Printf("📊 Diff: v%d → v%d\n", fromVer, toVer)
	fmt.Printf("File: %s\n\n", inputPath)

	embedder := index.NewMockEmbedder()
	indexer, _ := index.NewIndexer("./indexes", embedder)
	deltaManager, _ := index.NewDeltaManager("./deltas", indexer)

	memoryID := inputPath
	diff, err := deltaManager.GetVersionDiff(memoryID, fromVer, toVer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(diff) == 0 {
		fmt.Println("No differences found.")
		return
	}

	fmt.Printf("Changes: %d operations\n\n", len(diff))
	for i, op := range diff {
		fmt.Printf("%d. %s at frame %d\n", i+1, op.Type, op.FrameIndex)
		if op.OldHash != "" {
			fmt.Printf("   Old: %s\n", op.OldHash[:16])
		}
		if op.NewHash != "" {
			fmt.Printf("   New: %s\n", op.NewHash[:16])
		}
		fmt.Println()
	}
}

// handleInfo - Show detailed .pixe file information
func handleInfo() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file required")
		fmt.Println("Usage: pixe info <input.pixe>")
		os.Exit(1)
	}

	inputPath := os.Args[2]

	fmt.Printf("📋 File Information\n")
	fmt.Printf("══════════════════════════════════════════\n\n")

	// File stats
	fileInfo, err := os.Stat(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("File: %s\n", inputPath)
	fmt.Printf("Size: %d bytes (%.2f MB)\n", fileInfo.Size(), float64(fileInfo.Size())/1024/1024)
	fmt.Printf("Modified: %s\n\n", fileInfo.ModTime().Format("2006-01-02 15:04:05"))

	// Get frame count
	maker, _ := video.New()
	frameCount, err := maker.GetFrameCount(inputPath)
	if err == nil {
		fmt.Printf("Frames: %d\n", frameCount)
	}

	// Check if index exists
	memoryID := inputPath
	embedder := index.NewMockEmbedder()
	indexer, _ := index.NewIndexer("./indexes", embedder)
	idx, err := indexer.LoadIndex(memoryID)
	if err == nil {
		fmt.Printf("\n📚 Index Information:\n")
		fmt.Printf("  Indexed frames: %d\n", idx.TotalFrames)
		fmt.Printf("  Vector dimensions: %d\n", idx.VectorDim)
		fmt.Printf("  Created: %s\n", idx.CreatedAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("\n📚 Index: Not built (run 'pixe index %s')\n", inputPath)
	}

	// Check version history
	deltaManager, _ := index.NewDeltaManager("./deltas", indexer)
	versions, err := deltaManager.ListVersions(memoryID)
	if err == nil && len(versions) > 0 {
		fmt.Printf("\n🕰️  Version History:\n")
		fmt.Printf("  Total versions: %d\n", len(versions))
		fmt.Printf("  Latest: v%d (%s)\n", versions[len(versions)-1].Version, 
			versions[len(versions)-1].Message)
	} else {
		fmt.Printf("\n🕰️  Versions: None (run 'pixe version %s -m \"message\"')\n", inputPath)
	}

	fmt.Printf("\n══════════════════════════════════════════\n")
}

// handleVerify - Verify .pixe file integrity
func handleVerify() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file required")
		fmt.Println("Usage: pixe verify <input.pixe>")
		os.Exit(1)
	}

	inputPath := os.Args[2]

	fmt.Printf("🔍 Verifying %s...\n\n", inputPath)

	maker, _ := video.New()

	// Get frame count
	frameCount, err := maker.GetFrameCount(inputPath)
	if err != nil {
		fmt.Printf("❌ Error reading video: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Total frames: %d\n", frameCount)
	fmt.Println("Decoding all QR codes...")

	successCount := 0
	failCount := 0

	for i := 0; i < frameCount; i++ {
		_, err := maker.ExtractSingleFrame(inputPath, i)
		if err == nil {
			successCount++
		} else {
			failCount++
			if failCount <= 5 {
				fmt.Printf("  ❌ Frame %d failed: %v\n", i, err)
			}
		}

		if (i+1)%10 == 0 || i == frameCount-1 {
			fmt.Printf("\rProgress: %d/%d frames (%.1f%% success)", 
				i+1, frameCount, float64(successCount)/float64(i+1)*100)
		}
	}

	fmt.Printf("\n\n")

	if failCount == 0 {
		fmt.Printf("✅ All %d frames verified successfully!\n", frameCount)
		fmt.Println("File integrity: GOOD")
	} else {
		fmt.Printf("⚠️  %d/%d frames failed verification\n", failCount, frameCount)
		fmt.Println("File integrity: DEGRADED")
	}
}

func decompressIfNeeded(data string) string {
	// Check if data is GZIP-compressed
	if !strings.HasPrefix(data, "GZ:") {
		return data // Already decompressed
	}

	// Remove GZ: prefix and decode base64
	compressedData, err := base64.StdEncoding.DecodeString(data[3:])
	if err != nil {
		fmt.Printf("Warning: Failed to decode base64: %v\n", err)
		return data
	}

	// GZIP decompress
	gzipReader, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		fmt.Printf("Warning: Failed to create gzip reader: %v\n", err)
		return data
	}
	defer gzipReader.Close()

	var decompressed bytes.Buffer
	_, err = io.Copy(&decompressed, gzipReader)
	if err != nil {
		fmt.Printf("Warning: Failed to decompress: %v\n", err)
		return data
	}

	return decompressed.String()
}
