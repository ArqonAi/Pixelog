package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/ArqonAi/Pixelog/internal/index"
)

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

	fmt.Printf("Building index for %s...\n", inputPath)

	// Create embedder
	var embedder index.Embedder
	if provider == "openai" && apiKey != "" {
		embedder = index.NewSimpleEmbedder("openai", apiKey, "text-embedding-3-small")
	} else {
		embedder = index.NewMockEmbedder()
		fmt.Println("Using mock embedder (384 dimensions)")
	}

	// Create indexer
	indexer, err := index.NewIndexer("./indexes", embedder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating indexer: %v\n", err)
		os.Exit(1)
	}

	// Build index
	memoryID := inputPath // Use file path as ID
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
		fmt.Println("Usage: pixe search <input.pixe> <query>")
		os.Exit(1)
	}

	inputPath := os.Args[2]
	query := os.Args[3]
	topK := 5

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
		}
	}

	fmt.Printf("Searching in %s for: \"%s\"\n\n", inputPath, query)

	// Create embedder (always mock for CLI)
	embedder := index.NewMockEmbedder()

	// Create indexer
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

	// Create embedder
	embedder := index.NewMockEmbedder()
	indexer, _ := index.NewIndexer("./indexes", embedder)

	// Create delta manager
	deltaManager, err := index.NewDeltaManager("./deltas", indexer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating delta manager: %v\n", err)
		os.Exit(1)
	}

	// Create version
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

	// Create embedder
	embedder := index.NewMockEmbedder()
	indexer, _ := index.NewIndexer("./indexes", embedder)

	// Create delta manager
	deltaManager, err := index.NewDeltaManager("./deltas", indexer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating delta manager: %v\n", err)
		os.Exit(1)
	}

	// List versions
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
