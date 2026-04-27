package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// DedupWindow rejects duplicate content within a configurable time window
// using SHA-256 content hashing.
type DedupWindow struct {
	mu      sync.Mutex
	window  time.Duration
	entries map[string]time.Time // sha256 hash → first-seen timestamp
}

// NewDedupWindow creates a dedup window with the given duration.
// Content with the same SHA-256 hash submitted within the window is considered duplicate.
func NewDedupWindow(window time.Duration) *DedupWindow {
	return &DedupWindow{
		window:  window,
		entries: make(map[string]time.Time),
	}
}

// DefaultDedupWindow returns a 5-minute dedup window matching agentmemory defaults.
func DefaultDedupWindow() *DedupWindow {
	return NewDedupWindow(5 * time.Minute)
}

// IsDuplicate returns true if content with this hash was seen within the window.
// If not a duplicate, it records the hash.
func (d *DedupWindow) IsDuplicate(content string) bool {
	hash := ContentHash(content)
	return d.IsDuplicateHash(hash)
}

// IsDuplicateHash checks a pre-computed hash against the window.
func (d *DedupWindow) IsDuplicateHash(hash string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.evictStale()

	if firstSeen, exists := d.entries[hash]; exists {
		if time.Since(firstSeen) < d.window {
			return true
		}
		// Window expired — allow re-indexing
		d.entries[hash] = time.Now()
		return false
	}

	d.entries[hash] = time.Now()
	return false
}

// ContentHash computes a SHA-256 hex digest for the given content.
func ContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// Size returns the number of tracked hashes.
func (d *DedupWindow) Size() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.entries)
}

// Clear removes all entries.
func (d *DedupWindow) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries = make(map[string]time.Time)
}

// evictStale removes entries older than the window. Must be called under lock.
func (d *DedupWindow) evictStale() {
	cutoff := time.Now().Add(-d.window)
	for hash, ts := range d.entries {
		if ts.Before(cutoff) {
			delete(d.entries, hash)
		}
	}
}
