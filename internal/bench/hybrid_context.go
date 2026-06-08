package bench

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// retrieveHybridContext returns up-to-k turn-granular context excerpts
// for a question, using the same hybrid scorer that drives the
// Recall@K=92.08% Retrieve() path. Each excerpt is prefixed with
// [YYYY-MM-DD] when a turn timestamp is available so the answerer can
// resolve relative date references against absolute session dates.
//
// The returned (texts, ids) lists are aligned and chronologically
// ordered within the top-K so the LLM sees temporal flow. Empty
// (texts == nil) means the namespace has no ingested turns; callers
// should fall back to CategoryStore.Search.
//
// Scoring blends the parent session's full hybrid score (semantic +
// BM25 + temporal + preference + keyword + recency) with the per-turn
// (BM25 + temporal + preference + keyword) score. This recovers the
// session-level semantic signal — turns are not embedded — while
// preserving turn precision for evidence-grade questions.
func (m *PixelogMemory) retrieveHybridContext(ctx context.Context, namespace, query string, k int) ([]string, []string, error) {
	s := m.state(namespace)

	m.mu.Lock()
	cached := s.retrieverIndex
	indexed := s.indexedTurns
	turnCount := len(s.allTurns)
	allTurns := append([]Turn(nil), s.allTurns...)
	m.mu.Unlock()
	if turnCount == 0 {
		return nil, nil, nil
	}

	idx := cached
	if idx == nil || indexed != turnCount {
		idx = buildSessionIndexWith(allTurns, m.factExtractor)
		for i := range idx.sessions {
			if len(idx.sessions[i].Text) == 0 {
				continue
			}
			emb, err := m.embedder.GenerateEmbedding(ctx, truncateForEmbedding(idx.sessions[i].Text))
			if err != nil {
				return nil, nil, fmt.Errorf("embed session %s: %w", idx.sessions[i].ID, err)
			}
			idx.sessions[i].embedding = emb
		}
		// Mirror Retrieve()'s turn-embedding pass so the answerer
		// context built here benefits from per-turn semantic recall
		// as well — without this, retrieveHybridContext would lag
		// scoreTurnsGlobal on paraphrase-heavy queries.
		if m.embedTurns {
			_ = embedTurnsConcurrent(ctx, m.embedder, idx)
		}
		m.mu.Lock()
		s.retrieverIndex = idx
		s.indexedTurns = turnCount
		m.mu.Unlock()
	}

	queryEmb, err := m.embedder.GenerateEmbedding(ctx, truncateForEmbedding(query))
	if err != nil {
		return nil, nil, fmt.Errorf("embed query: %w", err)
	}

	weights := DefaultHybridWeights()
	rankedSessions := idx.scoreSessions(query, queryEmb, weights)

	// Pre-compute a per-session "top-K session indicator" so only turns
	// inside the best-ranked sessions inherit ANY session-level signal.
	// The previous approach (add 0.5×rawSessionScore to every turn)
	// diluted turn-level precision for direct-lookup single-hop
	// queries — a weakly-related session with mildly-high semantic
	// similarity could outrank a perfectly-matching turn in another
	// session. We now use the session score as a gentle tiebreaker
	// only, scaled so it can reorder ties but never override a strong
	// BM25/entity hit.
	sessScore := make(map[string]float64, len(rankedSessions))
	var maxSess float64
	for _, rs := range rankedSessions {
		sessScore[rs.rec.ID] = rs.score
		if rs.score > maxSess {
			maxSess = rs.score
		}
	}
	// Normalise to [0, 1] so the blend weight has a stable meaning
	// across conversations of different sizes.
	if maxSess > 0 {
		for k := range sessScore {
			sessScore[k] /= maxSess
		}
	}

	qTokens := tokeniseText(query)
	qDate, qDateSet := guessQueryDate(query)
	qIsPref := preferenceRE.MatchString(query)
	qEntities := extractEntities(query)

	// Fact-graph pre-pass: mirror scoreTurnsGlobal — map each
	// source-turn ID to the MAXIMUM confidence across every fact
	// whose subject matches the question's entities. The boost is
	// FactBoost × confidence, yielding a graded signal instead of a
	// binary flag.
	factTurnConf := map[string]float64{}
	var matchedFacts []Fact
	if idx.facts != nil {
		matchedFacts = idx.facts.Lookup(qEntities)
		for _, f := range matchedFacts {
			if f.SourceTurnID == "" {
				continue
			}
			if f.Confidence > factTurnConf[f.SourceTurnID] {
				factTurnConf[f.SourceTurnID] = f.Confidence
			}
		}
	}

	type turnHit struct {
		text  string
		id    string
		date  time.Time
		score float64
	}
	var hits []turnHit
	for si := range idx.sessions {
		rec := idx.sessions[si]
		for ti := range rec.Turns {
			tr := rec.Turns[ti]
			if tr.Text == "" {
				continue
			}
			bm := idx.turnBM25(tr, qTokens)
			temp := 0.0
			if qDateSet && !tr.Date.IsZero() {
				days := math.Abs(tr.Date.Sub(qDate).Hours()) / 24.0
				if days < 60 {
					temp = 1.0 - days/60.0
				}
			}
			pref := 0.0
			// Match against IndexText so coref hints (resolved
			// speaker / addressee / recent entities) participate in
			// preference and entity scoring. Falls back to Text for
			// older indices where IndexText was never populated.
			indexT := tr.IndexText
			if indexT == "" {
				indexT = tr.Text
			}
			if qIsPref && preferenceRE.MatchString(indexT) {
				pref = 1.0
			}
			kw := idx.entityScore(indexT, qEntities)
			// Per-turn semantic similarity — mirrors scoreTurnsGlobal.
			// Cheap when turn embeddings are populated (the common
			// case) and skipped cleanly when they're not, so this code
			// path never depends on m.embedTurns being true.
			sem := 0.0
			if len(queryEmb) > 0 && len(tr.Embedding) > 0 {
				sem = cosineF32(queryEmb, tr.Embedding)
			}
			// Fact-graph boost mirrors scoreTurnsGlobal: scaled by
			// the per-fact confidence so high-precision extractions
			// pull harder than hedged ones.
			fact := factTurnConf[tr.ID]
			turnScore := weights.Semantic*sem +
				weights.BM25*bm +
				weights.Temporal*temp +
				weights.Preference*pref +
				weights.Keyword*kw +
				weights.FactBoost*fact
			// Blend with normalised parent-session score. The session
			// ranker carries cross-session semantic + recency signals
			// the per-turn scorer doesn't (e.g. session topic match
			// when the right turn is a short pronoun-only utterance).
			// 0.5 on the normalised score is the empirical sweet spot
			// from the LoCoMo 60-QA dev slice.
			total := turnScore + 0.5*sessScore[rec.ID]
			if total <= 0 {
				continue
			}
			hits = append(hits, turnHit{
				text:  tr.Text,
				id:    tr.ID,
				date:  tr.Date,
				score: total,
			})
		}
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if k > 0 && len(hits) > k {
		hits = hits[:k]
	}

	// Re-order top-K chronologically so the LLM sees time flow.
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].date.IsZero() && hits[j].date.IsZero() {
			return false
		}
		if hits[i].date.IsZero() {
			return false
		}
		if hits[j].date.IsZero() {
			return true
		}
		return hits[i].date.Before(hits[j].date)
	})

	texts := make([]string, 0, len(hits)+1)
	ids := make([]string, 0, len(hits)+1)

	// Optional fact-as-evidence preamble: when m.factEvidence is set
	// AND any rule/LLM-extracted facts matched a query entity, emit a
	// structured KNOWN FACTS block as the FIRST context entry. This
	// mirrors atomic-fact-distillation pipelines (e.g. Mem0) that
	// surface pre-computed triples to the LLM rather than the raw
	// turn — closing the "answerer extraction" gap on direct-lookup
	// (LoCoMo single_hop) and counterfactual-inference (LoCoMo
	// temporal) questions where the evidence is buried in
	// conversational noise.
	//
	// The preamble carries the synthetic ID "FACTS" so it's
	// distinguishable from real turn evidence in downstream Hit@K
	// calculations (gold IDs are turn IDs of the form D{conv}:{msg}
	// and never match "FACTS").
	if m.factEvidence && len(matchedFacts) > 0 {
		if pre := formatFactPreamble(matchedFacts); pre != "" {
			texts = append(texts, pre)
			ids = append(ids, "FACTS")
		}
	}

	// Optional time-anchored entity-profile preamble (Lever 2).
	// Renders a compact per-entity chronology for every query-
	// mentioned subject whose profile index has entries. Emitted
	// with synthetic ID "TIMELINES" so it never collides with real
	// gold turn IDs in Hit@K calculations. Rendered AFTER FACTS so
	// the answerer reads in roughly "what we know" → "when it
	// happened" → "raw evidence" order.
	if m.profilePreamble && idx.profiles != nil {
		profs := idx.profiles.Lookup(qEntities)
		if pre := formatProfilePreamble(profs, defaultProfilePreambleConfig()); pre != "" {
			texts = append(texts, pre)
			ids = append(ids, "TIMELINES")
		}
	}

	for _, h := range hits {
		prefix := ""
		if !h.date.IsZero() {
			prefix = "[" + h.date.Format("2006-01-02") + "] "
		}
		texts = append(texts, prefix+h.text)
		ids = append(ids, h.id)
	}
	return texts, ids, nil
}

