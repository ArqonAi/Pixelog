package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ArqonAi/Pixelog/internal/index"
	"github.com/ArqonAi/Pixelog/internal/llm"
	"github.com/ArqonAi/Pixelog/internal/memory"
	"github.com/ArqonAi/Pixelog/internal/privacy"
	search "github.com/ArqonAi/Pixelog/internal/search"
	"github.com/ArqonAi/Pixelog/internal/video"
)

// ============================================================================
// HYBRID SEARCH (BM25 + Vector with RRF)
// ============================================================================

func handleHybridSearch() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file and query required")
		fmt.Println("Usage: pixe hybrid-search <input.pixe> <query> [options]")
		os.Exit(1)
	}

	inputPath := os.Args[2]
	query := os.Args[3]
	topK := 5
	provider := "openai"
	apiKey := ""
	indexDir := "./indexes"
	jsonOutput := false
	bm25Weight := 0.4
	vectorWeight := 0.6

	for i := 4; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--top":
			if i+1 < len(os.Args) {
				topK, _ = strconv.Atoi(os.Args[i+1])
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
		case "--index-dir":
			if i+1 < len(os.Args) {
				indexDir = os.Args[i+1]
				i++
			}
		case "--bm25-weight":
			if i+1 < len(os.Args) {
				bm25Weight, _ = strconv.ParseFloat(os.Args[i+1], 64)
				i++
			}
		case "--vector-weight":
			if i+1 < len(os.Args) {
				vectorWeight, _ = strconv.ParseFloat(os.Args[i+1], 64)
				i++
			}
		case "--json":
			jsonOutput = true
		}
	}

	// Resolve API key
	if apiKey == "" {
		apiKey = resolveAPIKey(&provider)
	}

	if !jsonOutput {
		fmt.Printf("Hybrid search in %s for: \"%s\"\n", inputPath, query)
		fmt.Printf("Weights: BM25=%.1f Vector=%.1f\n\n", bm25Weight, vectorWeight)
	}

	// Load the existing index
	embedder := index.NewSimpleEmbedder(provider, apiKey, "auto")
	indexer, err := index.NewIndexer(indexDir, embedder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	memoryID := filepath.Base(inputPath)
	idx, err := indexer.LoadIndex(memoryID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading index: %v\nHint: Run 'pixe index %s' first\n", err, inputPath)
		os.Exit(1)
	}

	// Vector search
	vectorResults, err := indexer.Search(idx, query, topK*3)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Vector search error: %v\n", err)
		os.Exit(1)
	}

	// BM25 search over frame previews
	bm25Idx := newBM25FromIndex(idx)
	bm25Results := bm25Idx.Search(query, topK*3)

	// RRF fusion
	type fusedResult struct {
		FrameNumber int
		Score       float64
		BM25Score   float64
		VectorScore float64
		SourceFile  string
		Preview     string
	}

	k := 60.0
	scoreMap := make(map[int]*fusedResult)

	for rank, vr := range vectorResults {
		fr, ok := scoreMap[vr.FrameNumber]
		if !ok {
			fr = &fusedResult{
				FrameNumber: vr.FrameNumber,
				SourceFile:  vr.SourceFile,
				Preview:     vr.Preview,
			}
			scoreMap[vr.FrameNumber] = fr
		}
		fr.VectorScore = float64(vr.Score)
		fr.Score += vectorWeight * (1.0 / (k + float64(rank+1)))
	}

	for bm25Rank, br := range bm25Results {
		frameNum, _ := strconv.Atoi(br.DocID)
		fr, ok := scoreMap[frameNum]
		if !ok {
			// Find frame info from index
			preview := ""
			sourceFile := ""
			for _, f := range idx.Frames {
				if f.FrameNumber == frameNum {
					preview = f.Preview
					sourceFile = f.SourceFile
					break
				}
			}
			fr = &fusedResult{
				FrameNumber: frameNum,
				SourceFile:  sourceFile,
				Preview:     preview,
			}
			scoreMap[frameNum] = fr
		}
		fr.BM25Score = br.Score
		fr.Score += bm25Weight * (1.0 / (k + float64(bm25Rank+1)))
	}

	// Sort and limit
	var results []fusedResult
	for _, fr := range scoreMap {
		results = append(results, *fr)
	}
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Score > results[j-1].Score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
	if len(results) > topK {
		results = results[:topK]
	}

	if jsonOutput {
		jsonResults := make([]map[string]interface{}, 0, len(results))
		for _, r := range results {
			jsonResults = append(jsonResults, map[string]interface{}{
				"frameNumber": r.FrameNumber,
				"score":       r.Score,
				"bm25Score":   r.BM25Score,
				"vectorScore": r.VectorScore,
				"sourceFile":  r.SourceFile,
				"preview":     r.Preview,
			})
		}
		data, _ := json.Marshal(map[string]interface{}{
			"query": query, "total": len(results), "results": jsonResults,
		})
		fmt.Println(string(data))
	} else {
		fmt.Printf("Top %d hybrid results:\n\n", len(results))
		for i, r := range results {
			fmt.Printf("%d. Frame %d (combined: %.4f | bm25: %.3f | vector: %.3f)\n",
				i+1, r.FrameNumber, r.Score, r.BM25Score, r.VectorScore)
			fmt.Printf("   Source: %s\n", r.SourceFile)
			fmt.Printf("   Preview: %s\n\n", r.Preview)
		}
	}
}

