package bench

import (
	"context"
	"errors"
	"testing"
)

type stubChat struct {
	resp string
	err  error
}

func (s stubChat) Chat(string) (string, error) { return s.resp, s.err }

func TestMem0JudgeParseLabel(t *testing.T) {
	cases := []struct {
		name  string
		resp  string
		want  string
		score float64
	}{
		{"strict_json_correct", `{"label":"CORRECT"}`, "CORRECT", 1.0},
		{"strict_json_wrong", `{"label": "WRONG"}`, "WRONG", 0.0},
		{"code_fenced_json", "```json\n{\"label\":\"CORRECT\"}\n```", "CORRECT", 1.0},
		{"explanation_then_json", "The answers match.\n{\"label\":\"CORRECT\"}", "CORRECT", 1.0},
		{"keyword_fallback_correct", "The predicted answer matches the gold. CORRECT", "CORRECT", 1.0},
		{"keyword_fallback_wrong", "Answer refers to a different topic. WRONG", "WRONG", 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			label, _ := parseMem0JudgeResponse(tc.resp)
			if label != tc.want {
				t.Fatalf("label: got %q want %q", label, tc.want)
			}
			j := NewMem0Judge(stubChat{resp: tc.resp})
			got, _, err := j.Score(context.Background(), QA{Question: "q", GoldAnswer: "g"}, "pred")
			if err != nil {
				t.Fatalf("Score: %v", err)
			}
			if got != tc.score {
				t.Fatalf("score: got %v want %v", got, tc.score)
			}
		})
	}
}

func TestMem0JudgeChatError(t *testing.T) {
	j := NewMem0Judge(stubChat{err: errors.New("boom")})
	// No gold/pred match -> ExactMatch fallback -> token F1 (likely 0).
	score, reason, err := j.Score(context.Background(),
		QA{Question: "q", GoldAnswer: "Hawaii"}, "Paris")
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if score != 0 {
		t.Fatalf("expected 0 fallback score, got %v", score)
	}
	if reason == "" {
		t.Fatalf("expected non-empty reason, got empty")
	}
}

func TestMem0JudgeAbstain(t *testing.T) {
	// Abstain questions must NOT hit the LLM — handled by ExactMatch.
	calls := 0
	chat := &countingChat{inner: stubChat{resp: `{"label":"CORRECT"}`}, calls: &calls}
	j := NewMem0Judge(chat)
	score, _, err := j.Score(context.Background(),
		QA{Question: "anything", GoldAnswer: "", Abstain: true},
		"I don't know")
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if score != 1.0 {
		t.Fatalf("abstention should score 1.0, got %v", score)
	}
	if calls != 0 {
		t.Fatalf("abstain path must not call LLM; calls=%d", calls)
	}
}

type countingChat struct {
	inner stubChat
	calls *int
}

func (c *countingChat) Chat(s string) (string, error) {
	*c.calls++
	return c.inner.Chat(s)
}
