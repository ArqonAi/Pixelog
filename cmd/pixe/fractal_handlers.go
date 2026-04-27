// fractal_handlers.go — CLI handlers for the fractal-memory commands.
//
// `pixe compact` folds existing era capsules at level L-1 into a single
// parent capsule at level L. It is the manual / one-shot equivalent of
// what the running FractalService does when its circadian scheduler
// fires; it is the right tool for offline batch jobs and for catching
// up after a long downtime.
//
// `pixe recall` runs DeepRetrieve over the era graph rooted at the
// highest-level capsules in the store, returning ranked hits. It is
// the read side of the same memory hierarchy.
//
// Both commands operate on a CapsuleStore directory layout — the same
// one the embedded ArchivalPipeline writes — so they compose with
// every other library entry point without a separate persistence
// model.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ArqonAi/Pixelog/internal/memory"
)

// ============================================================================
// COMPACT
// ============================================================================

func handleCompact() {
	dataDir := "./.pixe-data"
	namespace := "default"
	levelStr := "day"
	jsonOut := false

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--data-dir":
			if i+1 < len(os.Args) {
				dataDir = os.Args[i+1]
				i++
			}
		case "--namespace", "--ns":
			if i+1 < len(os.Args) {
				namespace = os.Args[i+1]
				i++
			}
		case "--level":
			if i+1 < len(os.Args) {
				levelStr = os.Args[i+1]
				i++
			}
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Println(`Usage: pixe compact --data-dir DIR --namespace NS --level LEVEL [--json]

Folds every era capsule at level (LEVEL - 1) inside the given
namespace into a single new capsule at LEVEL, and persists the
result back to the same store.

Levels (low → high): session, day, week, month, quarter, year, decade.
LEVEL must be > session; the source level is one rank below.

Examples:
  pixe compact --data-dir ./.pixe-data --namespace agent-1 --level day
  pixe compact --data-dir ./.pixe-data --namespace agent-1 --level week
  pixe compact --data-dir ./.pixe-data --ns agent-1 --level month --json`)
			return
		}
	}

	target, err := memory.ParseEraLevel(levelStr)
	if err != nil {
		fatalf("invalid --level %q: %v", levelStr, err)
	}
	if target <= memory.LevelSession {
		fatalf("--level must be > session (got %s); compact folds children at level-1 into level", target)
	}
	source := target - 1

	store, err := memory.NewCapsuleStore(filepath.Join(dataDir, "capsules"))
	if err != nil {
		fatalf("open capsule store: %v", err)
	}

	indices := store.ListByLevel(namespace, source)
	if len(indices) == 0 {
		fatalf("no capsules at level=%s in namespace=%q under %s", source, namespace, dataDir)
	}

	ctx := context.Background()
	refs := make([]memory.ChildRef, 0, len(indices))
	for _, idx := range indices {
		era, err := store.GetEra(ctx, idx.Hash)
		if err != nil {
			fatalf("load %s: %v", idx.Hash, err)
		}
		refs = append(refs, memory.RefFromEra(era))
	}

	// Heuristic L0 / L1 fallback (no LLM); access tracker optional.
	tracker, err := memory.NewAccessTracker(filepath.Join(dataDir, "access"))
	if err != nil {
		fatalf("open access tracker: %v", err)
	}
	compactor := memory.NewCompactor(memory.DefaultCompactionConfig(), tracker, nil)

	era, err := compactor.Compact(ctx, namespace, target, refs)
	if err != nil {
		fatalf("compact: %v", err)
	}
	if _, err := store.PutEra(era); err != nil {
		fatalf("persist era: %v", err)
	}
	// Best-effort flush; access logs are non-critical for the CLI path.
	_ = tracker.Persist()

	type result struct {
		URI       string `json:"uri"`
		Hash      string `json:"hash"`
		Level     string `json:"level"`
		Namespace string `json:"namespace"`
		Children  int    `json:"children"`
		L0        string `json:"l0"`
	}
	r := result{
		URI:       era.URI(),
		Hash:      era.Hash,
		Level:     era.Level.String(),
		Namespace: era.Namespace,
		Children:  len(era.Children),
		L0:        era.L0,
	}
	if jsonOut {
		_ = json.NewEncoder(os.Stdout).Encode(r)
		return
	}
	fmt.Printf("Compacted %d %s capsule(s) → 1 %s capsule\n", r.Children, source, target)
	fmt.Printf("  URI:       %s\n", r.URI)
	fmt.Printf("  Hash:      %s\n", r.Hash)
	fmt.Printf("  Namespace: %s\n", r.Namespace)
	if r.L0 != "" {
		fmt.Printf("  L0:        %s\n", r.L0)
	}
}

// ============================================================================
// RECALL  (DeepRetrieve over the era graph)
// ============================================================================

