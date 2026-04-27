package longmemeval

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ArqonAi/Pixelog/internal/bench"
)

// fixture mirrors a stripped-down LongMemEval example: two haystack
// sessions, one multi-session question with answer evidence in session 1.
const fixture = `[
	{
		"question_id": "lme-test-1",
		"question_type": "multi-session",
		"question": "Which restaurant did Alice recommend that Bob later visited?",
		"answer": "Trattoria Lucca",
		"question_date": "2024-03-15",
		"haystack_session_ids": ["sid_0", "sid_1"],
		"haystack_dates":       ["2024-02-10", "2024-03-10"],
		"haystack_sessions": [
			[
				{"role": "user",      "content": "Hey, have you been to Trattoria Lucca? It's amazing."},
				{"role": "assistant", "content": "I haven't, but I'll add it to my list."}
			],
			[
				{"role": "user",      "content": "I went to Trattoria Lucca last night, just like you suggested."},
				{"role": "assistant", "content": "Glad you finally tried it."}
			]
		],
		"answer_session_ids": ["sid_1"]
	},
	{
		"question_id": "lme-test-2",
		"question_type": "temporal-reasoning",
		"question": "When did Bob first hear about Trattoria Lucca?",
		"answer": "2024-02-10",
		"question_date": "2024-03-15",
		"haystack_session_ids": ["sid_0"],
		"haystack_dates":       ["2024-02-10"],
		"haystack_sessions": [
			[
				{"role": "user", "content": "Hey, have you been to Trattoria Lucca?"}
			]
		],
		"answer_session_ids": ["sid_0"]
	}
]`

func TestLoadAndConvert(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ds, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := ds.Suite(); got != bench.SuiteLongMemEval {
		t.Fatalf("Suite() = %q, want %q", got, bench.SuiteLongMemEval)
	}

	cases, err := ds.Cases(context.Background())
	if err != nil {
		t.Fatalf("Cases: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("got %d cases, want 2", len(cases))
	}

	c0 := cases[0]
	if c0.ID != "lme-test-1" {
		t.Errorf("case[0].ID = %q, want lme-test-1", c0.ID)
	}
	if c0.Suite != bench.SuiteLongMemEval {
		t.Errorf("case[0].Suite = %q", c0.Suite)
	}
	if len(c0.QA) != 1 {
		t.Fatalf("case[0].QA len = %d, want 1", len(c0.QA))
	}
	if c0.QA[0].Category != "multi_session" {
		t.Errorf("case[0].QA[0].Category = %q, want multi_session", c0.QA[0].Category)
	}
	if c0.QA[0].GoldAnswer != "Trattoria Lucca" {
		t.Errorf("case[0].QA[0].GoldAnswer = %q", c0.QA[0].GoldAnswer)
	}
	// 2 sessions x 2 turns each = 4 turns
	if len(c0.Turns) != 4 {
		t.Fatalf("case[0].Turns = %d, want 4", len(c0.Turns))
	}
	if c0.Turns[0].SessionID != "sid_0" {
		t.Errorf("Turns[0].SessionID = %q", c0.Turns[0].SessionID)
	}
	if c0.Turns[2].SessionID != "sid_1" {
		t.Errorf("Turns[2].SessionID = %q", c0.Turns[2].SessionID)
	}
	if c0.Turns[0].Timestamp.IsZero() {
		t.Error("Turns[0].Timestamp should be parsed")
	}
	if c0.Turns[0].Role != "user" || c0.Turns[1].Role != "assistant" {
		t.Errorf("roles not preserved: %v / %v", c0.Turns[0].Role, c0.Turns[1].Role)
	}

	c1 := cases[1]
	if c1.QA[0].Category != "temporal_reasoning" {
		t.Errorf("case[1].QA[0].Category = %q, want temporal_reasoning", c1.QA[0].Category)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load("/nonexistent/path/longmemeval.json"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseDate_RealLongMemEvalFormats(t *testing.T) {
	cases := map[string]string{
		"2023/04/10 (Mon) 17:50": "2023-04-10",
		"2023/04/10":             "2023-04-10",
		"2024-03-15":             "2024-03-15",
		"":                       "",
		"garbage":                "",
	}
	for in, want := range cases {
		got := parseDate(in)
		if want == "" {
			if !got.IsZero() {
				t.Errorf("parseDate(%q) = %v, want zero", in, got)
			}
			continue
		}
		if got.Format("2006-01-02") != want {
			t.Errorf("parseDate(%q) = %v, want %s", in, got, want)
		}
	}
}

func TestCategoryFromType(t *testing.T) {
	cases := map[string]string{
		"single-session-user":      "single_session_user",
		"single-session-assistant": "single_session_assistant",
		"multi-session":            "multi_session",
		"knowledge-update":         "knowledge_update",
		"temporal-reasoning":       "temporal_reasoning",
		"":                         "unknown",
	}
	for in, want := range cases {
		if got := categoryFromType(in); got != want {
			t.Errorf("categoryFromType(%q) = %q, want %q", in, got, want)
		}
	}
}
