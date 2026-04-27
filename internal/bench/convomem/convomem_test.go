package convomem

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ArqonAi/Pixelog/internal/bench"
)

const realFixture = `[
  {
    "conversations": [
      {
        "id": "conv-a",
        "containsEvidence": true,
        "messages": [
          {"speaker": "user",      "text": "My favourite colour is teal."},
          {"speaker": "assistant", "text": "Got it."}
        ]
      },
      {
        "id": "conv-b",
        "containsEvidence": false,
        "messages": [
          {"speaker": "user", "text": "Filler conversation about the weather."}
        ]
      }
    ],
    "evidenceItems": [
      {
        "question": "What is my favourite colour?",
        "answer": "teal",
        "category": "Personal Life",
        "message_evidences": [{"speaker": "user", "text": "My favourite colour is teal."}]
      }
    ],
    "contextSize": 8000
  }
]`

func TestLoad_RealConvoMemSchema(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "user_evidence", "1_evidence")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "batched_0.json"), []byte(realFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	ds, err := Load(root, LoadOpts{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ds.Suite() != bench.SuiteConvoMem {
		t.Errorf("suite: %s", ds.Suite())
	}
	cases, _ := ds.Cases(context.Background())
	if len(cases) != 1 {
		t.Fatalf("cases: %d", len(cases))
	}
	c := cases[0]
	if len(c.Turns) != 3 {
		t.Errorf("turns: %d, want 3", len(c.Turns))
	}
	// Both convs contribute turns; SessionID must match conv id.
	got := map[string]int{}
	for _, tn := range c.Turns {
		got[tn.SessionID]++
	}
	if got["conv-a"] != 2 || got["conv-b"] != 1 {
		t.Errorf("session distribution: %v", got)
	}

	if len(c.QA) != 1 {
		t.Fatalf("qa: %d", len(c.QA))
	}
	qa := c.QA[0]
	if qa.GoldAnswer != "teal" {
		t.Errorf("gold: %q", qa.GoldAnswer)
	}
	if qa.Category != "user_evidence" {
		t.Errorf("category: %q", qa.Category)
	}
	if qa.EvidenceN != 1 {
		t.Errorf("evidenceN: %d, want 1", qa.EvidenceN)
	}
	// Evidence must be the conv ids that contained evidence (conv-a only).
	if len(qa.Evidence) != 1 || qa.Evidence[0] != "conv-a" {
		t.Errorf("evidence: %v, want [conv-a]", qa.Evidence)
	}
	// Fine-grained category from the JSON should land on Rubric.
	if qa.Rubric != "Personal Life" {
		t.Errorf("rubric: %q", qa.Rubric)
	}
}

func TestLoad_AbstentionFlag(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "abstention_evidence", "1_evidence")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `[{"conversations":[{"id":"c","containsEvidence":true,"messages":[{"speaker":"user","text":"hi"}]}],
		"evidenceItems":[{"question":"foo?","answer":"i don't know","message_evidences":[]}]}]`
	os.WriteFile(filepath.Join(dir, "batched_0.json"), []byte(body), 0o644)

	ds, _ := Load(root, LoadOpts{})
	cases, _ := ds.Cases(context.Background())
	if len(cases) != 1 {
		t.Fatalf("cases=%d", len(cases))
	}
	if !cases[0].QA[0].Abstain {
		t.Error("abstention category should set Abstain=true")
	}
}

func TestLoad_FilterAndCap(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "user_evidence", "2_evidence")
	os.MkdirAll(dir, 0o755)
	body := `[
		{"conversations":[{"id":"c1","containsEvidence":true,"messages":[{"speaker":"user","text":"a"}]}],
		 "evidenceItems":[{"question":"q1","answer":"a1","message_evidences":[]}]},
		{"conversations":[{"id":"c2","containsEvidence":true,"messages":[{"speaker":"user","text":"b"}]}],
		 "evidenceItems":[{"question":"q2","answer":"a2","message_evidences":[]}]},
		{"conversations":[{"id":"c3","containsEvidence":true,"messages":[{"speaker":"user","text":"c"}]}],
		 "evidenceItems":[{"question":"q3","answer":"a3","message_evidences":[]}]}
	]`
	os.WriteFile(filepath.Join(dir, "batched_0.json"), []byte(body), 0o644)

	ds, _ := Load(root, LoadOpts{
		Categories:     []string{"user_evidence"},
		MaxPerCategory: 2,
	})
	cases, _ := ds.Cases(context.Background())
	if len(cases) != 2 {
		t.Errorf("cap: cases=%d, want 2", len(cases))
	}
}

func TestCategoryFromPath(t *testing.T) {
	cat, n := categoryFromPath("/r/user_evidence/3_evidence/batched_0.json")
	if cat != "user_evidence" || n != 3 {
		t.Errorf("got (%q,%d), want (user_evidence,3)", cat, n)
	}
	cat, n = categoryFromPath("/r/garbage/foo/x.json")
	if cat != "" || n != 0 {
		t.Errorf("expected empty, got (%q,%d)", cat, n)
	}
}
