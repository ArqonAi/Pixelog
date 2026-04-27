package memory

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestContextTierString(t *testing.T) {
	tests := []struct {
		tier ContextTier
		want string
	}{
		{TierL0, "L0"},
		{TierL1, "L1"},
		{TierL2, "L2"},
		{ContextTier(99), "L99"},
	}
	for _, tt := range tests {
		if got := tt.tier.String(); got != tt.want {
			t.Errorf("ContextTier(%d).String() = %q, want %q", tt.tier, got, tt.want)
		}
	}
}

func TestContextTierMaxTokens(t *testing.T) {
	if TierL0.MaxTokens() != 100 {
		t.Errorf("L0 max tokens = %d, want 100", TierL0.MaxTokens())
	}
	if TierL1.MaxTokens() != 2000 {
		t.Errorf("L1 max tokens = %d, want 2000", TierL1.MaxTokens())
	}
	if TierL2.MaxTokens() != 0 {
		t.Errorf("L2 max tokens = %d, want 0 (unlimited)", TierL2.MaxTokens())
	}
}

func TestTieredEntryGetTier(t *testing.T) {
	entry := TieredEntry{
		L0: "Short abstract.",
		L1: "Medium overview with more detail about the topic.",
		L2: "Full detailed content with all the information one could need about this particular topic.",
	}

	if got := entry.GetTier(TierL0); got != "Short abstract." {
		t.Errorf("GetTier(L0) = %q, want L0 content", got)
	}
	if got := entry.GetTier(TierL1); got != entry.L1 {
		t.Errorf("GetTier(L1) = %q, want L1 content", got)
	}
	if got := entry.GetTier(TierL2); got != entry.L2 {
		t.Errorf("GetTier(L2) = %q, want L2 content", got)
	}
}

func TestTieredEntryGetTierFallthrough(t *testing.T) {
	entry := TieredEntry{
		L0: "",
		L1: "Some overview text.",
		L2: "Full content.",
	}
	got := entry.GetTier(TierL0)
	if got != "Some overview text." {
		t.Errorf("GetTier(L0) fallthrough = %q, expected L1 content", got)
	}

	entry2 := TieredEntry{L0: "", L1: "", L2: "Full detail."}
	if got := entry2.GetTier(TierL0); got != "Full detail." {
		t.Errorf("GetTier(L0) double fallthrough = %q", got)
	}
	if got := entry2.GetTier(TierL1); got != "Full detail." {
		t.Errorf("GetTier(L1) fallthrough = %q", got)
	}
}

func TestTieredEntryTokenEstimate(t *testing.T) {
	entry := TieredEntry{
		L0: strings.Repeat("word ", 25),
		L1: strings.Repeat("word ", 500),
		L2: strings.Repeat("word ", 2000),
	}

	l0, l1, l2 := entry.TokenEstimate(TierL0), entry.TokenEstimate(TierL1), entry.TokenEstimate(TierL2)
	if l0 >= l1 {
		t.Errorf("L0 tokens (%d) should be < L1 tokens (%d)", l0, l1)
	}
	if l1 >= l2 {
		t.Errorf("L1 tokens (%d) should be < L2 tokens (%d)", l1, l2)
	}
}

func TestHeuristicL0(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"short sentence", "This is a test."},
		{"sentence boundary", "First sentence here. Second sentence follows."},
		{"long content", strings.Repeat("x", 1000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := heuristicL0(tt.content)
			if len(got) > 400 {
				t.Errorf("heuristicL0 produced %d chars, max should be ~400", len(got))
			}
		})
	}
}

func TestHeuristicL1(t *testing.T) {
	short := "Brief content."
	if got := heuristicL1(short); got != short {
		t.Errorf("heuristicL1 changed short content: %q", got)
	}

	long := strings.Repeat("Line of text.\n", 2000)
	got := heuristicL1(long)
	if len(got) > 8100 {
		t.Errorf("heuristicL1 produced %d chars, should be ~8000 max", len(got))
	}
	if len(got) < 100 {
		t.Errorf("heuristicL1 too short: %d chars", len(got))
	}
}

func TestGenerateTiersHeuristic(t *testing.T) {
	content := "The quick brown fox jumps over the lazy dog. This is the second sentence with more detail about the fox and the dog."

	l0, l1, err := GenerateTiers(context.Background(), content, nil)
	if err != nil {
		t.Fatalf("GenerateTiers failed: %v", err)
	}
	if l0 == "" {
		t.Error("L0 should not be empty")
	}
	if l1 == "" {
		t.Error("L1 should not be empty")
	}
	if len(l0) > len(l1) {
		t.Errorf("L0 (%d chars) should be <= L1 (%d chars)", len(l0), len(l1))
	}
}

