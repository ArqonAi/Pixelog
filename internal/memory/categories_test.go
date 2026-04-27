package memory

import "testing"

func TestAllCategories(t *testing.T) {
	cats := AllCategories()
	if len(cats) != 6 {
		t.Errorf("expected 6 categories, got %d", len(cats))
	}
	seen := make(map[MemoryCategory]bool)
	for _, c := range cats {
		if seen[c] {
			t.Errorf("duplicate category: %s", c)
		}
		seen[c] = true
	}
}

func TestValidCategory(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"fact", true},
		{"preference", true},
		{"instruction", true},
		{"relationship", true},
		{"event", true},
		{"skill", true},
		{"unknown", false},
		{"", false},
		{"FACT", false},
	}
	for _, tt := range tests {
		if got := ValidCategory(tt.input); got != tt.want {
			t.Errorf("ValidCategory(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestCategoryWeight(t *testing.T) {
	if CategoryInstruction.Weight() != 1.5 {
		t.Errorf("instruction weight = %f, want 1.5", CategoryInstruction.Weight())
	}
	if CategoryEvent.Weight() != 0.7 {
		t.Errorf("event weight = %f, want 0.7", CategoryEvent.Weight())
	}
	if MemoryCategory("nonexistent").Weight() != 1.0 {
		t.Errorf("unknown category should default to 1.0")
	}
}

func TestCategoryWeightsCoverage(t *testing.T) {
	for _, c := range AllCategories() {
		if _, ok := CategoryWeights[c]; !ok {
			t.Errorf("category %s missing from CategoryWeights", c)
		}
	}
}
