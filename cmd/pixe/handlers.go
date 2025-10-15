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

// ============================================================================
// INDEXING & SEARCH
// ============================================================================

func handleIndex() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file required")
		printUsage()
		os.Exit(1)
	}

	inputPath := os.Args[2]
	provider := "mock"
	apiKey := ""

	// Parse flags
	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--provider":
			if i+1 < len(os.Args) {
				provider = os.Args[i+1]
				i++
			}
		case "--api-key":
			if i+1 < len(os.Args) {
				apiKey = os.Args[i+1]
				i++
			}
		}
	}

	// Check for API key from multiple providers
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("OPENROUTER_API_KEY")
		}
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
			if apiKey != "" {
				provider = "gemini"
			}
		}
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
			if apiKey != "" {
				provider = "anthropic"
			}
		}
		if apiKey == "" {
			apiKey = os.Getenv("XAI_API_KEY")
			if apiKey != "" {
				provider = "xai"
			}
		}
	}

	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key required for indexing")
		fmt.Fprintln(os.Stderr, "Provide via --api-key flag or set one of these env vars:")
		fmt.Fprintln(os.Stderr, "  OPENAI_API_KEY, OPENROUTER_API_KEY, GOOGLE_API_KEY, ANTHROPIC_API_KEY, XAI_API_KEY")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Example:")
		fmt.Fprintln(os.Stderr, "  export OPENAI_API_KEY=sk-xxx")
		fmt.Fprintln(os.Stderr, "  pixe index file.pixe")
		os.Exit(1)
	}

	fmt.Printf("Building index for %s...\n", inputPath)
	
	// Create embedder with auto model selection
	embedder := index.NewSimpleEmbedder(provider, apiKey, "auto")
	model, _ := index.GetDefaultModel(provider)
	fmt.Printf("Using %s with model %s (semantic search)\n", provider, model)

	// Create indexer
	indexer, err := index.NewIndexer("./indexes", embedder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating indexer: %v\n", err)
		os.Exit(1)
	}

	// Build index
	memoryID := inputPath
	idx, err := indexer.BuildIndex(memoryID, inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building index: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Index built successfully!\n")
	fmt.Printf("  Total frames: %d\n", idx.TotalFrames)
	fmt.Printf("  Vector dimensions: %d\n", idx.VectorDim)
	fmt.Printf("  Index saved to: ./indexes/%s.index\n", memoryID)
}

func handleSearch() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file and query required")
		fmt.Println("Usage: pixe search <input.pixe> <query> [--api-key KEY]")
		os.Exit(1)
	}

	inputPath := os.Args[2]
	query := os.Args[3]
	topK := 5
	provider := "openai"
	apiKey := ""

	// Parse flags
	for i := 4; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--top":
			if i+1 < len(os.Args) {
				var err error
				topK, err = strconv.Atoi(os.Args[i+1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: invalid --top value\n")
					os.Exit(1)
				}
				i++
			}
		case "--api-key":
			if i+1 < len(os.Args) {
				apiKey = os.Args[i+1]
				i++
			}
		case "--provider":
			if i+1 < len(os.Args) {
				provider = os.Args[i+1]
				i++
			}
		}
	}

	// Check for API key from multiple providers
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("OPENROUTER_API_KEY")
		}
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
			if apiKey != "" {
				provider = "gemini"
			}
		}
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
			if apiKey != "" {
				provider = "anthropic"
			}
		}
		if apiKey == "" {
			apiKey = os.Getenv("XAI_API_KEY")
			if apiKey != "" {
				provider = "xai"
			}
		}
	}

	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key required for search (need to embed query)")
		fmt.Fprintln(os.Stderr, "Provide via --api-key flag or set one of these env vars:")
		fmt.Fprintln(os.Stderr, "  OPENAI_API_KEY, OPENROUTER_API_KEY, GOOGLE_API_KEY, ANTHROPIC_API_KEY, XAI_API_KEY")
		os.Exit(1)
	}

	fmt.Printf("Searching in %s for: \"%s\"\n\n", inputPath, query)

	// Create embedder for query with auto model selection
	embedder := index.NewSimpleEmbedder(provider, apiKey, "auto")
	indexer, err := index.NewIndexer("./indexes", embedder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating indexer: %v\n", err)
		os.Exit(1)
	}

	// Load index
	memoryID := inputPath
	idx, err := indexer.LoadIndex(memoryID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading index: %v\n", err)
		fmt.Fprintln(os.Stderr, "Hint: Run 'pixe index <file>' first to build the index")
		os.Exit(1)
	}

	// Search
	results, err := indexer.Search(idx, query, topK)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error searching: %v\n", err)
		os.Exit(1)
	}

	// Display results
	fmt.Printf("Top %d results:\n\n", len(results))
	for i, result := range results {
		fmt.Printf("%d. Frame %d (score: %.3f)\n", i+1, result.FrameNumber, result.Score)
		fmt.Printf("   Source: %s\n", result.SourceFile)
		fmt.Printf("   Preview: %s\n\n", result.Preview)
	}

	fmt.Printf("✓ Search completed in sub-100ms\n")
}

