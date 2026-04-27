package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// CondenserStrategy defines how history gets condensed
type CondenserStrategy string

const (
	CondenserNoop          CondenserStrategy = "noop"
	CondenserSlidingWindow CondenserStrategy = "sliding_window"
	CondenserLLMSummary    CondenserStrategy = "llm_summary"
	CondenserHybrid        CondenserStrategy = "hybrid"
)

// CondenserEvent represents a single conversation event for condensation
type CondenserEvent struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // "user", "assistant", "system", "tool"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	TokenEst  int       `json:"token_est"` // estimated token count
	Condensed bool      `json:"condensed"` // true if this is a summary
}

// CondensationResult holds the output of a condensation operation
type CondensationResult struct {
	OriginalCount  int       `json:"original_count"`
	CondensedCount int       `json:"condensed_count"`
	TokensSaved    int       `json:"tokens_saved"`
	Strategy       CondenserStrategy `json:"strategy"`
	Timestamp      time.Time `json:"timestamp"`
}

// CondenserConfig holds condenser settings
type CondenserConfig struct {
	Strategy          CondenserStrategy `json:"strategy"`
	MaxTokens         int               `json:"max_tokens"`           // max total tokens in context
	TargetTokens      int               `json:"target_tokens"`        // target after condensation
	WindowSize        int               `json:"window_size"`          // sliding window: keep last N events
	PreserveRecent    int               `json:"preserve_recent"`      // always keep last N events uncondensed
	PreserveSystem    bool              `json:"preserve_system"`      // always keep system messages
	SummaryMaxTokens  int               `json:"summary_max_tokens"`   // max tokens for a single summary
}

// DefaultCondenserConfig returns production defaults
func DefaultCondenserConfig() *CondenserConfig {
	return &CondenserConfig{
		Strategy:         CondenserHybrid,
		MaxTokens:        128000,
		TargetTokens:     96000,
		WindowSize:       50,
		PreserveRecent:   10,
		PreserveSystem:   true,
		SummaryMaxTokens: 500,
	}
}

// LLMSummarizer is the interface for generating summaries via LLM
type LLMSummarizer interface {
	Summarize(ctx context.Context, events []CondenserEvent) (string, error)
}

// Condenser manages conversation history within token limits
type Condenser struct {
	config     *CondenserConfig
	events     []CondenserEvent
	mu         sync.RWMutex
	summarizer LLMSummarizer
	metadata   []CondensationResult
}

// NewCondenser creates a new condenser with the given configuration
func NewCondenser(config *CondenserConfig, summarizer LLMSummarizer) *Condenser {
	if config == nil {
		config = DefaultCondenserConfig()
	}
	return &Condenser{
		config:     config,
		events:     make([]CondenserEvent, 0),
		summarizer: summarizer,
		metadata:   make([]CondensationResult, 0),
	}
}

// AddEvent adds a conversation event and returns whether condensation is needed
func (c *Condenser) AddEvent(event CondenserEvent) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.TokenEst == 0 {
		event.TokenEst = EstimateTokens(event.Content)
	}

	c.events = append(c.events, event)
	return c.totalTokens() > c.config.MaxTokens
}

// NeedsCondensation checks if the history exceeds the token limit
func (c *Condenser) NeedsCondensation() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.totalTokens() > c.config.MaxTokens
}

// Condense runs the configured condensation strategy
func (c *Condenser) Condense(ctx context.Context) (*CondensationResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	originalCount := len(c.events)
	originalTokens := c.totalTokens()

	if originalTokens <= c.config.MaxTokens {
		return nil, nil // no condensation needed
	}

	var err error
	switch c.config.Strategy {
	case CondenserNoop:
		return nil, nil
	case CondenserSlidingWindow:
		err = c.condenseSlidingWindow()
	case CondenserLLMSummary:
		err = c.condenseLLMSummary(ctx)
	case CondenserHybrid:
		err = c.condenseHybrid(ctx)
	default:
		return nil, fmt.Errorf("unknown condenser strategy: %s", c.config.Strategy)
	}

	if err != nil {
		return nil, err
	}

	result := &CondensationResult{
		OriginalCount:  originalCount,
		CondensedCount: len(c.events),
		TokensSaved:    originalTokens - c.totalTokens(),
		Strategy:       c.config.Strategy,
		Timestamp:      time.Now(),
	}

	c.metadata = append(c.metadata, *result)
	return result, nil
}

// GetEvents returns the current event history (for building LLM messages)
func (c *Condenser) GetEvents() []CondenserEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp := make([]CondenserEvent, len(c.events))
	copy(cp, c.events)
	return cp
}

// EventCount returns the number of events in history
func (c *Condenser) EventCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.events)
}

// TotalTokens returns the estimated total tokens
func (c *Condenser) TotalTokens() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.totalTokens()
}

// GetMetadata returns condensation history
func (c *Condenser) GetMetadata() []CondensationResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp := make([]CondensationResult, len(c.metadata))
	copy(cp, c.metadata)
	return cp
}

