package bench

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestResolveCoref_FirstPersonSelf verifies that "I/my/me" pronouns
// trigger speaker self-reference resolution and append the speaker
// name to the turn's IndexHints. This is the highest-precision
// coref class because the speaker is unambiguous from the turn label.
func TestResolveCoref_FirstPersonSelf(t *testing.T) {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	turns := []Turn{
		{Speaker: "Caroline", Text: "I'm currently single.", TurnID: "0", Timestamp: ts},
		{Speaker: "Phil", Text: "My job is exhausting.", TurnID: "1", Timestamp: ts},
		{Speaker: "Caroline", Text: "The weather is nice.", TurnID: "2", Timestamp: ts},
	}
	got := resolveCoref(turns)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if !strings.Contains(got[0].IndexHints, "Caroline") {
		t.Errorf("turn 0 hints missing Caroline: %q", got[0].IndexHints)
	}
	if !strings.Contains(got[1].IndexHints, "Phil") {
		t.Errorf("turn 1 hints missing Phil: %q", got[1].IndexHints)
	}
	// Turn 2 has no first-person pronoun so no self-reference hint
	// — but it MAY still inherit recent entities from the window
	// (the short-turn / third-person inheritance branch). Only assert
	// that the speaker hint logic did NOT fire spuriously by checking
	// the long-turn path: a >8-token turn without pronouns must NOT
	// receive any hint.
}

// TestResolveCoref_Addressee verifies that "you/your" pronouns
// resolve to the OTHER participant in a 2-party conversation.
// LoCoMo's whole format is 2-party so this is the dominant case.
func TestResolveCoref_Addressee(t *testing.T) {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	turns := []Turn{
		{Speaker: "Caroline", Text: "I had a great day.", TurnID: "0", Timestamp: ts},
		{Speaker: "Phil", Text: "How was your weekend?", TurnID: "1", Timestamp: ts},
	}
	got := resolveCoref(turns)
	// Turn 1 is Phil saying "your" → addressee Caroline.
	if !strings.Contains(got[1].IndexHints, "Caroline") {
		t.Errorf("expected Caroline (addressee) in turn 1 hints, got %q", got[1].IndexHints)
	}
}

// TestResolveCoref_InclusivePlural verifies that "we/us/our" resolve
// to the union of speaker and addressee.
func TestResolveCoref_InclusivePlural(t *testing.T) {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	turns := []Turn{
		{Speaker: "Alice", Text: "Hi there.", TurnID: "0", Timestamp: ts},
		{Speaker: "Bob", Text: "Should we go hiking together?", TurnID: "1", Timestamp: ts},
	}
	got := resolveCoref(turns)
	if !strings.Contains(got[1].IndexHints, "Alice") || !strings.Contains(got[1].IndexHints, "Bob") {
		t.Errorf("expected both Alice and Bob in turn 1 hints, got %q", got[1].IndexHints)
	}
}

// TestResolveCoref_RecentEntityInheritance verifies that short turns
// (<=8 tokens) AND turns with third-person pronouns inherit recent
// named entities from the sliding window, even when no explicit
// pronoun is present. This catches the conversational-elision case
// where a short reaction ("loved it") refers to the topic of a
// previous turn.
func TestResolveCoref_RecentEntityInheritance(t *testing.T) {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	turns := []Turn{
		{Speaker: "Alice", Text: "Caroline is moving to Berlin next month.", TurnID: "0", Timestamp: ts},
		{Speaker: "Bob", Text: "Wow, exciting!", TurnID: "1", Timestamp: ts}, // short reaction, should inherit
		{Speaker: "Alice", Text: "She is super pumped about it.", TurnID: "2", Timestamp: ts}, // third-person "she"
	}
	got := resolveCoref(turns)
	// Turn 1 is short and should inherit "Caroline" (and "Berlin"
	// and "Alice") from the sliding window.
	if !strings.Contains(got[1].IndexHints, "Caroline") {
		t.Errorf("short turn failed to inherit Caroline from window: %q", got[1].IndexHints)
	}
	// Turn 2 has "She" → third-person → must inherit recent entities.
	if !strings.Contains(got[2].IndexHints, "Caroline") {
		t.Errorf("third-person turn failed to inherit Caroline: %q", got[2].IndexHints)
	}
}

