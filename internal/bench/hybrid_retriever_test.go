package bench

import (
	"context"
	"testing"
	"time"
)

// Tests for the session-level hybrid retriever. The deterministic
// HashEmbedder from embedder.go gives a stable semantic score between
// token-overlapping sessions, so these assertions are reproducible.

func TestBuildSessionIndex(t *testing.T) {
	ts := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	turns := []Turn{
		{SessionID: "s1", TurnID: "s1:0", Speaker: "Alice", Text: "I love sushi and ramen.", Timestamp: ts},
		{SessionID: "s1", TurnID: "s1:1", Speaker: "Bob", Text: "Me too.", Timestamp: ts},
		{SessionID: "s2", TurnID: "s2:0", Speaker: "Alice", Text: "Let's go hiking in the mountains.", Timestamp: ts.Add(7 * 24 * time.Hour)},
	}
	idx := buildSessionIndex(turns)
	if idx.N != 2 {
		t.Fatalf("N=%d want 2", idx.N)
	}
	if idx.sessions[0].ID != "s1" {
		t.Errorf("first session = %s", idx.sessions[0].ID)
	}
	if len(idx.sessions[0].TurnIDs) != 2 {
		t.Errorf("s1 turn IDs = %v", idx.sessions[0].TurnIDs)
	}
	// Stopwords should be dropped from Tokens.
	for _, tok := range idx.sessions[0].Tokens {
		if stopwords[tok] {
			t.Errorf("stopword leaked into tokens: %s", tok)
		}
	}
}

func TestExtractEntities(t *testing.T) {
	cases := map[string][]string{
		"When did Caroline go to the LGBTQ support group?": {"Caroline", "LGBTQ"},
		"What is Alice Johnson's profession?":              {"Alice Johnson"},
		"What happened on 2023-05-07?":                     {"2023-05-07"},
	}
	for q, want := range cases {
		got := extractEntities(q)
		if len(got) < len(want) {
			t.Errorf("extractEntities(%q) = %v; want at least %v", q, got, want)
		}
	}
}

func TestComputeRecall(t *testing.T) {
	retrieved := []string{"sid_0", "sid_1", "sid_2", "turn_a", "turn_b"}
	evidence := []string{"sid_1"}
	recall, hit := computeRecall(retrieved, evidence, 0)
	if recall != 1.0 || hit != 1.0 {
		t.Errorf("session hit: recall=%.2f hit=%.2f", recall, hit)
	}

	// Turn-level evidence.
	evidence = []string{"turn_b"}
	recall, hit = computeRecall(retrieved, evidence, 0)
	if recall != 1.0 || hit != 1.0 {
		t.Errorf("turn hit: recall=%.2f hit=%.2f", recall, hit)
	}

	// Miss.
	evidence = []string{"sid_99"}
	recall, hit = computeRecall(retrieved, evidence, 0)
	if recall != 0 || hit != 0 {
		t.Errorf("miss: recall=%.2f hit=%.2f", recall, hit)
	}

	// Partial — 2 of 3 evidence items retrieved.
	retrieved = []string{"a", "b", "c"}
	evidence = []string{"a", "b", "z"}
	recall, hit = computeRecall(retrieved, evidence, 0)
	if recall < 0.66 || recall > 0.67 {
		t.Errorf("partial recall: %.3f want ~0.667", recall)
	}
	if hit != 1.0 {
		t.Errorf("partial hit should be 1.0, got %.2f", hit)
	}

	// Truncation.
	retrieved = []string{"a", "b", "c", "d", "e"}
	evidence = []string{"e"}
	recall, _ = computeRecall(retrieved, evidence, 3)
	if recall != 0 {
		t.Errorf("truncated recall should miss, got %.2f", recall)
	}
}

func TestRetrieveHitsExpectedSession(t *testing.T) {
	ctx := context.Background()
	turns := []Turn{
		{SessionID: "sid_0", TurnID: "sid_0:0", Speaker: "Alice", Text: "Trattoria Lucca is my favourite italian restaurant.", Timestamp: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)},
		{SessionID: "sid_1", TurnID: "sid_1:0", Speaker: "Bob", Text: "I went camping in Yosemite last weekend.", Timestamp: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)},
		{SessionID: "sid_2", TurnID: "sid_2:0", Speaker: "Alice", Text: "Let's grab coffee at the new spot.", Timestamp: time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)},
	}

	mem := NewPixelogMemory(PixelogConfig{})
	ns := "test"
	if err := mem.Ingest(ctx, ns, turns); err != nil {
		t.Fatal(err)
	}

	ids, err := mem.Retrieve(ctx, ns, "What is Alice's favourite Italian restaurant?", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 {
		t.Fatal("no IDs returned")
	}
	// Top session must be sid_0.
	if ids[0] != "sid_0" {
		t.Errorf("top session = %s, want sid_0. Full list: %v", ids[0], ids)
	}
}

