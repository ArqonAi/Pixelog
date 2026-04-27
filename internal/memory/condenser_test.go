package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockSummarizer implements LLMSummarizer for testing
type mockSummarizer struct {
	called    bool
	callCount int
	mu        sync.Mutex
	summarize func(ctx context.Context, events []CondenserEvent) (string, error)
}

func (m *mockSummarizer) Summarize(ctx context.Context, events []CondenserEvent) (string, error) {
	m.mu.Lock()
	m.called = true
	m.callCount++
	m.mu.Unlock()

	if m.summarize != nil {
		return m.summarize(ctx, events)
	}
	return fmt.Sprintf("Summary of %d events: key decisions were made.", len(events)), nil
}

// --- EstimateTokens Tests ---

func TestEstimateTokens_Empty(t *testing.T) {
	if EstimateTokens("") != 0 {
		t.Fatal("Expected 0 tokens for empty string")
	}
}

func TestEstimateTokens_Short(t *testing.T) {
	tokens := EstimateTokens("hello world")
	if tokens < 1 {
		t.Fatal("Expected at least 1 token")
	}
}

func TestEstimateTokens_Long(t *testing.T) {
	text := strings.Repeat("a", 4000)
	tokens := EstimateTokens(text)
	// ~4 chars per token → ~1000 tokens + overhead
	if tokens < 900 || tokens > 1100 {
		t.Fatalf("Expected ~1000 tokens for 4000 chars, got %d", tokens)
	}
}

// --- Condenser Config Tests ---

func TestDefaultCondenserConfig(t *testing.T) {
	config := DefaultCondenserConfig()
	if config.Strategy != CondenserHybrid {
		t.Fatalf("Expected hybrid strategy, got %s", config.Strategy)
	}
	if config.MaxTokens <= 0 {
		t.Fatal("Expected positive MaxTokens")
	}
	if config.PreserveRecent <= 0 {
		t.Fatal("Expected positive PreserveRecent")
	}
	if !config.PreserveSystem {
		t.Fatal("Expected PreserveSystem=true")
	}
}

// --- Condenser Core Tests ---

func TestCondenser_AddEvent(t *testing.T) {
	c := NewCondenser(nil, nil)
	needsCondense := c.AddEvent(CondenserEvent{
		Role:    "user",
		Content: "hello",
	})

	if needsCondense {
		t.Fatal("Single event should not trigger condensation")
	}
	if c.EventCount() != 1 {
		t.Fatalf("Expected 1 event, got %d", c.EventCount())
	}
}

func TestCondenser_TokenEstimation(t *testing.T) {
	c := NewCondenser(nil, nil)
	c.AddEvent(CondenserEvent{
		Role:    "user",
		Content: strings.Repeat("x", 400), // ~100 tokens
	})

	tokens := c.TotalTokens()
	if tokens < 90 || tokens > 120 {
		t.Fatalf("Expected ~100 tokens, got %d", tokens)
	}
}

func TestCondenser_NeedsCondensation(t *testing.T) {
	config := &CondenserConfig{
		Strategy:       CondenserSlidingWindow,
		MaxTokens:      100,
		TargetTokens:   50,
		WindowSize:     5,
		PreserveRecent: 3,
		PreserveSystem: true,
	}
	c := NewCondenser(config, nil)

	// Add events until we exceed max
	for i := 0; i < 20; i++ {
		c.AddEvent(CondenserEvent{
			Role:    "user",
			Content: strings.Repeat("word ", 10), // ~12 tokens each
		})
	}

	if !c.NeedsCondensation() {
		t.Fatal("Expected condensation needed after exceeding MaxTokens")
	}
}

func TestCondenser_NoopStrategy(t *testing.T) {
	config := &CondenserConfig{
		Strategy:  CondenserNoop,
		MaxTokens: 10,
	}
	c := NewCondenser(config, nil)

	// Exceed max tokens
	for i := 0; i < 20; i++ {
		c.AddEvent(CondenserEvent{
			Role:    "user",
			Content: strings.Repeat("word ", 10),
		})
	}

	result, err := c.Condense(context.Background())
	if err != nil {
		t.Fatalf("Noop should not error: %v", err)
	}
	if result != nil {
		t.Fatal("Noop should return nil result")
	}
}

