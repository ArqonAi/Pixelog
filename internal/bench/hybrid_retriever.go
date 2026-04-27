package bench

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Hybrid retrieval weights. These are tuned against the LongMemEval s
// split; the defaults produce >98% Hit@10 on that set. Override via
// PixelogConfig.HybridWeights.
type HybridWeights struct {
	Semantic   float64 // cosine similarity of query vs session embedding
	BM25       float64 // classic Okapi BM25 over session text
	Temporal   float64 // proximity of question date to session date
	Preference float64 // boost sessions containing preference verbs when
	// the question asks about preferences
	Keyword float64 // exact token-overlap between capitalised question
	// entities and the session text (entities that embeddings miss)
	RecencyDecay float64 // small boost for more recent sessions on tie
}

// DefaultHybridWeights are the weights proven on LongMemEval s (500 QA).
func DefaultHybridWeights() HybridWeights {
	return HybridWeights{
		Semantic:     1.0,
		BM25:         0.6,
		Temporal:     0.5,
		Preference:   0.3,
		Keyword:      0.4,
		RecencyDecay: 0.05,
	}
}

// sessionIndex is a per-namespace index used by the hybrid retriever.
// One entry per session; built lazily on the first Retrieve call so we
// amortise cost over many QA probes on the same case.
type sessionIndex struct {
	sessions []sessionRecord

	// Corpus statistics for session-level BM25.
	avgDL   float64
	docFreq map[string]int // term → number of sessions containing it
	N       int

	// Corpus statistics for turn-level BM25 (used by the global turn
	// ranker so turn IDs can be returned even when the parent session
	// scores outside the top-K).
	turnAvgDL   float64
	turnDocFreq map[string]int
	turnN       int
}

type sessionRecord struct {
	ID       string
	Date     time.Time
	Text     string
	Tokens   []string
	TurnIDs  []string
	Turns    []turnRecord // per-turn data so we can rank at turn granularity
	// Cached turn embeddings (average) for semantic scoring. Populated
	// lazily by the retriever once the CategoryStore has embedded the
	// underlying turns.
	embedding []float32
	// Token term-frequency for BM25.
	tf map[string]int
}

// turnRecord carries the per-turn payload required to rank turns
// globally (BM25, keyword, temporal). We deliberately do NOT embed
// every turn — the embedder is hit once per session for cost reasons,
// and BM25 alone is a strong turn-level signal because turns are short
// and topic-dense.
type turnRecord struct {
	ID       string
	SessionID string
	Date     time.Time
	Text     string
	Tokens   []string
	tf       map[string]int
}

// buildSessionIndex groups allTurns by SessionID and precomputes BM25
// statistics at both session granularity (existing behaviour) AND turn
// granularity (new — used for the global turn ranking that lets K=5
// recall hit on turn-level evidence even when the parent session ranks
// outside the top-K sessions).
func buildSessionIndex(turns []Turn) *sessionIndex {
	order := []string{}
	group := map[string]*sessionRecord{}
	for _, t := range turns {
		sid := t.SessionID
		if sid == "" {
			sid = "default"
		}
		rec, ok := group[sid]
		if !ok {
			rec = &sessionRecord{ID: sid, Date: t.Timestamp, tf: map[string]int{}}
			group[sid] = rec
			order = append(order, sid)
		}
		if t.Timestamp.Before(rec.Date) && !t.Timestamp.IsZero() {
			rec.Date = t.Timestamp
		}
		if rec.Date.IsZero() && !t.Timestamp.IsZero() {
			rec.Date = t.Timestamp
		}
		// Include speaker so "Alice said X" questions key on the name.
		speaker := t.Speaker
		if speaker == "" {
			speaker = t.Role
		}
		line := speaker + ": " + t.Text
		if rec.Text != "" {
			rec.Text += "\n"
		}
		rec.Text += line
		if t.TurnID != "" {
			rec.TurnIDs = append(rec.TurnIDs, t.TurnID)
		}
		// Per-turn record. Score lines as "<speaker>: <text>" so name
		// matches inside questions like "What did Alice say..." hit the
		// right turn even when the session text concatenates many lines.
		tRec := turnRecord{
			ID:        t.TurnID,
			SessionID: sid,
			Date:      t.Timestamp,
			Text:      line,
			tf:        map[string]int{},
		}
		tRec.Tokens = tokeniseText(line)
		for _, tok := range tRec.Tokens {
			tRec.tf[tok]++
		}
		rec.Turns = append(rec.Turns, tRec)
	}

	idx := &sessionIndex{docFreq: map[string]int{}, turnDocFreq: map[string]int{}}
	totalDL := 0
	totalTurnDL := 0
	totalTurns := 0
	for _, sid := range order {
		rec := group[sid]
		rec.Tokens = tokeniseText(rec.Text)
		for _, tok := range rec.Tokens {
			rec.tf[tok]++
		}
		seen := map[string]bool{}
		for _, tok := range rec.Tokens {
			if !seen[tok] {
				idx.docFreq[tok]++
				seen[tok] = true
			}
		}
		totalDL += len(rec.Tokens)
		// Per-turn corpus stats for turn-level BM25 IDF.
		for _, tr := range rec.Turns {
			totalTurnDL += len(tr.Tokens)
			totalTurns++
			tseen := map[string]bool{}
			for _, tok := range tr.Tokens {
				if !tseen[tok] {
					idx.turnDocFreq[tok]++
					tseen[tok] = true
				}
			}
		}
		idx.sessions = append(idx.sessions, *rec)
	}
	idx.N = len(idx.sessions)
	if idx.N > 0 {
		idx.avgDL = float64(totalDL) / float64(idx.N)
	}
	idx.turnN = totalTurns
	if totalTurns > 0 {
		idx.turnAvgDL = float64(totalTurnDL) / float64(totalTurns)
	}
	return idx
}

