package memory

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// FractalService is the orchestrator that turns the per-piece primitives
// (CapsuleStore + Compactor + PressureMonitor + CircadianScheduler) into
// a self-running consolidation system.
//
// Lifecycle:
//
//	svc := NewFractalService(cfg)
//	svc.Start(ctx)
//	defer svc.Stop()
//
//	svc.AddSession(ctx, sessionCapsule)   // call after each archival
//	svc.AddTokens(estimatedSessionTokens) // updates pressure
//
// The service is safe for concurrent use; all mutations of the level
// queues happen under a single mutex.
type FractalService struct {
	cfg       FractalConfig
	store     *CapsuleStore
	compactor *Compactor
	scheduler *CircadianScheduler
	pressure  *PressureMonitor

	mu     sync.Mutex
	queues map[EraLevel][]ChildRef // pending children per level
	cancel context.CancelFunc
	done   chan struct{}
}

// FractalConfig configures a FractalService.
type FractalConfig struct {
	Namespace     string
	Store         *CapsuleStore
	Compactor     *Compactor
	TickInterval  time.Duration // how often the circadian scheduler wakes
	TokenCap      int           // active context budget (0 → no pressure trigger)
	MinDayChildren int          // don't compact a day with fewer children unless pressure-forced
}

// DefaultFractalConfig returns reasonable defaults.
func DefaultFractalConfig(namespace string, store *CapsuleStore, compactor *Compactor) FractalConfig {
	return FractalConfig{
		Namespace:      namespace,
		Store:          store,
		Compactor:      compactor,
		TickInterval:   time.Hour,
		TokenCap:       8000,
		MinDayChildren: 1,
	}
}

// NewFractalService builds a service. Call Start() to begin processing.
func NewFractalService(cfg FractalConfig) *FractalService {
	if cfg.MinDayChildren <= 0 {
		cfg.MinDayChildren = 1
	}
	svc := &FractalService{
		cfg:    cfg,
		store:  cfg.Store,
		queues: map[EraLevel][]ChildRef{},
	}
	svc.compactor = cfg.Compactor

	svc.scheduler = NewCircadianScheduler(cfg.TickInterval, svc.onCircadian)
	if cfg.TokenCap > 0 {
		svc.pressure = NewPressureMonitor(cfg.TokenCap, svc.onPressure)
	}
	return svc
}

// Start launches the circadian scheduler. Returns immediately.
func (s *FractalService) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		s.scheduler.Run(ctx)
	}()
}

// Stop cancels the scheduler and waits for it to exit.
func (s *FractalService) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.done != nil {
		<-s.done
	}
}

// AddSession enqueues a freshly-archived session capsule. Sessions are
// stored under queues[LevelSession]; a circadian Day fire (or pressure
// trigger) folds them into a Day era. Caller is responsible for having
// run the existing Compress/Archive phases first.
func (s *FractalService) AddSession(ctx context.Context, ref ChildRef) {
	if ref.Level != LevelSession {
		log.Printf("[fractal] AddSession received non-session ref level=%s", ref.Level)
	}
	s.mu.Lock()
	s.queues[LevelSession] = append(s.queues[LevelSession], ref)
	s.mu.Unlock()
	s.scheduler.Notify()
}

// AddTokens informs the pressure monitor of token-budget changes.
// Pass positive deltas when content is added to the active context,
// negative when content is evicted.
func (s *FractalService) AddTokens(delta int) {
	if s.pressure != nil {
		s.pressure.Add(delta)
	}
}

