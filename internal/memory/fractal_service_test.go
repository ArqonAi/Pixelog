package memory

import (
	"context"
	"testing"
	"time"
)

// TestFractalService_EndToEnd exercises the full self-running pipeline:
// sessions → day era → week era, with each level's capsule landing on
// disk and showing up in the store's per-level index.
func TestFractalService_EndToEnd(t *testing.T) {
	tracker, _ := NewAccessTracker(t.TempDir())
	store, _ := NewCapsuleStore(t.TempDir())
	compactor := NewCompactor(DefaultCompactionConfig(), tracker, nil)

	cfg := DefaultFractalConfig("agent-1", store, compactor)
	cfg.TickInterval = 25 * time.Millisecond
	svc := NewFractalService(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)

	// Push 6 sessions covering 6 different hours of the same day.
	for i := 0; i < 6; i++ {
		ref := ChildRef{
			URI:       "pixe://capsule/sess" + string(rune('a'+i)),
			Hash:      "sess" + string(rune('a'+i)),
			Level:     LevelSession,
			L0:        "session " + string(rune('a'+i)),
			L1:        "longer session " + string(rune('a'+i)),
			StartedAt: time.Date(2026, 1, 1, i, 0, 0, 0, time.UTC),
			EndedAt:   time.Date(2026, 1, 1, i, 30, 0, 0, time.UTC),
			Concepts:  []string{string(CategoryFact)},
		}
		svc.AddSession(ctx, ref)
	}

	// Wait long enough for the scheduler's startup-catchup to run the
	// day-rollup. The scheduler ticks every 25ms; one tick is plenty.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if days := store.ListByLevel("agent-1", LevelDay); len(days) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	days := store.ListByLevel("agent-1", LevelDay)
	if len(days) < 1 {
		t.Fatalf("expected at least one day capsule on disk; got %d (queue=%d)",
			len(days), svc.PendingAtLevel(LevelSession))
	}

	// Each day capsule must reference all 6 sessions as children.
	dayEra, err := store.GetEra(ctx, days[0].Hash)
	if err != nil {
		t.Fatalf("GetEra: %v", err)
	}
	if got := len(dayEra.Children); got != 6 {
		t.Errorf("day era has %d children, want 6", got)
	}
	if dayEra.Level != LevelDay {
		t.Errorf("day era level = %s", dayEra.Level)
	}
}

// TestFractalService_PressureForcesCompaction verifies that the
// pressure trigger fires a compaction even before the circadian
// boundary is reached.
func TestFractalService_PressureForcesCompaction(t *testing.T) {
	tracker, _ := NewAccessTracker(t.TempDir())
	store, _ := NewCapsuleStore(t.TempDir())
	compactor := NewCompactor(DefaultCompactionConfig(), tracker, nil)

	cfg := DefaultFractalConfig("agent-2", store, compactor)
	// Long tick so circadian won't fire during the test.
	cfg.TickInterval = time.Hour
	cfg.TokenCap = 100
	svc := NewFractalService(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)

	// Push 3 sessions, then blow past the token cap to force pressure.
	for i := 0; i < 3; i++ {
		svc.AddSession(ctx, ChildRef{
			URI:       "pixe://capsule/p" + string(rune('a'+i)),
			Hash:      "p" + string(rune('a'+i)),
			Level:     LevelSession,
			L0:        "pressure session " + string(rune('a'+i)),
			L1:        "longer pressure session " + string(rune('a'+i)),
			StartedAt: time.Date(2026, 2, 1, i, 0, 0, 0, time.UTC),
			EndedAt:   time.Date(2026, 2, 1, i, 30, 0, 0, time.UTC),
			Concepts:  []string{string(CategoryEvent)},
		})
	}
	svc.AddTokens(500) // crosses cap=100

	// Pressure handler runs in a goroutine; poll for the resulting era.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if days := store.ListByLevel("agent-2", LevelDay); len(days) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := store.ListByLevel("agent-2", LevelDay); len(got) == 0 {
		t.Fatal("pressure trigger did not produce a day capsule")
	}
}