// Reset clears all events and metadata
func (c *Condenser) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = make([]CondenserEvent, 0)
	c.metadata = make([]CondensationResult, 0)
}

// --- Internal strategies ---

// condenseSlidingWindow keeps only the last N events plus system messages
func (c *Condenser) condenseSlidingWindow() error {
	if len(c.events) <= c.config.WindowSize {
		return nil
	}

	var preserved []CondenserEvent

	// Preserve system messages if configured
	if c.config.PreserveSystem {
		for _, e := range c.events {
			if e.Role == "system" {
				preserved = append(preserved, e)
			}
		}
	}

	// Keep the last WindowSize non-system events
	cutoff := len(c.events) - c.config.WindowSize
	if cutoff < 0 {
		cutoff = 0
	}

	for i := cutoff; i < len(c.events); i++ {
		e := c.events[i]
		if e.Role == "system" && c.config.PreserveSystem {
			continue // already preserved
		}
		preserved = append(preserved, e)
	}

	c.events = preserved
	return nil
}

// condenseLLMSummary uses an LLM to summarize older events
func (c *Condenser) condenseLLMSummary(ctx context.Context) error {
	if c.summarizer == nil {
		// Fall back to sliding window if no summarizer
		return c.condenseSlidingWindow()
	}

	preserveCount := c.config.PreserveRecent
	if preserveCount >= len(c.events) {
		return nil
	}

	// Split into old (to summarize) and recent (to keep)
	old := c.events[:len(c.events)-preserveCount]
	recent := c.events[len(c.events)-preserveCount:]

	// Separate system messages from old events
	var systemMsgs []CondenserEvent
	var toSummarize []CondenserEvent
	for _, e := range old {
		if e.Role == "system" && c.config.PreserveSystem {
			systemMsgs = append(systemMsgs, e)
		} else if !e.Condensed { // don't re-summarize existing summaries
			toSummarize = append(toSummarize, e)
		} else {
			systemMsgs = append(systemMsgs, e) // keep existing summaries
		}
	}

	if len(toSummarize) == 0 {
		return nil
	}

	// Chunk and summarize
	summary, err := c.summarizer.Summarize(ctx, toSummarize)
	if err != nil {
		// Fall back to sliding window on LLM failure
		return c.condenseSlidingWindow()
	}

	summaryEvent := CondenserEvent{
		ID:        fmt.Sprintf("summary-%d", time.Now().UnixNano()),
		Role:      "system",
		Content:   fmt.Sprintf("[Previous conversation summary]\n%s", summary),
		Timestamp: time.Now(),
		TokenEst:  EstimateTokens(summary),
		Condensed: true,
	}

	// Rebuild: system msgs + summary + recent
	var result []CondenserEvent
	result = append(result, systemMsgs...)
	result = append(result, summaryEvent)
	result = append(result, recent...)

	c.events = result
	return nil
}

// condenseHybrid uses LLM summary for early chunks, sliding window for rest
func (c *Condenser) condenseHybrid(ctx context.Context) error {
	// First try LLM summary
	if c.summarizer != nil {
		err := c.condenseLLMSummary(ctx)
		if err == nil && c.totalTokens() <= c.config.TargetTokens {
			return nil
		}
	}

	// If still over target, apply sliding window on top
	if c.totalTokens() > c.config.TargetTokens {
		return c.condenseSlidingWindow()
	}

	return nil
}

// totalTokens estimates total tokens across all events (must be called under lock)
func (c *Condenser) totalTokens() int {
	total := 0
	for _, e := range c.events {
		total += e.TokenEst
	}
	return total
}

// EstimateTokens gives a rough token count for text (≈4 chars per token for English)
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// Rough heuristic: ~4 characters per token for English text
	// Add overhead for message formatting (role, delimiters)
	tokens := len(text)/4 + 4
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

// SimpleLLMSummarizer wraps a function that calls an LLM to produce summaries
type SimpleLLMSummarizer struct {
	SummarizeFn func(ctx context.Context, text string) (string, error)
}

// Summarize implements LLMSummarizer by formatting events and calling the wrapped function
func (s *SimpleLLMSummarizer) Summarize(ctx context.Context, events []CondenserEvent) (string, error) {
	if s.SummarizeFn == nil {
		return "", fmt.Errorf("no summarize function configured")
	}

	// Build a text representation of the events
	var sb strings.Builder
	for _, e := range events {
		sb.WriteString(fmt.Sprintf("[%s] %s\n", e.Role, e.Content))
		if sb.Len() > 50000 {
			sb.WriteString("\n... (truncated for summarization)\n")
			break
		}
	}

	prompt := fmt.Sprintf(
		"Summarize the following conversation history concisely. "+
			"Preserve key decisions, code changes, file paths, and outcomes. "+
			"Omit redundant back-and-forth.\n\n%s", sb.String())

	return s.SummarizeFn(ctx, prompt)
}
