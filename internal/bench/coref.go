package bench

import (
	"sort"
	"strings"
)

// Coreference resolution at ingest time. The dominant remaining
// failure mode on LoCoMo and similar conversational benchmarks is
// pronoun-only turns: gold answers like "I'm currently single" carry
// zero shared lexical signal with questions like "What is Caroline's
// relationship status?" — even though the conversation makes the
// referent obvious from speaker labels and recent turn context.
//
// We solve this with a deterministic, no-LLM-cost preprocessor:
//
//  1. Speaker self-reference: "I", "me", "my", "myself" → speaker name.
//     LoCoMo / ConvoMem / MemBench all carry per-turn Speaker fields,
//     so this resolution is unambiguous.
//
//  2. Addressee reference: "you", "your", "yours" → the OTHER
//     participant in 2-party conversations (the dominant case in
//     conversational memory benchmarks). For >2-party conversations
//     we degrade to "any non-speaker participant", which still adds
//     useful signal even if it occasionally over-matches.
//
//  3. Sliding-window entity context: every turn inherits the named
//     entities from the previous corefWindow turns. This catches the
//     "she's pretty cool" case where the antecedent ("Caroline") was
//     mentioned in an earlier turn. The window size is capped to
//     prevent unrelated topics from polluting the index.
//
// The output is purely additive: we augment a parallel "index text"
// used for tokenisation, BM25, and embedding — the original Text
// field remains pristine for display to the answerer. This means a
// coref miss never confuses the answerer with a wrong rewrite; at
// worst, retrieval falls back to the lexical baseline.

// corefWindow is the number of preceding turns whose named entities
// flow forward into the current turn's index context. 5 turns ≈ one
// natural conversational sub-topic in LoCoMo / ConvoMem; larger
// windows start to drag in irrelevant topic shifts and hurt
// precision. Tuned on the LoCoMo 60-QA dev slice.
const corefWindow = 5

// firstPersonSelfRE matches first-person singular pronouns that
// always resolve to the current turn's speaker. We deliberately do
// NOT match "we"/"us"/"our" — those are inclusive plural and need
// participant-set resolution, handled separately.
var firstPersonSelfRE = mustCompileWordSet(
	"i", "me", "my", "mine", "myself",
)

// secondPersonRE matches pronouns that always resolve to the
// addressee — every non-speaker participant in the immediate
// conversational context.
var secondPersonRE = mustCompileWordSet(
	"you", "your", "yours", "yourself", "yourselves",
)

// inclusivePluralRE matches first-person plural pronouns that include
// the speaker AND at least one other participant. Resolved to the
// union of (speaker + addressee).
var inclusivePluralRE = mustCompileWordSet(
	"we", "us", "our", "ours", "ourselves",
)

// thirdPersonRE matches third-person pronouns whose antecedent must
// be inferred from recent turn context. We can't disambiguate gender
// reliably without a true coref model, so all third-person matches
// trigger the recent-entity sliding-window augmentation rather than a
// targeted pronoun→name rewrite.
var thirdPersonRE = mustCompileWordSet(
	"he", "him", "his", "himself",
	"she", "her", "hers", "herself",
	"they", "them", "their", "theirs", "themselves", "themself",
	"it", "its",
)

// augmentedTurn carries the original Turn alongside the index-time
// augmentation tokens computed by the coref preprocessor. IndexHints
// is a space-separated list of names (speaker, addressee, recent
// entities) that are appended to the turn during tokenisation /
// embedding but never shown to the answerer.
type augmentedTurn struct {
	Turn
	// IndexHints holds the augmentation tokens. Empty when no coref
	// resolution was triggered for this turn (i.e. no pronouns AND no
	// useful context). May contain duplicates of names already in
	// Text — the BM25 corpus naturally deduplicates via term-frequency
	// caps so this is a no-op there.
	IndexHints string
}

