package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// MatchFn scores a candidate text against a query. Score >= threshold
// counts as a hit. Implementations are typically lexical (BM25 / token
// overlap) or vector cosine — the deep retriever stays embedder-agnostic.
type MatchFn func(query, candidate string) float64

// DefaultMatchFn is a cheap deterministic token-overlap matcher used
// when callers don't plug in their own. Returns the Jaccard similarity
// of lower-cased word sets, which is good enough to validate the
// traversal mechanics; production callers should pass a vector-cosine
// matcher in.
func DefaultMatchFn(query, candidate string) float64 {
	q := tokenSet(query)
	c := tokenSet(candidate)
	if len(q) == 0 || len(c) == 0 {
		return 0
	}
	inter := 0
	for tok := range q {
		if _, ok := c[tok]; ok {
			inter++
		}
	}
	union := len(q) + len(c) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func tokenSet(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,;:!?\"'`()[]{}")
		if w != "" {
			out[w] = struct{}{}
		}
	}
	return out
}

// DeepRetrieveOptions configures a deep retrieval pass.
type DeepRetrieveOptions struct {
	// Query is the natural-language query string fed to MatchFn.
	Query string

	// MaxDepth caps URI-graph traversal depth. 0 = surface-only (no
	// children explored). Each successful child match increments depth.
	MaxDepth int

	// SurfaceOnly skips buried children when descending. False (the
	// default) is the "free association" mode that lets repressed
	// memories surface; true mirrors normal conscious recall.
	SurfaceOnly bool

	// Threshold is the minimum match score to consider a hit. Defaults
	// to 0.05 — low enough that lexical matches against L0 abstracts
	// still pass.
	Threshold float64

	// MaxResults caps the result list (after deduplication).
	MaxResults int

	// Match overrides the default token-overlap matcher.
	Match MatchFn
}

// DeepRetrieveResult is one hit from a deep retrieval pass.
type DeepRetrieveResult struct {
	URI         string   `json:"uri"`
	Hash        string   `json:"hash"`
	Level       EraLevel `json:"level"`
	Depth       int      `json:"depth"`        // distance from the original era roots
	Score       float64  `json:"score"`        // MatchFn score on the matched tier
	MatchedTier string   `json:"matched_tier"` // "L0" | "L1" | "concept"
	L0          string   `json:"l0,omitempty"`
	L1          string   `json:"l1,omitempty"`
	Buried      bool     `json:"buried,omitempty"`
}

// DeepRetrieve walks the URI graph rooted at `roots`, scoring each
// capsule and its children against opts.Query. The traversal models the
// agent equivalent of "free association": match against the era's
// surface tiers; on a hit, descend into matched children and repeat.
//
// Cost is O(branching · depth) in the worst case but bounded by
// MaxDepth and MaxResults.
func DeepRetrieve(ctx context.Context, resolver *CapsuleResolver, roots []*EraCapsule, opts DeepRetrieveOptions) ([]DeepRetrieveResult, error) {
	if opts.Match == nil {
		opts.Match = DefaultMatchFn
	}
	if opts.Threshold <= 0 {
		opts.Threshold = 0.05
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 20
	}

	var results []DeepRetrieveResult
	// Two distinct sets:
	//   walked:   nodes we've entered walk() on (cycle protection for
	//             the URI graph traversal — never enter the same node
	//             twice).
	//   recorded: nodes we've already appended to results (result-
	//             dedup; a node might be scored twice, once as a
	//             child-inline-ref and once on walk-entry, but should
	//             appear in results at most once).
	walked := map[string]struct{}{}
	recorded := map[string]struct{}{}

	var walk func(ec *EraCapsule, depth int) error
	walk = func(ec *EraCapsule, depth int) error {
		if ec == nil {
			return nil
		}
		if _, dup := walked[ec.Hash]; dup {
			return nil
		}
		walked[ec.Hash] = struct{}{}

		// Score this capsule's surface tiers against the query.
		l0Score := opts.Match(opts.Query, ec.L0)
		l1Score := opts.Match(opts.Query, ec.L1)
		conceptScore := opts.Match(opts.Query, strings.Join(ec.Concepts, " "))

		matchedScore, matchedTier := bestMatch(l0Score, l1Score, conceptScore)
		if matchedScore >= opts.Threshold {
			if _, dup := recorded[ec.Hash]; !dup {
				results = append(results, DeepRetrieveResult{
					URI:         ec.URI(),
					Hash:        ec.Hash,
					Level:       ec.Level,
					Depth:       depth,
					Score:       matchedScore,
					MatchedTier: matchedTier,
					L0:          ec.L0,
					L1:          ec.L1,
				})
				recorded[ec.Hash] = struct{}{}
			}
		}

		// Stop if we've hit the depth budget.
		if depth >= opts.MaxDepth {
			return nil
		}

		// For each child: record a hit if its inlined tiers match,
		// then *always* descend into era-level children up to MaxDepth.
		// This is the "free association" mechanic — the agent keeps
		// looking deeper even when the era's surface doesn't match,
		// because the answer might live in a buried sub-era. Session
		// children are leaves and need no descent.
		for _, ch := range ec.Children {
			if opts.SurfaceOnly && ch.Buried {
				continue
			}

			cl0 := opts.Match(opts.Query, ch.L0)
			cl1 := opts.Match(opts.Query, ch.L1)
			ccon := opts.Match(opts.Query, strings.Join(ch.Concepts, " "))
			cScore, cTier := bestMatch(cl0, cl1, ccon)
			if cScore >= opts.Threshold {
				if _, dup := recorded[ch.Hash]; !dup {
					results = append(results, DeepRetrieveResult{
						URI:         ch.URI,
						Hash:        ch.Hash,
						Level:       ch.Level,
						Depth:       depth + 1,
						Score:       cScore,
						MatchedTier: cTier,
						L0:          ch.L0,
						L1:          ch.L1,
						Buried:      ch.Buried,
					})
					recorded[ch.Hash] = struct{}{}
				}
			}

			// Descend into era-level children unconditionally within
			// the depth budget. Sessions are leaves so we stop there.
			if ch.Level > LevelSession && resolver != nil {
				child, err := resolver.Resolve(ctx, ch.URI)
				if err != nil {
					if errors.Is(err, CapsuleStoreErrNotFound) {
						continue
					}
					return fmt.Errorf("resolve %s: %w", ch.URI, err)
				}
				if err := walk(child, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, root := range roots {
		if err := walk(root, 0); err != nil {
			return nil, err
		}
		if len(results) >= opts.MaxResults*2 {
			break
		}
	}

	// Sort by score descending, then prefer shallower depth on ties so
	// surface hits appear ahead of equal-score deep hits.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Depth < results[j].Depth
	})
	if len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
	}
	return results, nil
}

// bestMatch picks the highest-scoring tier and returns the score + label.
func bestMatch(l0, l1, concept float64) (float64, string) {
	best := l0
	tier := "L0"
	if l1 > best {
		best, tier = l1, "L1"
	}
	if concept > best {
		best, tier = concept, "concept"
	}
	return best, tier
}
