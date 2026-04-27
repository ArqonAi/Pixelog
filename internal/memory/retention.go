package memory

import (
	"math"
	"sort"
	"time"
)

// DecayConfig controls the Ebbinghaus forgetting curve parameters.
type DecayConfig struct {
	Lambda         float64        `json:"lambda"`          // exponential decay rate (default 0.01)
	Sigma          float64        `json:"sigma"`           // reinforcement boost scaling (default 0.3)
	TierThresholds TierThresholds `json:"tier_thresholds"` // score boundaries for hot/warm/cold/evict
}

// TierThresholds defines score cutoffs for memory tiers.
type TierThresholds struct {
	Hot  float64 `json:"hot"`  // >= 0.7 → hot (high priority retention)
	Warm float64 `json:"warm"` // >= 0.4 → warm (normal retention)
	Cold float64 `json:"cold"` // >= 0.15 → cold (candidate for archival)
	// Below cold → evictable
}

// DefaultDecayConfig returns production-tuned defaults inspired by agentmemory.
func DefaultDecayConfig() DecayConfig {
	return DecayConfig{
		Lambda: 0.01,
		Sigma:  0.3,
		TierThresholds: TierThresholds{
			Hot:  0.7,
			Warm: 0.4,
			Cold: 0.15,
		},
	}
}

// RetentionScore represents the computed retention value for a single memory.
type RetentionScore struct {
	DocID              string  `json:"doc_id"`
	Score              float64 `json:"score"`               // combined retention score [0, 1]
	Salience           float64 `json:"salience"`            // base importance
	TemporalDecay      float64 `json:"temporal_decay"`      // time-based decay factor
	ReinforcementBoost float64 `json:"reinforcement_boost"` // access-frequency boost
	AccessCount        int     `json:"access_count"`
	LastAccessed       string  `json:"last_accessed"`
	Tier               string  `json:"tier"` // "hot", "warm", "cold", "evictable"
}

// RetentionTierStats summarizes how many memories fall in each tier.
type RetentionTierStats struct {
	Hot       int `json:"hot"`
	Warm      int `json:"warm"`
	Cold      int `json:"cold"`
	Evictable int `json:"evictable"`
	Total     int `json:"total"`
}

// MemoryEntry is the minimal interface a memory must satisfy for retention scoring.
type MemoryEntry struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"` // "fact", "pattern", "architecture", "preference", "bug", "workflow"
	CreatedAt time.Time `json:"created_at"`
}

// RetentionScorer computes Ebbinghaus-curve retention scores for memories.
type RetentionScorer struct {
	config  DecayConfig
	tracker *AccessTracker
}

// NewRetentionScorer creates a scorer with the given decay config and access tracker.
func NewRetentionScorer(config DecayConfig, tracker *AccessTracker) *RetentionScorer {
	return &RetentionScorer{
		config:  config,
		tracker: tracker,
	}
}

// ScoreAll computes retention scores for all provided memories.
func (rs *RetentionScorer) ScoreAll(entries []MemoryEntry) ([]RetentionScore, RetentionTierStats) {
	scores := make([]RetentionScore, 0, len(entries))

	for _, entry := range entries {
		score := rs.Score(entry)
		scores = append(scores, score)
	}

	// Sort descending by score
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	stats := rs.computeStats(scores)
	return scores, stats
}

// Score computes the retention score for a single memory entry.
func (rs *RetentionScorer) Score(entry MemoryEntry) RetentionScore {
	log := rs.tracker.GetLog(entry.ID)

	salience := computeSalience(entry.Category, log.Count)

	deltaT := time.Since(entry.CreatedAt).Hours() / 24 // days
	temporalDecay := math.Exp(-rs.config.Lambda * deltaT)

	reinforcement := computeReinforcementBoost(log.Recent, rs.config.Sigma)

	score := math.Min(1.0, salience*temporalDecay+reinforcement)

	tier := rs.classifyTier(score)

	return RetentionScore{
		DocID:              entry.ID,
		Score:              score,
		Salience:           salience,
		TemporalDecay:      temporalDecay,
		ReinforcementBoost: reinforcement,
		AccessCount:        log.Count,
		LastAccessed:       log.LastAt,
		Tier:               tier,
	}
}

// EvictionCandidates returns memory IDs with retention scores below the cold threshold.
func (rs *RetentionScorer) EvictionCandidates(entries []MemoryEntry, maxEvict int) []RetentionScore {
	scores, _ := rs.ScoreAll(entries)

	var candidates []RetentionScore
	for _, s := range scores {
		if s.Score < rs.config.TierThresholds.Cold {
			candidates = append(candidates, s)
		}
	}

	// Return lowest scores first (most stale)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score < candidates[j].Score
	})

	if maxEvict > 0 && len(candidates) > maxEvict {
		candidates = candidates[:maxEvict]
	}

	return candidates
}

func computeSalience(category string, accessCount int) float64 {
	typeWeights := map[string]float64{
		"architecture": 0.9,
		"pattern":      0.8,
		"preference":   0.85,
		"bug":          0.7,
		"workflow":     0.6,
		"fact":         0.5,
		"instruction":  0.85,
		"relationship": 0.6,
		"event":        0.5,
		"skill":        0.7,
	}

	base := 0.5
	if w, ok := typeWeights[category]; ok {
		base = w
	}

	// Access frequency bonus (diminishing returns)
	accessBonus := math.Min(0.2, float64(accessCount)*0.02)
	return math.Min(1.0, base+accessBonus)
}

func computeReinforcementBoost(recentAccessMs []int64, sigma float64) float64 {
	now := time.Now().UnixMilli()
	boost := 0.0

	for _, tAccess := range recentAccessMs {
		daysSince := float64(now-tAccess) / (1000 * 60 * 60 * 24)
		if daysSince > 0 {
			boost += 1.0 / daysSince
		}
	}

	return boost * sigma
}

func (rs *RetentionScorer) classifyTier(score float64) string {
	switch {
	case score >= rs.config.TierThresholds.Hot:
		return "hot"
	case score >= rs.config.TierThresholds.Warm:
		return "warm"
	case score >= rs.config.TierThresholds.Cold:
		return "cold"
	default:
		return "evictable"
	}
}

func (rs *RetentionScorer) computeStats(scores []RetentionScore) RetentionTierStats {
	stats := RetentionTierStats{Total: len(scores)}
	for _, s := range scores {
		switch s.Tier {
		case "hot":
			stats.Hot++
		case "warm":
			stats.Warm++
		case "cold":
			stats.Cold++
		default:
			stats.Evictable++
		}
	}
	return stats
}