func handleRecall() {
	dataDir := "./.pixe-data"
	namespace := "default"
	query := ""
	maxDepth := 3
	maxResults := 5
	threshold := 0.05
	surfaceOnly := false
	jsonOut := false

	// First positional after `recall` is the query, unless it starts with --
	for i := 2; i < len(os.Args); i++ {
		a := os.Args[i]
		switch a {
		case "--data-dir":
			if i+1 < len(os.Args) {
				dataDir = os.Args[i+1]
				i++
			}
		case "--namespace", "--ns":
			if i+1 < len(os.Args) {
				namespace = os.Args[i+1]
				i++
			}
		case "--depth":
			if i+1 < len(os.Args) {
				maxDepth, _ = strconv.Atoi(os.Args[i+1])
				i++
			}
		case "--top":
			if i+1 < len(os.Args) {
				maxResults, _ = strconv.Atoi(os.Args[i+1])
				i++
			}
		case "--threshold":
			if i+1 < len(os.Args) {
				threshold, _ = strconv.ParseFloat(os.Args[i+1], 64)
				i++
			}
		case "--surface-only":
			surfaceOnly = true
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Println(`Usage: pixe recall <query> [options]

Runs DeepRetrieve over the era graph in --data-dir, returning ranked
hits across every level (year → month → ... → session).

Options:
  --data-dir DIR        capsule store root (default: ./.pixe-data)
  --namespace NS        agent / tenant identifier (default: default)
  --depth N             max URI-graph traversal depth (default: 3)
  --top N               cap result list (default: 5)
  --threshold F         minimum match score (default: 0.05)
  --surface-only        skip buried children (conscious recall)
  --json                emit JSON array on stdout

Examples:
  pixe recall "quantum entanglement" --data-dir ./.pixe-data
  pixe recall "trip to Lisbon" --ns agent-1 --depth 5 --top 10
  pixe recall "battery tips" --surface-only --json`)
			return
		default:
			if query == "" && len(a) > 0 && a[0] != '-' {
				query = a
			}
		}
	}

	if query == "" {
		fmt.Fprintln(os.Stderr, "Error: query required")
		fmt.Println("Usage: pixe recall <query> [--data-dir DIR] [--ns NS] [--depth N] [--top N] [--surface-only] [--json]")
		os.Exit(1)
	}

	store, err := memory.NewCapsuleStore(filepath.Join(dataDir, "capsules"))
	if err != nil {
		fatalf("open capsule store: %v", err)
	}

	// Pick the highest level present as the traversal root.
	var rootLevel memory.EraLevel = memory.LevelSession
	var rootIdx []memory.CapsuleIndex
	for _, lvl := range []memory.EraLevel{
		memory.LevelDecade, memory.LevelYear, memory.LevelQuarter,
		memory.LevelMonth, memory.LevelWeek, memory.LevelDay, memory.LevelSession,
	} {
		if idx := store.ListByLevel(namespace, lvl); len(idx) > 0 {
			rootIdx = idx
			rootLevel = lvl
			break
		}
	}
	if len(rootIdx) == 0 {
		fatalf("no capsules in namespace=%q under %s", namespace, dataDir)
	}

	ctx := context.Background()
	roots := make([]*memory.EraCapsule, 0, len(rootIdx))
	for _, ci := range rootIdx {
		era, err := store.GetEra(ctx, ci.Hash)
		if err != nil {
			fatalf("load %s: %v", ci.Hash, err)
		}
		roots = append(roots, era)
	}

	resolver := memory.NewCapsuleResolver(store)
	results, err := memory.DeepRetrieve(ctx, resolver, roots, memory.DeepRetrieveOptions{
		Query:       query,
		MaxDepth:    maxDepth,
		MaxResults:  maxResults,
		Threshold:   threshold,
		SurfaceOnly: surfaceOnly,
	})
	if err != nil {
		fatalf("deep retrieve: %v", err)
	}

	if jsonOut {
		_ = json.NewEncoder(os.Stdout).Encode(results)
		return
	}

	fmt.Printf("Query: %q\n", query)
	fmt.Printf("Roots: %d capsule(s) at level=%s\n", len(roots), rootLevel)
	fmt.Printf("Results: %d (depth≤%d, threshold=%.2f, surface-only=%v)\n\n",
		len(results), maxDepth, threshold, surfaceOnly)
	if len(results) == 0 {
		fmt.Println("(no matches above threshold)")
		return
	}
	for i, r := range results {
		buried := ""
		if r.Buried {
			buried = " [buried]"
		}
		fmt.Printf("[%d] %s%s\n", i+1, r.Level, buried)
		fmt.Printf("    score=%.3f tier=%s depth=%d\n", r.Score, r.MatchedTier, r.Depth)
		fmt.Printf("    %s\n", r.URI)
		if r.L0 != "" {
			fmt.Printf("    L0: %s\n", r.L0)
		}
		fmt.Println()
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
