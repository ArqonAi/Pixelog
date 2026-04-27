package bench

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeMemory stores ingested turns verbatim and answers by simple substring match.
type fakeMemory struct {
	mu     sync.Mutex
	stores map[string][]Turn
}

func newFakeMemory() *fakeMemory {
	return &fakeMemory{stores: make(map[string][]Turn)}
}

func (m *fakeMemory) Reset(_ context.Context, namespace string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.stores, namespace)
	return nil
}

func (m *fakeMemory) Ingest(_ context.Context, namespace string, turns []Turn) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stores[namespace] = append(m.stores[namespace], turns...)
	return nil
}

func (m *fakeMemory) Consolidate(_ context.Context, _ string) error { return nil }

func (m *fakeMemory) Answer(_ context.Context, namespace, question string) (Answer, error) {
	m.mu.Lock()
	turns := append([]Turn(nil), m.stores[namespace]...)
	m.mu.Unlock()

	q := strings.ToLower(question)
	var matches []string
	for _, t := range turns {
		if strings.Contains(strings.ToLower(t.Text), q) || strings.Contains(q, strings.ToLower(t.Text)) {
			matches = append(matches, t.Text)
		}
	}
	return Answer{
		Text:    strings.Join(matches, " "),
		Latency: 1 * time.Millisecond,
	}, nil
}

// staticDataset returns canned cases.
type staticDataset struct {
	suite Suite
	cases []Case
}

func (s *staticDataset) Suite() Suite                                  { return s.suite }
func (s *staticDataset) Cases(_ context.Context) ([]Case, error)       { return s.cases, nil }

func TestRunner_Session(t *testing.T) {
	mem := newFakeMemory()
	ds := &staticDataset{
		suite: SuiteLoCoMo,
		cases: []Case{{
			ID:        "c1",
			Namespace: "c1",
			Turns: []Turn{
				{Speaker: "alice", Role: "user", Text: "I love coffee.", SessionID: "s1"},
				{Speaker: "bob", Role: "assistant", Text: "Coffee is great.", SessionID: "s1"},
				{Speaker: "alice", Role: "user", Text: "Tomorrow I have a meeting.", SessionID: "s2"},
			},
			QA: []QA{
				{ID: "q1", Question: "coffee", GoldAnswer: "coffee", Category: "single_hop"},
				{ID: "q2", Question: "meeting", GoldAnswer: "meeting", Category: "temporal"},
				{ID: "q3", Question: "blockchain", GoldAnswer: "no info", Abstain: true, Category: "abstention"},
			},
		}},
	}

	runner := NewRunner(mem, ExactMatchJudge{}, Config{Mode: ModeSession, IncludeCases: true})
	rep, err := runner.Run(context.Background(), ds)
	if err != nil {
		t.Fatalf("runner failed: %v", err)
	}
	if rep.NumCases != 1 {
		t.Errorf("NumCases = %d, want 1", rep.NumCases)
	}
	if rep.NumQA != 3 {
		t.Errorf("NumQA = %d, want 3", rep.NumQA)
	}
	if rep.Aggregate.Count != 3 {
		t.Errorf("aggregate count = %d, want 3", rep.Aggregate.Count)
	}
	if rep.Aggregate.JudgeMean <= 0 {
		t.Errorf("expected positive judge mean, got %f", rep.Aggregate.JudgeMean)
	}
	if _, ok := rep.ByCategory["abstention"]; !ok {
		t.Error("expected category breakdown for 'abstention'")
	}
}

func TestRunner_Full(t *testing.T) {
	mem := newFakeMemory()
	ds := &staticDataset{
		suite: SuiteMemBench,
		cases: []Case{{
			ID:        "m1",
			Namespace: "m1",
			Turns:     []Turn{{Role: "user", Text: "tokyo capital japan"}},
			QA:        []QA{{ID: "q1", Question: "tokyo", GoldAnswer: "tokyo capital japan"}},
		}},
	}
	r := NewRunner(mem, ExactMatchJudge{}, Config{Mode: ModeFull})
	rep, err := r.Run(context.Background(), ds)
	if err != nil {
		t.Fatalf("runner failed: %v", err)
	}
	if rep.NumQA != 1 {
		t.Fatalf("NumQA = %d", rep.NumQA)
	}
	if rep.Aggregate.JudgeMean < 0.99 {
		t.Errorf("expected near-1 judge mean, got %f", rep.Aggregate.JudgeMean)
	}
}

func TestExactMatchJudge_Abstain(t *testing.T) {
	j := ExactMatchJudge{}
	score, _, err := j.Score(context.Background(), QA{Abstain: true}, "I don't know")
	if err != nil {
		t.Fatal(err)
	}
	if score != 1.0 {
		t.Errorf("abstain correct: score = %f, want 1.0", score)
	}
	score, _, _ = j.Score(context.Background(), QA{Abstain: true}, "the answer is 42")
	if score != 0 {
		t.Errorf("abstain incorrect: score = %f, want 0", score)
	}
}

func TestParseJudgeResponse(t *testing.T) {
	tests := []struct {
		name string
		resp string
		want float64
	}{
		{"clean", "SCORE: 5 | REASON: perfect match", 1.0},
		{"score 3", "SCORE: 3 | REASON: partial", 0.6},
		{"lowercase", "score: 4", 0.8},
		{"digit-only fallback", "the answer is 2 out of 5", 0.4},
		{"clamp high", "SCORE: 99", 1.0},
		{"unparseable", "totally invalid response", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := parseJudgeResponse(tt.resp)
			if got != tt.want {
				t.Errorf("parseJudgeResponse(%q) = %f, want %f", tt.resp, got, tt.want)
			}
		})
	}
}

func TestTokenF1(t *testing.T) {
	tests := []struct {
		gold, pred string
		want       float64
	}{
		{"hello world", "hello world", 1.0},
		{"hello world", "hello", 2.0 / 3.0},
		{"foo bar baz", "qux", 0},
		{"", "", 1.0},
	}
	for _, tt := range tests {
		got := tokenF1(tt.gold, tt.pred)
		if absDiff(got, tt.want) > 0.001 {
			t.Errorf("tokenF1(%q,%q) = %f, want %f", tt.gold, tt.pred, got, tt.want)
		}
	}
}

func absDiff(a, b float64) float64 {
	if a < b {
		return b - a
	}
	return a - b
}
