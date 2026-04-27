package membench

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ArqonAi/Pixelog/internal/bench"
)

// realFixture mirrors MemBench's FirstAgent schema where ``sid``
// values increment globally across sessions. target_step_id keys on
// those global sids — sid=1 lives in session_1 below.
const realFixture = `{
  "roles": [
    {
      "tid": 0,
      "message_list": [
        [
          {"sid": 0, "user_message": "I want to tell you about my uncle, Landon Pierce.",
           "assistant_message": "Got it.", "time": "2024-10-01", "place": "Boston, MA"}
        ],
        [
          {"sid": 1, "user_message": "I have a niece who runs TechInnovate Systems.",
           "assistant_message": "Cool!", "time": "2024-10-02", "place": "Boston, MA"}
        ]
      ],
      "QA": {
        "qid": 0,
        "question": "What is the name of my niece's company?",
        "answer": "TechInnovate Systems LLC",
        "target_step_id": [[1, 0]],
        "choices": {"A": "TechInnovate Solutions LLC", "B": "TechInnovate Systems LLC"}
      }
    }
  ],
  "events": []
}`

func TestLoad_RealMemBenchSchema(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "FirstAgent")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "simple.json"), []byte(realFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	ds, err := Load(filepath.Join(dir, "simple.json"), LoadOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if ds.Suite() != bench.SuiteMemBench {
		t.Errorf("suite: %s", ds.Suite())
	}
	cases, _ := ds.Cases(context.Background())
	if len(cases) != 1 {
		t.Fatalf("cases: %d", len(cases))
	}
	c := cases[0]
	// 2 sessions × 1 message × 2 turns each (user+assistant) = 4 turns
	if len(c.Turns) != 4 {
		t.Errorf("turns: %d, want 4", len(c.Turns))
	}
	if len(c.QA) != 1 {
		t.Fatalf("qa: %d", len(c.QA))
	}
	qa := c.QA[0]
	if qa.Question == "" {
		t.Error("question empty")
	}
	if qa.GoldAnswer != "TechInnovate Systems LLC" {
		t.Errorf("gold: %q", qa.GoldAnswer)
	}
	if qa.Category != "participation_simple" {
		t.Errorf("category: %q, want participation_simple", qa.Category)
	}
	// Evidence must include session_1 (target_step_id [[1, 0]]).
	hasSession1 := false
	for _, e := range qa.Evidence {
		if e == "session_1" {
			hasSession1 = true
		}
	}
	if !hasSession1 {
		t.Errorf("evidence missing session_1: %v", qa.Evidence)
	}
}

func TestPerspectiveFromPath(t *testing.T) {
	cases := map[string]Perspective{
		"/tmp/MemData/FirstAgent/simple.json": Participation,
		"/tmp/MemData/ThirdAgent/simple.json": Observation,
		"/tmp/MemData/Other/x.json":           "",
	}
	for in, want := range cases {
		if got := PerspectiveFromPath(in); got != want {
			t.Errorf("PerspectiveFromPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMembenchAnswer_LetterChoiceExpands(t *testing.T) {
	choices := map[string]string{"A": "Foo", "B": "Bar"}
	got := membenchAnswer([]byte(`"B"`), choices)
	if got != "B. Bar" {
		t.Errorf("got %q, want %q", got, "B. Bar")
	}
}

func TestLoadAllInDir(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "FirstAgent")
	third := filepath.Join(root, "ThirdAgent")
	os.MkdirAll(first, 0o755)
	os.MkdirAll(third, 0o755)
	os.WriteFile(filepath.Join(first, "simple.json"), []byte(realFixture), 0o644)
	os.WriteFile(filepath.Join(third, "simple.json"), []byte(realFixture), 0o644)

	ds, err := LoadAllInDir(root, LoadOpts{})
	if err != nil {
		t.Fatal(err)
	}
	cases, _ := ds.Cases(context.Background())
	if len(cases) != 2 {
		t.Errorf("cases: %d, want 2 (FirstAgent + ThirdAgent)", len(cases))
	}
	gotCats := map[string]bool{}
	for _, c := range cases {
		gotCats[c.QA[0].Category] = true
	}
	if !gotCats["participation_simple"] || !gotCats["observation_simple"] {
		t.Errorf("categories: %v", gotCats)
	}
}
