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
	// embedTurns toggles per-turn embedding at index-build time. When
	// true (the default), every turn is embedded once per
	// sessionIndex build and the resulting vector feeds into the
	// turn-level semantic score in scoreTurnsGlobal /
	// retrieveHybridContext. Disabling it skips the N×embed-RPC cost
	// for cost-sensitive deployments at the price of recall on
	// paraphrase-heavy single-hop questions.
	embedTurns bool
	// factExtractor is the FactExtractor used during buildSessionIndex
	// to populate the per-namespace fact micro-graph. nil means
	// "use the default rule-based extractor" — see facts.go. Callers
	// can plug in LLMFactExtractor (or a composite of both) via
	// PixelogConfig.FactExtractor for higher-recall extraction at the
	// price of per-turn LLM calls during the first index build.
	factExtractor FactExtractor
	// decomposer is an optional QuestionDecomposer that breaks a
	// compositional question into atomic sub-questions before
	// retrieval. Each sub-question fires a fresh hybrid retrieval;
	// the union (deduplicated by turn ID) is fed to the answerer
	// alongside hits for the original question. Lifts counterfactual
	// (LoCoMo temporal) and multi-hop categories where no single
	// turn carries enough evidence. Nil = single-shot retrieval.
	decomposer QuestionDecomposer
	// factEvidence, when true, prepends a structured FACT preamble
	// derived from the fact micro-graph to the answerer context.
	// The preamble lists fact triples whose subject matches a query
	// entity; the source turns themselves still follow chronologically.
	// Mirrors how vendors that pre-distill atomic facts surface them
	// at QA time, closing the "answerer extraction" gap on direct-
	// lookup and counterfactual-inference questions.
	factEvidence bool
	// profilePreamble, when true, prepends a compact per-entity
	// timeline block ("KNOWN TIMELINES: ...") to the answerer
	// context for questions whose subject entity has a profile.
	// Complements factEvidence: where factEvidence surfaces the
	// latest winning facts, profilePreamble surfaces the full
	// chronology, converting temporal-diff / counterfactual
	// reasoning into a direct-lookup over a dated projection.
	// The Lever-2 mechanism targeting the LoCoMo temporal gap.
	profilePreamble bool

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

// QuestionDecomposer breaks a compositional question into atomic
// sub-questions for retrieval augmentation. Returning an empty slice
// (or nil) signals the question is atomic and no decomposition is
// needed — the caller falls back to single-shot retrieval.
//
// Used at Answer() time to lift counterfactual / multi-hop categories
// where the original question is too compositional for any single
// turn to satisfy. Each sub-question fires a fresh hybrid retrieval;
// the union (deduplicated by turn ID) is fed to the answerer
// alongside hits for the original question.
type QuestionDecomposer interface {
	Decompose(ctx context.Context, question string) ([]string, error)
}

// questionNeedsDecompose gates the decomposer: only fire when the
// question looks compositional (temporal-diff, counterfactual,
// multi-hop, comparative). Atomic single-hop lookups would only
// suffer from decomposition — sub-questions introduce retrieval
// noise and dilute the answerer's attention on an already-sufficient
// context. Deterministic regex gate is free; the LLM decomposer call
// is the expensive part.
func questionNeedsDecompose(q string) bool {
	if q == "" {
		return false
	}
	lq := strings.ToLower(q)
	// Temporal-diff / time-arithmetic triggers.
	temporalTriggers := []string{
		"how long", "how many days", "how many weeks", "how many months",
		"how many years", "how old", "how much time",
		"days between", "weeks between", "months between", "years between",
		"before ", "after ", "prior to ", "since ", " ago", "between ",
	}
	// Counterfactual / conditional triggers.
	counterfactualTriggers := []string{
		"what if", "would have", "had not", "hadn't", "instead of",
		"rather than", "if not", "without ",
	}
	// Explicit multi-hop markers.
	multiHopTriggers := []string{
		" and also ", " and then ", "both ", " versus ", " compared to ",
		"which one", "each of", "all of",
	}
	for _, t := range temporalTriggers {
		if strings.Contains(lq, t) {
			return true
		}
	}
	for _, t := range counterfactualTriggers {
		if strings.Contains(lq, t) {
			return true
		}
	}
	for _, t := range multiHopTriggers {
		if strings.Contains(lq, t) {
			return true
		}
	}
	return false
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
	// EmbedTurns enables per-turn semantic embedding (in addition to
	// per-session). Defaults to true. Set false to skip the N×embed
	// cost on paraphrase-tolerant deployments where lexical signals
	// alone are sufficient.
	EmbedTurns *bool
	// FactExtractor is the optional micro-graph fact extractor. When
	// nil, the rule-based extractor (zero LLM cost) is used. Callers
	// can supply NewLLMFactExtractor(client) to enable LLM-driven
	// extraction at the price of per-turn API calls during index
	// build. Composing a rule + LLM cascade is left to the caller —
	// implement FactExtractor and chain internally.
	FactExtractor FactExtractor
	// FactEvidence, when true, prepends matched fact triples to the
	// answerer context as a structured preamble ("FACT: Subject
	// predicate Object"). Mirrors atomic-fact distillation pipelines
	// (e.g. Mem0) that surface facts directly to the LLM rather than
	// the original conversational turn. Set this on for benchmarks
	// where direct-lookup or counterfactual-inference questions
	// dominate (LoCoMo single_hop, temporal).
	FactEvidence bool
	// ProfilePreamble, when true, prepends a compact "KNOWN
	// TIMELINES" block per query-mentioned entity derived from the
	// pre-supersession fact chronology. Independent of FactEvidence:
	// you can enable both (they render as separate preamble blocks),
	// either, or neither. Tuned for LoCoMo temporal / counterfactual
	// categories where the answerer needs to do time arithmetic
	// across dated events.
	ProfilePreamble bool
	// Decomposer is an optional sub-question generator that runs
	// before retrieval. When set, each sub-question fires a fresh
	// hybrid retrieval and the union (deduplicated by turn ID) is
	// fed to the answerer alongside hits for the original question.
	// Most useful for counterfactual / multi-hop categories where
	// the original question is too compositional for any single
	// turn to satisfy. ~3x retrieval cost.
	Decomposer QuestionDecomposer
}