func TestCondenser_SlidingWindow(t *testing.T) {
	config := &CondenserConfig{
		Strategy:       CondenserSlidingWindow,
		MaxTokens:      50,
		TargetTokens:   30,
		WindowSize:     5,
		PreserveRecent: 3,
		PreserveSystem: true,
	}
	c := NewCondenser(config, nil)

	// Add 20 events
	for i := 0; i < 20; i++ {
		c.AddEvent(CondenserEvent{
			ID:      fmt.Sprintf("evt-%d", i),
			Role:    "user",
			Content: fmt.Sprintf("message %d with some text", i),
		})
	}

	result, err := c.Condense(context.Background())
	if err != nil {
		t.Fatalf("SlidingWindow error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected condensation result")
	}

	if result.OriginalCount != 20 {
		t.Fatalf("Expected original count 20, got %d", result.OriginalCount)
	}
	if result.CondensedCount > 5 {
		t.Fatalf("Expected at most 5 events after window, got %d", result.CondensedCount)
	}
	if result.TokensSaved <= 0 {
		t.Fatal("Expected tokens saved > 0")
	}
	if result.Strategy != CondenserSlidingWindow {
		t.Fatalf("Expected strategy sliding_window, got %s", result.Strategy)
	}
}

func TestCondenser_SlidingWindowPreservesSystem(t *testing.T) {
	config := &CondenserConfig{
		Strategy:       CondenserSlidingWindow,
		MaxTokens:      50,
		TargetTokens:   30,
		WindowSize:     3,
		PreserveRecent: 2,
		PreserveSystem: true,
	}
	c := NewCondenser(config, nil)

	// Add system message first
	c.AddEvent(CondenserEvent{
		ID:      "sys-1",
		Role:    "system",
		Content: "You are a helpful assistant",
	})

	// Add many user messages
	for i := 0; i < 15; i++ {
		c.AddEvent(CondenserEvent{
			ID:      fmt.Sprintf("usr-%d", i),
			Role:    "user",
			Content: fmt.Sprintf("user message %d", i),
		})
	}

	c.Condense(context.Background())

	events := c.GetEvents()
	hasSystem := false
	for _, e := range events {
		if e.Role == "system" && e.Content == "You are a helpful assistant" {
			hasSystem = true
		}
	}

	if !hasSystem {
		t.Fatal("System message should be preserved after sliding window condensation")
	}
}

func TestCondenser_LLMSummary(t *testing.T) {
	summarizer := &mockSummarizer{}
	config := &CondenserConfig{
		Strategy:         CondenserLLMSummary,
		MaxTokens:        50,
		TargetTokens:     30,
		WindowSize:       5,
		PreserveRecent:   3,
		PreserveSystem:   true,
		SummaryMaxTokens: 100,
	}
	c := NewCondenser(config, summarizer)

	// Add events
	for i := 0; i < 20; i++ {
		c.AddEvent(CondenserEvent{
			ID:      fmt.Sprintf("evt-%d", i),
			Role:    "user",
			Content: fmt.Sprintf("message %d", i),
		})
	}

	result, err := c.Condense(context.Background())
	if err != nil {
		t.Fatalf("LLMSummary error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected condensation result")
	}

	if !summarizer.called {
		t.Fatal("Expected summarizer to be called")
	}

	// Check that events contain a summary
	events := c.GetEvents()
	hasSummary := false
	for _, e := range events {
		if e.Condensed && strings.Contains(e.Content, "summary") {
			hasSummary = true
		}
	}

	if !hasSummary {
		t.Fatal("Expected a condensed summary event")
	}

	// Recent events should be preserved
	if result.CondensedCount < 3 {
		t.Fatalf("Expected at least 3 events (preserved recent), got %d", result.CondensedCount)
	}
}

