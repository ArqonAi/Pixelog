package bench

import (
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// extractFactsConcurrency caps the number of goroutines dispatching
// FactExtractor.Extract in parallel. Sized for LLM-backed extractors
// that make one HTTP call per turn — matches embedTurnsConcurrency so
// both index-build passes scale the same way. Rule-extraction is
// CPU-bound and doesn't benefit from more parallelism anyway.
const extractFactsConcurrency = 8

// Fact is a single (subject, predicate, object) triple extracted from
// a conversational turn. Facts are the retrieval unit of the Pixelog
// micro-graph: they let the hybrid retriever answer direct-lookup
// questions ("What is Caroline's relationship status?") without
// needing to rank the source turn to the top-K via lexical or
// semantic scores.
//
// Each Fact carries enough provenance to:
//  1. Boost the source turn at retrieval time (SourceTurnID) so the
//     answerer still sees the original utterance as grounding.
//  2. Disambiguate contradictory facts by timestamp (later supersedes
//     earlier) and by source confidence (LLM > rule).
//  3. Link to the source session for session-level scoring fallbacks.
//
// The schema is deliberately flat — no nested edges, no property
// bags — because every fact represents a single assertion that
// either matches a query or doesn't. Graph-walk reasoning (multi-hop
// chains) is done implicitly by the answerer over the retrieved turn
// set, not structurally inside the fact store.
type Fact struct {
	// Subject is the entity the fact is about (normalised to title
	// case: "Caroline", not "caroline"). Always non-empty.
	Subject string
	// Predicate is a short machine-readable relation: "is", "likes",
	// "lives-in", "has-job", "has-hobby", "relationship-status", etc.
	// Always non-empty and lowercase. The taxonomy is open — new
	// predicates can be added by extractors without a migration.
	Predicate string
	// Object is the asserted value. Can be multi-word ("a software
	// engineer", "single", "Berlin"). Always non-empty.
	Object string
	// SourceTurnID points back to the conversational turn that gave
	// rise to the fact. Used by the retriever to surface the original
	// utterance alongside the fact lookup. Empty only for facts
	// loaded from non-turn sources (not currently used).
	SourceTurnID string
	// SourceSessionID is the session the source turn belongs to.
	SourceSessionID string
	// Timestamp of the source turn. Used for recency-based
	// disambiguation when multiple facts share a (subject, predicate)
	// key — the latest wins.
	Timestamp time.Time
	// Confidence in [0, 1]. Rule-extracted facts default to 0.7;
	// LLM-extracted facts typically land at 0.9+; uncertain heuristic
	// matches 0.5. Used only for tie-breaking in the fact ranker.
	Confidence float64
	// Source records the extractor origin for auditability and for
	// LLM-over-rule precedence in the store's de-duplication pass.
	Source FactSource
}

// FactSource identifies the extractor that produced a fact. Kept as a
// typed enum rather than a free-form string so the store can apply
// precedence rules (LLM > rule) deterministically during de-dup.
type FactSource int

const (
	// FactSourceRule means the fact came from the deterministic
	// pattern-based extractor in ruleFactExtractor. Zero-cost but
	// limited recall — only fires on templated utterances.
	FactSourceRule FactSource = iota
	// FactSourceLLM means the fact was extracted by an LLM via the
	// FactExtractor interface. Higher recall and precision but adds
	// per-turn API cost at ingest time.
	FactSourceLLM
)

// FactExtractor extracts facts from a conversational turn. The
// interface is deliberately stateless — implementations MUST NOT
// share mutable state across calls, because the hybrid retriever
// invokes the extractor in parallel with bounded concurrency during
// index build.
//
// Implementations return the empty slice (not nil, not an error) when
// the turn contains no extractable facts. A non-nil error signals a
// real infrastructure failure (LLM unavailable, context cancelled)
// and causes the retriever to fall back to lexical scoring only for
// that turn — without aborting the whole index build.
type FactExtractor interface {
	Extract(turn Turn) ([]Fact, error)
}

// ruleFactExtractor is the deterministic pattern-based extractor.
// It handles the dominant first-person assertion patterns that LoCoMo
// / ConvoMem / MemBench conversations are built on:
//
//  - "I am / I'm X"              → (speaker, is, X)
//  - "I live in X"               → (speaker, lives-in, X)
//  - "I work at X" / "I work in X" → (speaker, works-at, X)
//  - "my X is Y"                 → (speaker, has-X, Y)
//  - "I like / love / enjoy X"   → (speaker, likes, X)
//  - "I hate / dislike X"        → (speaker, dislikes, X)
//  - "I moved from X"            → (speaker, moved-from, X)
//  - "I'm currently X"           → (speaker, status, X)
//
// The patterns are tuned conservatively — they prefer high precision
// over recall. A missed fact is recovered by lexical / semantic
// scoring; a wrong fact poisons the retrieval signal.
type ruleFactExtractor struct{}

// RuleFactExtractor returns a stateless rule-based extractor. Safe to
// call from any goroutine; caller should reuse the returned value.
func RuleFactExtractor() FactExtractor { return ruleFactExtractor{} }

// factPattern is one (regex, predicate) rule for the rule-based
// extractor. The regex must have exactly one capture group (the
// object); the subject is always the turn's speaker.
type factPattern struct {
	re        *regexp.Regexp
	predicate string
	// minConfidence is the confidence assigned when the pattern
	// matches. Short literal patterns get higher confidence than
	// long heuristic ones.
	confidence float64
}

// factPatterns is the compiled rule set. Order matters: more
// specific patterns come first so "my favorite X is Y" wins over
// the more general "my X is Y".
var factPatterns = []factPattern{
	// "I'm currently X" / "I am currently X" → status=X
	{re: regexp.MustCompile(`(?i)\bi(?:'m| am)\s+currently\s+([a-z][a-z\- ]{2,40}?)(?:[.,!?]|\s+(?:and|but|because)\b|$)`), predicate: "status", confidence: 0.85},
	// "I'm single / married / dating X" → relationship-status
	{re: regexp.MustCompile(`(?i)\bi(?:'m| am)\s+(single|married|dating|divorced|separated|engaged|widowed)\b`), predicate: "relationship-status", confidence: 0.9},
	// "I live in X" → lives-in
	{re: regexp.MustCompile(`(?i)\bi\s+(?:live|reside|stay)\s+in\s+([A-Z][A-Za-z ]{2,40}?)(?:[.,!?]|\s+(?:and|but|because|since|for)\b|$)`), predicate: "lives-in", confidence: 0.85},
	// "I moved from X" → moved-from
	{re: regexp.MustCompile(`(?i)\bi\s+moved\s+(?:here\s+)?from\s+([A-Z][A-Za-z ]{2,40}?)(?:[.,!?]|\s+(?:and|but|to|because|in|when)\b|$)`), predicate: "moved-from", confidence: 0.85},
	// "I work at/for X" → works-at
	{re: regexp.MustCompile(`(?i)\bi\s+work\s+(?:at|for)\s+([A-Z][A-Za-z0-9 ]{2,40}?)(?:[.,!?]|\s+(?:and|but|as|because|since|in)\b|$)`), predicate: "works-at", confidence: 0.85},
	// "my job is X" / "my profession is X" → has-job
	{re: regexp.MustCompile(`(?i)\bmy\s+(?:job|profession|career|occupation)\s+is\s+([a-z][a-z\- ]{2,40}?)(?:[.,!?]|\s+(?:and|but|because)\b|$)`), predicate: "has-job", confidence: 0.8},
	// "I like/love/enjoy X" → likes
	{re: regexp.MustCompile(`(?i)\bi\s+(?:like|love|enjoy|adore)\s+([a-z][A-Za-z\- ]{2,40}?)(?:[.,!?]|\s+(?:and|but|because|when|so)\b|$)`), predicate: "likes", confidence: 0.75},
	// "I hate/dislike X" → dislikes
	{re: regexp.MustCompile(`(?i)\bi\s+(?:hate|dislike|can'?t\s+stand|loathe)\s+([a-z][A-Za-z\- ]{2,40}?)(?:[.,!?]|\s+(?:and|but|because|when|so)\b|$)`), predicate: "dislikes", confidence: 0.75},
	// "my favorite X is Y" → favorite-X=Y
	{re: regexp.MustCompile(`(?i)\bmy\s+favou?rite\s+([a-z]{3,20})\s+is\s+([A-Za-z][A-Za-z0-9\- ]{2,40}?)(?:[.,!?]|\s+(?:and|but|because)\b|$)`), predicate: "favorite", confidence: 0.9},

	// === Fix 2: counterfactual-inference predicates ===
	// LoCoMo cat-3 ("Would Caroline likely..." / "What fields
	// would X pursue") needs the retriever to surface turns about
	// interests, identity, upbringing, pursuits, and possessions
	// even when none of the question's literal tokens appear in
	// the gold turn. These predicates expand the fact graph's
	// coverage from "stated direct facts" to "biographical
	// signals that ground hypothetical reasoning".

	// "I grew up in X" / "I was raised in X" → raised-in
	// Captures upbringing context that grounds questions like
	// "What was Caroline's childhood like?". Object pattern is
	// permissive because the captured phrase may be a place
	// (Berlin), a culture (a small village), or a structure (a
	// large family).
	{re: regexp.MustCompile(`(?i)\bi\s+(?:grew\s+up|was\s+raised|was\s+born)\s+(?:in|on|near|with)\s+([A-Za-z][A-Za-z\- ]{2,50}?)(?:[.,!?]|\s+(?:and|but|because|when|where|so)\b|$)`), predicate: "raised-in", confidence: 0.8},

	// "I'm pursuing X" / "I'm studying X" / "I'm getting a degree in X"
	// → pursuing. Targets cat-3 questions about career trajectory
	// and education paths.
	{re: regexp.MustCompile(`(?i)\bi(?:'m| am)\s+(?:pursuing|studying|majoring\s+in|getting\s+(?:a\s+degree\s+in|into))\s+([A-Za-z][A-Za-z0-9\- ]{2,40}?)(?:[.,!?]|\s+(?:and|but|because|so|to)\b|$)`), predicate: "pursuing", confidence: 0.8},

	// "I'm interested in X" / "I'm into X" / "I'm passionate about X"
	// → has-interest. The single biggest cat-3 lever — interests
	// rarely appear lexically in counterfactual questions but are
	// the connective tissue the answerer reasons over.
	{re: regexp.MustCompile(`(?i)\bi(?:'m| am)\s+(?:interested\s+in|into|passionate\s+about|fascinated\s+by|curious\s+about)\s+([A-Za-z][A-Za-z0-9\- ]{2,40}?)(?:[.,!?]|\s+(?:and|but|because|so|when)\b|$)`), predicate: "has-interest", confidence: 0.85},

	// "I'm a/an X" → identifies-as (career, role, identity).
	// Distinct from has-job: this captures self-attributed
	// identity terms ("I'm a Christian", "I'm a runner", "I'm a
	// vegetarian") that don't fit the formal "my job is" frame.
	// Restricted to single-noun objects to avoid over-matching
	// "I'm a person who loves...".
	{re: regexp.MustCompile(`(?i)\bi(?:'m| am)\s+(?:a|an)\s+([a-z]{3,20})(?:[.,!?]|\s+(?:and|but|because|who|that|so)\b|$)`), predicate: "identifies-as", confidence: 0.7},

	// "I have a/an X" / "I own X" / "I bought X" → owns.
	// Catches possessions referenced in cat-3 questions ("Does
	// Caroline have a pet?" type inferences).
	{re: regexp.MustCompile(`(?i)\bi\s+(?:have|own|bought|got)\s+(?:a|an|my)\s+([a-z][a-z\- ]{2,40}?)(?:[.,!?]|\s+(?:and|but|because|that|which|when)\b|$)`), predicate: "owns", confidence: 0.7},

	// === Fix 4 lite: event-time predicate ===
	// "I [verb-past-tense] [event] [time-phrase]" — e.g. "I went
	// hiking last weekend". The whole right-hand side becomes the
	// object so a question like "When did Caroline go hiking?"
	// boosts the source turn even when the question doesn't
	// share the time-phrase tokens. The exact temporal value is
	// recoverable from the turn's timestamp + the captured
	// phrase; we don't try to compute it here.
	{re: regexp.MustCompile(`(?i)\bi\s+(?:went|attended|visited|started|finished|joined|completed)\s+([a-z][a-z\- ]{2,60}?)(?:[.,!?]|$)`), predicate: "did-event", confidence: 0.7},
}

// Extract implements FactExtractor. Runs every pattern against the
// turn text and emits one Fact per match. Multiple patterns can
// match the same turn (e.g. "I live in Berlin and I love sushi"
// produces two facts).
func (ruleFactExtractor) Extract(turn Turn) ([]Fact, error) {
	if turn.Text == "" || turn.Speaker == "" {
		return nil, nil
	}
	var facts []Fact
	for _, p := range factPatterns {
		matches := p.re.FindAllStringSubmatch(turn.Text, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			// "my favorite X is Y" pattern has two captures: X is the
			// specific predicate suffix, Y is the object.
			predicate := p.predicate
			var object string
			if p.predicate == "favorite" && len(m) >= 3 {
				predicate = "favorite-" + strings.ToLower(strings.TrimSpace(m[1]))
				object = strings.TrimSpace(m[2])
			} else {
				object = strings.TrimSpace(m[1])
			}
			if object == "" {
				continue
			}
			facts = append(facts, Fact{
				Subject:         titleCaseName(turn.Speaker),
				Predicate:       predicate,
				Object:          object,
				SourceTurnID:    turn.TurnID,
				SourceSessionID: turn.SessionID,
				Timestamp:       turn.Timestamp,
				Confidence:      p.confidence,
				Source:          FactSourceRule,
			})
		}
	}
	return facts, nil
}

// titleCaseName normalises a speaker name to "Title Case". Handles
// single words ("caroline" → "Caroline") and already-cased names
// ("Caroline" → "Caroline") uniformly. Leaves mixed-case multi-word
// names alone to avoid corrupting legitimate stylisations like
// "McDonald" or "O'Brien".
func titleCaseName(s string) string {
	if s == "" {
		return s
	}
	// If the name is already capitalised anywhere, trust it.
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			return s
		}
	}
	// All lowercase: capitalise first letter only (single-word case).
	return strings.ToUpper(s[:1]) + s[1:]
}

