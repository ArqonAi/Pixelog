package memory

import (
	"context"
	"strings"
	"testing"
	"time"
)

// helpers for building a known era graph deterministically.
func childAt(name, l0, l1, concept string, hour int, buried bool) ChildRef {
	return ChildRef{
		URI:       "pixe://capsule/" + name,
		Hash:      name,
		Level:     LevelSession,
		L0:        l0,
		L1:        l1,
		Concepts:  []string{concept},
		StartedAt: time.Date(2026, 1, 1, hour, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 1, 1, hour, 30, 0, 0, time.UTC),
		Buried:    buried,
	}
}

// TestDeepRetrieve_SurfaceMatch finds matches at depth 0 without descent.
func TestDeepRetrieve_SurfaceMatch(t *testing.T) {
	era := NewEraCapsule("ns", LevelDay, []ChildRef{
		childAt("a", "talked about quantum entanglement", "", "fact", 1, false),
		childAt("b", "lunch order at the deli", "", "event", 2, false),
	})
	era.L0 = "today's day summary"
	era.L1 = "longer overview about science and food"
	era.Finalize()

	res, err := DeepRetrieve(context.Background(), nil, []*EraCapsule{era}, DeepRetrieveOptions{
		Query:      "quantum entanglement",
		MaxDepth:   1,
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("DeepRetrieve: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected at least one match")
	}
	// Top hit should be the surface child a; the era root may also rank
	// because of the L1 overlap.
	foundA := false
	for _, r := range res {
		if r.Hash == "a" {
			foundA = true
			break
		}
	}
	if !foundA {
		t.Errorf("child 'a' missing from results: %+v", res)
	}
}

// TestDeepRetrieve_BuriedSkippedWhenSurfaceOnly: SurfaceOnly excludes
// buried children, modeling normal "conscious" recall.
func TestDeepRetrieve_BuriedSkippedWhenSurfaceOnly(t *testing.T) {
	era := NewEraCapsule("ns", LevelDay, []ChildRef{
		childAt("kept", "ordinary chitchat", "", "event", 1, false),
		childAt("repressed", "secret about quantum entanglement", "", "fact", 2, true),
	})
	era.L0 = "regular day"
	era.L1 = "overview"
	era.Finalize()

	conscious, err := DeepRetrieve(context.Background(), nil, []*EraCapsule{era}, DeepRetrieveOptions{
		Query:       "quantum entanglement",
		MaxDepth:    1,
		MaxResults:  10,
		SurfaceOnly: true,
	})
	if err != nil {
		t.Fatalf("DeepRetrieve: %v", err)
	}
	for _, r := range conscious {
		if r.Hash == "repressed" {
			t.Error("SurfaceOnly should exclude buried children")
		}
	}

	// Without SurfaceOnly ("free association"), the repressed memory surfaces.
	subconscious, _ := DeepRetrieve(context.Background(), nil, []*EraCapsule{era}, DeepRetrieveOptions{
		Query:      "quantum entanglement",
		MaxDepth:   1,
		MaxResults: 10,
	})
	found := false
	for _, r := range subconscious {
		if r.Hash == "repressed" {
			found = true
			if !r.Buried {
				t.Errorf("expected Buried=true on repressed result")
			}
			break
		}
	}
	if !found {
		t.Errorf("repressed memory should surface in deep retrieval; got %+v", subconscious)
	}
}

// TestDeepRetrieve_DepthBudget caps traversal depth.
func TestDeepRetrieve_DepthBudget(t *testing.T) {
	// Build a 3-level hierarchy: week → day → session.
	store, _ := NewCapsuleStore(t.TempDir())

	// Day era with one matching session.
	day := NewEraCapsule("ns", LevelDay, []ChildRef{
		childAt("session-deep", "rare phrase tachyon ribbons", "", "fact", 1, false),
	})
	day.L0 = "day overview"
	day.L1 = "day overview"
	day.Finalize()
	store.PutEra(day)

	// Week era pointing at the day era.
	week := NewEraCapsule("ns", LevelWeek, []ChildRef{{
		URI:       day.URI(),
		Hash:      day.Hash,
		Level:     LevelDay,
		L0:        "day overview",
		L1:        "day overview",
		StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 1, 1, 23, 59, 0, 0, time.UTC),
	}})
	week.L0 = "this week"
	week.L1 = "weekly notes"
	week.Finalize()

	r := NewCapsuleResolver(store)

	// MaxDepth=0 → only the week's own tiers checked; tachyon is two
	// hops away so no match expected.
	depth0, err := DeepRetrieve(context.Background(), r, []*EraCapsule{week}, DeepRetrieveOptions{
		Query:      "tachyon ribbons",
		MaxDepth:   0,
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("depth0: %v", err)
	}
	for _, hit := range depth0 {
		if hit.Hash == "session-deep" {
			t.Errorf("MaxDepth=0 should not reach session: %+v", hit)
		}
	}

	// MaxDepth=2 → week → day (resolve) → session, match found.
	depth2, err := DeepRetrieve(context.Background(), r, []*EraCapsule{week}, DeepRetrieveOptions{
		Query:      "tachyon ribbons",
		MaxDepth:   3,
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("depth2: %v", err)
	}
	if !containsHash(depth2, "session-deep") {
		t.Errorf("MaxDepth=3 should reach session-deep: %+v", depth2)
	}
}

// TestDeepRetrieve_Threshold filters out weak matches.
func TestDeepRetrieve_Threshold(t *testing.T) {
	era := NewEraCapsule("ns", LevelDay, []ChildRef{
		childAt("a", "a single shared word zebra", "", "fact", 1, false),
	})
	era.L0 = "totally unrelated content here"
	era.L1 = "more unrelated content"
	era.Finalize()

	high, _ := DeepRetrieve(context.Background(), nil, []*EraCapsule{era}, DeepRetrieveOptions{
		Query: "zebra crossing",
		// Default threshold is 0.05 — Jaccard("zebra crossing", "a single shared word zebra") ≈ 1/(2+5-1) = 0.166
		MaxDepth:   1,
		MaxResults: 10,
	})
	if !containsHash(high, "a") {
		t.Errorf("expected match for child 'a': %+v", high)
	}

	none, _ := DeepRetrieve(context.Background(), nil, []*EraCapsule{era}, DeepRetrieveOptions{
		Query:      "zebra crossing",
		Threshold:  0.9, // impossibly high
		MaxDepth:   1,
		MaxResults: 10,
	})
	if len(none) != 0 {
		t.Errorf("threshold should exclude all results, got %d", len(none))
	}
}

// TestDefaultMatchFn is a sanity check on the lexical baseline.
func TestDefaultMatchFn(t *testing.T) {
	if got := DefaultMatchFn("alpha beta", "beta gamma"); got <= 0 {
		t.Errorf("expected positive Jaccard, got %v", got)
	}
	if got := DefaultMatchFn("alpha", "alpha"); got != 1 {
		t.Errorf("identical strings = %v, want 1", got)
	}
	if got := DefaultMatchFn("alpha", "beta"); got != 0 {
		t.Errorf("disjoint strings = %v, want 0", got)
	}
	if got := DefaultMatchFn("", ""); got != 0 {
		t.Errorf("empty strings = %v, want 0", got)
	}
}

func containsHash(results []DeepRetrieveResult, hash string) bool {
	for _, r := range results {
		if r.Hash == hash {
			return true
		}
	}
	return false
}

// _ keep imports tidy across tests (strings used in helpers above).
var _ = strings.Join