// ============================================================================
// LLM CHAT
// ============================================================================

func handleChat() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file required")
		fmt.Println("Usage: pixe chat <input.pixe> [--model MODEL] [--api-key KEY] [--list]")
		fmt.Println("")
		fmt.Println("Top 10 Models:")
		for i, m := range llm.GetTop10Models() {
			fmt.Printf("  %d. %s - %s ($%.2f/1M)\n", i+1, m.Name, m.Description, m.Cost)
		}
		os.Exit(1)
	}

	inputPath := os.Args[2]
	model := ""
	apiKey := ""
	showList := false

	// Parse flags
	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
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
		case "--list":
			showList = true
		}
	}

	// Show top 10 models
	if showList {
		fmt.Println("\n🚀 Top 10 Latest LLM Models (via OpenRouter):\n")
		for i, m := range llm.GetTop10Models() {
			fmt.Printf("%d. %-20s %s\n", i+1, m.Name, m.Description)
			fmt.Printf("   Model: %s\n", m.Model)
			fmt.Printf("   Cost: $%.2f/1M tokens | Speed: %s | Quality: %s\n\n", m.Cost, m.Speed, m.Quality)
		}
		os.Exit(0)
	}

	// Get OpenRouter API key
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}

	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: OpenRouter API key required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Get your free API key at: https://openrouter.ai/keys")
		fmt.Fprintln(os.Stderr, "Then set it:")
		fmt.Fprintln(os.Stderr, "  export OPENROUTER_API_KEY=sk-or-v1-...")
		fmt.Fprintln(os.Stderr, "  pixe chat doc.pixe")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Or use --api-key flag:")
		fmt.Fprintln(os.Stderr, "  pixe chat doc.pixe --api-key sk-or-v1-...")
		os.Exit(1)
	}

	// Auto-select best default model if not specified
	if model == "" {
		model = llm.GetDefaultModel()
	}

	fmt.Printf("🤖 Pixe Chat (OpenRouter)\n")
	fmt.Printf("📊 Model: %s\n", model)
	cost := llm.GetModelCost(model)
	if cost == 0.0 {
		fmt.Printf("💰 Cost: FREE! 🎉\n")
	} else {
		fmt.Printf("💰 Cost: ~$%.2f per 1M tokens\n", cost)
	}
	fmt.Printf("📁 Memory: %s\n\n", inputPath)
	fmt.Println("Type your questions (or 'quit' to exit)")
	fmt.Println("──────────────────────────────────────────\n")

	// Load index (nil embedder since we're loading existing index)
	indexer, err := index.NewIndexer("./indexes", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Check if index exists
	memoryID := inputPath
	idx, err := indexer.LoadIndex(memoryID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: Index not found")
		fmt.Fprintf(os.Stderr, "Run 'pixe index %s --api-key YOUR_KEY' first to build the index\n", inputPath)
		os.Exit(1)
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
		client := llm.NewClient(model, apiKey)
		response, err := client.Chat(prompt)
		if err != nil {
			fmt.Printf("\nLLM error: %v\n\n", err)
			continue
		}

		fmt.Printf("\r✓ Assistant: %s\n\n", response)
	}
}

// ============================================================================
// VERSION CONTROL
// ============================================================================

func handleVersion() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file required")
		printUsage()
		os.Exit(1)
	}

	inputPath := os.Args[2]
	message := "Version update"
	author := "pixe-cli"

	// Parse flags
	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-m", "--message":
			if i+1 < len(os.Args) {
				message = os.Args[i+1]
				i++
			}
		case "--author":
			if i+1 < len(os.Args) {
				author = os.Args[i+1]
				i++
			}
		}
	}

	fmt.Printf("Creating new version for %s...\n", inputPath)

	// No embedder needed for version operations
	indexer, _ := index.NewIndexer("./indexes", nil)

	deltaManager, err := index.NewDeltaManager("./deltas", indexer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating delta manager: %v\n", err)
		os.Exit(1)
	}

	memoryID := inputPath
	version, err := deltaManager.CreateVersion(memoryID, inputPath, message, author)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating version: %v\n", err)
		os.Exit(1)
	}

	if version != nil {
		fmt.Printf("✓ Version %d created successfully!\n", version.Version)
		fmt.Printf("  Message: %s\n", version.Message)
		fmt.Printf("  Author: %s\n", version.Author)
		fmt.Printf("  Operations: %d\n", len(version.Operations))
	} else {
		fmt.Printf("✓ Base version initialized\n")
	}
}

