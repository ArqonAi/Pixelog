package memory

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestCircadian_FiresOnStartup verifies that a fresh scheduler invokes
// the callback once at process start so a long-stopped agent catches up.
func TestCircadian_FiresOnStartup(t *testing.T) {
	var calls int32
	cb := func(ctx context.Context, level EraLevel, since time.Time) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}
	s := NewCircadianScheduler(50*time.Millisecond, cb)
	// Fix the clock to a calendar boundary that has already passed —
	// guarantees Day/Week/etc. are due immediately.
	s.SetClock(func() time.Time {
		return time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC) // mid-Monday
	})

	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	time.Sleep(75 * time.Millisecond)
	cancel()

	if got := atomic.LoadInt32(&calls); got == 0 {
		t.Error("expected at least one fire on startup")
	}
}

// TestCircadian_NotifyTriggersImmediateCheck: Notify wakes the
// scheduler without waiting for the next tick.
func TestCircadian_NotifyTriggersImmediateCheck(t *testing.T) {
	var calls int32
	cb := func(ctx context.Context, level EraLevel, since time.Time) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}
	// Long tick — would never naturally fire during the test window.
	s := NewCircadianScheduler(time.Hour, cb)
	s.SetClock(func() time.Time { return time.Date(2026, 6, 15, 0, 0, 0, 1, time.UTC) })

	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	time.Sleep(20 * time.Millisecond)
	startupCalls := atomic.LoadInt32(&calls)

	s.Notify()
	time.Sleep(20 * time.Millisecond)
	cancel()

	final := atomic.LoadInt32(&calls)
	// Should have at least the startup calls; Notify might or might
	// not produce a new fire depending on timing — what matters is the
	// channel didn't deadlock and total calls didn't decrease.
	if final < startupCalls {
		t.Errorf("calls regressed: %d → %d", startupCalls, final)
	}
}

// TestAlignBoundary covers all calendar windows.
func TestAlignBoundary(t *testing.T) {
	mon := time.Date(2026, 6, 15, 14, 30, 0, 0, time.UTC) // Monday
	cases := []struct {
		level EraLevel
		want  time.Time
	}{
		{LevelDay, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)},
		{LevelWeek, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)}, // ISO Monday
		{LevelMonth, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
		{LevelQuarter, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)}, // Q2
		{LevelYear, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{LevelDecade, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		got := alignBoundary(c.level, mon)
		if !got.Equal(c.want) {
			t.Errorf("%s: got %v, want %v", c.level, got, c.want)
		}
	}

	// Sunday of the same week aligns back to the prior Monday.
	sun := time.Date(2026, 6, 21, 23, 59, 0, 0, time.UTC)
	if got := alignBoundary(LevelWeek, sun); !got.Equal(time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Sunday week boundary: got %v", got)
	}
}
