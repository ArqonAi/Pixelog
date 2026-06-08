package bench

import (
	"context"
	"testing"
	"time"
)

// testingContext is a tiny helper so per-file tests can share a
// single context without pulling the full net/http/test harness.
func testingContext() context.Context { return context.Background() }

// TestRuleFactExtractor_Patterns verifies each regex pattern emits
// the expected (subject, predicate, object) triple. Subject is always
// the turn's speaker; object is the captured text; predicate is the
// rule-level tag.
func TestRuleFactExtractor_Patterns(t *testing.T) {
	ts := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	ex := RuleFactExtractor()

	cases := []struct {
		name           string
		turn           Turn
		wantPredicates []string // predicates expected (order-insensitive)
		wantObject     string   // a substring to check in the first matching object
	}{
		{
			name:           "relationship-status single",
			turn:           Turn{Speaker: "Caroline", Text: "I'm single right now.", TurnID: "t1", Timestamp: ts},
			wantPredicates: []string{"relationship-status"},
			wantObject:     "single",
		},
		{
			name:           "lives-in",
			turn:           Turn{Speaker: "Alice", Text: "I live in Berlin since 2020.", TurnID: "t2", Timestamp: ts},
			wantPredicates: []string{"lives-in"},
			wantObject:     "Berlin",
		},
		{
			name:           "moved-from",
			turn:           Turn{Speaker: "Caroline", Text: "I moved here from Sweden four years ago.", TurnID: "t3", Timestamp: ts},
			wantPredicates: []string{"moved-from"},
			wantObject:     "Sweden",
		},
		{
			name:           "works-at",
			turn:           Turn{Speaker: "Phil", Text: "I work at Acme Corp as a designer.", TurnID: "t4", Timestamp: ts},
			wantPredicates: []string{"works-at"},
			wantObject:     "Acme",
		},
		{
			name:           "likes",
			turn:           Turn{Speaker: "Melanie", Text: "I love pottery and painting.", TurnID: "t5", Timestamp: ts},
			wantPredicates: []string{"likes"},
			wantObject:     "pottery",
		},
		{
			name:           "favorite-composite",
			turn:           Turn{Speaker: "Bob", Text: "My favorite book is Dune.", TurnID: "t6", Timestamp: ts},
			wantPredicates: []string{"favorite-book"},
			wantObject:     "Dune",
		},
		// Fix 2: counterfactual-inference predicates.
		{
			name:           "raised-in",
			turn:           Turn{Speaker: "Caroline", Text: "I grew up in a small village.", TurnID: "t7", Timestamp: ts},
			wantPredicates: []string{"raised-in"},
			wantObject:     "small village",
		},
		{
			name:           "pursuing",
			turn:           Turn{Speaker: "Caroline", Text: "I'm studying counseling psychology.", TurnID: "t8", Timestamp: ts},
			wantPredicates: []string{"pursuing"},
			wantObject:     "counseling",
		},
		{
			name:           "has-interest",
			turn:           Turn{Speaker: "Caroline", Text: "I'm passionate about social justice.", TurnID: "t9", Timestamp: ts},
			wantPredicates: []string{"has-interest"},
			wantObject:     "social justice",
		},
		{
			name:           "identifies-as",
			turn:           Turn{Speaker: "Caroline", Text: "I'm a runner.", TurnID: "t10", Timestamp: ts},
			wantPredicates: []string{"identifies-as"},
			wantObject:     "runner",
		},
		{
			name:           "owns",
			turn:           Turn{Speaker: "Caroline", Text: "I have a hand-painted bowl.", TurnID: "t11", Timestamp: ts},
			wantPredicates: []string{"owns"},
			wantObject:     "hand-painted bowl",
		},
		{
			name:           "did-event",
			turn:           Turn{Speaker: "Melanie", Text: "I attended a charity race last weekend.", TurnID: "t12", Timestamp: ts},
			wantPredicates: []string{"did-event"},
			wantObject:     "charity race",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			facts, err := ex.Extract(c.turn)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if len(facts) == 0 {
				t.Fatalf("no facts extracted for %q", c.turn.Text)
			}
			// Every fact must carry the expected provenance.
			for _, f := range facts {
				if f.Subject != titleCaseName(c.turn.Speaker) {
					t.Errorf("subject=%q want %q", f.Subject, c.turn.Speaker)
				}
				if f.SourceTurnID != c.turn.TurnID {
					t.Errorf("source turn id mismatch: %q", f.SourceTurnID)
				}
				if f.Source != FactSourceRule {
					t.Errorf("source=%d want FactSourceRule", f.Source)
				}
			}
			// Assert at least one predicate matches the expected set.
			gotPreds := map[string]bool{}
			for _, f := range facts {
				gotPreds[f.Predicate] = true
			}
			matched := false
			for _, p := range c.wantPredicates {
				if gotPreds[p] {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("no predicate matched want=%v got=%v (facts=%+v)",
					c.wantPredicates, gotPreds, facts)
			}
		})
	}
}

