package memory

import (
	"strings"
	"testing"
	"time"
)

// TestEraLevel_RoundTrip validates name parsing for every level.
func TestEraLevel_RoundTrip(t *testing.T) {
	for _, lvl := range []EraLevel{LevelSession, LevelDay, LevelWeek, LevelMonth, LevelQuarter, LevelYear, LevelDecade} {
		got, err := ParseEraLevel(lvl.String())
		if err != nil || got != lvl {
			t.Errorf("roundtrip %v: parse(%q)=%v err=%v", lvl, lvl.String(), got, err)
		}
	}
	if _, err := ParseEraLevel("nonsense"); err == nil {
		t.Error("ParseEraLevel(nonsense) should error")
	}
}

// TestEraLevel_Parent confirms the Parent ladder and decade saturation.
func TestEraLevel_Parent(t *testing.T) {
	cases := []struct {
		in, want EraLevel
	}{
		{LevelSession, LevelDay},
		{LevelDay, LevelWeek},
		{LevelWeek, LevelMonth},
		{LevelMonth, LevelQuarter},
		{LevelQuarter, LevelYear},
		{LevelYear, LevelDecade},
		{LevelDecade, LevelDecade}, // saturates
	}
	for _, c := range cases {
		if got := c.in.Parent(); got != c.want {
			t.Errorf("%s.Parent() = %s, want %s", c.in, got, c.want)
		}
	}
}

// TestEraCapsule_FinalizeIsDeterministic ensures the canonical hash is
// stable across runs and changes when content changes.
func TestEraCapsule_FinalizeIsDeterministic(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	build := func() *EraCapsule {
		ec := NewEraCapsule("ns-1", LevelDay, []ChildRef{
			{URI: "pixe://capsule/aaa", Hash: "aaa", Level: LevelSession, L0: "session 1", StartedAt: now, EndedAt: now.Add(time.Hour)},
			{URI: "pixe://capsule/bbb", Hash: "bbb", Level: LevelSession, L0: "session 2", StartedAt: now.Add(2 * time.Hour), EndedAt: now.Add(3 * time.Hour)},
		})
		ec.CreatedAt = now
		ec.L0 = "day summary"
		ec.L1 = "day overview"
		return ec
	}

	a := build()
	b := build()
	hashA, err := a.Finalize()
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	hashB, err := b.Finalize()
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if hashA != hashB {
		t.Errorf("hash not deterministic: %s vs %s", hashA, hashB)
	}
	if !strings.HasPrefix(a.URI(), "pixe://capsule/") {
		t.Errorf("URI not built: %q", a.URI())
	}

	// Mutate L0; hash must change.
	c := build()
	c.L0 = "different summary"
	hashC, _ := c.Finalize()
	if hashC == hashA {
		t.Error("hash should change when L0 changes")
	}
}

// TestEraCapsule_SortChronological proves children are stored in
// timeline order regardless of construction order.
func TestEraCapsule_SortChronological(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ec := NewEraCapsule("ns", LevelDay, []ChildRef{
		{Hash: "late", StartedAt: now.Add(2 * time.Hour), EndedAt: now.Add(3 * time.Hour)},
		{Hash: "early", StartedAt: now, EndedAt: now.Add(time.Hour)},
		{Hash: "mid", StartedAt: now.Add(time.Hour), EndedAt: now.Add(2 * time.Hour)},
	})
	want := []string{"early", "mid", "late"}
	for i, w := range want {
		if ec.Children[i].Hash != w {
			t.Errorf("child[%d].Hash = %s, want %s", i, ec.Children[i].Hash, w)
		}
	}
	if !ec.StartedAt.Equal(now) {
		t.Errorf("StartedAt = %v, want %v", ec.StartedAt, now)
	}
	if !ec.EndedAt.Equal(now.Add(3 * time.Hour)) {
		t.Errorf("EndedAt = %v, want %v", ec.EndedAt, now.Add(3*time.Hour))
	}
}

// TestEraCapsule_SurfaceVsBuried partitions children correctly.
func TestEraCapsule_SurfaceVsBuried(t *testing.T) {
	now := time.Now()
	ec := NewEraCapsule("ns", LevelDay, []ChildRef{
		{Hash: "k1", StartedAt: now, Buried: false},
		{Hash: "b1", StartedAt: now.Add(time.Hour), Buried: true},
		{Hash: "k2", StartedAt: now.Add(2 * time.Hour), Buried: false},
		{Hash: "b2", StartedAt: now.Add(3 * time.Hour), Buried: true},
	})
	if got := len(ec.SurfaceChildren()); got != 2 {
		t.Errorf("SurfaceChildren count = %d, want 2", got)
	}
	if got := len(ec.BuriedChildren()); got != 2 {
		t.Errorf("BuriedChildren count = %d, want 2", got)
	}
}
