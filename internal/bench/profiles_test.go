package bench

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Predicates "started", "came-out-to", "attended" are NOT in the
// trait classification list (they are transient actions) so they
// land in Events and get rendered on a dated timeline.
func TestBuildProfileIndex_EventChronology(t *testing.T) {
	t1 := time.Date(2023, 5, 14, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2023, 8, 22, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2023, 11, 3, 0, 0, 0, 0, time.UTC)
	facts := []Fact{
		// Out-of-order on purpose to verify sorting.
		{Subject: "Caroline", Predicate: "started", Object: "HRT", SourceTurnID: "D1:5", Confidence: 0.9, Timestamp: t2},
		{Subject: "Caroline", Predicate: "came-out-to", Object: "parents", SourceTurnID: "D1:1", Confidence: 0.9, Timestamp: t1},
		{Subject: "Caroline", Predicate: "attended", Object: "first LGBTQ support group", SourceTurnID: "D1:9", Confidence: 0.9, Timestamp: t3},
	}

	idx := buildProfileIndex(facts)
	prof, ok := idx.byName["Caroline"]
	if !ok {
		t.Fatalf("Caroline profile missing")
	}
	if len(prof.Events) != 3 {
		t.Fatalf("expected 3 events, got %d (traits=%d)", len(prof.Events), len(prof.Traits))
	}
	if len(prof.Traits) != 0 {
		t.Errorf("expected 0 traits, got %d: %+v", len(prof.Traits), prof.Traits)
	}
	if !prof.Events[0].Date.Equal(t1) || !prof.Events[1].Date.Equal(t2) || !prof.Events[2].Date.Equal(t3) {
		t.Errorf("events not chronologically sorted: %+v", prof.Events)
	}
}

// Trait predicates ("lives-in") dedupe by (subject, predicate,
// object) with NO date component — so the same assertion echoed
// across turns collapses to one entry, keeping the highest-
// confidence source.
func TestBuildProfileIndex_TraitDedupCollapsesAcrossTurns(t *testing.T) {
	day := time.Date(2023, 5, 14, 12, 0, 0, 0, time.UTC)
	facts := []Fact{
		{Subject: "Caroline", Predicate: "lives-in", Object: "Toronto", SourceTurnID: "D1:1", Confidence: 0.7, Timestamp: day, Source: FactSourceRule},
		{Subject: "Caroline", Predicate: "lives-in", Object: "Toronto", SourceTurnID: "D1:2", Confidence: 0.95, Timestamp: day, Source: FactSourceLLM},
	}
	idx := buildProfileIndex(facts)
	prof := idx.byName["Caroline"]
	if prof == nil || len(prof.Traits) != 1 || len(prof.Events) != 0 {
		t.Fatalf("expected 1 trait & 0 events, got %+v", prof)
	}
	tr := prof.Traits[0]
	if tr.Confidence != 0.95 || tr.SourceTurnID != "D1:2" {
		t.Errorf("expected higher-confidence entry kept, got %+v", tr)
	}
	// Traits carry NO date so the renderer never leaks a misleading
	// timestamp for an enduring preference.
	if !tr.Date.IsZero() {
		t.Errorf("trait should carry zero Date, got %v", tr.Date)
	}
}

// Contradictory trait values for the same (subject, predicate) pair
// — e.g. Caroline lives-in Toronto at t1 and lives-in Berlin at t2
// — are BOTH preserved as separate trait entries, because the
// profile records everything known, not just the current state.
// Supersession is the job of factIndex, not profileIndex.
func TestBuildProfileIndex_TraitPreservesBothObjects(t *testing.T) {
	t1 := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	facts := []Fact{
		{Subject: "Caroline", Predicate: "lives-in", Object: "Toronto", SourceTurnID: "D1:1", Confidence: 0.9, Timestamp: t1},
		{Subject: "Caroline", Predicate: "lives-in", Object: "Berlin", SourceTurnID: "D2:5", Confidence: 0.9, Timestamp: t2},
	}
	idx := buildProfileIndex(facts)
	prof := idx.byName["Caroline"]
	if prof == nil || len(prof.Traits) != 2 {
		t.Fatalf("expected 2 traits (both kept), got %+v", prof)
	}
	// Trait ordering: predicate asc, then object asc. Both are
	// predicate="lives-in" so object-asc: Berlin < Toronto.
	if prof.Traits[0].Object != "Berlin" || prof.Traits[1].Object != "Toronto" {
		t.Errorf("trait object order unexpected: %+v", prof.Traits)
	}
}

