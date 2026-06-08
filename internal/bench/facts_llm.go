package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// FactCompletion is the minimal interface required of an LLM provider
// to drive LLMFactExtractor. The interface is deliberately narrower
// than AnswerLLM (which has a benchmark-specific shape) so any chat
// completion client can be adapted without pulling in answer-format
// concerns. A nil response is treated as "no facts extractable from
// this turn" — equivalent to the rule extractor returning empty.
//
// Implementations MUST be safe for concurrent use: the index-build
// path may invoke the extractor from multiple goroutines bounded by
// extractFactsConcurrency.
type FactCompletion interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// LLMFactExtractor extracts facts from a turn by prompting an LLM to
// emit a strict JSON array of triples. It complements ruleFactExtractor
// (which has perfect precision but narrow recall) by handling the
// long tail of natural-language assertions that don't match any of
// the templated patterns:
//
//   - "Caroline mentioned she went hiking last weekend."
//     (third-person assertion)
//   - "After my divorce I started running marathons."
//     (multi-clause biographical fact)
//   - "I think I'd describe myself as introverted."
//     (hedged self-attribution)
//
// The extractor is OFF by default — callers must supply an
// LLMFactExtractor explicitly via PixelogConfig.FactExtractor when
// they want LLM-driven extraction. This keeps the default path
// zero-cost and lets benchmark runners control spend deliberately.
//
// Output facts always carry FactSource=FactSourceLLM and
// Confidence=0.9, which beats ruleFactExtractor (0.7-0.9) under the
// supersession comparator — so an LLM extraction overrides a rule
// extraction at the same (subject, predicate) only when timestamps
// tie, which is the safest default.
type LLMFactExtractor struct {
	// LLM is the completion client. Required; New panics if nil.
	LLM FactCompletion
	// Model is an opaque identifier passed through to the LLM client
	// (used by some providers for routing / billing). Optional; the
	// client implementation owns its meaning.
	Model string
	// MaxFactsPerTurn caps the number of facts emitted from a single
	// turn so a degenerate model output can't blow up the index.
	// Defaults to 5; values <= 0 use the default.
	MaxFactsPerTurn int
	// MaxObjectChars trims overly-long object strings the LLM may
	// emit (e.g. when it copies a paragraph instead of an entity).
	// Defaults to 80; values <= 0 use the default.
	MaxObjectChars int
}

// NewLLMFactExtractor wires a FactCompletion client into the
// extractor. Panics on nil LLM — a programming error in the caller.
func NewLLMFactExtractor(llm FactCompletion) *LLMFactExtractor {
	if llm == nil {
		panic("NewLLMFactExtractor: nil LLM client")
	}
	return &LLMFactExtractor{LLM: llm}
}

// Extract implements FactExtractor. Sends the speaker + utterance to
// the LLM, parses the JSON response, and clamps the result against
// MaxFactsPerTurn / MaxObjectChars before returning.
//
// Failure modes (all non-fatal — return empty + nil error so the
// caller's index build keeps going):
//   - LLM returns non-JSON / malformed output: skip the turn.
//   - JSON parses but contains no facts: empty list.
//   - JSON contains too many facts: truncate to MaxFactsPerTurn.
//
// The extractor only returns a non-nil error when the LLM client
// itself fails (network, auth, context-cancelled). Callers in
// buildFactIndex already swallow extractor errors per turn, so
// transient infra failures degrade gracefully to lexical-only.
func (e *LLMFactExtractor) Extract(turn Turn) ([]Fact, error) {
	if turn.Text == "" || turn.Speaker == "" {
		return nil, nil
	}
	maxFacts := e.MaxFactsPerTurn
	if maxFacts <= 0 {
		maxFacts = 5
	}
	maxObj := e.MaxObjectChars
	if maxObj <= 0 {
		maxObj = 80
	}

	prompt := buildLLMFactPrompt(turn)
	// 12-second budget is generous for a single extraction; production
	// callers can wrap with their own context if they need stricter
	// limits.
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	resp, err := e.LLM.Complete(ctx, llmFactSystemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm fact extract: %w", err)
	}
	parsed, err := parseLLMFactResponse(resp)
	if err != nil {
		// Malformed output is not an infra error — degrade silently.
		return nil, nil
	}
	if len(parsed) == 0 {
		return nil, nil
	}
	if len(parsed) > maxFacts {
		parsed = parsed[:maxFacts]
	}

	facts := make([]Fact, 0, len(parsed))
	for _, p := range parsed {
		subj := strings.TrimSpace(p.Subject)
		pred := strings.ToLower(strings.TrimSpace(p.Predicate))
		obj := strings.TrimSpace(p.Object)
		if subj == "" || pred == "" || obj == "" {
			continue
		}
		if len(obj) > maxObj {
			obj = obj[:maxObj]
		}
		// Default the subject to the speaker when the LLM emits a
		// pronoun ("I", "me") rather than a name. The rule extractor
		// has the same convention, so consumers see consistent
		// subjects regardless of which extractor produced the fact.
		if isFirstPersonPronoun(subj) {
			subj = turn.Speaker
		}
		facts = append(facts, Fact{
			Subject:         titleCaseName(subj),
			Predicate:       pred,
			Object:          obj,
			SourceTurnID:    turn.TurnID,
			SourceSessionID: turn.SessionID,
			Timestamp:       turn.Timestamp,
			Confidence:      0.9,
			Source:          FactSourceLLM,
		})
	}
	return facts, nil
}