// factIndex is the per-sessionIndex fact store. Facts are grouped by
// (subject, predicate) so the retriever's fact-lookup path is O(1)
// on known-subject queries. When two facts share a key, the later
// (by timestamp) fact wins — this captures the "update" semantics of
// conversational memory where "I'm single" later supersedes "I'm
// dating Phil".
type factIndex struct {
	// bySubjectPredicate keys "<subject>\x00<predicate>" → winning fact.
	bySubjectPredicate map[string]Fact
	// bySubject groups all facts for a subject so "tell me about
	// Caroline" queries can surface every known fact at once.
	bySubject map[string][]Fact
	// all is the full ordered fact list — retained for iteration and
	// for the retrieval-side source-turn boost.
	all []Fact
	// allRaw is EVERY fact extracted from EVERY turn, pre-supersession.
	// Unlike `all`, this slice preserves superseded (older) facts so
	// downstream consumers (buildProfileIndex, temporal timelines)
	// can reconstruct the full chronology. Keep in mind: duplicates
	// across paraphrasing turns are preserved here too — dedup is the
	// consumer's responsibility if it matters.
	allRaw []Fact
}

// buildFactIndex extracts facts from every turn and de-duplicates by
// (subject, predicate) with later-supersedes-earlier semantics.
// Callers supply the extractor so LLM-backed extractors can be plugged
// in without code changes here.
func buildFactIndex(turns []Turn, extractor FactExtractor) *factIndex {
	if extractor == nil {
		extractor = ruleFactExtractor{}
	}
	idx := &factIndex{
		bySubjectPredicate: map[string]Fact{},
		bySubject:          map[string][]Fact{},
	}

	// Fan out extractor calls with bounded concurrency. Critical for
	// LLM-backed extractors — sequential calls make a 3,500-turn
	// LoCoMo case take ~30 min at ~500ms per HTTP call. With the
	// semaphore we complete in ~4 min at the same per-call cost.
	//
	// perTurn[i] holds the facts extracted from turns[i]; the index
	// preserves input order so supersession (later timestamp wins)
	// stays deterministic regardless of completion order.
	perTurn := make([][]Fact, len(turns))
	sem := make(chan struct{}, extractFactsConcurrency)
	var wg sync.WaitGroup
	for i, t := range turns {
		i, t := i, t
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			facts, err := extractor.Extract(t)
			if err != nil {
				// Extractor failures are non-fatal. The turn's facts
				// stay empty so it contributes zero to the micro-graph
				// but retrieval still falls back cleanly on lexical
				// and semantic signals.
				return
			}
			perTurn[i] = facts
		}()
	}
	wg.Wait()

	// Merge the per-turn results in input order so supersession is
	// deterministic. Late-timestamped facts still win on timestamp
	// ties because the "later wins" comparator in factSupersedes
	// doesn't depend on iteration order for the tie case.
	for _, facts := range perTurn {
		for _, f := range facts {
			// Preserve every extracted fact in allRaw BEFORE the
			// supersession pass so timeline consumers see the full
			// chronology, not just the "latest winning" snapshot.
			idx.allRaw = append(idx.allRaw, f)
			key := f.Subject + "\x00" + f.Predicate
			existing, has := idx.bySubjectPredicate[key]
			if has {
				if !factSupersedes(f, existing) {
					continue
				}
			}
			idx.bySubjectPredicate[key] = f
		}
	}
	// Rebuild bySubject / all from the final winning set so consumers
	// never see superseded facts.
	for _, f := range idx.bySubjectPredicate {
		idx.bySubject[f.Subject] = append(idx.bySubject[f.Subject], f)
		idx.all = append(idx.all, f)
	}
	// Deterministic order for tests and snapshot reproducibility.
	for sub := range idx.bySubject {
		sort.Slice(idx.bySubject[sub], func(i, j int) bool {
			return idx.bySubject[sub][i].Predicate < idx.bySubject[sub][j].Predicate
		})
	}
	sort.Slice(idx.all, func(i, j int) bool {
		if idx.all[i].Subject != idx.all[j].Subject {
			return idx.all[i].Subject < idx.all[j].Subject
		}
		return idx.all[i].Predicate < idx.all[j].Predicate
	})
	return idx
}