// newBM25FromIndex builds a BM25 index from frame previews.
func newBM25FromIndex(idx *index.MemoryIndex) *search.BM25Index {
	b := search.NewBM25Index()
	for _, frame := range idx.Frames {
		b.AddDocument(strconv.Itoa(frame.FrameNumber), frame.Preview)
	}
	return b
}

// ============================================================================
// PRIVACY SCRUB
// ============================================================================

func handleScrub() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input file required")
		fmt.Println("Usage: pixe scrub <input> [--dry-run] [-o output]")
		os.Exit(1)
	}

	inputPath := os.Args[2]
	outputPath := ""
	dryRun := false

	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--dry-run":
			dryRun = true
		case "-o", "--output":
			if i+1 < len(os.Args) {
				outputPath = os.Args[i+1]
				i++
			}
		}
	}

	content, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	filter := privacy.NewFilter()

	if dryRun {
		secrets := filter.DetectedSecrets(string(content))
		if len(secrets) == 0 {
			fmt.Println("No secrets detected.")
		} else {
			fmt.Printf("Detected %d secret type(s) in %s:\n\n", len(secrets), inputPath)
			for _, s := range secrets {
				fmt.Printf("  - %s\n", s)
			}
			fmt.Println("\nRun without --dry-run to scrub them.")
		}
		return
	}

	scrubbed, count := filter.ScrubLines(string(content))

	if count == 0 {
		fmt.Println("No secrets found — file is clean.")
		return
	}

	fmt.Printf("Scrubbed %d secret(s) from %s\n", count, inputPath)

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(scrubbed), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Scrubbed output written to %s\n", outputPath)
	} else {
		fmt.Println(scrubbed)
	}
}

// ============================================================================
// RETENTION SCORING
// ============================================================================

func handleRetention() {
	dataDir := "./.pixe-data"
	indexDir := "./indexes"
	threshold := 0.15
	dryRun := false
	jsonOutput := false

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--data-dir":
			if i+1 < len(os.Args) {
				dataDir = os.Args[i+1]
				i++
			}
		case "--index-dir":
			if i+1 < len(os.Args) {
				indexDir = os.Args[i+1]
				i++
			}
		case "--threshold":
			if i+1 < len(os.Args) {
				threshold, _ = strconv.ParseFloat(os.Args[i+1], 64)
				i++
			}
		case "--dry-run":
			dryRun = true
		case "--json":
			jsonOutput = true
		}
	}

	tracker, err := memory.NewAccessTracker(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Scan indexes directory for memory entries
	entries := scanMemoryEntries(indexDir)
	if len(entries) == 0 {
		fmt.Println("No indexed memories found. Run 'pixe index <file>' first.")
		return
	}

	config := memory.DefaultDecayConfig()
	config.TierThresholds.Cold = threshold
	scorer := memory.NewRetentionScorer(config, tracker)

	scores, stats := scorer.ScoreAll(entries)

	if jsonOutput {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"scores": scores, "stats": stats,
		}, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Println("Memory Retention Scores")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Total: %d | Hot: %d | Warm: %d | Cold: %d | Evictable: %d\n\n",
		stats.Total, stats.Hot, stats.Warm, stats.Cold, stats.Evictable)

	for _, s := range scores {
		tier := s.Tier
		icon := "🟢"
		switch tier {
		case "warm":
			icon = "🟡"
		case "cold":
			icon = "🔵"
		case "evictable":
			icon = "🔴"
		}
		fmt.Printf("%s %-20s score=%.3f (salience=%.2f decay=%.2f boost=%.2f) accesses=%d [%s]\n",
			icon, s.DocID, s.Score, s.Salience, s.TemporalDecay, s.ReinforcementBoost, s.AccessCount, tier)
	}

	if dryRun && stats.Evictable > 0 {
		fmt.Printf("\n%d memories below threshold %.2f would be evicted.\n", stats.Evictable, threshold)
	}
}

func scanMemoryEntries(indexDir string) []memory.MemoryEntry {
	files, err := os.ReadDir(indexDir)
	if err != nil {
		return nil
	}

	var entries []memory.MemoryEntry
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".index" {
			continue
		}
		info, err := f.Info()
		if err != nil {
			continue
		}
		id := f.Name()[:len(f.Name())-6] // strip .index
		entries = append(entries, memory.MemoryEntry{
			ID:        id,
			Category:  "fact",
			CreatedAt: info.ModTime(),
		})
	}
	return entries
}

// ============================================================================
// LLM COMPRESSION
// ============================================================================

