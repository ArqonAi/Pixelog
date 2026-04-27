package bench

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ArqonAi/Pixelog/internal/memory"
)

// PixelogMemory is the canonical bench MemorySystem implementation backed
// by pixelog's typed CategoryStore + ArchivalPipeline. It maintains a
// fresh per-namespace category store so multiple cases stay isolated.
type PixelogMemory struct {
	embedder       memory.Embedder
	summarizer     memory.LLMSummarizer
	contentSumm    memory.ContentSummarizer
	answerer       AnswerLLM
	condenserCfg   *memory.CondenserConfig
	retrievalK     int
	fullContext    bool
	reflector      SessionReflector

	mu     sync.Mutex
	states map[string]*nsState
}

// AnswerLLM produces a final answer text given a question and retrieved context.
type AnswerLLM interface {
	Answer(ctx context.Context, question string, retrieved []string) (string, error)
}

// SessionReflector summarises a single session's worth of turns into a
// dense, structured recap. Used during Consolidate to build the "memory
// palace" — a sequence of session summaries that the answerer can
// reason over far more efficiently than raw turns.
type SessionReflector interface {
	Reflect(ctx context.Context, sessionID string, date time.Time, turns []Turn) (string, error)
}

// PixelogConfig configures PixelogMemory. All fields are optional;
// nil dependencies cause graceful degradation (e.g. no LLM = stub answers).
type PixelogConfig struct {
	Embedder     memory.Embedder
	Summarizer   memory.LLMSummarizer
	ContentSumm  memory.ContentSummarizer
	Answerer     AnswerLLM
	CondenserCfg *memory.CondenserConfig
	RetrievalK   int  // top-k for semantic + keyword union retrieval; default 15
	FullContext  bool // pass entire transcript to the answerer (skip retrieval)
	Reflector    SessionReflector // optional: builds session-level summaries during Consolidate
}

// NewPixelogMemory builds a PixelogMemory with sensible defaults.
// If cfg.Embedder is nil, a deterministic HashEmbedder(384) is used.
func NewPixelogMemory(cfg PixelogConfig) *PixelogMemory {
	if cfg.Embedder == nil {
		cfg.Embedder = NewHashEmbedder(384)
	}
	if cfg.RetrievalK <= 0 {
		cfg.RetrievalK = 15
	}
	return &PixelogMemory{
		embedder:     cfg.Embedder,
		summarizer:   cfg.Summarizer,
		contentSumm:  cfg.ContentSumm,
		answerer:     cfg.Answerer,
		condenserCfg: cfg.CondenserCfg,
		retrievalK:   cfg.RetrievalK,
		fullContext:  cfg.FullContext,
		reflector:    cfg.Reflector,
		states:       make(map[string]*nsState),
	}
}

// nsState is the per-namespace store + pipeline.
type nsState struct {
	categoryStore    *memory.CategoryStore
	pipeline         *memory.ArchivalPipeline
	sessionTurns     []memory.CondenserEvent
	allTurns         []Turn
	sessionSummaries []sessionSummary
	reflected        map[string]bool // sessionID → already reflected
	// retrieverIndex caches the hybrid retriever's session index so
	// embeddings are computed once per namespace, not once per Retrieve.
	// Invalidated by Reset and any new Ingest call.
	retrieverIndex *sessionIndex
	indexedTurns   int // len(allTurns) when retrieverIndex was built
}

// sessionSummary is a dense structured recap of a single session,
// anchored to its date and produced by the reflection pass.
type sessionSummary struct {
	SessionID string
	Date      time.Time
	Summary   string
}

func (m *PixelogMemory) state(namespace string) *nsState {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.states[namespace]
	if !ok {
		s = m.newState(namespace)
		m.states[namespace] = s
	}
	return s
}

func (m *PixelogMemory) newState(namespace string) *nsState {
	cs := memory.NewCategoryStore(m.embedder, nil)
	pipeline := memory.NewArchivalPipeline(&memory.ArchivalConfig{
		Namespace:         namespace,
		Condenser:         m.condenserCfg,
		Summarizer:        m.summarizer,
		ContentSummarizer: m.contentSumm,
		Categories:        cs,
	})
	return &nsState{categoryStore: cs, pipeline: pipeline, reflected: make(map[string]bool)}
}

// Reset implements MemorySystem.
func (m *PixelogMemory) Reset(_ context.Context, namespace string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[namespace] = m.newState(namespace)
	return nil
}

