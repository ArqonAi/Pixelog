package bench

import (
	"context"
	"strings"
	"testing"
)

// stubAnswerer concatenates retrieved context as the "answer".
type stubAnswerer struct{}

func (stubAnswerer) Answer(_ context.Context, q string, retrieved []string) (string, error) {
	return strings.Join(retrieved, " | "), nil
}

func TestPixelogMemory_EndToEnd(t *testing.T) {
	mem := NewPixelogMemory(PixelogConfig{
		Embedder: NewHashEmbedder(64),
		Answerer: stubAnswerer{},
	})

	ctx := context.Background()
	ns := "case1"

	if err := mem.Reset(ctx, ns); err != nil {
		t.Fatal(err)
	}

	turns := []Turn{
		{Role: "user", Text: "I prefer functional programming patterns.", SessionID: "s1"},
		{Role: "user", Text: "Always use tabs for indentation.", SessionID: "s1"},
		{Role: "user", Text: "Tomorrow I have a meeting with the design team.", SessionID: "s2"},
	}
	if err := mem.Ingest(ctx, ns, turns[:2]); err != nil {
		t.Fatal(err)
	}
	if err := mem.Consolidate(ctx, ns); err != nil {
		t.Fatal(err)
	}
	if err := mem.Ingest(ctx, ns, turns[2:]); err != nil {
		t.Fatal(err)
	}
	if err := mem.Consolidate(ctx, ns); err != nil {
		t.Fatal(err)
	}

	ans, err := mem.Answer(ctx, ns, "tabs indentation")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text == "" {
		t.Errorf("empty answer")
	}
	if !strings.Contains(strings.ToLower(ans.Text), "tabs") {
		t.Errorf("expected answer to mention tabs, got %q", ans.Text)
	}
	if ans.Latency <= 0 {
		t.Errorf("latency should be positive")
	}
}

func TestPixelogMemory_Reset(t *testing.T) {
	mem := NewPixelogMemory(PixelogConfig{Embedder: NewHashEmbedder(32)})
	ctx := context.Background()

	_ = mem.Ingest(ctx, "ns", []Turn{{Role: "user", Text: "I prefer foo"}})
	_ = mem.Consolidate(ctx, "ns")

	ans, _ := mem.Answer(ctx, "ns", "foo")
	if ans.Text == "" {
		t.Error("pre-reset retrieval empty")
	}

	if err := mem.Reset(ctx, "ns"); err != nil {
		t.Fatal(err)
	}
	ans, _ = mem.Answer(ctx, "ns", "foo")
	if ans.Text != "" {
		t.Errorf("post-reset should return empty, got %q", ans.Text)
	}
}

func TestHashEmbedder_Deterministic(t *testing.T) {
	e := NewHashEmbedder(64)
	a, _ := e.GenerateEmbedding(context.Background(), "hello world")
	b, _ := e.GenerateEmbedding(context.Background(), "hello world")
	if len(a) != 64 || len(b) != 64 {
		t.Fatalf("dim mismatch: %d %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("nondeterministic at index %d", i)
			break
		}
	}
}

func TestHashEmbedder_Empty(t *testing.T) {
	e := NewHashEmbedder(16)
	v, err := e.GenerateEmbedding(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 16 {
		t.Errorf("empty input dim = %d", len(v))
	}
	for _, x := range v {
		if x != 0 {
			t.Errorf("empty input should be zero vector")
			break
		}
	}
}