func handleCompress() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file required")
		fmt.Println("Usage: pixe compress <input.pixe> [--api-key KEY] [--json]")
		os.Exit(1)
	}

	inputPath := os.Args[2]
	apiKey := ""
	jsonOutput := false

	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--api-key":
			if i+1 < len(os.Args) {
				apiKey = os.Args[i+1]
				i++
			}
		case "--json":
			jsonOutput = true
		}
	}

	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}

	maker, err := video.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	frameCount, err := maker.GetFrameCount(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Build LLM chat function if API key available
	var chatFn func(string) (string, error)
	if apiKey != "" {
		model := llm.GetDefaultModel()
		client := llm.NewClient(model, apiKey)
		chatFn = client.Chat
		if !jsonOutput {
			fmt.Printf("Using LLM compression with %s\n", model)
		}
	} else if !jsonOutput {
		fmt.Println("No API key — using heuristic compression (set OPENROUTER_API_KEY for LLM)")
	}

	compressor := memory.NewLLMCompressor(chatFn)
	privacyFilter := privacy.NewFilter()

	if !jsonOutput {
		fmt.Printf("Compressing %d frames from %s...\n\n", frameCount, inputPath)
	}

	var frames []*memory.CompressedFrame
	maxFrames := frameCount
	if maxFrames > 20 {
		maxFrames = 20 // limit for reasonable API usage
	}

	for i := 0; i < maxFrames; i++ {
		chunk, err := maker.ExtractSingleFrame(inputPath, i)
		if err != nil {
			continue
		}

		data := decompressIfNeeded(chunk.Data)
		// Scrub secrets before compression
		data, _ = privacyFilter.Scrub(data)

		frameID := fmt.Sprintf("frame_%d", i)
		frame, err := compressor.CompressFrame(frameID, data, inputPath)
		if err != nil {
			continue
		}

		frames = append(frames, frame)

		if !jsonOutput {
			fmt.Printf("Frame %d:\n", i)
			fmt.Printf("  Narrative: %s\n", frame.Narrative)
			if len(frame.Facts) > 0 {
				fmt.Printf("  Facts: %d extracted\n", len(frame.Facts))
			}
			if len(frame.Concepts) > 0 {
				fmt.Printf("  Concepts: %v\n", frame.Concepts)
			}
			fmt.Println()
		}
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"file":   inputPath,
			"frames": frames,
			"total":  len(frames),
		}, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Printf("Compressed %d frames from %s\n", len(frames), inputPath)
	}
}

// ============================================================================
// MEMORY STATS
// ============================================================================

func handleMemoryStats() {
	dataDir := "./.pixe-data"
	indexDir := "./indexes"

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--data-dir":
			if i+1 < len(os.Args) {
				dataDir = os.Args[i+1]
				i++
			}
		case "--index-dir":
			if i+1 < len(os.Args) {
				indexDir = os.Args[i+1]
				i++
			}
		}
	}

	fmt.Println("Pixe Memory Statistics")
	fmt.Println("═══════════════════════════════════════")

	// Index stats
	indexFiles, _ := filepath.Glob(filepath.Join(indexDir, "*.index"))
	fmt.Printf("\nIndexed memories: %d\n", len(indexFiles))
	for _, f := range indexFiles {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		name := filepath.Base(f)
		fmt.Printf("  %s (%s, modified %s)\n", name,
			formatSize(info.Size()), info.ModTime().Format("2006-01-02 15:04"))
	}

	// Access tracking stats
	tracker, err := memory.NewAccessTracker(dataDir)
	if err != nil {
		fmt.Printf("\nAccess tracking: not initialized (run searches to generate data)\n")
		return
	}

	logs := tracker.GetAllLogs()
	if len(logs) == 0 {
		fmt.Printf("\nAccess tracking: no data yet\n")
		return
	}

	fmt.Printf("\nAccess tracking: %d documents tracked\n", len(logs))
	totalAccesses := 0
	for _, log := range logs {
		totalAccesses += log.Count
		fmt.Printf("  %s: %d accesses (last: %s)\n", log.DocID, log.Count, log.LastAt)
	}
	fmt.Printf("Total accesses: %d\n", totalAccesses)

	// Retention summary
	entries := scanMemoryEntries(indexDir)
	if len(entries) > 0 {
		scorer := memory.NewRetentionScorer(memory.DefaultDecayConfig(), tracker)
		_, stats := scorer.ScoreAll(entries)
		fmt.Printf("\nRetention tiers: Hot=%d Warm=%d Cold=%d Evictable=%d\n",
			stats.Hot, stats.Warm, stats.Cold, stats.Evictable)
	}

	fmt.Println("\n═══════════════════════════════════════")
}

// ============================================================================
// HELPERS
// ============================================================================

func resolveAPIKey(provider *string) string {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
		*provider = "gemini"
		return key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		*provider = "anthropic"
		return key
	}
	if key := os.Getenv("XAI_API_KEY"); key != "" {
		*provider = "xai"
		return key
	}
	return ""
}