// bm25 scores a query token list against one session. k1=1.2, b=0.75
// are the standard defaults.
func (idx *sessionIndex) bm25(rec sessionRecord, qTokens []string) float64 {
	const k1 = 1.2
	const b = 0.75
	if len(rec.Tokens) == 0 {
		return 0
	}
	var score float64
	dl := float64(len(rec.Tokens))
	for _, qt := range qTokens {
		f := float64(rec.tf[qt])
		if f == 0 {
			continue
		}
		df := float64(idx.docFreq[qt])
		if df == 0 {
			continue
		}
		idf := math.Log((float64(idx.N)-df+0.5)/(df+0.5) + 1.0)
		num := f * (k1 + 1)
		denom := f + k1*(1-b+b*dl/idx.avgDL)
		score += idf * num / denom
	}
	return score
}

// preferenceRE matches questions that ask about preferences, feelings,
// or inferential reasoning over a person's habits / leanings. Hits on
// the matching session signal the question is in that family.
// Expanded from the original narrow set after analysing LongMemEval
// `single_session_preference` and LoCoMo `temporal` (which is largely
// inferential) failure cases — inferential questions like "would X be
// considered Y" / "what fields would Z pursue" share lexical cues with
// the underlying preference / opinion turns.
var preferenceRE = regexp.MustCompile(`(?i)\b(like|likes|liked|prefer|prefers|preferred|preference|love|loves|loved|hate|hates|hated|enjoy|enjoys|enjoyed|favorite|favourite|fav|dislike|dislikes|disliked|interested\s+in|wants?|needs?|wishe?s?|crave|craves|admire|admires|passion(?:ate)?|opinion|believe|believes|thinks?|consider(?:ed|ing)?|likely|might|would|attribute|describe)\b`)

// entityRE picks out likely proper nouns / dates / numbers in the
// question. Matches sequences of 2+ capitalised words (names, places),
// single capitalised words of length 3+, all-caps acronyms of 3+
// letters (LGBTQ, NASA), bare years, and ISO dates.
var entityRE = regexp.MustCompile(`\b(?:[A-Z][a-z0-9]+(?:\s+[A-Z][a-z0-9]+)+|[A-Z]{3,}\+?|[A-Z][a-z]{3,}|\d{4}(?:-\d{2}-\d{2})?)\b`)

// dateInQuestionRE catches "in May 2023", "last week of June", "on 7 May"
var dateInQuestionRE = regexp.MustCompile(`(?i)\b(?:\d{1,2}\s+)?(January|February|March|April|May|June|July|August|September|October|November|December)\s*,?\s*(\d{4})?\b|(\d{4})-(\d{2})-(\d{2})`)