// Ingest implements MemorySystem. Each turn is stored verbatim as a
// CategoryFact entry in the CategoryStore — this gives semantic retrieval
// over raw conversation without relying on the heuristic classifier
// (which only fires on a narrow keyword set). Heuristic typed-memory
// extraction still runs separately in Consolidate.
func (m *PixelogMemory) Ingest(ctx context.Context, namespace string, turns []Turn) error {
	s := m.state(namespace)
	m.mu.Lock()
	s.allTurns = append(s.allTurns, turns...)
	for _, t := range turns {
		role := t.Role
		if role == "" {
			role = "user"
		}
		s.sessionTurns = append(s.sessionTurns, memory.CondenserEvent{
			ID:        t.TurnID,
			Role:      role,
			Content:   t.Text,
			Timestamp: t.Timestamp,
		})
	}
	cs := s.categoryStore
	m.mu.Unlock()

	// In full-context mode retrieval is bypassed at answer time — skip
	// per-turn embedding (saves N×embed-latency for every conversation).
	if m.fullContext {
		return nil
	}

	// Best-effort: store every turn as a fact for retrieval. Failures
	// (embedding errors, etc.) degrade to fallback substring scan.
	for i, t := range turns {
		if t.Text == "" {
			continue
		}
		id := t.TurnID
		if id == "" {
			id = fmt.Sprintf("turn_%s_%d_%d", namespace, time.Now().UnixNano(), i)
		}
		key := t.Speaker
		if key == "" {
			key = t.Role
		}
		if key == "" {
			key = "turn"
		}
		// Annotate with session + date so temporal queries can hit them.
		value := t.Text
		if !t.Timestamp.IsZero() {
			value = fmt.Sprintf("[%s] %s", t.Timestamp.Format("2006-01-02"), t.Text)
		} else if t.SessionID != "" {
			value = fmt.Sprintf("[%s] %s", t.SessionID, t.Text)
		}
		mem := memory.TypedMemory{
			ID:         id,
			Category:   memory.CategoryFact,
			Key:        key,
			Value:      value,
			Source:     t.Role,
			Timestamp:  t.Timestamp,
			Confidence: 0.5,
		}
		if err := cs.Store(ctx, mem); err != nil {
			// Log-and-continue: failures here are tolerable since the
			// runner has fallbackScan as a backstop.
			_ = err
		}
	}
	return nil
}

// Consolidate implements MemorySystem. Runs the archival pipeline's
// Compress phase, which extracts typed memories and stores them in
// the per-namespace CategoryStore.
func (m *PixelogMemory) Consolidate(ctx context.Context, namespace string) error {
	s := m.state(namespace)
	m.mu.Lock()
	batch := s.sessionTurns
	s.sessionTurns = nil
	pipeline := s.pipeline
	allTurns := append([]Turn(nil), s.allTurns...)
	m.mu.Unlock()

	if len(batch) == 0 && m.reflector == nil {
		return nil
	}

	// Heuristic typed-memory extraction (preference, instruction, ...).
	if len(batch) > 0 {
		if _, err := pipeline.Compress(ctx, batch); err != nil {
			return fmt.Errorf("pixelog: consolidate: %w", err)
		}
	}

	// Reflection pass: for every session that hasn't been reflected
	// yet, generate a structured summary. Sessions are detected via
	// SessionID grouping over the full turn history.
	if m.reflector != nil {
		groups := groupBySession(allTurns)
		for _, g := range groups {
			m.mu.Lock()
			already := s.reflected[g.id]
			m.mu.Unlock()
			if already {
				continue
			}
			summary, err := m.reflector.Reflect(ctx, g.id, g.date, g.turns)
			if err != nil {
				// Reflection failures are non-fatal — degrade to raw turns.
				continue
			}
			m.mu.Lock()
			s.reflected[g.id] = true
			s.sessionSummaries = append(s.sessionSummaries, sessionSummary{
				SessionID: g.id,
				Date:      g.date,
				Summary:   summary,
			})
			m.mu.Unlock()
		}
		// Keep summaries chronologically ordered.
		m.mu.Lock()
		sort.SliceStable(s.sessionSummaries, func(i, j int) bool {
			a, b := s.sessionSummaries[i].Date, s.sessionSummaries[j].Date
			if a.IsZero() && b.IsZero() {
				return s.sessionSummaries[i].SessionID < s.sessionSummaries[j].SessionID
			}
			return a.Before(b)
		})
		m.mu.Unlock()
	}

	return nil
}

// sessionGroup is an internal helper for grouping turns by SessionID.
type sessionGroup struct {
	id    string
	date  time.Time
	turns []Turn
}

func groupBySession(turns []Turn) []sessionGroup {
	if len(turns) == 0 {
		return nil
	}
	idx := make(map[string]int)
	groups := make([]sessionGroup, 0, 8)
	for _, t := range turns {
		key := t.SessionID
		if key == "" {
			key = "_default"
		}
		i, ok := idx[key]
		if !ok {
			idx[key] = len(groups)
			groups = append(groups, sessionGroup{id: key, date: t.Timestamp, turns: []Turn{t}})
			continue
		}
		groups[i].turns = append(groups[i].turns, t)
		if groups[i].date.IsZero() && !t.Timestamp.IsZero() {
			groups[i].date = t.Timestamp
		}
	}
	return groups
}