func TestCondenser_LLMSummaryFallback(t *testing.T) {
	// Summarizer that fails → should fall back to sliding window
	summarizer := &mockSummarizer{
		summarize: func(ctx context.Context, events []CondenserEvent) (string, error) {
			return "", fmt.Errorf("LLM unavailable")
		},
	}

	config := &CondenserConfig{
		Strategy:       CondenserLLMSummary,
		MaxTokens:      50,
		TargetTokens:   30,
		WindowSize:     5,
		PreserveRecent: 3,
		PreserveSystem: true,
	}
	c := NewCondenser(config, summarizer)

	for i := 0; i < 20; i++ {
		c.AddEvent(CondenserEvent{
			Role:    "user",
			Content: fmt.Sprintf("message %d", i),
		})
	}

	result, err := c.Condense(context.Background())
	if err != nil {
		t.Fatalf("Should fall back to sliding window, not error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected fallback result")
	}
}

func TestCondenser_HybridStrategy(t *testing.T) {
	summarizer := &mockSummarizer{}
	config := &CondenserConfig{
		Strategy:       CondenserHybrid,
		MaxTokens:      50,
		TargetTokens:   30,
		WindowSize:     5,
		PreserveRecent: 3,
		PreserveSystem: true,
	}
	c := NewCondenser(config, summarizer)

	for i := 0; i < 20; i++ {
		c.AddEvent(CondenserEvent{
			Role:    "user",
			Content: fmt.Sprintf("message %d", i),
		})
	}

	result, err := c.Condense(context.Background())
	if err != nil {
		t.Fatalf("Hybrid error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected condensation result")
	}

	if !summarizer.called {
		t.Fatal("Hybrid should try LLM summary first")
	}
}

func TestCondenser_HybridNoSummarizer(t *testing.T) {
	config := &CondenserConfig{
		Strategy:       CondenserHybrid,
		MaxTokens:      50,
		TargetTokens:   30,
		WindowSize:     5,
		PreserveRecent: 3,
		PreserveSystem: true,
	}
	c := NewCondenser(config, nil) // no summarizer

	for i := 0; i < 20; i++ {
		c.AddEvent(CondenserEvent{
			Role:    "user",
			Content: fmt.Sprintf("message %d", i),
		})
	}

	result, err := c.Condense(context.Background())
	if err != nil {
		t.Fatalf("Hybrid without summarizer should fall back: %v", err)
	}
	if result == nil {
		t.Fatal("Expected fallback result")
	}
}

func TestCondenser_NoCondensationNeeded(t *testing.T) {
	config := &CondenserConfig{
		Strategy:  CondenserSlidingWindow,
		MaxTokens: 100000, // very high limit
		WindowSize: 50,
	}
	c := NewCondenser(config, nil)

	c.AddEvent(CondenserEvent{Role: "user", Content: "hello"})

	result, err := c.Condense(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("Expected nil result when condensation not needed")
	}
}

func TestCondenser_GetEvents_Copy(t *testing.T) {
	c := NewCondenser(nil, nil)
	c.AddEvent(CondenserEvent{Role: "user", Content: "test"})

	events := c.GetEvents()
	events[0].Content = "modified"

	original := c.GetEvents()
	if original[0].Content == "modified" {
		t.Fatal("GetEvents should return a copy")
	}
}

func TestCondenser_GetMetadata(t *testing.T) {
	config := &CondenserConfig{
		Strategy:       CondenserSlidingWindow,
		MaxTokens:      20,
		WindowSize:     3,
		PreserveRecent: 2,
		PreserveSystem: true,
	}
	c := NewCondenser(config, nil)

	for i := 0; i < 10; i++ {
		c.AddEvent(CondenserEvent{
			Role:    "user",
			Content: fmt.Sprintf("message %d with content", i),
		})
	}

	c.Condense(context.Background())

	metadata := c.GetMetadata()
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 condensation record, got %d", len(metadata))
	}
	if metadata[0].OriginalCount != 10 {
		t.Fatalf("Expected original count 10, got %d", metadata[0].OriginalCount)
	}
}

func TestCondenser_Reset(t *testing.T) {
	c := NewCondenser(nil, nil)
	c.AddEvent(CondenserEvent{Role: "user", Content: "test"})

	c.Reset()

	if c.EventCount() != 0 {
		t.Fatal("Expected 0 events after reset")
	}
	if len(c.GetMetadata()) != 0 {
		t.Fatal("Expected 0 metadata after reset")
	}
}