func TestGenerateTiersWithSummarizer(t *testing.T) {
	summarizer := func(_ context.Context, content string, instruction string) (string, error) {
		if strings.Contains(instruction, "single-sentence") {
			return "LLM abstract.", nil
		}
		return "LLM overview with more detail.", nil
	}

	content := strings.Repeat("Detailed content. ", 500)

	l0, l1, err := GenerateTiers(context.Background(), content, summarizer)
	if err != nil {
		t.Fatalf("GenerateTiers failed: %v", err)
	}
	if l0 != "LLM abstract." {
		t.Errorf("L0 = %q, want LLM abstract", l0)
	}
	if l1 != "LLM overview with more detail." {
		t.Errorf("L1 = %q, want LLM overview", l1)
	}
}

func TestGenerateTiersSummarizerFallback(t *testing.T) {
	failingSummarizer := func(_ context.Context, _ string, _ string) (string, error) {
		return "", fmt.Errorf("LLM unavailable")
	}

	content := "Important fact about the topic. More details follow here."

	l0, l1, err := GenerateTiers(context.Background(), content, failingSummarizer)
	if err != nil {
		t.Fatalf("GenerateTiers should not fail when summarizer fails: %v", err)
	}
	if l0 == "" || l1 == "" {
		t.Error("tiers should fall back to heuristic when summarizer fails")
	}
}

func TestGenerateTiersEmpty(t *testing.T) {
	l0, l1, err := GenerateTiers(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("GenerateTiers on empty content: %v", err)
	}
	if l0 != "" || l1 != "" {
		t.Errorf("Empty content should produce empty tiers, got L0=%q L1=%q", l0, l1)
	}
}

func TestGenerateTiersShortContent(t *testing.T) {
	content := "Short fact about Go programming."
	_, l1, err := GenerateTiers(context.Background(), content, func(_ context.Context, _ string, inst string) (string, error) {
		if strings.Contains(inst, "single-sentence") {
			return "Abstract.", nil
		}
		return "Should not be called for short content L1.", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l1 != content {
		t.Errorf("Short content L1 = %q, want original content %q", l1, content)
	}
}

func TestTieredFromTypedMemory(t *testing.T) {
	mem := TypedMemory{
		ID:         "pref_123",
		Category:   CategoryPreference,
		Key:        "coding style",
		Value:      "prefers functional programming patterns",
		Source:     "user",
		Timestamp:  time.Now(),
		Confidence: 0.8,
	}

	tiered := TieredFromTypedMemory("ns-test", mem)

	if tiered.ID != mem.ID {
		t.Errorf("ID = %q, want %q", tiered.ID, mem.ID)
	}
	if tiered.Category != mem.Category {
		t.Errorf("Category = %q, want %q", tiered.Category, mem.Category)
	}
	if tiered.L2 == "" {
		t.Error("L2 should contain the full content")
	}
	if tiered.L0 == "" {
		t.Error("L0 should be generated")
	}
	if tiered.L1 == "" {
		t.Error("L1 should be generated")
	}
	if !strings.Contains(tiered.URI, "pixe://memory/ns-test/") {
		t.Errorf("URI = %q, should contain pixe://memory/ns-test/", tiered.URI)
	}
	if tiered.Metadata["confidence"] != 0.8 {
		t.Errorf("Metadata confidence = %v, want 0.8", tiered.Metadata["confidence"])
	}
}

func TestTieredSearchResultFields(t *testing.T) {
	result := TieredSearchResult{
		Entry: TieredEntry{
			ID:       "test-1",
			Category: CategoryFact,
			L0:       "Abstract.",
			L1:       "Overview.",
			L2:       "Full content.",
		},
		Score:    0.95,
		Category: CategoryFact,
		Tier:     TierL1,
	}

	if result.Score != 0.95 {
		t.Errorf("Score = %f, want 0.95", result.Score)
	}
	if result.Tier != TierL1 {
		t.Errorf("Tier = %v, want L1", result.Tier)
	}
	if result.Entry.GetTier(TierL0) != "Abstract." {
		t.Errorf("GetTier(L0) = %q", result.Entry.GetTier(TierL0))
	}
}

func TestTruncateContent(t *testing.T) {
	if got := truncateContent("hello", 10); got != "hello" {
		t.Errorf("truncateContent short = %q", got)
	}
	// Long input gets ellipsis suffix
	if got := truncateContent("hello world this is long", 10); got != "hello w..." {
		t.Errorf("truncateContent long = %q", got)
	}
}
