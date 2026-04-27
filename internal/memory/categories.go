package memory

import (
	"time"
)

// MemoryCategory represents a typed classification for extracted memories.
// Categories drive weighted retrieval, tier policies, and capsule indexing.
type MemoryCategory string

const (
	CategoryFact         MemoryCategory = "fact"
	CategoryPreference   MemoryCategory = "preference"
	CategoryInstruction  MemoryCategory = "instruction"
	CategoryRelationship MemoryCategory = "relationship"
	CategoryEvent        MemoryCategory = "event"
	CategorySkill        MemoryCategory = "skill"
)

// AllCategories returns all valid memory categories.
func AllCategories() []MemoryCategory {
	return []MemoryCategory{
		CategoryFact,
		CategoryPreference,
		CategoryInstruction,
		CategoryRelationship,
		CategoryEvent,
		CategorySkill,
	}
}

// ValidCategory checks if a string is a valid memory category.
func ValidCategory(s string) bool {
	for _, c := range AllCategories() {
		if string(c) == s {
			return true
		}
	}
	return false
}

// CategoryWeights defines retrieval weighting per category.
// Higher weight = more influence in hybrid search scoring.
var CategoryWeights = map[MemoryCategory]float64{
	CategoryFact:         1.0,
	CategoryPreference:   1.2, // boost preferences for personalization
	CategoryInstruction:  1.5, // instructions are high-priority context
	CategoryRelationship: 0.8,
	CategoryEvent:        0.7, // events decay in relevance faster
	CategorySkill:        0.9,
}

// Weight returns the retrieval weight for a category, defaulting to 1.0.
func (c MemoryCategory) Weight() float64 {
	if w, ok := CategoryWeights[c]; ok {
		return w
	}
	return 1.0
}

// TypedMemory represents a classified memory entry. This is the primary
// payload type used by the archival pipeline and category retrieval.
type TypedMemory struct {
	ID         string         `json:"id"`
	Category   MemoryCategory `json:"category"`
	Key        string         `json:"key"`
	Value      string         `json:"value"`
	Source     string         `json:"source,omitempty"`     // where this was extracted from
	Timestamp  time.Time      `json:"timestamp"`
	Confidence float64        `json:"confidence,omitempty"` // extraction confidence 0-1
}
