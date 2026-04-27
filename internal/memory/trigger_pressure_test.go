package memory

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestPressureMonitor_FiresOnceOnCross verifies the rising-edge contract:
// onCross fires exactly once when the budget transitions from under to over.
func TestPressureMonitor_FiresOnceOnCross(t *testing.T) {
	var fired int32
	m := NewPressureMonitor(100, func(over int) {
		atomic.AddInt32(&fired, 1)
		if over <= 0 {
			t.Errorf("over should be positive, got %d", over)
		}
	})

	// Below cap: no fire.
	m.Add(50)
	if got := atomic.LoadInt32(&fired); got != 0 {
		t.Errorf("fired=%d below cap; want 0", got)
	}

	// Cross: one fire.
	m.Add(60)
	if got := atomic.LoadInt32(&fired); got != 1 {
		t.Errorf("fired=%d after first cross; want 1", got)
	}

	// Adding more while still over: no extra fire.
	m.Add(20)
	if got := atomic.LoadInt32(&fired); got != 1 {
		t.Errorf("fired=%d after second add over cap; want 1", got)
	}

	// Drop back under and re-cross: another fire.
	m.Reset()
	m.Add(150)
	if got := atomic.LoadInt32(&fired); got != 2 {
		t.Errorf("fired=%d after re-cross; want 2", got)
	}
}

// TestPressureMonitor_NegativeDoesNotUnderflow guards the floor.
func TestPressureMonitor_NegativeDoesNotUnderflow(t *testing.T) {
	m := NewPressureMonitor(100, nil)
	m.Add(-50)
	if got := m.Current(); got != 0 {
		t.Errorf("Current after negative = %d; want 0", got)
	}
}

// TestPressureMonitor_SetCapTriggersIfOver: lowering the cap below
// current usage fires the cross callback exactly once.
func TestPressureMonitor_SetCapTriggersIfOver(t *testing.T) {
	var fired int32
	m := NewPressureMonitor(1000, func(over int) {
		atomic.AddInt32(&fired, 1)
	})
	m.Add(500)
	if got := atomic.LoadInt32(&fired); got != 0 {
		t.Errorf("fired before SetCap = %d", got)
	}
	m.SetCap(100)
	if got := atomic.LoadInt32(&fired); got != 1 {
		t.Errorf("fired after SetCap drop = %d, want 1", got)
	}
}

// TestPressureMonitor_ConcurrentAddIsSafe: the race detector flags any
// missing locks and we check the cap-cross fires exactly once even
// under concurrent load.
func TestPressureMonitor_ConcurrentAddIsSafe(t *testing.T) {
	var fired int32
	m := NewPressureMonitor(1000, func(over int) {
		atomic.AddInt32(&fired, 1)
	})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Add(15)
		}()
	}
	wg.Wait()
	if m.Current() != 1500 {
		t.Errorf("Current = %d, want 1500", m.Current())
	}
	if atomic.LoadInt32(&fired) != 1 {
		t.Errorf("fired = %d under concurrent crosses; want exactly 1", atomic.LoadInt32(&fired))
	}
}
