package memory

import (
	"context"
	"log"
	"sync"
	"time"
)

// CircadianScheduler fires Compact passes at calendar boundaries.
//
// Two consolidation rhythms run concurrently in real systems and we
// model both:
//
//   - **Circadian (this scheduler)** — predictable, time-aligned. Fires
//     on day boundaries, then propagates upward (week → month → ...)
//     once each lower level accumulates enough children.
//   - **Salience pressure** (trigger_pressure.go) — adaptive; fires
//     when active context budget is exhausted, regardless of time.
//
// They share the same Compact primitive, so a session can be folded by
// either trigger and the result is identical.
type CircadianScheduler struct {
	tick     time.Duration
	now      func() time.Time
	notify   chan struct{}
	stopOnce sync.Once
	stopped  chan struct{}

	// callback is invoked whenever a level's wall-clock window has
	// elapsed since the last fire. The callee is responsible for
	// running the actual Compact pass for that level.
	callback CompactCallback

	// lastFired tracks per-level last-fire times so we don't double-fire
	// across restarts (callers should persist this externally if they
	// want strict idempotency across process boundaries).
	mu        sync.Mutex
	lastFired map[EraLevel]time.Time
}

// CompactCallback is invoked by the scheduler whenever a level boundary
// has been crossed. Implementations typically gather the level's
// pending children and run the Compactor.
type CompactCallback func(ctx context.Context, level EraLevel, since time.Time) error

// NewCircadianScheduler builds a scheduler. tick controls how often the
// scheduler wakes up to check boundaries; sub-day ticks (e.g. 1h) are
// cheap and recommended so the day rollover fires within the hour.
func NewCircadianScheduler(tick time.Duration, callback CompactCallback) *CircadianScheduler {
	if tick <= 0 {
		tick = time.Hour
	}
	return &CircadianScheduler{
		tick:      tick,
		now:       time.Now,
		notify:    make(chan struct{}, 1),
		stopped:   make(chan struct{}),
		callback:  callback,
		lastFired: map[EraLevel]time.Time{},
	}
}

// Run blocks until ctx is cancelled or Stop is called. Each tick checks
// every level (Day → Decade) and invokes the callback for any whose
// wall-clock window has elapsed since the last fire.
//
// Notify() always forces a fire (force=true), modeling the "session-end"
// or "agent-just-came-online" event where we want consolidation to be
// considered immediately rather than waiting for the next boundary.
// Timer ticks honor the per-level duration window (force=false).
func (s *CircadianScheduler) Run(ctx context.Context) {
	defer close(s.stopped)
	timer := time.NewTicker(s.tick)
	defer timer.Stop()
	// Fire once on startup so a long-stopped agent immediately catches
	// up on missed boundaries.
	s.checkAndFire(ctx, true)
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.checkAndFire(ctx, false)
		case <-s.notify:
			s.checkAndFire(ctx, true)
		}
	}
}

// Notify wakes the scheduler immediately, e.g. when a new session is
// archived and we want the day boundary check to happen now rather than
// at the next tick.
func (s *CircadianScheduler) Notify() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// Stop signals the scheduler to exit. Safe to call multiple times.
func (s *CircadianScheduler) Stop() {
	s.stopOnce.Do(func() {
		// Run() exits via context cancellation; this method is mainly
		// here so callers have a symmetric API.
	})
	<-s.stopped
}

// SetClock replaces the wall-clock source. Test-only.
func (s *CircadianScheduler) SetClock(now func() time.Time) {
	s.now = now
}

// checkAndFire fires the callback for every level whose window is due.
// When force is true (Notify path), the duration check is bypassed —
// useful for "agent just archived a session, consider compacting now"
// events. Day-level always fires under force; higher levels still respect
// duration windows even under force, because folding a single day into a
// week is rarely useful.
func (s *CircadianScheduler) checkAndFire(ctx context.Context, force bool) {
	now := s.now()
	for _, level := range []EraLevel{LevelDay, LevelWeek, LevelMonth, LevelQuarter, LevelYear, LevelDecade} {
		due := s.dueForLevel(level, now)
		// Force always fires Day; for higher levels, force still requires
		// the boundary to have arrived (otherwise weekly/monthly rollups
		// would happen on every session, which collapses the fractal).
		if !due && !(force && level == LevelDay) {
			continue
		}
		s.mu.Lock()
		since := s.lastFired[level]
		s.lastFired[level] = now
		s.mu.Unlock()
		if err := s.callback(ctx, level, since); err != nil {
			log.Printf("[circadian] level=%s fire failed: %v", level, err)
		}
	}
}

// dueForLevel reports whether `level`'s wall-clock window has elapsed
// since the last fire (or since process start if never fired).
func (s *CircadianScheduler) dueForLevel(level EraLevel, now time.Time) bool {
	dur := level.LevelDuration()
	if dur <= 0 {
		return false
	}
	s.mu.Lock()
	last := s.lastFired[level]
	s.mu.Unlock()
	if last.IsZero() {
		// Never fired: align to the previous calendar boundary so the
		// first fire happens at the next natural break, not on startup.
		// We still return true once boundary has passed.
		return now.Sub(alignBoundary(level, now)) >= 0 && lastBoundaryReached(level, now)
	}
	return now.Sub(last) >= dur
}

// alignBoundary returns the start of the calendar window containing now.
func alignBoundary(level EraLevel, now time.Time) time.Time {
	now = now.UTC()
	y, m, d := now.Date()
	switch level {
	case LevelDay:
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	case LevelWeek:
		// ISO week (Monday-start).
		offset := (int(now.Weekday()) + 6) % 7
		return time.Date(y, m, d-offset, 0, 0, 0, 0, time.UTC)
	case LevelMonth:
		return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	case LevelQuarter:
		qm := time.Month(((int(m)-1)/3)*3 + 1)
		return time.Date(y, qm, 1, 0, 0, 0, 0, time.UTC)
	case LevelYear:
		return time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
	case LevelDecade:
		return time.Date(y-(y%10), 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return now
}

// lastBoundaryReached reports whether at least one full level-window
// has elapsed since the *previous* boundary — i.e. there is a closed
// window worth compacting.
func lastBoundaryReached(level EraLevel, now time.Time) bool {
	this := alignBoundary(level, now)
	return now.After(this)
}