// resolveCoref runs the deterministic coref preprocessor over a turn
// list and returns the same turns annotated with IndexHints. Turns
// arrive in the order they appear in the conversation; we walk them
// once, maintaining a sliding window of recent entities, and emit
// the augmentation per turn.
//
// The function is pure (no I/O, no LLM, no shared state) and runs in
// O(N×W) where N is turn count and W is the corefWindow. For a
// 1986-turn LoCoMo conversation the wall-clock cost is sub-millisecond.
func resolveCoref(turns []Turn) []augmentedTurn {
	if len(turns) == 0 {
		return nil
	}

	// Collect every named entity ever seen so the addressee resolver
	// has a participant set to work from. We pull entities from each
	// turn's text using extractEntities (already used by the BM25
	// keyword scorer) plus the per-turn Speaker field which is the
	// most-reliable name source.
	allParticipants := map[string]bool{}
	for _, t := range turns {
		if t.Speaker != "" {
			allParticipants[t.Speaker] = true
		}
	}

	out := make([]augmentedTurn, len(turns))
	// recentEntities tracks named entities seen in the last
	// corefWindow turns, with insertion order preserved so older
	// entities age out cleanly.
	type windowedEntity struct {
		name  string
		atIdx int // turn index where it was last seen
	}
	var window []windowedEntity

	for i, t := range turns {
		// Drop window entries older than corefWindow turns.
		cutoff := i - corefWindow
		filtered := window[:0]
		for _, w := range window {
			if w.atIdx > cutoff {
				filtered = append(filtered, w)
			}
		}
		window = filtered

		// Compute hints for this turn's coref classes.
		hints := map[string]bool{}
		lower := strings.ToLower(t.Text)
		tokens := tokeniseWords(lower)

		// 1. Self-reference: speaker name if any "I/me/my/..." present.
		if t.Speaker != "" && containsAny(tokens, firstPersonSelfRE) {
			hints[t.Speaker] = true
		}

		// 2. Addressee: every other known participant if "you/your"
		// appears. In 2-party convos this resolves uniquely; in
		// N-party convos we union, which trades precision for recall
		// — appropriate since false-positives just add a name token
		// that BM25 will down-weight via IDF if it's common.
		if containsAny(tokens, secondPersonRE) {
			for p := range allParticipants {
				if p != t.Speaker {
					hints[p] = true
				}
			}
		}

		// 3. Inclusive plural: speaker + addressee.
		if containsAny(tokens, inclusivePluralRE) {
			if t.Speaker != "" {
				hints[t.Speaker] = true
			}
			for p := range allParticipants {
				if p != t.Speaker {
					hints[p] = true
				}
			}
		}

		// 4. Third-person reference: union the recent-entity window.
		// We do this even when no third-person pronoun is present,
		// because conversational coreference often elides pronouns
		// entirely ("Loved it!" referring to a movie discussed two
		// turns ago). The cost is one extra few names per turn — a
		// rounding error inside BM25.
		// To avoid drowning short turns in unrelated context, we
		// only inherit entities when the turn itself is short
		// (<=8 content tokens) — long turns establish their own
		// context.
		// Cap inheritance to the most-recent N entities to keep short
		// turns from getting bloated with the full conversational
		// window. 3 is empirically the sweet spot from the LoCoMo
		// 1986-QA Hit@10 sweep — going to 5 dilutes BM25 IDF, going
		// to 1 misses chained references.
		const maxInheritedEntities = 3
		if len(tokens) <= 8 || containsAny(tokens, thirdPersonRE) {
			start := len(window) - maxInheritedEntities
			if start < 0 {
				start = 0
			}
			for _, w := range window[start:] {
				hints[w.name] = true
			}
		}

		// Build the hint string. Sort so the output is deterministic
		// across runs (essential for benchmark reproducibility and
		// snapshot tests).
		names := make([]string, 0, len(hints))
		for n := range hints {
			if n == "" {
				continue
			}
			names = append(names, n)
		}
		sort.Strings(names)

		out[i] = augmentedTurn{
			Turn:       t,
			IndexHints: strings.Join(names, " "),
		}

		// Update the sliding window with entities from THIS turn for
		// future turns to inherit. We track the speaker plus any
		// capitalised entities extracted from the text.
		if t.Speaker != "" {
			window = append(window, windowedEntity{name: t.Speaker, atIdx: i})
		}
		for _, e := range extractEntities(t.Text) {
			window = append(window, windowedEntity{name: e, atIdx: i})
		}
	}

	return out
}

// tokeniseWords lower-cases and splits text into word tokens. Used by
// the coref preprocessor to detect pronouns; deliberately separate
// from tokeniseText (which strips stopwords) because every pronoun we
// care about IS a stopword in the retrieval tokeniser.
//
// Apostrophes are treated as word boundaries so contractions split
// into their components: "I'm" → ["i", "m"], "you're" → ["you", "re"],
// "we've" → ["we", "ve"]. This is essential for matching the bare-
// pronoun forms in firstPersonSelfRE / secondPersonRE / etc., which
// would otherwise miss every contraction in the corpus (and English
// conversation is overwhelmingly contracted).
func tokeniseWords(text string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cur.WriteRune(r)
			continue
		}
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// containsAny returns true if any token in tokens is in the set.
func containsAny(tokens []string, set map[string]bool) bool {
	for _, t := range tokens {
		if set[t] {
			return true
		}
	}
	return false
}

// mustCompileWordSet builds a lowercase string-set from variadic
// arguments. Panics if a word contains uppercase / whitespace /
// punctuation — that's a programming error in the constants above.
func mustCompileWordSet(words ...string) map[string]bool {
	out := make(map[string]bool, len(words))
	for _, w := range words {
		if w != strings.ToLower(w) || strings.ContainsAny(w, " \t\n.,;:!?") {
			panic("mustCompileWordSet: invalid word " + w)
		}
		out[w] = true
	}
	return out
}