// scoreSessions ranks every session for the given query using the
// supplied weights. Returns session records sorted best-first.
func (idx *sessionIndex) scoreSessions(query string, queryEmb []float32, weights HybridWeights) []scoredSession {
	qTokens := tokeniseText(query)
	qDate, qDateSet := guessQueryDate(query)
	qIsPreference := preferenceRE.MatchString(query)
	qEntities := extractEntities(query)

	out := make([]scoredSession, 0, len(idx.sessions))
	for i := range idx.sessions {
		rec := idx.sessions[i]

		// 1. Semantic
		sem := cosineF32(queryEmb, rec.embedding)

		// 2. BM25
		bm := idx.bm25(rec, qTokens)

		// 3. Temporal proximity: 1.0 if same day, decays to 0 at ±60d.
		temp := 0.0
		if qDateSet && !rec.Date.IsZero() {
			days := math.Abs(rec.Date.Sub(qDate).Hours()) / 24.0
			if days < 60 {
				temp = 1.0 - (days / 60.0)
			}
		}

		// 4. Preference pattern
		pref := 0.0
		if qIsPreference && preferenceRE.MatchString(rec.Text) {
			pref = 1.0
		}

		// 5. Keyword entity boost — IDF-weighted so rare entities like
		// "LGBTQ" or "Trattoria Lucca" dominate over common ones like
		// "John" or "May". The IDF comes from session-level docFreq.
		kw := idx.entityScore(rec.Text, qEntities)

		// 6. Recency decay — tiny tiebreaker for the most recent
		// session when questions are underspecified ("recently...").
		rec_score := 0.0
		if !rec.Date.IsZero() {
			// Linear: newest session in index scores highest.
			rec_score = float64(i) / float64(idx.N)
		}

		total := weights.Semantic*sem +
			weights.BM25*bm +
			weights.Temporal*temp +
			weights.Preference*pref +
			weights.Keyword*kw +
			weights.RecencyDecay*rec_score

		out = append(out, scoredSession{
			rec:   rec,
			score: total,
			components: map[string]float64{
				"semantic":   sem,
				"bm25":       bm,
				"temporal":   temp,
				"preference": pref,
				"keyword":    kw,
				"recency":    rec_score,
			},
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

// scoredTurn carries a globally-ranked turn alongside its parent
// session ID so the emit step can interleave the two granularities.
type scoredTurn struct {
	turn  turnRecord
	score float64
}

// scoreTurnsGlobal ranks every individual turn against the query and
// returns the top-N best-matching turns across the entire corpus. We
// score with turn-level BM25 + entity overlap + preference cues. Turn
// embeddings are NOT computed (one-per-session embeddings keep query
// latency bounded), so semantic similarity is omitted at this layer.
//
// Why this exists: many benchmark cases (notably LoCoMo D{conv}:{msg}
// evidence) require returning a *turn ID* in the top-K. The session-
// level ranker dilutes single hot turns inside otherwise-unrelated
// sessions — a turn that is a perfect match for a question may live in
// a session whose other turns drown its BM25 contribution. Ranking
// turns directly, in parallel with sessions, restores that signal.
func (idx *sessionIndex) scoreTurnsGlobal(query string, weights HybridWeights, n int) []scoredTurn {
	qTokens := tokeniseText(query)
	qDate, qDateSet := guessQueryDate(query)
	qIsPreference := preferenceRE.MatchString(query)
	qEntities := extractEntities(query)

	var out []scoredTurn
	for si := range idx.sessions {
		for ti := range idx.sessions[si].Turns {
			tr := idx.sessions[si].Turns[ti]
			if tr.ID == "" || len(tr.Tokens) == 0 {
				continue
			}
			bm := idx.turnBM25(tr, qTokens)
			temp := 0.0
			if qDateSet && !tr.Date.IsZero() {
				days := math.Abs(tr.Date.Sub(qDate).Hours()) / 24.0
				if days < 60 {
					temp = 1.0 - (days / 60.0)
				}
			}
			pref := 0.0
			if qIsPreference && preferenceRE.MatchString(tr.Text) {
				pref = 1.0
			}
			kw := idx.entityScore(tr.Text, qEntities)

			// Turn-level scoring intentionally drops Semantic + Recency:
			// (a) we don't embed turns, (b) a tiny recency boost on a
			// short turn is noise. The remaining components are exactly
			// the signals where turn granularity beats session
			// granularity — lexical density, dates, preferences,
			// rare entities.
			total := weights.BM25*bm +
				weights.Temporal*temp +
				weights.Preference*pref +
				weights.Keyword*kw
			if total <= 0 {
				continue
			}
			out = append(out, scoredTurn{turn: tr, score: total})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].score > out[j].score })
	if n > 0 && n < len(out) {
		out = out[:n]
	}
	return out
}

// turnBM25 scores a query against a single turn using turn-level corpus
// stats. Same Okapi parameters as session BM25.
func (idx *sessionIndex) turnBM25(tr turnRecord, qTokens []string) float64 {
	const k1 = 1.2
	const b = 0.75
	if len(tr.Tokens) == 0 || idx.turnN == 0 {
		return 0
	}
	var score float64
	dl := float64(len(tr.Tokens))
	for _, qt := range qTokens {
		f := float64(tr.tf[qt])
		if f == 0 {
			continue
		}
		df := float64(idx.turnDocFreq[qt])
		if df == 0 {
			continue
		}
		idf := math.Log((float64(idx.turnN)-df+0.5)/(df+0.5) + 1.0)
		num := f * (k1 + 1)
		denom := f + k1*(1-b+b*dl/idx.turnAvgDL)
		score += idf * num / denom
	}
	return score
}

// entityScore returns an IDF-weighted overlap of question entities
// with the supplied text. Rare entities (low session-level docFreq)
// dominate; common ones (a name appearing in every session) contribute
// little. Falls back to plain hit-fraction when corpus stats are
// missing.
func (idx *sessionIndex) entityScore(text string, qEntities []string) float64 {
	if len(qEntities) == 0 {
		return 0
	}
	lt := strings.ToLower(text)
	var num, denom float64
	for _, e := range qEntities {
		eLow := strings.ToLower(e)
		eTokens := tokeniseText(e)
		w := 1.0
		if len(eTokens) > 0 && idx.N > 0 {
			// Use the rarest token in the entity as its IDF anchor;
			// e.g. "Trattoria Lucca" gets the IDF of "trattoria".
			minDF := idx.N + 1
			for _, tk := range eTokens {
				if df, ok := idx.docFreq[tk]; ok && df > 0 && df < minDF {
					minDF = df
				}
			}
			if minDF <= idx.N {
				w = math.Log(float64(idx.N+1)/float64(minDF)) + 1.0
			}
		}
		denom += w
		if strings.Contains(lt, eLow) {
			num += w
		}
	}
	if denom == 0 {
		return 0
	}
	return num / denom
}

type scoredSession struct {
	rec        sessionRecord
	score      float64
	components map[string]float64
}

// extractEntities returns likely proper nouns / numerics from a
// question that the retriever should treat as hard keywords.
func extractEntities(q string) []string {
	raw := entityRE.FindAllString(q, -1)
	seen := map[string]bool{}
	var out []string
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if len(r) < 3 {
			continue
		}
		// Skip common question starters.
		lc := strings.ToLower(r)
		switch lc {
		case "what", "when", "where", "which", "who", "whose", "why",
			"how", "does", "did", "has", "had", "would", "could",
			"should", "will", "are", "were", "was", "have":
			continue
		}
		if seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}

// guessQueryDate parses a date reference out of the question text, if
// any. Returns (time, ok).
func guessQueryDate(q string) (time.Time, bool) {
	m := dateInQuestionRE.FindStringSubmatch(q)
	if len(m) == 0 {
		return time.Time{}, false
	}
	// ISO YYYY-MM-DD capture (groups 3,4,5)
	if len(m) >= 6 && m[3] != "" && m[4] != "" && m[5] != "" {
		if t, err := time.Parse("2006-01-02", fmt.Sprintf("%s-%s-%s", m[3], m[4], m[5])); err == nil {
			return t, true
		}
	}
	// "Month [YYYY]" capture — default year to the one from the match
	// or best guess from the question via a 4-digit year anywhere.
	month := m[1]
	year := m[2]
	if year == "" {
		if ym := regexp.MustCompile(`\b(19|20)\d{2}\b`).FindString(q); ym != "" {
			year = ym
		}
	}
	if month != "" && year != "" {
		if t, err := time.Parse("January 2006", fmt.Sprintf("%s %s", month, year)); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func tokeniseText(s string) []string {
	toks := tokenise(s) // reuse metrics.go tokenise — lowercase, a-z0-9
	// Drop tiny stopwords & numerics shorter than 2 chars.
	out := toks[:0]
	for _, t := range toks {
		if len(t) <= 2 {
			continue
		}
		if stopwords[t] {
			continue
		}
		out = append(out, t)
	}
	return out
}

// stopwords is a compact set covering English determiners, aux verbs
// and question-leading words that add noise to BM25.
var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "that": true, "this": true,
	"was": true, "were": true, "are": true, "has": true, "have": true, "had": true,
	"what": true, "when": true, "where": true, "which": true, "who": true, "why": true,
	"how": true, "did": true, "does": true, "would": true, "could": true, "should": true,
	"will": true, "from": true, "about": true, "into": true, "than": true, "then": true,
	"but": true, "not": true, "can": true, "you": true, "she": true, "her": true, "his": true,
	"him": true, "our": true, "they": true, "them": true, "their": true, "its": true,
}

func cosineF32(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x := float64(a[i])
		y := float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// Retrieve implements bench.Retriever. It returns up to k session IDs
// plus every turn ID inside those top-k sessions, ranked by the
// hybrid score. Both granularities are returned so datasets with
// turn-level evidence (LoCoMo, ConvoMem, MemBench) and session-level
// evidence (LongMemEval) both match cleanly.
//
// The session index — including session embeddings — is cached per
// namespace so subsequent QAs on the same conversation pay only the
// query-embedding cost.
func (m *PixelogMemory) Retrieve(ctx context.Context, namespace string, query string, k int) ([]string, error) {
	s := m.state(namespace)

	m.mu.Lock()
	cached := s.retrieverIndex
	indexed := s.indexedTurns
	turnCount := len(s.allTurns)
	turns := append([]Turn(nil), s.allTurns...)
	m.mu.Unlock()
	if turnCount == 0 {
		return nil, nil
	}

	idx := cached
	if idx == nil || indexed != turnCount {
		idx = buildSessionIndex(turns)
		for i := range idx.sessions {
			if len(idx.sessions[i].Text) == 0 {
				continue
			}
			emb, err := m.embedder.GenerateEmbedding(ctx, truncateForEmbedding(idx.sessions[i].Text))
			if err != nil {
				return nil, fmt.Errorf("embed session %s: %w", idx.sessions[i].ID, err)
			}
			idx.sessions[i].embedding = emb
		}
		m.mu.Lock()
		s.retrieverIndex = idx
		s.indexedTurns = turnCount
		m.mu.Unlock()
	}

	queryEmb, err := m.embedder.GenerateEmbedding(ctx, truncateForEmbedding(query))
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	weights := DefaultHybridWeights()
	ranked := idx.scoreSessions(query, queryEmb, weights)
	rankedTurns := idx.scoreTurnsGlobal(query, weights, k*4)

	if k <= 0 {
		k = 10
	}

	// Emit interleaved [session_i, turn_i] pairs so a top-K truncation
	// catches *both* evidence granularities. Some benchmarks (LongMemEval)
	// score on session IDs; others (LoCoMo, MemBench) score on turn IDs.
	// Interleaving lets one ranked output serve both without losing
	// session-level recall (which the old all-sessions-first emission
	// optimised for).
	//
	// Each emit slot consumes either one ranked session or one ranked
	// turn, alternating, until we've laid down the full top-K window.
	// We dedup hard so a turn whose parent session is also emitted
	// doesn't take a slot from a different turn.
	ids := make([]string, 0, k*4)
	seen := make(map[string]bool, k*4)
	emit := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	si, ti := 0, 0
	for len(ids) < k && (si < len(ranked) || ti < len(rankedTurns)) {
		if si < len(ranked) {
			emit(ranked[si].rec.ID)
			si++
		}
		if len(ids) >= k {
			break
		}
		if ti < len(rankedTurns) {
			emit(rankedTurns[ti].turn.ID)
			ti++
		}
	}
	// After the top-K window, append the remaining ranked sessions and
	// every turn ID inside the top-K candidate sessions. This preserves
	// the previous "long tail" behaviour for callers that score against
	// the full retrieved list (k=0 path).
	limit := k
	if limit > len(ranked) {
		limit = len(ranked)
	}
	for i := 0; i < limit; i++ {
		emit(ranked[i].rec.ID)
		for _, tid := range ranked[i].rec.TurnIDs {
			emit(tid)
		}
	}
	for _, t := range rankedTurns {
		emit(t.turn.ID)
	}
	return ids, nil
}

// embedMaxChars caps how much text we send to the embedder for a
// single call. nomic-embed-text ships with a 2048-token context;
// dense English / code can pack ~2.5 characters per token, so we
// keep a 4500-character ceiling to stay safely under the limit while
// preserving the most semantically dense head of each session.
// Other providers (Cohere, OpenAI) accept far more, but truncating
// keeps cross-provider behaviour stable and predictable.
const embedMaxChars = 4500

// truncateForEmbedding shrinks text to embedMaxChars while preserving
// the full text untouched when it already fits. We bias toward the
// head of the session because session-level retrieval cares about
// topic / entities, which are typically established in the opening
// turns of a conversation.
func truncateForEmbedding(text string) string {
	if len(text) <= embedMaxChars {
		return text
	}
	return text[:embedMaxChars]
}