// TestRetrieveTurnLevelEvidence exercises the global turn ranking +
// interleaved emit. The right turn lives inside a session whose other
// turns are unrelated noise, so session-level BM25 dilutes its signal.
// The turn must still appear inside the top-K window.
func TestRetrieveTurnLevelEvidence(t *testing.T) {
	ctx := context.Background()
	turns := []Turn{
		// Session 1: one hot turn buried in 9 unrelated noise turns.
		{SessionID: "session_1", TurnID: "D1:0", Speaker: "A", Text: "we walked along the river"},
		{SessionID: "session_1", TurnID: "D1:1", Speaker: "B", Text: "the weather was nice"},
		{SessionID: "session_1", TurnID: "D1:2", Speaker: "A", Text: "we got ice cream"},
		{SessionID: "session_1", TurnID: "D1:3", Speaker: "B", Text: "it was a fun day"},
		{SessionID: "session_1", TurnID: "D1:4", Speaker: "A", Text: "Caroline mentioned majoring in early childhood education studies"},
		{SessionID: "session_1", TurnID: "D1:5", Speaker: "B", Text: "we headed home"},
		{SessionID: "session_1", TurnID: "D1:6", Speaker: "A", Text: "what a great evening"},
		{SessionID: "session_1", TurnID: "D1:7", Speaker: "B", Text: "see you next time"},
		// Session 2..4: each contains one passing mention of a different topic.
		{SessionID: "session_2", TurnID: "D2:0", Speaker: "A", Text: "we discussed travel plans for the summer"},
		{SessionID: "session_3", TurnID: "D3:0", Speaker: "A", Text: "we cooked dinner together"},
		{SessionID: "session_4", TurnID: "D4:0", Speaker: "A", Text: "we watched a movie at home"},
	}

	mem := NewPixelogMemory(PixelogConfig{})
	if err := mem.Ingest(ctx, "ns", turns); err != nil {
		t.Fatal(err)
	}

	ids, err := mem.Retrieve(ctx, "ns", "What did Caroline say about her education studies?", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 {
		t.Fatal("no ids returned")
	}
	// Top-K=5 must contain the hot turn D1:4 even though session_1 is
	// drowning in noise. Pre-fix, the session-only emit emitted only 5
	// session IDs and never surfaced the turn.
	found := false
	for _, id := range ids[:5] {
		if id == "D1:4" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected D1:4 in top-5; got %v", ids[:5])
	}
}

// TestEntityScore_IDFWeighted asserts a rare entity dominates a common
// one — a session that matches only the rare entity must score higher
// than a session that matches only the common one.
func TestEntityScore_IDFWeighted(t *testing.T) {
	turns := []Turn{
		{SessionID: "s_common", TurnID: "s_common:0", Text: "John went to the park"},
		{SessionID: "s_common2", TurnID: "s_common2:0", Text: "John ate lunch"},
		{SessionID: "s_common3", TurnID: "s_common3:0", Text: "John played guitar"},
		{SessionID: "s_rare", TurnID: "s_rare:0", Text: "they visited Trattoria Lucca for dinner"},
	}
	idx := buildSessionIndex(turns)
	entities := []string{"John", "Trattoria"}

	// s_common matches only the common "John".
	commonScore := idx.entityScore("John went to the park", entities)
	// s_rare matches only the rare "Trattoria".
	rareScore := idx.entityScore("they visited Trattoria Lucca", entities)

	if !(rareScore > commonScore) {
		t.Errorf("expected rareScore > commonScore, got rare=%.4f common=%.4f", rareScore, commonScore)
	}
}

// TestPreferenceRE_InferentialCues guards the expanded vocabulary that
// catches LoCoMo-style inferential questions ("would X be considered
// Y", "what fields would Z pursue", "what attributes describe ...").
func TestPreferenceRE_InferentialCues(t *testing.T) {
	hits := []string{
		"What fields would Caroline be likely to pursue?",
		"Would Melanie be considered an ally?",
		"What might John's financial status be?",
		"What attributes describe John?",
		"I prefer espresso to drip coffee.",
		"My favourite city is Lisbon.",
		"I'm interested in modular synthesizers.",
	}
	for _, q := range hits {
		if !preferenceRE.MatchString(q) {
			t.Errorf("preferenceRE missed: %q", q)
		}
	}
	misses := []string{
		"What is the capital of France?",
		"How many doctors are at this clinic?",
	}
	for _, q := range misses {
		if preferenceRE.MatchString(q) {
			t.Errorf("preferenceRE false-positive on: %q", q)
		}
	}
}

func TestRetrieveTemporalBoost(t *testing.T) {
	ctx := context.Background()
	// Three sessions with the same content but different dates. The
	// question references May so the May session must rank first.
	turns := []Turn{
		{SessionID: "s_apr", TurnID: "s_apr:0", Text: "We met for dinner.", Timestamp: time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC)},
		{SessionID: "s_may", TurnID: "s_may:0", Text: "We met for dinner.", Timestamp: time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC)},
		{SessionID: "s_jun", TurnID: "s_jun:0", Text: "We met for dinner.", Timestamp: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)},
	}
	mem := NewPixelogMemory(PixelogConfig{})
	if err := mem.Ingest(ctx, "t", turns); err != nil {
		t.Fatal(err)
	}
	ids, err := mem.Retrieve(ctx, "t", "When did they meet in May 2024?", 1)
	if err != nil {
		t.Fatal(err)
	}
	if ids[0] != "s_may" {
		t.Errorf("top = %s, want s_may (temporal boost failed). Full: %v", ids[0], ids)
	}
}