// Answer implements MemorySystem. Performs hybrid retrieval (semantic
// top-k from the CategoryStore UNION keyword-matching turns from the raw
// transcript), then asks the configured AnswerLLM for a final response.
// If no AnswerLLM is configured, returns the concatenated retrieved
// context (useful for retrieval-only eval).
func (m *PixelogMemory) Answer(ctx context.Context, namespace, question string) (Answer, error) {
	start := time.Now()
	s := m.state(namespace)

	// Full-context mode: bypass retrieval entirely. If session reflection
	// has been run (session summaries built), feed the dense summaries
	// which scale far better than raw transcripts. Otherwise fall back
	// to all turns with [date] / speaker prefixes.
	if m.fullContext {
		m.mu.Lock()
		summaries := append([]sessionSummary(nil), s.sessionSummaries...)
		turns := append([]Turn(nil), s.allTurns...)
		m.mu.Unlock()

		var retrieved []string
		if len(summaries) > 0 {
			retrieved = make([]string, 0, len(summaries))
			for _, sum := range summaries {
				prefix := sum.SessionID
				if !sum.Date.IsZero() {
					prefix = sum.Date.Format("2006-01-02") + " (" + sum.SessionID + ")"
				}
				retrieved = append(retrieved, fmt.Sprintf("=== SESSION %s ===\n%s", prefix, sum.Summary))
			}
		} else {
			retrieved = make([]string, 0, len(turns))
			for _, t := range turns {
				if t.Text == "" {
					continue
				}
				prefix := ""
				if !t.Timestamp.IsZero() {
					prefix = "[" + t.Timestamp.Format("2006-01-02") + "] "
				} else if t.SessionID != "" {
					prefix = "[" + t.SessionID + "] "
				}
				speaker := t.Speaker
				if speaker == "" {
					speaker = t.Role
				}
				retrieved = append(retrieved, fmt.Sprintf("%s%s: %s", prefix, speaker, t.Text))
			}
		}
		if m.answerer == nil {
			return Answer{
				Text:    strings.Join(retrieved, "\n"),
				Context: retrieved,
				Latency: time.Since(start),
			}, nil
		}
		text, err := m.answerer.Answer(ctx, question, retrieved)
		if err != nil {
			return Answer{Latency: time.Since(start)}, fmt.Errorf("pixelog: answer: %w", err)
		}
		return Answer{
			Text:    text,
			Context: retrieved,
			Latency: time.Since(start),
		}, nil
	}

	k := m.retrievalK

	results, err := s.categoryStore.Search(ctx, question, nil, k)
	if err != nil {
		return Answer{Latency: time.Since(start)}, fmt.Errorf("pixelog: search: %w", err)
	}

	seen := make(map[string]bool, k*2)
	retrieved := make([]string, 0, k*2)
	ids := make([]string, 0, k)
	for _, r := range results {
		if seen[r.Document.Content] {
			continue
		}
		seen[r.Document.Content] = true
		retrieved = append(retrieved, r.Document.Content)
		ids = append(ids, r.Document.ID)
	}

	// Hybrid: union keyword-matching turns to catch entity/date queries
	// that often miss embedding-similarity. Capped to k more results.
	for _, hit := range m.fallbackScan(s, question, k) {
		if seen[hit] {
			continue
		}
		seen[hit] = true
		retrieved = append(retrieved, hit)
	}

	if m.answerer == nil {
		return Answer{
			Text:         strings.Join(retrieved, "\n"),
			Context:      retrieved,
			Latency:      time.Since(start),
			RetrievedIDs: ids,
		}, nil
	}

	text, err := m.answerer.Answer(ctx, question, retrieved)
	if err != nil {
		return Answer{Latency: time.Since(start)}, fmt.Errorf("pixelog: answer: %w", err)
	}
	return Answer{
		Text:         text,
		Context:      retrieved,
		Latency:      time.Since(start),
		RetrievedIDs: ids,
	}, nil
}

// fallbackScan returns up to k turn texts whose lowercased contents
// contain any whitespace-tokenised word from the question.
func (m *PixelogMemory) fallbackScan(s *nsState, question string, k int) []string {
	m.mu.Lock()
	turns := append([]Turn(nil), s.allTurns...)
	m.mu.Unlock()

	qTokens := strings.Fields(strings.ToLower(question))
	if len(qTokens) == 0 {
		return nil
	}

	var hits []string
	for _, t := range turns {
		lt := strings.ToLower(t.Text)
		for _, qt := range qTokens {
			if len(qt) > 2 && strings.Contains(lt, qt) {
				hits = append(hits, t.Text)
				break
			}
		}
		if len(hits) >= k {
			break
		}
	}
	return hits
}
