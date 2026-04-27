package memory

import (
	"context"
	"strings"
	"testing"
	"time"
)

// childWithSalience builds a deterministic ChildRef whose access tracker
// log will produce the requested base salience when the default scorer
// runs on it.
func childWithSalience(t *testing.T, hash string, idx int, accessCount int, concept MemoryCategory, tracker *AccessTracker) ChildRef {
	t.Helper()
	uri := "pixe://capsule/" + hash
	if accessCount > 0 && tracker != nil {
		for i := 0; i < accessCount; i++ {
			tracker.RecordAccess(uri)
		}
	}
	return ChildRef{
		URI:       uri,
		Hash:      hash,
		Level:     LevelSession,
		L0:        "summary " + hash,
		L1:        "longer overview for " + hash,
		StartedAt: time.Date(2026, 1, 1, idx, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 1, 1, idx, 30, 0, 0, time.UTC),
		Concepts:  []string{string(concept)},
	}
}

// TestCompactor_RemovedMiddle proves the Sierpinski-style collapse:
// top-K by salience are kept (full L1), the rest are buried (L1 stripped).
func TestCompactor_RemovedMiddle(t *testing.T) {
	tracker, _ := NewAccessTracker(t.TempDir())

	// 10 children: salience varies via access count + category.
	var children []ChildRef
	for i := 0; i < 10; i++ {
		access := 0
		concept := CategoryFact
		switch {
		case i < 3:
			access = 50
			concept = CategoryPreference
		case i < 6:
			access = 5
			concept = CategoryEvent
		default:
			access = 0
			concept = CategoryFact
		}
		children = append(children, childWithSalience(t, "h"+string(rune('a'+i)), i, access, concept, tracker))
	}

	cfg := DefaultCompactionConfig()
	cfg.SurfaceRatio = 0.30
	cfg.MinSurfaceCount = 3
	c := NewCompactor(cfg, tracker, nil)

	era, err := c.Compact(context.Background(), "ns", LevelDay, children)
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if era.Level != LevelDay {
		t.Errorf("level = %s, want day", era.Level)
	}
	if got := len(era.Children); got != 10 {
		t.Errorf("child count = %d, want 10", got)
	}

	// Exactly 3 surfaced, 7 buried.
	surfaced := 0
	buried := 0
	for _, ch := range era.Children {
		if ch.Buried {
			buried++
			if ch.L1 != "" {
				t.Errorf("buried child %s still has L1", ch.Hash)
			}
		} else {
			surfaced++
			if ch.L1 == "" {
				t.Errorf("surfaced child %s lost L1", ch.Hash)
			}
		}
	}
	if surfaced != 3 || buried != 7 {
		t.Errorf("surfaced=%d buried=%d, want 3+7", surfaced, buried)
	}

	// The kept set should be the high-salience preferences (first three children).
	for i := 0; i < 3; i++ {
		ref := era.Children[i]
		if ref.Concepts[0] != string(CategoryPreference) {
			// Children list is chronological; the surfaced set should
			// still contain all 3 preferences (they were the first
			// chronologically in this fixture).
			continue
		}
		if ref.Buried {
			t.Errorf("expected high-salience preference %s to be surfaced", ref.Hash)
		}
	}

	// Era-level concepts merge from all children.
	if len(era.Concepts) < 3 {
		t.Errorf("era concepts not merged; got %v", era.Concepts)
	}

	// Hash + URI populated.
	if era.Hash == "" {
		t.Error("era hash not finalized")
	}
	if !strings.HasPrefix(era.URI(), "pixe://capsule/") {
		t.Errorf("URI not built: %q", era.URI())
	}
}

// TestCompactor_MinSurfaceCount honors the floor even at small N.
func TestCompactor_MinSurfaceCount(t *testing.T) {
	tracker, _ := NewAccessTracker(t.TempDir())
	var children []ChildRef
	for i := 0; i < 4; i++ {
		children = append(children, childWithSalience(t, "x"+string(rune('a'+i)), i, 1, CategoryFact, tracker))
	}
	cfg := DefaultCompactionConfig()
	cfg.SurfaceRatio = 0.10  // 10% of 4 = 0; floor must apply
	cfg.MinSurfaceCount = 3
	c := NewCompactor(cfg, tracker, nil)
	era, err := c.Compact(context.Background(), "ns", LevelDay, children)
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	surfaced := 0
	for _, ch := range era.Children {
		if !ch.Buried {
			surfaced++
		}
	}
	if surfaced != 3 {
		t.Errorf("surfaced = %d, want 3 (MinSurfaceCount floor)", surfaced)
	}
}

// TestCompactor_SingleChildPassthrough handles the degenerate case.
func TestCompactor_SingleChildPassthrough(t *testing.T) {
	tracker, _ := NewAccessTracker(t.TempDir())
	c := NewCompactor(DefaultCompactionConfig(), tracker, nil)
	child := childWithSalience(t, "only", 0, 1, CategoryPreference, tracker)
	era, err := c.Compact(context.Background(), "ns", LevelDay, []ChildRef{child})
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if len(era.Children) != 1 || era.Children[0].Buried {
		t.Errorf("single child should be surfaced; got %+v", era.Children)
	}
}

// TestCompactor_RecursiveCompaction validates two-level fold:
// session→day, then day→week, with the resulting week capsule
// referencing day capsules whose URIs are themselves resolvable.
func TestCompactor_RecursiveCompaction(t *testing.T) {
	tracker, _ := NewAccessTracker(t.TempDir())
	c := NewCompactor(DefaultCompactionConfig(), tracker, nil)

	// Build 3 day eras, each from 4 sessions.
	var days []ChildRef
	for d := 0; d < 3; d++ {
		var sessions []ChildRef
		for s := 0; s < 4; s++ {
			sessions = append(sessions, childWithSalience(t, "d"+string(rune('a'+d))+"s"+string(rune('a'+s)), s, 2, CategoryFact, tracker))
		}
		era, err := c.Compact(context.Background(), "ns", LevelDay, sessions)
		if err != nil {
			t.Fatalf("day Compact: %v", err)
		}
		days = append(days, refFromEra(era))
	}

	week, err := c.Compact(context.Background(), "ns", LevelWeek, days)
	if err != nil {
		t.Fatalf("week Compact: %v", err)
	}
	if week.Level != LevelWeek {
		t.Errorf("level = %s", week.Level)
	}
	if len(week.Children) != 3 {
		t.Errorf("week has %d children, want 3", len(week.Children))
	}
	for _, child := range week.Children {
		if child.Level != LevelDay {
			t.Errorf("week child level = %s, want day", child.Level)
		}
		if child.URI == "" || child.Hash == "" {
			t.Errorf("week child missing URI/Hash: %+v", child)
		}
	}
}
