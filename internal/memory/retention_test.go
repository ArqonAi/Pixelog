package memory

import (
	"testing"
	"time"
)

func TestRetentionScorer_Score(t *testing.T) {
	dir := t.TempDir()
	tracker, err := NewAccessTracker(dir)
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}

	config := DefaultDecayConfig()
	scorer := NewRetentionScorer(config, tracker)

	tests := []struct {
		name       string
		entry      MemoryEntry
		accesses   int
		wantMinScore float64
		wantMaxScore float64
		wantTier   string
	}{
		{
			name: "fresh architecture memory",
			entry: MemoryEntry{
				ID:        "arch1",
				Category:  "architecture",
				CreatedAt: time.Now(),
			},
			accesses:     0,
			wantMinScore: 0.8,
			wantMaxScore: 1.0,
			wantTier:     "hot",
		},
		{
			name: "old fact with no accesses",
			entry: MemoryEntry{
				ID:        "old_fact",
				Category:  "fact",
				CreatedAt: time.Now().Add(-365 * 24 * time.Hour), // 1 year old
			},
			accesses:     0,
			wantMinScore: 0.0,
			wantMaxScore: 0.1,
			wantTier:     "evictable",
		},
		{
			name: "medium-age pattern with accesses",
			entry: MemoryEntry{
				ID:        "pattern1",
				Category:  "pattern",
				CreatedAt: time.Now().Add(-30 * 24 * time.Hour), // 30 days old
			},
			accesses:     5,
			wantMinScore: 0.4,
			wantMaxScore: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < tt.accesses; i++ {
				tracker.RecordAccess(tt.entry.ID)
			}

			score := scorer.Score(tt.entry)

			if score.Score < tt.wantMinScore || score.Score > tt.wantMaxScore {
				t.Errorf("score %.4f not in range [%.2f, %.2f]", score.Score, tt.wantMinScore, tt.wantMaxScore)
			}
			if tt.wantTier != "" && score.Tier != tt.wantTier {
				t.Errorf("expected tier %q, got %q (score=%.4f)", tt.wantTier, score.Tier, score.Score)
			}
			if score.AccessCount != tt.accesses {
				t.Errorf("expected %d accesses, got %d", tt.accesses, score.AccessCount)
			}
		})
	}
}

func TestRetentionScorer_ScoreAll(t *testing.T) {
	dir := t.TempDir()
	tracker, err := NewAccessTracker(dir)
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}

	scorer := NewRetentionScorer(DefaultDecayConfig(), tracker)

	entries := []MemoryEntry{
		{ID: "a", Category: "architecture", CreatedAt: time.Now()},
		{ID: "b", Category: "fact", CreatedAt: time.Now().Add(-60 * 24 * time.Hour)},
		{ID: "c", Category: "bug", CreatedAt: time.Now().Add(-7 * 24 * time.Hour)},
	}

	scores, stats := scorer.ScoreAll(entries)

	if len(scores) != 3 {
		t.Fatalf("expected 3 scores, got %d", len(scores))
	}

	if stats.Total != 3 {
		t.Errorf("expected total 3, got %d", stats.Total)
	}

	// Verify sorted descending
	for i := 1; i < len(scores); i++ {
		if scores[i].Score > scores[i-1].Score {
			t.Errorf("scores not sorted descending: [%d]=%.4f > [%d]=%.4f",
				i, scores[i].Score, i-1, scores[i-1].Score)
		}
	}
}

func TestRetentionScorer_EvictionCandidates(t *testing.T) {
	dir := t.TempDir()
	tracker, err := NewAccessTracker(dir)
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}

	scorer := NewRetentionScorer(DefaultDecayConfig(), tracker)

	entries := []MemoryEntry{
		{ID: "fresh", Category: "architecture", CreatedAt: time.Now()},
		{ID: "stale1", Category: "fact", CreatedAt: time.Now().Add(-365 * 24 * time.Hour)},
		{ID: "stale2", Category: "fact", CreatedAt: time.Now().Add(-400 * 24 * time.Hour)},
	}

	candidates := scorer.EvictionCandidates(entries, 10)

	// At least the old ones should be candidates
	if len(candidates) < 1 {
		t.Error("expected at least 1 eviction candidate")
	}

	for _, c := range candidates {
		if c.DocID == "fresh" {
			t.Error("fresh memory should not be an eviction candidate")
		}
	}
}

func TestComputeSalience(t *testing.T) {
	tests := []struct {
		category    string
		accessCount int
		wantMin     float64
		wantMax     float64
	}{
		{"architecture", 0, 0.85, 0.95},
		{"fact", 0, 0.45, 0.55},
		{"bug", 10, 0.7, 1.0},
		{"unknown", 0, 0.45, 0.55},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			s := computeSalience(tt.category, tt.accessCount)
			if s < tt.wantMin || s > tt.wantMax {
				t.Errorf("salience(%q, %d) = %.4f, want [%.2f, %.2f]",
					tt.category, tt.accessCount, s, tt.wantMin, tt.wantMax)
			}
		})
	}
}