// onCircadian is invoked by the scheduler when level's wall-clock window
// has crossed (or when AddSession's Notify forces an opportunistic check).
// It folds children queued at the level immediately below into a parent
// capsule at `level`.
//
// Queue convention: queues[L] stores capsules at level L waiting to be
// folded into a level-L.Parent() capsule. So a Day fire drains
// queues[Session] and produces a Day era; a Week fire drains queues[Day],
// etc.
func (s *FractalService) onCircadian(ctx context.Context, level EraLevel, _ time.Time) error {
	below := level - 1
	if below < LevelSession {
		below = LevelSession
	}
	s.mu.Lock()
	pending := s.queues[below]
	s.queues[below] = nil
	s.mu.Unlock()

	if len(pending) == 0 {
		return nil
	}
	if level == LevelDay && len(pending) < s.cfg.MinDayChildren {
		// Push back; not enough children for a meaningful day rollup.
		s.mu.Lock()
		s.queues[below] = append(pending, s.queues[below]...)
		s.mu.Unlock()
		return nil
	}

	era, err := s.compactor.Compact(ctx, s.cfg.Namespace, level, pending)
	if err != nil {
		return fmt.Errorf("circadian compact level=%s: %w", level, err)
	}
	if _, err := s.store.PutEra(era); err != nil {
		return fmt.Errorf("store era: %w", err)
	}

	// Promote: the new era becomes a pending child of the next level up.
	parentRef := refFromEra(era)
	s.mu.Lock()
	s.queues[level] = append(s.queues[level], parentRef)
	s.mu.Unlock()

	if s.pressure != nil {
		// A fresh era replaces N children's L1s with one — reduce the
		// active token estimate by the difference.
		delta := estimateTokens(pending) - estimateTokens([]ChildRef{parentRef})
		if delta > 0 {
			s.pressure.Add(-delta)
		}
	}
	return nil
}

// CompactNow forces an immediate fold of queues[level-1] into a parent
// capsule at `level`, bypassing the scheduler. Returns the new era's URI
// (empty if no work was pending). Useful for tests, manual triggering,
// and CLI "pixe compact" invocations.
func (s *FractalService) CompactNow(ctx context.Context, level EraLevel) (string, error) {
	if err := s.onCircadian(ctx, level, time.Time{}); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	q := s.queues[level]
	if len(q) == 0 {
		return "", nil
	}
	return q[len(q)-1].URI, nil
}

// onPressure is invoked when the pressure monitor crosses the cap.
// It folds the lowest-salience pending children at the most-loaded
// level, freeing budget without waiting for the next circadian boundary.
func (s *FractalService) onPressure(over int) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		// Find the level with the most pending children — that's where
		// pressure relief has the largest effect per compaction pass.
		s.mu.Lock()
		var hottest EraLevel
		max := -1
		for lvl, q := range s.queues {
			if len(q) > max {
				hottest = lvl
				max = len(q)
			}
		}
		pending := s.queues[hottest]
		s.queues[hottest] = nil
		s.mu.Unlock()

		if len(pending) == 0 {
			return
		}
		era, err := s.compactor.Compact(ctx, s.cfg.Namespace, hottest.Parent(), pending)
		if err != nil {
			log.Printf("[fractal] pressure compact failed: %v", err)
			return
		}
		if _, err := s.store.PutEra(era); err != nil {
			log.Printf("[fractal] pressure store failed: %v", err)
			return
		}
		s.mu.Lock()
		s.queues[hottest.Parent()] = append(s.queues[hottest.Parent()], refFromEra(era))
		s.mu.Unlock()
		if s.pressure != nil {
			s.pressure.Reset()
		}
	}()
}

// PendingAtLevel returns a snapshot of the queue length at level.
// Useful for diagnostics and tests.
func (s *FractalService) PendingAtLevel(level EraLevel) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queues[level])
}

// RefFromEra builds a ChildRef from a finalized EraCapsule. Exported so
// CLI tooling (`pixe compact`) and embedded library users can promote a
// loaded era to a child reference without duplicating field-mapping
// logic.
func RefFromEra(ec *EraCapsule) ChildRef {
	return ChildRef{
		URI:       ec.URI(),
		Hash:      ec.Hash,
		Level:     ec.Level,
		L0:        ec.L0,
		L1:        ec.L1,
		StartedAt: ec.StartedAt,
		EndedAt:   ec.EndedAt,
		Concepts:  ec.Concepts,
	}
}

// refFromEra is the unexported alias kept for internal callers.
func refFromEra(ec *EraCapsule) ChildRef { return RefFromEra(ec) }

// estimateTokens approximates token count using the same ~4 chars/token
// heuristic the rest of the package uses.
func estimateTokens(refs []ChildRef) int {
	total := 0
	for _, r := range refs {
		total += (len(r.L0) + len(r.L1)) / 4
	}
	return total
}