func TestCondenser_AddEventTimestamp(t *testing.T) {
	c := NewCondenser(nil, nil)
	c.AddEvent(CondenserEvent{Role: "user", Content: "test"})

	events := c.GetEvents()
	if events[0].Timestamp.IsZero() {
		t.Fatal("Expected auto-populated timestamp")
	}
}

func TestCondenser_AddEventTokenEstimate(t *testing.T) {
	c := NewCondenser(nil, nil)
	c.AddEvent(CondenserEvent{Role: "user", Content: "hello world"})

	events := c.GetEvents()
	if events[0].TokenEst <= 0 {
		t.Fatal("Expected auto-calculated token estimate")
	}
}

func TestCondenser_ConcurrentAccess(t *testing.T) {
	c := NewCondenser(nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.AddEvent(CondenserEvent{
				Role:    "user",
				Content: fmt.Sprintf("concurrent message %d", n),
			})
			_ = c.EventCount()
			_ = c.TotalTokens()
			_ = c.NeedsCondensation()
		}(i)
	}
	wg.Wait()

	if c.EventCount() != 50 {
		t.Fatalf("Expected 50 events, got %d", c.EventCount())
	}
}

func TestCondenser_MultipleCondensations(t *testing.T) {
	config := &CondenserConfig{
		Strategy:       CondenserSlidingWindow,
		MaxTokens:      50,
		WindowSize:     5,
		PreserveRecent: 3,
		PreserveSystem: true,
	}
	c := NewCondenser(config, nil)

	// First batch
	for i := 0; i < 15; i++ {
		c.AddEvent(CondenserEvent{
			Role:    "user",
			Content: fmt.Sprintf("batch1 message %d", i),
		})
	}
	c.Condense(context.Background())

	// Second batch
	for i := 0; i < 15; i++ {
		c.AddEvent(CondenserEvent{
			Role:    "user",
			Content: fmt.Sprintf("batch2 message %d", i),
		})
	}
	c.Condense(context.Background())

	metadata := c.GetMetadata()
	if len(metadata) != 2 {
		t.Fatalf("Expected 2 condensation records, got %d", len(metadata))
	}
}

func TestCondenser_UnknownStrategy(t *testing.T) {
	config := &CondenserConfig{
		Strategy:  "invalid_strategy",
		MaxTokens: 10,
	}
	c := NewCondenser(config, nil)

	for i := 0; i < 20; i++ {
		c.AddEvent(CondenserEvent{
			Role:    "user",
			Content: "fill",
		})
	}

	_, err := c.Condense(context.Background())
	if err == nil {
		t.Fatal("Expected error for unknown strategy")
	}
}

// --- SimpleLLMSummarizer Tests ---

func TestSimpleLLMSummarizer(t *testing.T) {
	summarizer := &SimpleLLMSummarizer{
		SummarizeFn: func(ctx context.Context, text string) (string, error) {
			if !strings.Contains(text, "Summarize") {
				t.Fatal("Expected prompt to contain 'Summarize'")
			}
			return "Concise summary of the conversation.", nil
		},
	}

	events := []CondenserEvent{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
		{Role: "user", Content: "How are you?"},
	}

	result, err := summarizer.Summarize(context.Background(), events)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != "Concise summary of the conversation." {
		t.Fatalf("Unexpected result: %s", result)
	}
}

func TestSimpleLLMSummarizer_NoFn(t *testing.T) {
	summarizer := &SimpleLLMSummarizer{}

	_, err := summarizer.Summarize(context.Background(), nil)
	if err == nil {
		t.Fatal("Expected error when no function configured")
	}
}

