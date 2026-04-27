package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ContextTier represents the abstraction level of a context entry.
// L0/L1/L2 give cheap-to-expensive retrieval cascade.
type ContextTier int

const (
	// TierL0 is the abstract layer (~100 tokens). One-sentence summary
	// used for fast relevance checking. Cheapest to load.
	TierL0 ContextTier = 0

	// TierL1 is the overview layer (~500-2000 tokens). Core information
	// for agent planning and decision-making.
	TierL1 ContextTier = 1

	// TierL2 is the detail layer (full content). Loaded only when the
	// agent needs deep reading.
	TierL2 ContextTier = 2
)

// String returns the tier name.
func (t ContextTier) String() string {
	switch t {
	case TierL0:
		return "L0"
	case TierL1:
		return "L1"
	case TierL2:
		return "L2"
	default:
		return fmt.Sprintf("L%d", t)
	}
}

// MaxTokens returns the approximate token budget for each tier.
// L2 is unlimited (returns 0).
func (t ContextTier) MaxTokens() int {
	switch t {
	case TierL0:
		return 100
	case TierL1:
		return 2000
	default:
		return 0
	}
}

// TieredEntry represents a knowledge entry with L0/L1/L2 abstraction layers.
// L2 (full content) is always stored. L0 and L1 are generated summaries.
type TieredEntry struct {
	ID        string                 `json:"id"`
	Category  MemoryCategory         `json:"category"`
	L0        string                 `json:"l0"`
	L1        string                 `json:"l1"`
	L2        string                 `json:"l2"`
	Source    string                 `json:"source,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	URI       string                 `json:"uri,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// GetTier returns the content at the requested tier level.
// Falls through to the next available tier (truncated) if requested tier is empty.
func (te *TieredEntry) GetTier(tier ContextTier) string {
	switch tier {
	case TierL0:
		if te.L0 != "" {
			return te.L0
		}
		if te.L1 != "" {
			return truncateContent(te.L1, 400)
		}
		return truncateContent(te.L2, 400)
	case TierL1:
		if te.L1 != "" {
			return te.L1
		}
		return truncateContent(te.L2, 8000)
	default:
		return te.L2
	}
}

// TokenEstimate returns approximate token count for a tier.
// Uses ~4 chars per token heuristic for English text.
func (te *TieredEntry) TokenEstimate(tier ContextTier) int {
	return len(te.GetTier(tier)) / 4
}

// ContentSummarizer generates a text summary from content with an instruction.
// Caller-supplied so the memory package stays decoupled from any specific LLM client.
type ContentSummarizer func(ctx context.Context, content string, instruction string) (string, error)

// GenerateTiers creates L0/L1 abstractions from L2 (full content).
// If a ContentSummarizer is provided, it generates high-quality summaries.
// Otherwise, falls back to heuristic extraction.
func GenerateTiers(ctx context.Context, content string, summarizer ContentSummarizer) (l0, l1 string, err error) {
	if content == "" {
		return "", "", nil
	}

	if summarizer != nil {
		l0, err = summarizer(ctx, content,
			"Generate a single-sentence abstract (max 100 tokens) that captures the core meaning. Be precise and factual.")
		if err != nil {
			l0 = heuristicL0(content)
			err = nil
		}

		if len(content) > 8000 {
			l1, err = summarizer(ctx, content,
				"Generate a concise overview (max 2000 tokens) preserving key facts, relationships, and context.")
			if err != nil {
				l1 = heuristicL1(content)
				err = nil
			}
		} else {
			l1 = content
		}
	} else {
		l0 = heuristicL0(content)
		l1 = heuristicL1(content)
	}

	return l0, l1, nil
}

// heuristicL0 extracts a one-sentence abstract from content.
func heuristicL0(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	for _, sep := range []string{". ", "! ", "? ", "\n"} {
		idx := strings.Index(content, sep)
		if idx > 10 && idx < 400 {
			return strings.TrimSpace(content[:idx+1])
		}
	}
	return truncateContent(content, 400)
}

// heuristicL1 builds an overview by keeping the first ~2000 tokens.
func heuristicL1(content string) string {
	if len(content) <= 8000 {
		return content
	}

	lines := strings.Split(content, "\n")
	var overview strings.Builder
	charBudget := 8000

	for _, line := range lines {
		if overview.Len()+len(line) > charBudget {
			break
		}
		overview.WriteString(line)
		overview.WriteString("\n")
	}

	result := strings.TrimSpace(overview.String())
	if result == "" {
		return truncateContent(content, 8000)
	}
	return result
}

// TieredFromTypedMemory converts a TypedMemory into a TieredEntry
// with heuristic tier generation (no LLM needed for short memories).
// Namespace is the capsule/instance identifier used in the URI.
func TieredFromTypedMemory(namespace string, mem TypedMemory) TieredEntry {
	content := fmt.Sprintf("%s: %s", mem.Key, mem.Value)
	return TieredEntry{
		ID:        mem.ID,
		Category:  mem.Category,
		L0:        heuristicL0(content),
		L1:        content,
		L2:        content,
		Source:    mem.Source,
		Timestamp: mem.Timestamp,
		URI:       BuildMemoryURI(namespace, mem.Category, mem.ID),
		Metadata: map[string]interface{}{
			"confidence": mem.Confidence,
			"key":        mem.Key,
		},
	}
}

// TieredSearchResult extends search results with tier information.
type TieredSearchResult struct {
	Entry    TieredEntry    `json:"entry"`
	Score    float64        `json:"score"`
	Category MemoryCategory `json:"category"`
	Tier     ContextTier    `json:"tier"`
}

// truncateContent is defined in archival.go.
