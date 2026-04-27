package memory

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AccessLog records every retrieval hit for a document/frame.
type AccessLog struct {
	DocID     string  `json:"doc_id"`
	Count     int     `json:"count"`
	LastAt    string  `json:"last_at"`
	Recent    []int64 `json:"recent"` // unix-millis of last N accesses (capped at 50)
	CreatedAt string  `json:"created_at"`
}

// AccessTracker records search hits to feed retention scoring and search boosting.
type AccessTracker struct {
	mu       sync.Mutex
	logs     map[string]*AccessLog
	dataDir  string
	maxRecent int
}

// NewAccessTracker creates a tracker backed by a JSON file in dataDir.
func NewAccessTracker(dataDir string) (*AccessTracker, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create access tracker dir: %w", err)
	}
	t := &AccessTracker{
		logs:      make(map[string]*AccessLog),
		dataDir:   dataDir,
		maxRecent: 50,
	}
	_ = t.load()
	return t, nil
}

// RecordAccess records a single search hit for a document.
func (t *AccessTracker) RecordAccess(docID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	log, ok := t.logs[docID]
	if !ok {
		log = &AccessLog{
			DocID:     docID,
			CreatedAt: now.Format(time.RFC3339),
		}
		t.logs[docID] = log
	}

	log.Count++
	log.LastAt = now.Format(time.RFC3339)
	log.Recent = append(log.Recent, now.UnixMilli())

	// Cap recent list
	if len(log.Recent) > t.maxRecent {
		log.Recent = log.Recent[len(log.Recent)-t.maxRecent:]
	}
}

// RecordBatch records access for multiple doc IDs.
func (t *AccessTracker) RecordBatch(docIDs []string) {
	for _, id := range docIDs {
		t.RecordAccess(id)
	}
}

// GetLog retrieves access stats for a single document.
func (t *AccessTracker) GetLog(docID string) *AccessLog {
	t.mu.Lock()
	defer t.mu.Unlock()
	if log, ok := t.logs[docID]; ok {
		cp := *log
		return &cp
	}
	return &AccessLog{DocID: docID}
}

// GetAllLogs returns a snapshot of all access logs.
func (t *AccessTracker) GetAllLogs() map[string]*AccessLog {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[string]*AccessLog, len(t.logs))
	for k, v := range t.logs {
		cp := *v
		out[k] = &cp
	}
	return out
}

// AccessBoost computes a boost factor (0.0–1.0) for search ranking based on access frequency.
// Higher access count and more recent accesses yield higher boost.
func (t *AccessTracker) AccessBoost(docID string) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	log, ok := t.logs[docID]
	if !ok || log.Count == 0 {
		return 0.0
	}

	// Frequency component: diminishing returns via log scale
	freqBoost := 0.0
	if log.Count > 0 {
		// log2(count+1) / log2(101) normalizes to [0, 1] for count in [0, 100]
		freqBoost = logBase2(float64(log.Count)+1) / logBase2(101)
		if freqBoost > 1.0 {
			freqBoost = 1.0
		}
	}

	// Recency component: days since last access
	recencyBoost := 0.0
	if len(log.Recent) > 0 {
		lastMs := log.Recent[len(log.Recent)-1]
		daysSince := float64(time.Now().UnixMilli()-lastMs) / (1000 * 60 * 60 * 24)
		if daysSince < 1 {
			recencyBoost = 1.0
		} else if daysSince < 30 {
			recencyBoost = 1.0 / daysSince
		}
	}

	return 0.6*freqBoost + 0.4*recencyBoost
}

// Persist writes all access logs to disk.
func (t *AccessTracker) Persist() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.save()
}

// Delete removes a document's access log.
func (t *AccessTracker) Delete(docID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.logs, docID)
}

func (t *AccessTracker) filePath() string {
	return filepath.Join(t.dataDir, "access_logs.json")
}

func (t *AccessTracker) load() error {
	data, err := os.ReadFile(t.filePath())
	if err != nil {
		return err
	}
	var logs map[string]*AccessLog
	if err := json.Unmarshal(data, &logs); err != nil {
		return err
	}
	t.logs = logs
	return nil
}

func (t *AccessTracker) save() error {
	data, err := json.Marshal(t.logs)
	if err != nil {
		return err
	}
	return os.WriteFile(t.filePath(), data, 0644)
}

func logBase2(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return math.Log2(x)
}