// TestBuildFactIndex_Supersession verifies that a later "I'm single"
// supersedes an earlier "I'm dating Phil" on the same (subject,
// predicate) key. This is the update semantic of conversational
// memory.
func TestBuildFactIndex_Supersession(t *testing.T) {
	early := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	turns := []Turn{
		{Speaker: "Caroline", Text: "I'm dating Phil now.", TurnID: "t1", Timestamp: early},
		{Speaker: "Caroline", Text: "I'm single now.", TurnID: "t2", Timestamp: late},
	}
	idx := buildFactIndex(turns, RuleFactExtractor())
	facts := idx.Lookup([]string{"Caroline"})
	if len(facts) == 0 {
		t.Fatal("no facts for Caroline")
	}
	var status Fact
	for _, f := range facts {
		if f.Predicate == "relationship-status" {
			status = f
			break
		}
	}
	if status.Object != "single" {
		t.Errorf("latest status = %q, want 'single' (supersession failed)", status.Object)
	}
	if status.SourceTurnID != "t2" {
		t.Errorf("source turn id = %q, want t2", status.SourceTurnID)
	}
}

// TestFactIndex_LookupBySubject verifies multi-fact retrieval for a
// single subject returns every predicate we know about.
func TestFactIndex_LookupBySubject(t *testing.T) {
	ts := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	turns := []Turn{
		{Speaker: "Caroline", Text: "I'm single.", TurnID: "t1", Timestamp: ts},
		{Speaker: "Caroline", Text: "I live in Berlin.", TurnID: "t2", Timestamp: ts},
		{Speaker: "Caroline", Text: "I love sushi.", TurnID: "t3", Timestamp: ts},
		{Speaker: "Phil", Text: "I work at Acme Corp.", TurnID: "t4", Timestamp: ts},
	}
	idx := buildFactIndex(turns, RuleFactExtractor())
	caroline := idx.Lookup([]string{"Caroline"})
	if len(caroline) < 3 {
		t.Errorf("Caroline facts = %d, want >= 3 (got %+v)", len(caroline), caroline)
	}
	// Phil should not appear in Caroline's lookup.
	for _, f := range caroline {
		if f.Subject != "Caroline" {
			t.Errorf("cross-subject leak: %+v", f)
		}
	}
}

// TestFactIndex_NilSafe verifies nil-receiver Lookup returns empty
// without panic — important for callers that haven't yet populated
// the index.
func TestFactIndex_NilSafe(t *testing.T) {
	var idx *factIndex
	out := idx.Lookup([]string{"Anyone"})
	if len(out) != 0 {
		t.Errorf("nil Lookup returned %d facts, want 0", len(out))
	}
}

// TestFactBoost_Integration_RetrieverLiftsSourceTurn verifies the
// end-to-end retrieval path: a question about Caroline's relationship
// status must surface the fact-source turn ("I'm single") above a
// distractor that shares MORE question tokens. This only passes when
// FactBoost is live.
func TestFactBoost_Integration_RetrieverLiftsSourceTurn(t *testing.T) {
	ts := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	turns := []Turn{
		// Distractor shares "Caroline" + "relationship" tokens with
		// the question but is not the fact source.
		{Speaker: "Alice", SessionID: "s1", TurnID: "t_distract",
			Text: "Caroline's new relationship book club is fun.", Timestamp: ts},
		// Gold fact-source turn: short, no "Caroline" token, no
		// "relationship" token — purely the assertion.
		{Speaker: "Caroline", SessionID: "s1", TurnID: "t_gold",
			Text: "I'm single.", Timestamp: ts.Add(time.Minute)},
	}
	mem := NewPixelogMemory(PixelogConfig{})
	ctx := testingContext()
	if err := mem.Ingest(ctx, "ns", turns); err != nil {
		t.Fatal(err)
	}
	ids, err := mem.Retrieve(ctx, "ns", "What is Caroline's relationship status?", 2)
	if err != nil {
		t.Fatal(err)
	}
	posGold := indexOf(ids, "t_gold")
	posDistract := indexOf(ids, "t_distract")
	if posGold < 0 {
		t.Fatalf("gold missing from results: %v", ids)
	}
	if posGold > posDistract {
		t.Errorf("fact boost failed: gold=%d distract=%d (ids=%v)",
			posGold, posDistract, ids)
	}
}

// TestTitleCaseName covers the edge cases of speaker normalisation.
func TestTitleCaseName(t *testing.T) {
	cases := map[string]string{
		"":          "",
		"caroline":  "Caroline",
		"Caroline":  "Caroline",
		"McDonald":  "McDonald",
		"o'brien":   "O'brien", // first-letter only; fine for our use
		"ALICE":     "ALICE",   // already has uppercase; left alone
	}
	for in, want := range cases {
		got := titleCaseName(in)
		if got != want {
			t.Errorf("titleCaseName(%q) = %q, want %q", in, got, want)
		}
	}
}
