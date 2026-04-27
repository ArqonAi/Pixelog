package memory

import (
	"context"
	"testing"
	"time"
)

// TestFractalE2E_FullPipeline drives every fractal-memory component
// in a single test:
//
//  1. Boot FractalService + CapsuleStore + Compactor + Resolver.
//  2. Push 10 sessions over "day 1" (varying salience).
//  3. Force a Day-level compaction. Verify the Day era lands on disk
//     with the expected surfaced/buried split.
//  4. Push 10 more sessions over "day 2" with a single rare-phrase
//     session that will end up buried.
//  5. Force a second Day-level compaction.
//  6. Force a Week-level compaction. Verify the Week era references
//     both Day eras and is itself addressable via the resolver.
//  7. From the Week era, DeepRetrieve a query that only matches the
//     buried rare-phrase session. Verify:
//       - SurfaceOnly mode misses it (buried means buried).
//       - Free-association mode finds it via URI traversal.
//  8. Verify the resolver round-trips the Day capsule from disk by URI.
//
// This is the test that proves the agent equivalent of "I had a thought
// I'd forgotten until I deliberately reflected on it" actually works
// across the recursive hierarchy.
func TestFractalE2E_FullPipeline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tracker, err := NewAccessTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewAccessTracker: %v", err)
	}
	store, err := NewCapsuleStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewCapsuleStore: %v", err)
	}
	cfg := DefaultCompactionConfig()
	cfg.SurfaceRatio = 0.30
	cfg.MinSurfaceCount = 3
	compactor := NewCompactor(cfg, tracker, nil)

	svcCfg := DefaultFractalConfig("agent-e2e", store, compactor)
	svcCfg.TickInterval = time.Hour
	svc := NewFractalService(svcCfg)
	// Deliberately do NOT call svc.Start(): in this E2E we drive every
	// fold manually via CompactNow so the test is fully deterministic.
	// (A separate test, TestFractalService_EndToEnd, exercises the
	// background-scheduler path.)

	resolver := NewCapsuleResolver(store)

	// ---------- DAY 1: 10 ordinary sessions ----------
	day1 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		ref := buildSessionRef("d1s"+string(rune('a'+i)),
			"day-1 session "+string(rune('a'+i))+" about regular topics",
			"longer day-1 transcript "+string(rune('a'+i)),
			day1.Add(time.Duration(i)*time.Hour),
			[]string{string(CategoryEvent)},
		)
		// First three sessions get high access counts so they surface.
		if i < 3 {
			for j := 0; j < 50; j++ {
				tracker.RecordAccess(ref.URI)
			}
		}
		svc.AddSession(ctx, ref)
	}

	// Force Day rollup.
	if _, err := svc.CompactNow(ctx, LevelDay); err != nil {
		t.Fatalf("Day1 CompactNow: %v", err)
	}
	day1Eras := store.ListByLevel("agent-e2e", LevelDay)
	if len(day1Eras) != 1 {
		t.Fatalf("after Day1 fold, want 1 day era, got %d", len(day1Eras))
	}

	// Verify surfaced/buried split: 10 children, 30% surfaced
	// (min-floor 3) → exactly 3 surfaced, 7 buried.
	day1Era, err := store.GetEra(ctx, day1Eras[0].Hash)
	if err != nil {
		t.Fatalf("GetEra day1: %v", err)
	}
	if got := len(day1Era.Children); got != 10 {
		t.Errorf("day1 era children = %d, want 10", got)
	}
	surfaced := len(day1Era.SurfaceChildren())
	buried := len(day1Era.BuriedChildren())
	if surfaced != 3 || buried != 7 {
		t.Errorf("day1 surfaced=%d buried=%d, want 3+7", surfaced, buried)
	}
	// Buried children must have their L1 stripped.
	for _, ch := range day1Era.BuriedChildren() {
		if ch.L1 != "" {
			t.Errorf("buried child %s still has L1 inlined", ch.Hash)
		}
	}

	// ---------- DAY 2: 10 sessions, one carrying a rare phrase ----------
	day2 := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	const rarePhrase = "tachyon ribbon convergence"
	const buriedHash = "d2-buried"
	for i := 0; i < 10; i++ {
		hash := "d2s" + string(rune('a'+i))
		l0 := "day-2 session " + string(rune('a'+i)) + " about generic topics"
		l1 := "longer day-2 transcript " + string(rune('a'+i))
		// Inject the rare phrase into the LAST session (which will get
		// no access boost and is therefore likely to be buried).
		if i == 9 {
			hash = buriedHash
			l0 = "private musing about " + rarePhrase + " from the agent"
			l1 = "extended reflection on " + rarePhrase + " — full transcript"
		}
		ref := buildSessionRef(hash, l0, l1,
			day2.Add(time.Duration(i)*time.Hour),
			[]string{string(CategoryEvent)},
		)
		// Boost first three again so they outrank the rare-phrase session.
		if i < 3 {
			for j := 0; j < 50; j++ {
				tracker.RecordAccess(ref.URI)
			}
		}
		svc.AddSession(ctx, ref)
	}

	if _, err := svc.CompactNow(ctx, LevelDay); err != nil {
		t.Fatalf("Day2 CompactNow: %v", err)
	}
	day2Eras := store.ListByLevel("agent-e2e", LevelDay)
	if len(day2Eras) != 2 {
		t.Fatalf("after Day2 fold, want 2 day eras, got %d", len(day2Eras))
	}

	// Locate Day2 era and confirm the rare-phrase session was buried.
	var day2Era *EraCapsule
	for _, idx := range day2Eras {
		ec, err := store.GetEra(ctx, idx.Hash)
		if err != nil {
			t.Fatalf("GetEra: %v", err)
		}
		// Day2's earliest child is at day2's start.
		if ec.StartedAt.Equal(day2) {
			day2Era = ec
			break
		}
	}
	if day2Era == nil {
		t.Fatal("could not locate Day2 era")
	}
	var rareChildBuried bool
	var rareChildPresent bool
	for _, ch := range day2Era.Children {
		if ch.Hash == buriedHash {
			rareChildPresent = true
			rareChildBuried = ch.Buried
			if ch.Buried && ch.L1 != "" {
				t.Errorf("buried rare child still has L1: %q", ch.L1)
			}
		}
	}
	if !rareChildPresent {
		t.Fatal("rare-phrase session not present in Day2 era at all")
	}
	if !rareChildBuried {
		t.Fatal("rare-phrase session should have been buried; it was surfaced")
	}

	// ---------- WEEK fold ----------
	if _, err := svc.CompactNow(ctx, LevelWeek); err != nil {
		t.Fatalf("Week CompactNow: %v", err)
	}
	weekEras := store.ListByLevel("agent-e2e", LevelWeek)
	if len(weekEras) != 1 {
		t.Fatalf("after Week fold, want 1 week era, got %d", len(weekEras))
	}
	weekEra, err := store.GetEra(ctx, weekEras[0].Hash)
	if err != nil {
		t.Fatalf("GetEra week: %v", err)
	}
	if got := len(weekEra.Children); got != 2 {
		t.Errorf("week era children = %d, want 2 (the two days)", got)
	}
	for _, ch := range weekEra.Children {
		if ch.Level != LevelDay {
			t.Errorf("week child level = %s, want day", ch.Level)
		}
		if ch.URI == "" || ch.Hash == "" {
			t.Errorf("week child missing URI/Hash: %+v", ch)
		}
	}

	// ---------- Resolver round-trip via URI ----------
	for _, ch := range weekEra.Children {
		got, err := resolver.Resolve(ctx, ch.URI)
		if err != nil {
			t.Fatalf("resolver.Resolve(%s): %v", ch.URI, err)
		}
		if got.Hash != ch.Hash {
			t.Errorf("resolver returned wrong hash: got %s want %s", got.Hash, ch.Hash)
		}
	}

	// ---------- DeepRetrieve: SurfaceOnly should miss the buried memory ----------
	conscious, err := DeepRetrieve(ctx, resolver, []*EraCapsule{weekEra}, DeepRetrieveOptions{
		Query:       rarePhrase,
		MaxDepth:    3,
		MaxResults:  20,
		SurfaceOnly: true,
	})
	if err != nil {
		t.Fatalf("DeepRetrieve conscious: %v", err)
	}
	for _, hit := range conscious {
		if hit.Hash == buriedHash {
			t.Errorf("SurfaceOnly should have missed the buried rare-phrase session, but it surfaced: %+v", hit)
		}
	}

	// ---------- DeepRetrieve: free association should find it ----------
	subconscious, err := DeepRetrieve(ctx, resolver, []*EraCapsule{weekEra}, DeepRetrieveOptions{
		Query:      rarePhrase,
		MaxDepth:   3,
		MaxResults: 20,
		// SurfaceOnly defaults to false ("free association")
	})
	if err != nil {
		t.Fatalf("DeepRetrieve subconscious: %v", err)
	}
	found := false
	for _, hit := range subconscious {
		if hit.Hash == buriedHash {
			found = true
			if !hit.Buried {
				t.Errorf("expected hit.Buried=true for the rare-phrase session, got %+v", hit)
			}
			if hit.Depth < 2 {
				t.Errorf("buried session should be at depth >= 2 (week→day→session), got %d", hit.Depth)
			}
			break
		}
	}
	if !found {
		t.Fatalf("free-association DeepRetrieve failed to surface the buried rare-phrase session.\nresults: %+v", subconscious)
	}
}

// buildSessionRef is a small helper for the E2E test.
func buildSessionRef(hash, l0, l1 string, start time.Time, concepts []string) ChildRef {
	return ChildRef{
		URI:       "pixe://capsule/" + hash,
		Hash:      hash,
		Level:     LevelSession,
		L0:        l0,
		L1:        l1,
		StartedAt: start,
		EndedAt:   start.Add(45 * time.Minute),
		Concepts:  concepts,
	}
}