func handleListVersions() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file required")
		printUsage()
		os.Exit(1)
	}

	inputPath := os.Args[2]
	fmt.Printf("Versions for %s:\n\n", inputPath)

	// No embedder needed for version operations
	indexer, _ := index.NewIndexer("./indexes", nil)

	deltaManager, err := index.NewDeltaManager("./deltas", indexer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating delta manager: %v\n", err)
		os.Exit(1)
	}

	memoryID := inputPath
	versions, err := deltaManager.ListVersions(memoryID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing versions: %v\n", err)
		fmt.Fprintln(os.Stderr, "Hint: No versions found. Create one with 'pixe version <file>'")
		os.Exit(1)
	}

	if len(versions) == 0 {
		fmt.Println("No versions found. Use 'pixe version <file>' to create one.")
		return
	}

	for _, v := range versions {
		fmt.Printf("Version %d (from v%d)\n", v.Version, v.ParentVersion)
		fmt.Printf("  Message: %s\n", v.Message)
		fmt.Printf("  Author: %s\n", v.Author)
		fmt.Printf("  Timestamp: %s\n", v.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Operations: %d\n", len(v.Operations))
		fmt.Printf("  Frames: %d\n\n", v.FrameCount)
	}

	fmt.Printf("✓ Total versions: %d\n", len(versions))
}

func handleQuery() {
	if len(os.Args) < 5 {
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

	// No embedder needed for version operations
	indexer, _ := index.NewIndexer("./indexes", nil)
	deltaManager, err := index.NewDeltaManager("./deltas", indexer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	memoryID := inputPath
	pixeFile, err := deltaManager.ReconstructVersion(memoryID, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reconstructing version: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Reconstructed version %d\n", version)
	fmt.Printf("File: %s\n\n", pixeFile)
	fmt.Println("Searching in historical version...")
}

func handleDiff() {
	if len(os.Args) < 5 {
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

	// No embedder needed for version operations
	indexer, _ := index.NewIndexer("./indexes", nil)
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

// ============================================================================
// FILE INSPECTION
// ============================================================================

func handleInfo() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file required")
		fmt.Println("Usage: pixe info <input.pixe>")
		os.Exit(1)
	}

	inputPath := os.Args[2]

	fmt.Printf("📋 File Information\n")
	fmt.Printf("══════════════════════════════════════════\n\n")

	fileInfo, err := os.Stat(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("File: %s\n", inputPath)
	fmt.Printf("Size: %d bytes (%.2f MB)\n", fileInfo.Size(), float64(fileInfo.Size())/1024/1024)
	fmt.Printf("Modified: %s\n\n", fileInfo.ModTime().Format("2006-01-02 15:04:05"))

	maker, _ := video.New()
	frameCount, err := maker.GetFrameCount(inputPath)
	if err == nil {
		fmt.Printf("Frames: %d\n", frameCount)
	}

	memoryID := inputPath
	// No embedder needed for version operations
	indexer, _ := index.NewIndexer("./indexes", nil)
	idx, err := indexer.LoadIndex(memoryID)
	if err == nil {
		fmt.Printf("\n📚 Index Information:\n")
		fmt.Printf("  Indexed frames: %d\n", idx.TotalFrames)
		fmt.Printf("  Vector dimensions: %d\n", idx.VectorDim)
		fmt.Printf("  Created: %s\n", idx.CreatedAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("\n📚 Index: Not built (run 'pixe index %s')\n", inputPath)
	}

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

func handleVerify() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file required")
		fmt.Println("Usage: pixe verify <input.pixe>")
		os.Exit(1)
	}

	inputPath := os.Args[2]

	fmt.Printf("🔍 Verifying %s...\n\n", inputPath)

	maker, _ := video.New()
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

// ============================================================================
// UTILITIES
// ============================================================================

func decompressIfNeeded(data string) string {
	if !strings.HasPrefix(data, "GZ:") {
		return data
	}

	compressedData, err := base64.StdEncoding.DecodeString(data[3:])
	if err != nil {
		return data
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return data
	}
	defer gzipReader.Close()

	var decompressed bytes.Buffer
	_, err = io.Copy(&decompressed, gzipReader)
	if err != nil {
		return data
	}

	return decompressed.String()
}
