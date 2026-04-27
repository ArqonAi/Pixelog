package memory

import (
	"math"
	"time"
)

// CapsuleSalienceConfig controls the four-component salience score used
// by the compactor to decide which children stay surfaced and which are
// pushed into the era's "removed middle".
//
// Score = α·access_freq + β·recency + γ·emotional_weight + δ·tier_weight
//
// Defaults are tuned for daily compaction of ~10–20 sessions; they
// produce a roughly even split between kept and buried at typical agent
// usage levels.
type CapsuleSalienceConfig struct {
	Alpha float64 `json:"alpha"` // access frequency weight
	Beta  float64 `json:"beta"`  // recency weight
	Gamma float64 `json:"gamma"` // emotional / category-confidence weight
	Delta float64 `json:"delta"` // tier-weight (preferences/instructions surface harder)

	// HalfLifeDays sets the recency decay constant: a child accessed
	// HalfLifeDays ago contributes exactly 0.5 to the recency component.
	HalfLifeDays float64 `json:"half_life_days"`
}

// DefaultCapsuleSalienceConfig returns production defaults.
func DefaultCapsuleSalienceConfig() CapsuleSalienceConfig {
	return CapsuleSalienceConfig{
		Alpha:        0.30,
		Beta:         0.30,
		Gamma:        0.25,
		Delta:        0.15,
		HalfLifeDays: 7.0,
	}
}

// CapsuleSalienceScorer computes salience for a candidate child capsule
// at the moment its parent era is about to be sealed.
type CapsuleSalienceScorer struct {
	cfg     CapsuleSalienceConfig
	tracker *AccessTracker
	now     func() time.Time // injectable for tests
}

// NewCapsuleSalienceScorer builds a scorer. tracker may be nil; in that
// case access-derived components contribute zero (recency from EndedAt
// still applies).
func NewCapsuleSalienceScorer(cfg CapsuleSalienceConfig, tracker *AccessTracker) *CapsuleSalienceScorer {
	if cfg.HalfLifeDays <= 0 {
		cfg.HalfLifeDays = 7
	}
	return &CapsuleSalienceScorer{cfg: cfg, tracker: tracker, now: time.Now}
}

// Score returns a normalised salience in [0, 1] for child.
func (s *CapsuleSalienceScorer) Score(child ChildRef) float64 {
	freq := s.frequencyComponent(child.URI)
	rec := s.recencyComponent(child.EndedAt)
	emo := emotionalComponent(child.Concepts)
	tw := tierWeight(child.Concepts)

	raw := s.cfg.Alpha*freq + s.cfg.Beta*rec + s.cfg.Gamma*emo + s.cfg.Delta*tw
	return clamp01(raw)
}

// frequencyComponent maps access count to [0, 1] via a saturating log.
func (s *CapsuleSalienceScorer) frequencyComponent(uri string) float64 {
	if s.tracker == nil || uri == "" {
		return 0
	}
	log := s.tracker.GetLog(uri)
	if log == nil || log.Count == 0 {
		return 0
	}
	// log2(count+1) normalised against a "very accessed" reference of 64.
	return math.Min(1, math.Log2(float64(log.Count)+1)/math.Log2(65))
}

// recencyComponent uses an exponential decay so a child accessed
// HalfLifeDays ago scores 0.5.
func (s *CapsuleSalienceScorer) recencyComponent(endedAt time.Time) float64 {
	if endedAt.IsZero() {
		return 0
	}
	days := s.now().Sub(endedAt).Hours() / 24.0
	if days <= 0 {
		return 1
	}
	// Exponential decay with the configured half-life.
	return math.Pow(0.5, days/s.cfg.HalfLifeDays)
}

// emotionalComponent treats a child's typed-memory concepts as a proxy
// for emotional/personal weight: preferences and instructions are sticky
// signals, events and skills are mid-weight, plain facts are light.
func emotionalComponent(concepts []string) float64 {
	if len(concepts) == 0 {
		return 0
	}
	max := 0.0
	for _, c := range concepts {
		if w, ok := emotionalWeights[c]; ok && w > max {
			max = w
		}
	}
	return max
}

var emotionalWeights = map[string]float64{
	string(CategoryPreference):   1.00,
	string(CategoryInstruction):  0.95,
	string(CategoryRelationship): 0.80,
	string(CategorySkill):        0.60,
	string(CategoryEvent):        0.50,
	string(CategoryFact):         0.30,
}

// tierWeight maps category concepts to the existing CategoryWeights so
// the same retrieval-time priorities apply at compaction time.
func tierWeight(concepts []string) float64 {
	if len(concepts) == 0 {
		return 0.5
	}
	max := 0.0
	for _, c := range concepts {
		if w, ok := CategoryWeights[MemoryCategory(c)]; ok {
			// Normalise: existing weights are 0.7–1.5; map onto [0, 1].
			n := (w - 0.7) / 0.8
			if n > max {
				max = clamp01(n)
			}
		}
	}
	return max
}

func clamp01(x float64) float64 {
	switch {
	case x < 0:
		return 0
	case x > 1:
		return 1
	default:
		return x
	}
}