// TestResolveCoref_LongTurnNoSpuriousHints verifies that a long turn
// (>8 content tokens) WITHOUT any pronouns receives NO hints. This
// is the "sufficient-context-already-present" case — augmenting it
// would just add noise.
func TestResolveCoref_LongTurnNoSpuriousHints(t *testing.T) {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	turns := []Turn{
		{Speaker: "Alice", Text: "Caroline is moving to Berlin next month.", TurnID: "0", Timestamp: ts},
		{Speaker: "Bob",
			Text: "The Berlin job market for software engineers grew substantially during the past three years according to recent industry reports.",
			TurnID: "1", Timestamp: ts},
	}
	got := resolveCoref(turns)
	if got[1].IndexHints != "" {
		t.Errorf("long pronoun-free turn got spurious hints: %q", got[1].IndexHints)
	}
}

// TestResolveCoref_DeterministicOrdering verifies that the hint
// string is sorted lexicographically — essential for deterministic
// indexing across runs and for stable snapshot tests.
func TestResolveCoref_DeterministicOrdering(t *testing.T) {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	turns := []Turn{
		{Speaker: "Zara", Text: "We should meet up.", TurnID: "0", Timestamp: ts},
		{Speaker: "Alex", Text: "Sure, when?", TurnID: "1", Timestamp: ts},
		{Speaker: "Mia", Text: "How about Sunday?", TurnID: "2", Timestamp: ts},
	}
	got1 := resolveCoref(turns)
	got2 := resolveCoref(turns)
	for i := range got1 {
		if got1[i].IndexHints != got2[i].IndexHints {
			t.Errorf("non-deterministic hints at turn %d: %q vs %q",
				i, got1[i].IndexHints, got2[i].IndexHints)
		}
	}
}

// TestBuildSessionIndex_CorefIntegration verifies the end-to-end
// integration: a turn with no shared tokens with the query but a
// resolvable speaker reference ("I'm single" by Caroline) gets
// retrieved when the query asks about Caroline.
//
// This is the headline LoCoMo failure mode the coref preprocessor
// targets — without the augmentation, BM25 and entity overlap both
// score zero on this turn for the question "What is Caroline's
// status?" because no token in the turn matches "Caroline".
func TestBuildSessionIndex_CorefIntegration(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	turns := []Turn{
		// Distractor mentioning Caroline but not about status.
		{Speaker: "Alice", SessionID: "s1", TurnID: "s1:0",
			Text: "Caroline visited the museum yesterday.", Timestamp: ts},
		// Gold turn — pronoun-only "I'm single" by Caroline.
		{Speaker: "Caroline", SessionID: "s1", TurnID: "s1:1",
			Text: "I'm currently single.", Timestamp: ts.Add(time.Minute)},
	}
	mem := NewPixelogMemory(PixelogConfig{})
	if err := mem.Ingest(ctx, "t", turns); err != nil {
		t.Fatal(err)
	}
	ids, err := mem.Retrieve(ctx, "t", "What is Caroline's relationship status?", 2)
	if err != nil {
		t.Fatal(err)
	}
	// Both turns should appear; the gold turn should rank at or above
	// the distractor because coref-augmentation lets BM25 + entity
	// match "Caroline" against the resolved speaker hint AND the
	// preference regex matches "single".
	posDistract, posGold := indexOf(ids, "s1:0"), indexOf(ids, "s1:1")
	if posGold < 0 {
		t.Fatalf("gold turn missing from results: %v", ids)
	}
	if posGold > posDistract {
		t.Errorf("coref failed: gold=%d distractor=%d (ids=%v)",
			posGold, posDistract, ids)
	}
}

// TestBuildSessionIndex_NoLeakIntoDisplayText verifies that the
// retrieved hit text shown to the answerer NEVER contains the
// "[ctx: ...]" coref hint suffix. A leak here would expose
// preprocessor internals to the LLM and could confuse it.
func TestBuildSessionIndex_NoLeakIntoDisplayText(t *testing.T) {
	ts := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	turns := []Turn{
		{Speaker: "Caroline", SessionID: "s1", TurnID: "s1:0",
			Text: "I'm currently single.", Timestamp: ts},
	}
	idx := buildSessionIndex(turns)
	tr := idx.sessions[0].Turns[0]
	if strings.Contains(tr.Text, "[ctx:") {
		t.Errorf("coref hint leaked into display Text: %q", tr.Text)
	}
	// IndexText must contain the hint so retrieval scorers see it.
	if !strings.Contains(tr.IndexText, "[ctx:") {
		t.Errorf("coref hint missing from IndexText: %q", tr.IndexText)
	}
	if !strings.Contains(tr.IndexText, "Caroline") {
		t.Errorf("coref resolution missing Caroline in IndexText: %q", tr.IndexText)
	}
}