// formatFactPreamble renders a deduplicated, deterministically-ordered
// "KNOWN FACTS" block from a slice of matched Fact triples. Output
// shape:
//
//	KNOWN FACTS:
//	- Caroline favorite-color: blue (D3:7, conf=0.90)
//	- Caroline raised-in: Toronto (D5:2, conf=0.85)
//
// Sort order: subject ASC, then timestamp DESC so later supersedes
// earlier when subject+predicate collide. Duplicate (subject,
// predicate, object) tuples are deduped — only the highest-confidence
// instance survives — to keep the preamble compact and reduce
// answerer noise.
func formatFactPreamble(facts []Fact) string {
	if len(facts) == 0 {
		return ""
	}
	type key struct{ subj, pred, obj string }
	best := make(map[key]Fact, len(facts))
	for _, f := range facts {
		k := key{f.Subject, f.Predicate, f.Object}
		if cur, ok := best[k]; !ok || f.Confidence > cur.Confidence {
			best[k] = f
		}
	}
	out := make([]Fact, 0, len(best))
	for _, f := range best {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Subject != out[j].Subject {
			return out[i].Subject < out[j].Subject
		}
		if out[i].Predicate != out[j].Predicate {
			return out[i].Predicate < out[j].Predicate
		}
		return out[i].Timestamp.After(out[j].Timestamp)
	})
	var b strings.Builder
	b.WriteString("KNOWN FACTS:\n")
	for _, f := range out {
		fmt.Fprintf(&b, "- %s %s: %s", f.Subject, f.Predicate, f.Object)
		if f.SourceTurnID != "" {
			fmt.Fprintf(&b, " (%s", f.SourceTurnID)
			if f.Confidence > 0 {
				fmt.Fprintf(&b, ", conf=%.2f", f.Confidence)
			}
			b.WriteString(")")
		}
		b.WriteString("\n")
	}
	return b.String()
}