// factSupersedes returns true when incoming should replace existing.
// Rules:
//  1. Later timestamp wins.
//  2. On timestamp tie, higher confidence wins.
//  3. On confidence tie, LLM source beats rule source.
//  4. Otherwise keep existing (stable: first write wins on complete
//     ties, so tests are deterministic).
func factSupersedes(incoming, existing Fact) bool {
	if incoming.Timestamp.After(existing.Timestamp) {
		return true
	}
	if existing.Timestamp.After(incoming.Timestamp) {
		return false
	}
	if incoming.Confidence > existing.Confidence {
		return true
	}
	if existing.Confidence > incoming.Confidence {
		return false
	}
	return incoming.Source > existing.Source
}

// predicateIsTrait reports whether a fact predicate describes an
// enduring trait / preference / identity marker (rendered as an
// undated profile entry) versus a dated event (rendered on a
// timeline). The classification is intentional and matters at
// render time: showing a trait like "likes classical music" with a
// specific date misleads the answerer into treating enduring
// preferences as transient events, which regresses counterfactual
// inference questions on LoCoMo cat-3 ("would X likely enjoy Y?").
//
// Everything not explicitly classified as a trait is treated as an
// event so that future / LLM-extracted predicates default to the
// timestamped surface (safe for time-arithmetic questions).
func predicateIsTrait(predicate string) bool {
	if predicate == "" {
		return false
	}
	// favorite-X family is always a trait (stable preference).
	if strings.HasPrefix(predicate, "favorite-") {
		return true
	}
	switch predicate {
	case
		"is",
		"likes", "dislikes",
		"has-interest", "has-job", "has-hobby",
		"identifies-as",
		"lives-in", "works-at",
		"raised-in", "moved-from",
		"pursuing", "owns",
		"status", "relationship-status":
		return true
	}
	return false
}

// Lookup returns all facts matching a query's subject entities. The
// caller passes the set of capitalised entities extracted from the
// question (via extractEntities). Returns the union of matching
// facts sorted by predicate for deterministic output.
//
// Nil receiver is safe and returns an empty slice — useful for
// callers that don't yet have a fact index populated.
func (idx *factIndex) Lookup(subjects []string) []Fact {
	if idx == nil || len(subjects) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []Fact
	for _, s := range subjects {
		key := titleCaseName(s)
		for _, f := range idx.bySubject[key] {
			id := f.SourceTurnID + "\x00" + f.Predicate
			if seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, f)
		}
	}
	return out
}
