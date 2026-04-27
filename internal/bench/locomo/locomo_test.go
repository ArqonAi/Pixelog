package locomo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ArqonAi/Pixelog/internal/bench"
)

func TestLoadAndConvert(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "locomo10.json")
	sample := `[
	  {
	    "sample_id": "S1",
	    "conversation": {
	      "speaker_a": "Alice",
	      "speaker_b": "Bob",
	      "session_1_date_time": "1 January 2024, 10:00 AM",
	      "session_1": [
	        {"speaker": "Alice", "dia_id": "D1:1", "text": "Hello"},
	        {"speaker": "Bob", "dia_id": "D1:2", "text": "Hi"}
	      ],
	      "session_2_date_time": "2 January 2024, 11:00 AM",
	      "session_2": [
	        {"speaker": "Alice", "dia_id": "D2:1", "text": "Coffee?"}
	      ]
	    },
	    "qa": [
	      {"question": "Who said hello?", "answer": "Alice", "category": 1, "evidence": ["D1:1"]},
	      {"question": "Adversarial probe", "answer": "no info", "category": 5}
	    ]
	  }
	]`
	if err := os.WriteFile(path, []byte(sample), 0644); err != nil {
		t.Fatal(err)
	}

	ds, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ds.Suite() != bench.SuiteLoCoMo {
		t.Errorf("suite = %s", ds.Suite())
	}
	cases, err := ds.Cases(context.Background())
	if err != nil {
		t.Fatalf("Cases: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("cases = %d, want 1", len(cases))
	}
	c := cases[0]
	if c.ID != "S1" {
		t.Errorf("ID = %q", c.ID)
	}
	if len(c.Turns) != 3 {
		t.Errorf("turns = %d, want 3", len(c.Turns))
	}
	if c.Turns[0].SessionID != "session_1" || c.Turns[2].SessionID != "session_2" {
		t.Errorf("session IDs not assigned correctly")
	}
	if len(c.QA) != 2 {
		t.Fatalf("qa = %d, want 2", len(c.QA))
	}
	if c.QA[0].Category != "single_hop" {
		t.Errorf("cat0 = %q, want single_hop", c.QA[0].Category)
	}
	if !c.QA[1].Abstain {
		t.Errorf("category 5 should set Abstain=true")
	}
}

func TestParseDateTime(t *testing.T) {
	cases := map[string]string{
		"1:56 pm on 8 May, 2023":  "2023-05-08",
		"10:00 am on 1 January, 2024": "2024-01-01",
		"7:55 PM on 9 June, 2023": "2023-06-09",
		"1 January 2024, 10:00 AM": "2024-01-01",
		"January 2, 2006":         "2006-01-02",
		"":                        "",
	}
	for in, want := range cases {
		got, _ := parseDateTime(in)
		if want == "" {
			if !got.IsZero() {
				t.Errorf("parseDateTime(%q) = %v, want zero", in, got)
			}
			continue
		}
		if got.Format("2006-01-02") != want {
			t.Errorf("parseDateTime(%q) = %v, want %s", in, got, want)
		}
	}
}

func TestLoCoMoCanonicalDateRoundtrips(t *testing.T) {
	// Real LoCoMo session_*_date_time values; flaky parsing here is the
	// difference between the model anchoring "yesterday" or not.
	dir := t.TempDir()
	path := filepath.Join(dir, "real.json")
	sample := `[{
	  "sample_id": "real",
	  "conversation": {
		"speaker_a": "Alice", "speaker_b": "Bob",
		"session_1_date_time": "1:56 pm on 8 May, 2023",
		"session_1": [{"speaker": "Alice", "dia_id": "D1:1", "text": "hi"}]
	  },
	  "qa": [{"question": "?", "answer": "?", "category": 1}]
	}]`
	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	ds, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cases, _ := ds.Cases(context.Background())
	if len(cases) != 1 || len(cases[0].Turns) == 0 {
		t.Fatal("no turns")
	}
	ts := cases[0].Turns[0].Timestamp
	if ts.IsZero() {
		t.Fatal("LoCoMo date not parsed — multi-hop temporal grounding will fail")
	}
	if got := ts.Format("2006-01-02"); got != "2023-05-08" {
		t.Errorf("date = %s, want 2023-05-08", got)
	}
}

func TestStringify(t *testing.T) {
	if got := stringify("foo"); got != "foo" {
		t.Errorf("string passthrough: %q", got)
	}
	if got := stringify([]interface{}{"a", "b"}); got != "a; b" {
		t.Errorf("list join: %q", got)
	}
	if got := stringify(nil); got != "" {
		t.Errorf("nil: %q", got)
	}
}
