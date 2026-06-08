package bench

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestRetrieveHybridContext_RanksRelevantTurnFirst verifies the new
// Answer-path context source ranks the turn that directly answers the
// question ahead of noise turns — this is the signal that drives QA
// accuracy above the old CategoryStore.Search + fallbackScan path.
func TestRetrieveHybridContext_RanksRelevantTurnFirst(t *testing.T) {
	mem := NewPixelogMemory(PixelogConfig{Embedder: NewHashEmbedder(128)})
	ctx := context.Background()
	ns := "hybrid-ctx"

	base := time.Date(2023, 5, 1, 12, 0, 0, 0, time.UTC)
	turns := []Turn{
		{TurnID: "t1", Role: "user", Speaker: "Alice", Text: "The weather has been cold lately.", SessionID: "s1", Timestamp: base},
		{TurnID: "t2", Role: "user", Speaker: "Alice", Text: "My favorite restaurant is Trattoria Lucca in Dublin.", SessionID: "s1", Timestamp: base.Add(time.Hour)},
		{TurnID: "t3", Role: "user", Speaker: "Bob", Text: "I hate cilantro in my food.", SessionID: "s2", Timestamp: base.Add(48 * time.Hour)},
		{TurnID: "t4", Role: "user", Speaker: "Alice", Text: "I plan to visit Paris next year.", SessionID: "s2", Timestamp: base.Add(50 * time.Hour)},
	}
	if err := mem.Ingest(ctx, ns, turns); err != nil {
		t.Fatal(err)
	}

	texts, ids, err := mem.retrieveHybridContext(ctx, ns, "What is Alice's favorite restaurant?", 3)
	if err != nil {
		t.Fatalf("retrieveHybridContext: %v", err)
	}
	if len(texts) == 0 {
		t.Fatalf("expected non-empty context")
	}

	// Top hit should mention the restaurant.
	found := false
	for i, txt := range texts {
		if strings.Contains(strings.ToLower(txt), "trattoria") {
			found = true
			if ids[i] != "t2" {
				t.Errorf("id mismatch at pos %d: got %q want t2", i, ids[i])
			}
			break
		}
	}
	if !found {
		t.Errorf("expected Trattoria turn in top-3 context, got %v", texts)
	}

	// Texts should carry [YYYY-MM-DD] prefixes for temporal reasoning.
	for _, txt := range texts {
		if !strings.Contains(txt, "[2023-") {
			t.Errorf("expected ISO-date prefix in %q", txt)
		}
	}
}

// TestRetrieveHybridContext_FactEvidencePreamble verifies that when
// PixelogConfig.FactEvidence=true, the hybrid retriever prepends a
// "KNOWN FACTS:" preamble derived from rule-extracted triples whose
// subject matches a query entity. This is the production lever for
// closing the answerer-extraction gap on direct-lookup and
// counterfactual-inference questions.
func TestRetrieveHybridContext_FactEvidencePreamble(t *testing.T) {
	mem := NewPixelogMemory(PixelogConfig{
		Embedder:     NewHashEmbedder(128),
		FactEvidence: true,
	})
	ctx := context.Background()
	ns := "fact-pre"

	base := time.Date(2023, 5, 1, 12, 0, 0, 0, time.UTC)
	turns := []Turn{
		{TurnID: "t1", Role: "user", Speaker: "Alice", Text: "The weather has been cold lately.", SessionID: "s1", Timestamp: base},
		{TurnID: "t2", Role: "user", Speaker: "Alice", Text: "My favorite restaurant is Trattoria Lucca in Dublin.", SessionID: "s1", Timestamp: base.Add(time.Hour)},
		{TurnID: "t3", Role: "user", Speaker: "Bob", Text: "I hate cilantro in my food.", SessionID: "s2", Timestamp: base.Add(48 * time.Hour)},
	}
	if err := mem.Ingest(ctx, ns, turns); err != nil {
		t.Fatal(err)
	}

	texts, ids, err := mem.retrieveHybridContext(ctx, ns, "What is Alice's favorite restaurant?", 3)
	if err != nil {
		t.Fatalf("retrieveHybridContext: %v", err)
	}
	if len(texts) == 0 {
		t.Fatalf("expected non-empty context")
	}
	if !strings.HasPrefix(texts[0], "KNOWN FACTS:") {
		t.Fatalf("expected KNOWN FACTS preamble at index 0, got %q", texts[0])
	}
	if ids[0] != "FACTS" {
		t.Errorf("expected synthetic FACTS id at index 0, got %q", ids[0])
	}
	// Preamble must reference Alice's favorite-restaurant fact.
	pre := strings.ToLower(texts[0])
	if !strings.Contains(pre, "alice") {
		t.Errorf("preamble missing subject Alice: %q", texts[0])
	}
	if !strings.Contains(pre, "favorite-restaurant") {
		t.Errorf("preamble missing favorite-restaurant predicate: %q", texts[0])
	}
	if !strings.Contains(pre, "trattoria") {
		t.Errorf("preamble missing object Trattoria: %q", texts[0])
	}
	// Source-turn evidence must still follow the preamble.
	turnFound := false
	for i := 1; i < len(texts); i++ {
		if strings.Contains(strings.ToLower(texts[i]), "trattoria") {
			turnFound = true
			break
		}
	}
	if !turnFound {
		t.Errorf("expected source turn t2 to follow preamble, texts=%v", texts)
	}
}