func TestProfileIndex_Lookup_NormalisesCase(t *testing.T) {
	t1 := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	idx := buildProfileIndex([]Fact{
		// "is" is a trait predicate.
		{Subject: "caroline", Predicate: "is", Object: "a writer", SourceTurnID: "D1:1", Confidence: 0.9, Timestamp: t1},
	})
	got := idx.Lookup([]string{"Caroline"})
	if len(got) != 1 || got[0].Name != "Caroline" {
		t.Fatalf("case-insensitive lookup failed: %+v", got)
	}
	if len(got[0].Traits) != 1 {
		t.Errorf("expected 1 trait on 'is' predicate, got %+v", got[0])
	}
	// Empty / nil receiver should be safe.
	if (*profileIndex)(nil).Lookup([]string{"X"}) != nil {
		t.Error("nil-receiver lookup should return nil")
	}
}

// formatProfilePreamble renders traits undated and events on a
// timeline in the same profile block. Header is "KNOWN PROFILES:".
func TestFormatProfilePreamble_Shape(t *testing.T) {
	t1 := time.Date(2023, 5, 14, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2023, 8, 22, 0, 0, 0, 0, time.UTC)
	profs := []*EntityProfile{{
		Name: "Caroline",
		Traits: []ProfileEntry{
			{Predicate: "likes", Object: "classical music"},
			{Predicate: "identifies-as", Object: "counselor"},
		},
		Events: []ProfileEntry{
			{Date: t1, Predicate: "came-out-to", Object: "parents"},
			{Date: t2, Predicate: "started", Object: "HRT"},
		},
	}}
	out := formatProfilePreamble(profs, defaultProfilePreambleConfig())
	if !strings.HasPrefix(out, "KNOWN PROFILES:\n") {
		t.Fatalf("missing header: %q", out)
	}
	if !strings.Contains(out, "Caroline:") {
		t.Errorf("missing entity name: %q", out)
	}
	// Traits section: undated, dash-prefixed.
	if !strings.Contains(out, "    - likes classical music") {
		t.Errorf("missing trait entry 'likes classical music': %q", out)
	}
	if !strings.Contains(out, "    - identifies-as counselor") {
		t.Errorf("missing trait entry 'identifies-as counselor': %q", out)
	}
	// Events section: dated.
	if !strings.Contains(out, "    2023-05-14: came-out-to parents") {
		t.Errorf("missing event entry came-out-to: %q", out)
	}
	if !strings.Contains(out, "    2023-08-22: started HRT") {
		t.Errorf("missing event entry started HRT: %q", out)
	}
	// Traits section must appear before events so the answerer
	// sees enduring context first.
	trIdx := strings.Index(out, "  traits:")
	evIdx := strings.Index(out, "  events:")
	if trIdx < 0 || evIdx < 0 || trIdx >= evIdx {
		t.Errorf("traits section must precede events section; trIdx=%d evIdx=%d\n%s", trIdx, evIdx, out)
	}
	// Empty / nil should render empty.
	if formatProfilePreamble(nil, defaultProfilePreambleConfig()) != "" {
		t.Error("nil profiles should yield empty string")
	}
	// Zero budget should yield empty.
	if formatProfilePreamble(profs, profilePreambleConfig{}) != "" {
		t.Error("zero-budget config should yield empty string")
	}
}

func TestFormatProfilePreamble_RespectsCaps(t *testing.T) {
	// 3 entities, cap to 2. Each carries a single trait so the
	// block renders deterministically.
	profs := []*EntityProfile{
		{Name: "Alice", Traits: []ProfileEntry{{Predicate: "is", Object: "A"}}},
		{Name: "Bob", Traits: []ProfileEntry{{Predicate: "is", Object: "B"}}},
		{Name: "Carol", Traits: []ProfileEntry{{Predicate: "is", Object: "C"}}},
	}
	out := formatProfilePreamble(profs, profilePreambleConfig{MaxEntities: 2, MaxTraits: 5, MaxEvents: 5})
	if !strings.Contains(out, "Alice:") || !strings.Contains(out, "Bob:") {
		t.Errorf("expected first two entities: %q", out)
	}
	if strings.Contains(out, "Carol:") {
		t.Errorf("expected Carol to be dropped past cap: %q", out)
	}

	// Per-section caps. Build one entity with 10 traits + 10 events
	// and assert the renderer respects both limits independently.
	long := &EntityProfile{Name: "Long"}
	for i := 0; i < 10; i++ {
		long.Traits = append(long.Traits, ProfileEntry{Predicate: "likes", Object: "thing"})
		long.Events = append(long.Events, ProfileEntry{Date: time.Date(2024, 1, i+1, 0, 0, 0, 0, time.UTC), Predicate: "did-event", Object: "thing"})
	}
	out2 := formatProfilePreamble([]*EntityProfile{long}, profilePreambleConfig{MaxEntities: 1, MaxTraits: 3, MaxEvents: 2})
	// header(1) + "Long:"(1) + "  traits:"(1) + 3 trait lines + "  events:"(1) + 2 event lines = 9 lines
	lines := strings.Split(strings.TrimRight(out2, "\n"), "\n")
	if len(lines) != 9 {
		t.Errorf("expected 9 lines (header+name+traits-header+3+events-header+2), got %d:\n%s", len(lines), out2)
	}
}