// NewPixelogMemory builds a PixelogMemory with sensible defaults.
// If cfg.Embedder is nil, a deterministic HashEmbedder(384) is used.
func NewPixelogMemory(cfg PixelogConfig) *PixelogMemory {
	if cfg.Embedder == nil {
		cfg.Embedder = NewHashEmbedder(384)
	}
	embedTurns := true
	if cfg.EmbedTurns != nil {
		embedTurns = *cfg.EmbedTurns
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
		embedTurns:    embedTurns,
		factExtractor: cfg.FactExtractor,
		factEvidence:    cfg.FactEvidence,
		profilePreamble: cfg.ProfilePreamble,
		decomposer:      cfg.Decomposer,
		states:        make(map[string]*nsState),
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

	// Primary path: CategoryStore semantic search over typed memories
	// produced by the archival pipeline's Consolidate phase. This is
	// the Mem0-style "distilled atomic fact" retrieval surface and was
	// empirically the strongest single signal on LoCoMo (R5: Hit@K
	// 91.42%). We keep it as primary; the turn-level hybrid scorer is
	// available for recall-mode eval and for decompose sub-question
	// augmentation below.
	results, err := s.categoryStore.Search(ctx, question, nil, k)
	if err != nil {
		return Answer{Latency: time.Since(start)}, fmt.Errorf("pixelog: search: %w", err)
	}

	seen := make(map[string]bool, k*2)
	retrieved := make([]string, 0, k*2)
	ids := make([]string, 0, k*2)
	for _, r := range results {
		if seen[r.Document.Content] {
			continue
		}
		seen[r.Document.Content] = true
		retrieved = append(retrieved, r.Document.Content)
		ids = append(ids, r.Document.ID)
	}
	seenIDs := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id != "" {
			seenIDs[id] = true
		}
	}

	// Optional sub-question augmentation. When a decomposer is
	// configured AND the question matches a compositional pattern
	// (temporal-diff, counterfactual, multi-hop), break it into
	// atomic sub-questions and fire a fresh turn-level hybrid
	// retrieval per sub-question. Hits are merged into the existing
	// context with dedup-by-turn-id. CRITICAL: total context stays
	// capped at k — decompose reorders/replaces low-ranked primary
	// hits, never inflates the answerer's budget. Mem0-style prompts
	// are tuned for k=30; doubling dilutes attention and regressed
	// LoCoMo judge by ~3.6pp in R6.
	if m.decomposer != nil && questionNeedsDecompose(question) {
		subQs, derr := m.decomposer.Decompose(ctx, question)
		if derr == nil && len(subQs) > 0 {
			subK := k / (len(subQs) + 1)
			if subK < 3 {
				subK = 3
			}
			for _, sq := range subQs {
				sq = strings.TrimSpace(sq)
				if sq == "" || strings.EqualFold(sq, question) {
					continue
				}
				if len(retrieved) >= k {
					break
				}
				subTexts, subIDs, sErr := m.retrieveHybridContext(ctx, namespace, sq, subK)
				if sErr != nil {
					continue
				}
				for i, txt := range subTexts {
					if len(retrieved) >= k {
						break
					}
					id := ""
					if i < len(subIDs) {
						id = subIDs[i]
					}
					if id != "" && seenIDs[id] {
						continue
					}
					if seen[txt] {
						continue
					}
					seen[txt] = true
					if id != "" {
						seenIDs[id] = true
					}
					retrieved = append(retrieved, txt)
					ids = append(ids, id)
				}
			}
		}
	}

	// Keyword fallback backstop: entity / date queries that miss
	// both semantic and hybrid retrieval. Capped to k more results.
	for _, hit := range m.fallbackScan(s, question, k) {
		if seen[hit] {
			continue
		}
		seen[hit] = true
		retrieved = append(retrieved, hit)
	}

	// Optional Lever-2 profile preamble. Prepended AFTER all primary
	// retrieval is done so it never competes for budget against the
	// top-K hits — it is purely additive context for the answerer.
	// Synthetic ID "TIMELINES" keeps Hit@K uncontaminated (gold IDs
	// are turn IDs of the form D{conv}:{msg} and never match the
	// sentinel). Builds the profile index lazily via a throwaway
	// retrieveHybridContext call when the cache is cold; subsequent
	// questions in the same namespace hit the cached sessionIndex.
	if m.profilePreamble {
		m.mu.Lock()
		idx := s.retrieverIndex
		m.mu.Unlock()
		if idx == nil {
			// Warm the retriever index; discard its retrieved context
			// (we already have retrieval from CategoryStore).
			_, _, _ = m.retrieveHybridContext(ctx, namespace, question, 1)
			m.mu.Lock()
			idx = s.retrieverIndex
			m.mu.Unlock()
		}
		if idx != nil && idx.profiles != nil {
			qEntities := extractEntities(question)
			profs := idx.profiles.Lookup(qEntities)
			if pre := formatProfilePreamble(profs, defaultProfilePreambleConfig()); pre != "" {
				retrieved = append([]string{pre}, retrieved...)
				ids = append([]string{"TIMELINES"}, ids...)
			}
		}
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