// TestRetrieveHybridContext_FactEvidenceDisabled verifies the default
// path (FactEvidence=false) emits NO preamble and ids[0] is a real
// turn id, preserving Hit@K backwards compatibility.
func TestRetrieveHybridContext_FactEvidenceDisabled(t *testing.T) {
	mem := NewPixelogMemory(PixelogConfig{Embedder: NewHashEmbedder(128)}) // FactEvidence default false
	ctx := context.Background()
	ns := "fact-off"

	base := time.Date(2023, 5, 1, 12, 0, 0, 0, time.UTC)
	turns := []Turn{
		{TurnID: "t1", Role: "user", Speaker: "Alice", Text: "My favorite restaurant is Trattoria Lucca.", SessionID: "s1", Timestamp: base},
	}
	if err := mem.Ingest(ctx, ns, turns); err != nil {
		t.Fatal(err)
	}
	_, ids, err := mem.retrieveHybridContext(ctx, ns, "What is Alice's favorite restaurant?", 3)
	if err != nil {
		t.Fatalf("retrieveHybridContext: %v", err)
	}
	if len(ids) == 0 {
		t.Fatalf("expected at least one hit")
	}
	if ids[0] == "FACTS" {
		t.Errorf("preamble emitted with FactEvidence=false: ids=%v", ids)
	}
}

// TestFormatFactPreamble_DedupAndOrder verifies the preamble formatter
// dedupes (subject, predicate, object) tuples (keeping highest
// confidence) and emits a deterministic, human-readable layout.
func TestFormatFactPreamble_DedupAndOrder(t *testing.T) {
	t1 := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(24 * time.Hour)
	facts := []Fact{
		{Subject: "Caroline", Predicate: "favorite-color", Object: "blue", SourceTurnID: "D3:7", Confidence: 0.9, Timestamp: t1},
		{Subject: "Caroline", Predicate: "favorite-color", Object: "blue", SourceTurnID: "D3:7", Confidence: 0.7, Timestamp: t1}, // dup, lower conf
		{Subject: "Caroline", Predicate: "raised-in", Object: "Toronto", SourceTurnID: "D5:2", Confidence: 0.85, Timestamp: t2},
		{Subject: "Andrew", Predicate: "owns", Object: "a Tesla", SourceTurnID: "D2:4", Confidence: 0.8, Timestamp: t1},
	}
	out := formatFactPreamble(facts)
	if !strings.HasPrefix(out, "KNOWN FACTS:\n") {
		t.Fatalf("missing header: %q", out)
	}
	// Three lines (one dup removed).
	bodyLines := strings.Count(out, "\n- ")
	if bodyLines != 3 {
		t.Errorf("expected 3 fact lines after dedup, got %d in %q", bodyLines, out)
	}
	// Andrew sorts before Caroline.
	andrewIdx := strings.Index(out, "Andrew")
	carolineIdx := strings.Index(out, "Caroline")
	if andrewIdx <= 0 || carolineIdx <= 0 || andrewIdx > carolineIdx {
		t.Errorf("expected Andrew before Caroline: %q", out)
	}
	// Confidence is rendered.
	if !strings.Contains(out, "conf=0.90") {
		t.Errorf("expected highest confidence (0.90) rendered, got %q", out)
	}
	// Empty input returns empty string.
	if formatFactPreamble(nil) != "" {
		t.Error("formatFactPreamble(nil) should be empty")
	}
}

func TestRetrieveHybridContext_EmptyNamespace(t *testing.T) {
	mem := NewPixelogMemory(PixelogConfig{Embedder: NewHashEmbedder(32)})
	texts, ids, err := mem.retrieveHybridContext(context.Background(), "empty", "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 0 || len(ids) != 0 {
		t.Errorf("expected empty result, got texts=%v ids=%v", texts, ids)
	}
}
