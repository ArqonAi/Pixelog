package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// CompactionConfig governs how children are folded into a parent era.
type CompactionConfig struct {
	// SurfaceRatio is the fraction of children kept "surfaced" (full L1
	// inlined). Defaults to 0.30 — Sierpinski's removed-middle leaves
	// roughly the top + tail of the salience distribution visible.
	SurfaceRatio float64 `json:"surface_ratio"`

	// MinSurfaceCount guarantees at least this many children stay
	// surfaced even when SurfaceRatio · len(children) rounds down to a
	// smaller number. Prevents single-session days from collapsing
	// everything into the L0 alone.
	MinSurfaceCount int `json:"min_surface_count"`

	// SalienceConfig lets callers override the scorer weights.
	Salience CapsuleSalienceConfig `json:"salience"`
}

// DefaultCompactionConfig returns production defaults.
func DefaultCompactionConfig() CompactionConfig {
	return CompactionConfig{
		SurfaceRatio:    0.30,
		MinSurfaceCount: 3,
		Salience:        DefaultCapsuleSalienceConfig(),
	}
}

// Compactor folds N child capsules into a parent EraCapsule.
//
// The fold preserves Sierpinski self-similarity: the parent has the same
// (L0, L1, L2) triplet as a leaf capsule, with L2 expressed as the list
// of child URIs. Salience-driven collapse decides which children stay
// surfaced ("kept" — full L1 inlined) and which are buried (only L0
// inlined; full content reachable via deep retrieval).
type Compactor struct {
	cfg        CompactionConfig
	scorer     *CapsuleSalienceScorer
	summarizer ContentSummarizer
	now        func() string // injectable for deterministic tests
}

// NewCompactor builds a Compactor. summarizer may be nil; in that case
// L0/L1 fall back to the deterministic heuristic in tiered.go so
// compaction works fully offline.
func NewCompactor(cfg CompactionConfig, tracker *AccessTracker, summarizer ContentSummarizer) *Compactor {
	if cfg.SurfaceRatio <= 0 {
		cfg.SurfaceRatio = 0.30
	}
	if cfg.MinSurfaceCount <= 0 {
		cfg.MinSurfaceCount = 3
	}
	return &Compactor{
		cfg:        cfg,
		scorer:     NewCapsuleSalienceScorer(cfg.Salience, tracker),
		summarizer: summarizer,
	}
}

// Compact folds children at level (level-1) into a parent EraCapsule at
// the supplied parent level. Returns the finalized (hashed) EraCapsule.
//
// Invariants:
//   - len(children) >= 1 (compacting a single child is a degenerate but
//     legal pass-through that just lifts it to a higher level).
//   - all children share the same Namespace.
//   - parent.Level == children[0].Level.Parent() (callers may bypass
//     this with a custom level for tests / out-of-band promotions).
func (c *Compactor) Compact(ctx context.Context, namespace string, parentLevel EraLevel, children []ChildRef) (*EraCapsule, error) {
	if len(children) == 0 {
		return nil, fmt.Errorf("compact: no children")
	}

	// 1. Score every child and rank by salience descending.
	type scored struct {
		ref      ChildRef
		salience float64
	}
	ranked := make([]scored, len(children))
	for i, ch := range children {
		ranked[i] = scored{ref: ch, salience: c.scorer.Score(ch)}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].salience > ranked[j].salience
	})

	// 2. Decide cut: kept = top SurfaceRatio (≥ MinSurfaceCount); rest is
	//    the "removed middle" — buried but URI-reachable.
	keep := int(float64(len(ranked)) * c.cfg.SurfaceRatio)
	if keep < c.cfg.MinSurfaceCount {
		keep = c.cfg.MinSurfaceCount
	}
	if keep > len(ranked) {
		keep = len(ranked)
	}

	// 3. Build the child list with `Buried` flags. Re-sort chronologically
	//    so the resulting EraCapsule is timeline-coherent, not salience-coherent.
	resultChildren := make([]ChildRef, 0, len(ranked))
	for i, s := range ranked {
		ref := s.ref
		ref.Salience = s.salience
		ref.Buried = i >= keep
		// For buried children: drop L1 to encode the "removed middle".
		// Full L1 is still recoverable via the URI; this is the active
		// "subconscious" mechanic — content is present but not loaded
		// at the surface.
		if ref.Buried {
			ref.L1 = ""
		}
		resultChildren = append(resultChildren, ref)
	}
	sort.SliceStable(resultChildren, func(i, j int) bool {
		return resultChildren[i].StartedAt.Before(resultChildren[j].StartedAt)
	})

	// 4. Generate the parent's own L0/L1 from the surface children's L1
	//    concatenation. Buried children contribute their L0 only — they
	//    influence the era's narrative but don't dominate the summary
	//    budget.
	narrative := buildEraNarrative(resultChildren)
	l0, l1, _ := GenerateTiers(ctx, narrative, c.summarizer)

	// 5. Aggregate concept tags from every child (kept + buried) so deep
	//    retrieval can still match on era-level concept queries.
	concepts := mergeConcepts(children)

	era := NewEraCapsule(namespace, parentLevel, resultChildren)
	era.L0 = l0
	era.L1 = l1
	era.Concepts = concepts

	if _, err := era.Finalize(); err != nil {
		return nil, fmt.Errorf("compact: finalize: %w", err)
	}
	return era, nil
}

// buildEraNarrative concatenates surface children's L1 (or L0 fallback)
// in chronological order; buried children contribute a single L0 line.
// This becomes the input to GenerateTiers for the parent era's own
// L0/L1.
func buildEraNarrative(children []ChildRef) string {
	var b strings.Builder
	for _, c := range children {
		if c.Buried {
			if c.L0 != "" {
				b.WriteString("- ")
				b.WriteString(c.L0)
				b.WriteString("\n")
			}
			continue
		}
		if c.L1 != "" {
			b.WriteString(c.L1)
			b.WriteString("\n\n")
			continue
		}
		if c.L0 != "" {
			b.WriteString(c.L0)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

// mergeConcepts deduplicates the concept tags across all children while
// preserving first-seen order so the era's concept list reflects the
// timeline.
func mergeConcepts(children []ChildRef) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, c := range children {
		for _, k := range c.Concepts {
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, k)
		}
	}
	return out
}
