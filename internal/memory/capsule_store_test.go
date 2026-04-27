package memory

import (
	"context"
	"errors"
	"testing"
	"time"
)

func mkChild(hash string, n int) ChildRef {
	return ChildRef{Hash: hash, URI: "pixe://capsule/" + hash, L0: "child " + hash,
		StartedAt: time.Date(2026, 1, n, 0, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 1, n, 23, 0, 0, 0, time.UTC),
	}
}

// TestCapsuleStore_PutGetDelete is the table-driven roundtrip.
func TestCapsuleStore_PutGetDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCapsuleStore(dir)
	if err != nil {
		t.Fatalf("NewCapsuleStore: %v", err)
	}

	ec := NewEraCapsule("ns-a", LevelDay, []ChildRef{mkChild("aaaaaaaa1", 1), mkChild("bbbbbbbb1", 2)})
	ec.L0 = "test era"
	ec.L1 = "longer overview"

	hash, err := store.PutEra(ec)
	if err != nil {
		t.Fatalf("PutEra: %v", err)
	}
	if hash != ec.Hash || hash == "" {
		t.Fatalf("hash mismatch")
	}
	if !store.HasHash(hash) {
		t.Errorf("HasHash(%s) returned false", hash)
	}

	got, err := store.GetEra(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetEra: %v", err)
	}
	if got.L0 != ec.L0 || got.Namespace != ec.Namespace || got.Level != ec.Level {
		t.Errorf("read back mismatch: %+v vs %+v", got, ec)
	}

	// Reopen the store to confirm the index survives a process restart.
	store2, err := NewCapsuleStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if !store2.HasHash(hash) {
		t.Error("index not persisted across reopen")
	}
	if got := store2.ListByLevel("ns-a", LevelDay); len(got) != 1 {
		t.Errorf("ListByLevel after reopen: got %d, want 1", len(got))
	}

	if err := store2.Delete(hash); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store2.GetEra(context.Background(), hash); !errors.Is(err, CapsuleStoreErrNotFound) {
		t.Errorf("after Delete, want NotFound, got %v", err)
	}
	// Idempotent.
	if err := store2.Delete(hash); err != nil {
		t.Errorf("Delete twice: %v", err)
	}
}

// TestCapsuleStore_ListByLevel filters and sorts correctly.
func TestCapsuleStore_ListByLevel(t *testing.T) {
	store, _ := NewCapsuleStore(t.TempDir())

	makeEra := func(ns string, lvl EraLevel, day int) {
		ec := NewEraCapsule(ns, lvl, []ChildRef{mkChild("h"+ns+string(rune('0'+day)), day)})
		if _, err := store.PutEra(ec); err != nil {
			t.Fatalf("put: %v", err)
		}
	}
	makeEra("alice", LevelDay, 3)
	makeEra("alice", LevelDay, 1)
	makeEra("alice", LevelDay, 2)
	makeEra("alice", LevelWeek, 1)
	makeEra("bob", LevelDay, 1)

	got := store.ListByLevel("alice", LevelDay)
	if len(got) != 3 {
		t.Fatalf("alice/day count = %d, want 3", len(got))
	}
	// Must be sorted ascending by StartedAt.
	for i := 1; i < len(got); i++ {
		if got[i].StartedAt < got[i-1].StartedAt {
			t.Errorf("not sorted ascending: %d < %d", got[i].StartedAt, got[i-1].StartedAt)
		}
	}
	if got := store.ListByLevel("alice", LevelWeek); len(got) != 1 {
		t.Errorf("alice/week count = %d, want 1", len(got))
	}
	if got := store.ListByLevel("bob", LevelDay); len(got) != 1 {
		t.Errorf("bob/day count = %d, want 1", len(got))
	}
}

// TestCapsuleStore_GetMissing returns a typed not-found.
func TestCapsuleStore_GetMissing(t *testing.T) {
	store, _ := NewCapsuleStore(t.TempDir())
	_, err := store.GetEra(context.Background(), "deadbeef")
	if !errors.Is(err, CapsuleStoreErrNotFound) {
		t.Errorf("missing capsule: want NotFound, got %v", err)
	}
}
