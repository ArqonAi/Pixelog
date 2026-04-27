package memory

import (
	"testing"
	"time"
)

func TestDedupWindow_IsDuplicate(t *testing.T) {
	dedup := NewDedupWindow(1 * time.Second)

	content := "hello world this is test content"

	// First submission — not duplicate
	if dedup.IsDuplicate(content) {
		t.Error("first submission should not be duplicate")
	}

	// Immediate re-submission — duplicate
	if !dedup.IsDuplicate(content) {
		t.Error("immediate re-submission should be duplicate")
	}

	// Different content — not duplicate
	if dedup.IsDuplicate("completely different content") {
		t.Error("different content should not be duplicate")
	}
}

func TestDedupWindow_Expiry(t *testing.T) {
	dedup := NewDedupWindow(50 * time.Millisecond)

	content := "test content for expiry"

	if dedup.IsDuplicate(content) {
		t.Error("first submission should not be duplicate")
	}

	// Wait for window to expire
	time.Sleep(100 * time.Millisecond)

	if dedup.IsDuplicate(content) {
		t.Error("should not be duplicate after window expires")
	}
}

func TestDedupWindow_Size(t *testing.T) {
	dedup := NewDedupWindow(1 * time.Second)

	dedup.IsDuplicate("content 1")
	dedup.IsDuplicate("content 2")
	dedup.IsDuplicate("content 3")

	if dedup.Size() != 3 {
		t.Errorf("expected 3 entries, got %d", dedup.Size())
	}
}

func TestDedupWindow_Clear(t *testing.T) {
	dedup := NewDedupWindow(1 * time.Second)

	dedup.IsDuplicate("content 1")
	dedup.IsDuplicate("content 2")

	dedup.Clear()

	if dedup.Size() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", dedup.Size())
	}

	// Previously seen content should now be accepted
	if dedup.IsDuplicate("content 1") {
		t.Error("content should not be duplicate after clear")
	}
}

func TestContentHash(t *testing.T) {
	h1 := ContentHash("hello world")
	h2 := ContentHash("hello world")
	h3 := ContentHash("hello world!")

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex SHA-256, got %d chars", len(h1))
	}
}

func TestDefaultDedupWindow(t *testing.T) {
	dedup := DefaultDedupWindow()
	if dedup.window != 5*time.Minute {
		t.Errorf("expected 5-minute default window, got %v", dedup.window)
	}
}