func TestSimpleLLMSummarizer_LargeInput(t *testing.T) {
	summarizer := &SimpleLLMSummarizer{
		SummarizeFn: func(ctx context.Context, text string) (string, error) {
			// Verify truncation happens
			if len(text) > 60000 {
				t.Fatal("Expected input to be truncated")
			}
			return "summary", nil
		},
	}

	// Generate large input
	events := make([]CondenserEvent, 1000)
	for i := range events {
		events[i] = CondenserEvent{
			Role:    "user",
			Content: strings.Repeat("long content ", 50),
		}
	}

	_, err := summarizer.Summarize(context.Background(), events)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// --- Integration: Condenser + Stuck Detector Workflow ---

func TestCondenser_ContextOverflowTriggersCondensation(t *testing.T) {
	// Simulates: StuckDetector detects context_overflow → Condenser condenses
	config := &CondenserConfig{
		Strategy:       CondenserSlidingWindow,
		MaxTokens:      100,
		TargetTokens:   50,
		WindowSize:     5,
		PreserveRecent: 3,
		PreserveSystem: true,
	}
	c := NewCondenser(config, nil)

	// Fill up context
	for i := 0; i < 30; i++ {
		c.AddEvent(CondenserEvent{
			Role:    "assistant",
			Content: fmt.Sprintf("response %d with lots of detail about the implementation", i),
		})
	}

	if !c.NeedsCondensation() {
		t.Fatal("Expected condensation needed")
	}

	result, err := c.Condense(context.Background())
	if err != nil {
		t.Fatalf("Condensation error: %v", err)
	}

	if result.TokensSaved <= 0 {
		t.Fatal("Expected tokens saved > 0")
	}

	// After condensation, should be under limit or at window size
	if c.EventCount() > config.WindowSize+5 {
		t.Fatalf("Expected condensed event count <= %d, got %d", config.WindowSize+5, c.EventCount())
	}
}

func TestCondenser_PreserveRecentIntegrity(t *testing.T) {
	config := &CondenserConfig{
		Strategy:       CondenserLLMSummary,
		MaxTokens:      50,
		TargetTokens:   30,
		WindowSize:     5,
		PreserveRecent: 5,
		PreserveSystem: true,
	}
	summarizer := &mockSummarizer{}
	c := NewCondenser(config, summarizer)

	// Add system + 15 user messages
	c.AddEvent(CondenserEvent{ID: "sys", Role: "system", Content: "You are helpful"})
	for i := 0; i < 15; i++ {
		c.AddEvent(CondenserEvent{
			ID:      fmt.Sprintf("msg-%d", i),
			Role:    "user",
			Content: fmt.Sprintf("message %d", i),
		})
	}

	c.Condense(context.Background())

	events := c.GetEvents()

	// Last 5 user messages should be preserved
	recentIDs := make(map[string]bool)
	for _, e := range events {
		if !e.Condensed && e.Role != "system" {
			recentIDs[e.ID] = true
		}
	}

	for i := 10; i < 15; i++ {
		id := fmt.Sprintf("msg-%d", i)
		if !recentIDs[id] {
			t.Errorf("Expected recent message %s to be preserved", id)
		}
	}
}

func TestCondenser_AddEventReturnsTrueWhenOverLimit(t *testing.T) {
	config := &CondenserConfig{
		Strategy:  CondenserSlidingWindow,
		MaxTokens: 20,
		WindowSize: 5,
	}
	c := NewCondenser(config, nil)

	triggered := false
	for i := 0; i < 20; i++ {
		if c.AddEvent(CondenserEvent{
			Role:    "user",
			Content: "some content here",
		}) {
			triggered = true
			break
		}
	}

	if !triggered {
		t.Fatal("Expected AddEvent to return true when over MaxTokens")
	}
}

func TestCondenser_SlidingWindowEmptyHistory(t *testing.T) {
	config := &CondenserConfig{
		Strategy:  CondenserSlidingWindow,
		MaxTokens: 0, // force condensation check
		WindowSize: 5,
	}
	c := NewCondenser(config, nil)

	// No events → no error
	result, err := c.Condense(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("Expected nil result for empty history")
	}
}

func TestCondenser_TimestampPreserved(t *testing.T) {
	c := NewCondenser(nil, nil)
	now := time.Now().Add(-1 * time.Hour)

	c.AddEvent(CondenserEvent{
		Role:      "user",
		Content:   "old message",
		Timestamp: now,
	})

	events := c.GetEvents()
	if !events[0].Timestamp.Equal(now) {
		t.Fatal("Expected provided timestamp to be preserved")
	}
}
