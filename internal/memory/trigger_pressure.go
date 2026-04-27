package memory

import (
	"sync"
)

// PressureMonitor tracks the active L0+L1 token budget and fires a
// "pressure" signal when usage exceeds a configured cap.
//
// In biological terms this is the "cortisol" pathway — adaptive,
// load-driven consolidation. Whereas the circadian scheduler runs on a
// predictable rhythm, the pressure monitor reacts to attention pressure
// regardless of time. Both feed the same Compact primitive.
type PressureMonitor struct {
	mu       sync.Mutex
	cap      int
	current  int
	onCross  func(over int)
	overFlag bool
}

// NewPressureMonitor builds a monitor with a token-budget cap.
// onCross is invoked exactly once when the budget transitions from
// "under cap" to "over cap"; it does not fire again until the budget
// drops back below and recrosses.
func NewPressureMonitor(tokenCap int, onCross func(over int)) *PressureMonitor {
	return &PressureMonitor{cap: tokenCap, onCross: onCross}
}

// Add records a delta to the current usage and fires onCross if the cap
// is crossed. delta may be negative when capsules are evicted.
func (m *PressureMonitor) Add(delta int) {
	m.mu.Lock()
	m.current += delta
	if m.current < 0 {
		m.current = 0
	}
	over := m.current - m.cap
	wasOver := m.overFlag
	m.overFlag = over > 0
	cb := m.onCross
	m.mu.Unlock()

	if !wasOver && over > 0 && cb != nil {
		cb(over)
	}
}

// Current returns a snapshot of the active token budget.
func (m *PressureMonitor) Current() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

// Cap returns the configured token cap.
func (m *PressureMonitor) Cap() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cap
}

// SetCap updates the cap; useful when the agent's working-memory budget
// changes (e.g. switching between models with different context windows).
// If the new cap leaves the monitor in an "over" state, the cross
// callback fires immediately.
func (m *PressureMonitor) SetCap(newCap int) {
	m.mu.Lock()
	m.cap = newCap
	over := m.current - m.cap
	wasOver := m.overFlag
	m.overFlag = over > 0
	cb := m.onCross
	m.mu.Unlock()
	if !wasOver && over > 0 && cb != nil {
		cb(over)
	}
}

// Reset clears current usage; called after a successful compaction
// pass that has freed budget.
func (m *PressureMonitor) Reset() {
	m.mu.Lock()
	m.current = 0
	m.overFlag = false
	m.mu.Unlock()
}