// llmFactSystemPrompt is the role-fixing system message. It primes
// the model for strict-JSON triple emission and discourages it from
// inventing predicates that don't match any plausible question form.
const llmFactSystemPrompt = `You extract structured facts from one conversational utterance at a time.
Output STRICT JSON only — an array of objects with exactly the keys
{"subject", "predicate", "object"}. No prose, no markdown, no
trailing commas. Emit at most 5 facts. If the utterance contains no
extractable assertion, emit an empty array [].

A fact is a concrete assertion the speaker (or the utterance)
explicitly makes about a person, place, thing, or relationship.
Examples:
  utterance: "I'm a software engineer at Google."
  facts: [{"subject":"speaker","predicate":"works-at","object":"Google"},
          {"subject":"speaker","predicate":"has-job","object":"software engineer"}]

  utterance: "Caroline moved to Berlin three years ago."
  facts: [{"subject":"Caroline","predicate":"lives-in","object":"Berlin"}]

  utterance: "I think the weather might be okay."
  facts: []

Use "speaker" as the subject for first-person assertions; the
caller will resolve it to the actual speaker name. Predicates must
be lowercase, hyphenated where multi-word: "lives-in", "works-at",
"relationship-status", "favorite-book", etc.`

// buildLLMFactPrompt assembles the per-turn user message. The turn
// text is wrapped in a clearly-delimited block so prompt-injected
// content can't escape into instruction territory. The speaker is
// surfaced explicitly so the LLM can resolve "I" without our
// post-processing — though we still resolve it ourselves for safety.
func buildLLMFactPrompt(turn Turn) string {
	var sb strings.Builder
	sb.WriteString("Extract facts from the following utterance.\n\n")
	sb.WriteString("Speaker: ")
	sb.WriteString(turn.Speaker)
	sb.WriteString("\n")
	if !turn.Timestamp.IsZero() {
		sb.WriteString("Timestamp: ")
		sb.WriteString(turn.Timestamp.Format("2006-01-02"))
		sb.WriteString("\n")
	}
	sb.WriteString("Utterance:\n<<<\n")
	sb.WriteString(turn.Text)
	sb.WriteString("\n>>>\n\nReturn JSON array now:")
	return sb.String()
}

// llmFactRaw is the parse target for the LLM's JSON response. Lower-
// cased so the parser tolerates inconsistent casing in the model's
// output (Claude / GPT both occasionally drift between "Subject" and
// "subject").
type llmFactRaw struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
}

// parseLLMFactResponse handles two shapes the LLM commonly emits:
//  1. A bare JSON array (the requested shape).
//  2. A markdown-fenced JSON block ```json [ ... ] ``` (Claude's
//     reflexive default).
// Anything else returns an error and the caller treats the turn as
// having no extractable facts.
func parseLLMFactResponse(s string) ([]llmFactRaw, error) {
	body := strings.TrimSpace(s)
	if body == "" {
		return nil, fmt.Errorf("empty response")
	}
	// Strip ```json ... ``` fences if present.
	if strings.HasPrefix(body, "```") {
		body = strings.TrimPrefix(body, "```json")
		body = strings.TrimPrefix(body, "```")
		if i := strings.LastIndex(body, "```"); i >= 0 {
			body = body[:i]
		}
		body = strings.TrimSpace(body)
	}
	// Tolerate prefix prose by clipping to the first '['.
	if i := strings.Index(body, "["); i > 0 {
		body = body[i:]
	}
	if !strings.HasPrefix(body, "[") {
		return nil, fmt.Errorf("not a JSON array")
	}
	var out []llmFactRaw
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// isFirstPersonPronoun returns true for the subject placeholders the
// LLM tends to emit instead of a name. Deliberately narrow — we do
// NOT match "we", "us", "they", since those imply multiple subjects
// and the caller can't reliably resolve them without coref context.
func isFirstPersonPronoun(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "speaker", "i", "me", "myself", "the speaker":
		return true
	}
	return false
}