// Undated trait renders in the traits section without a date.
// Dated event renders in the events section with its date. The
// two sections never interleave.
func TestFormatProfilePreamble_UndatedTraitDatedEvent(t *testing.T) {
	profs := []*EntityProfile{{
		Name: "Tom",
		Traits: []ProfileEntry{
			{Predicate: "lives-in", Object: "Portland"},
		},
		Events: []ProfileEntry{
			{Date: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC), Predicate: "started", Object: "new job"},
		},
	}}
	out := formatProfilePreamble(profs, defaultProfilePreambleConfig())
	if !strings.Contains(out, "    - lives-in Portland") {
		t.Errorf("trait not rendered undated: %q", out)
	}
	if !strings.Contains(out, "    2023-06-01: started new job") {
		t.Errorf("event not rendered dated: %q", out)
	}
	if strings.Contains(out, "(undated)") {
		t.Errorf("trait should render without (undated) sentinel — it lives in the traits section instead: %q", out)
	}
}

// TestRetrieveHybridContext_ProfilePreamble verifies the end-to-end
// path: when ProfilePreamble=true, the retriever prepends a TIMELINES
// block synthesised from the rule-extracted fact chronology, with the
// synthetic ID "TIMELINES" so Hit@K isn't polluted.
func TestRetrieveHybridContext_ProfilePreamble(t *testing.T) {
	mem := NewPixelogMemory(PixelogConfig{
		Embedder:        NewHashEmbedder(128),
		ProfilePreamble: true,
	})
	ctx := context.Background()
	ns := "profile-pre"

	day1 := time.Date(2023, 5, 14, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2023, 8, 22, 0, 0, 0, 0, time.UTC)
	turns := []Turn{
		{TurnID: "D1:1", Role: "user", Speaker: "Caroline", Text: "I'm Caroline.", SessionID: "s1", Timestamp: day1},
		{TurnID: "D1:2", Role: "user", Speaker: "Caroline", Text: "I live in Toronto.", SessionID: "s1", Timestamp: day1},
		{TurnID: "D2:1", Role: "user", Speaker: "Caroline", Text: "I live in Berlin now.", SessionID: "s2", Timestamp: day2},
	}
	if err := mem.Ingest(ctx, ns, turns); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	texts, ids, err := mem.retrieveHybridContext(ctx, ns, "Where does Caroline live?", 5)
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	if len(texts) == 0 || len(ids) == 0 {
		t.Fatalf("no context returned")
	}
	// First entry should be the timelines preamble (or at most second
	// behind a FACTS preamble — but factEvidence is off here).
	foundTL := false
	for i, id := range ids {
		if id == "TIMELINES" {
			foundTL = true
			if !strings.HasPrefix(texts[i], "KNOWN PROFILES:\n") {
				t.Errorf("timelines text malformed: %q", texts[i])
			}
			if !strings.Contains(texts[i], "Caroline") {
				t.Errorf("timelines missing Caroline entry: %q", texts[i])
			}
			break
		}
	}
	if !foundTL {
		t.Errorf("expected TIMELINES preamble in ids=%v", ids)
	}
}

func TestRetrieveHybridContext_ProfilePreambleDisabled(t *testing.T) {
	mem := NewPixelogMemory(PixelogConfig{Embedder: NewHashEmbedder(128)}) // ProfilePreamble default false
	ctx := context.Background()
	ns := "profile-off"

	if err := mem.Ingest(ctx, ns, []Turn{{
		TurnID: "D1:1", Role: "user", Speaker: "Caroline",
		Text:      "I'm Caroline and I live in Toronto.",
		SessionID: "s1", Timestamp: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
	}}); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	_, ids, err := mem.retrieveHybridContext(ctx, ns, "Where does Caroline live?", 5)
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	for _, id := range ids {
		if id == "TIMELINES" {
			t.Errorf("preamble emitted when ProfilePreamble=false: ids=%v", ids)
		}
	}
}
