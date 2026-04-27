package memory

import (
	"os"
	"testing"
)

func TestAccessTracker_RecordAndGet(t *testing.T) {
	dir := t.TempDir()
	tracker, err := NewAccessTracker(dir)
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}

	// No data yet
	log := tracker.GetLog("doc1")
	if log.Count != 0 {
		t.Errorf("expected 0 access count for new doc, got %d", log.Count)
	}

	// Record accesses
	tracker.RecordAccess("doc1")
	tracker.RecordAccess("doc1")
	tracker.RecordAccess("doc1")
	tracker.RecordAccess("doc2")

	log = tracker.GetLog("doc1")
	if log.Count != 3 {
		t.Errorf("expected 3 accesses for doc1, got %d", log.Count)
	}
	if log.DocID != "doc1" {
		t.Errorf("expected doc ID 'doc1', got %q", log.DocID)
	}
	if len(log.Recent) != 3 {
		t.Errorf("expected 3 recent timestamps, got %d", len(log.Recent))
	}

	log = tracker.GetLog("doc2")
	if log.Count != 1 {
		t.Errorf("expected 1 access for doc2, got %d", log.Count)
	}
}

func TestAccessTracker_RecordBatch(t *testing.T) {
	dir := t.TempDir()
	tracker, err := NewAccessTracker(dir)
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}

	tracker.RecordBatch([]string{"a", "b", "c", "a"})

	if tracker.GetLog("a").Count != 2 {
		t.Errorf("expected 2 accesses for 'a', got %d", tracker.GetLog("a").Count)
	}
	if tracker.GetLog("b").Count != 1 {
		t.Errorf("expected 1 access for 'b', got %d", tracker.GetLog("b").Count)
	}
}

func TestAccessTracker_AccessBoost(t *testing.T) {
	dir := t.TempDir()
	tracker, err := NewAccessTracker(dir)
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}

	// No accesses → 0 boost
	boost := tracker.AccessBoost("doc1")
	if boost != 0.0 {
		t.Errorf("expected 0 boost for unaccessed doc, got %.4f", boost)
	}

	// Recent accesses → positive boost
	tracker.RecordAccess("doc1")
	tracker.RecordAccess("doc1")
	tracker.RecordAccess("doc1")

	boost = tracker.AccessBoost("doc1")
	if boost <= 0.0 {
		t.Errorf("expected positive boost after accesses, got %.4f", boost)
	}
	if boost > 1.0 {
		t.Errorf("expected boost <= 1.0, got %.4f", boost)
	}
}

func TestAccessTracker_Persist(t *testing.T) {
	dir := t.TempDir()
	tracker, err := NewAccessTracker(dir)
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}

	tracker.RecordAccess("doc1")
	tracker.RecordAccess("doc1")

	if err := tracker.Persist(); err != nil {
		t.Fatalf("failed to persist: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(tracker.filePath()); os.IsNotExist(err) {
		t.Fatal("access_logs.json not created")
	}

	// Reload and verify
	tracker2, err := NewAccessTracker(dir)
	if err != nil {
		t.Fatalf("failed to reload tracker: %v", err)
	}

	log := tracker2.GetLog("doc1")
	if log.Count != 2 {
		t.Errorf("expected 2 accesses after reload, got %d", log.Count)
	}
}

func TestAccessTracker_Delete(t *testing.T) {
	dir := t.TempDir()
	tracker, err := NewAccessTracker(dir)
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}

	tracker.RecordAccess("doc1")
	tracker.Delete("doc1")

	log := tracker.GetLog("doc1")
	if log.Count != 0 {
		t.Errorf("expected 0 accesses after delete, got %d", log.Count)
	}
}

func TestAccessTracker_GetAllLogs(t *testing.T) {
	dir := t.TempDir()
	tracker, err := NewAccessTracker(dir)
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}

	tracker.RecordAccess("doc1")
	tracker.RecordAccess("doc2")
	tracker.RecordAccess("doc3")

	logs := tracker.GetAllLogs()
	if len(logs) != 3 {
		t.Errorf("expected 3 logs, got %d", len(logs))
	}
}
