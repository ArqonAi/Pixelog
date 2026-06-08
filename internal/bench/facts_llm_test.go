package bench

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeFactCompletion is a deterministic FactCompletion stub so the
// LLMFactExtractor can be tested without any network dependency.
// Each test instance carries a fixed response (or error) and records
// the calls it received for assertion.
type fakeFactCompletion struct {
	response string
	err      error
	calls    int
}

func (f *fakeFactCompletion) Complete(_ context.Context, _, _ string) (string, error) {
	f.calls++
	return f.response, f.err
}

func TestLLMFactExtractor_ParsesBareJSONArray(t *testing.T) {
	stub := &fakeFactCompletion{response: `[{"subject":"speaker","predicate":"lives-in","object":"Berlin"}]`}
	ex := NewLLMFactExtractor(stub)
	turn := Turn{Speaker: "Alice", TurnID: "t1", Text: "I live in Berlin.",
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}

	facts, err := ex.Extract(turn)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("len=%d want 1", len(facts))
	}
	if facts[0].Subject != "Alice" {
		t.Errorf("subject=%q want Alice (speaker placeholder must resolve)", facts[0].Subject)
	}
	if facts[0].Source != FactSourceLLM {
		t.Errorf("source=%d want FactSourceLLM", facts[0].Source)
	}
	if facts[0].Confidence != 0.9 {
		t.Errorf("confidence=%v want 0.9", facts[0].Confidence)
	}
}

func TestLLMFactExtractor_ParsesMarkdownFencedJSON(t *testing.T) {
	stub := &fakeFactCompletion{response: "```json\n[{\"subject\":\"Caroline\",\"predicate\":\"works-at\",\"object\":\"Acme\"}]\n```"}
	ex := NewLLMFactExtractor(stub)
	facts, err := ex.Extract(Turn{Speaker: "Caroline", Text: "I work at Acme.", TurnID: "t1"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts) != 1 || facts[0].Object != "Acme" {
		t.Errorf("got %+v", facts)
	}
}

func TestLLMFactExtractor_ToleratesPrefixProse(t *testing.T) {
	stub := &fakeFactCompletion{response: `Sure, here are the facts: [{"subject":"speaker","predicate":"likes","object":"sushi"}]`}
	ex := NewLLMFactExtractor(stub)
	facts, err := ex.Extract(Turn{Speaker: "Bob", Text: "I love sushi.", TurnID: "t1"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts) != 1 {
		t.Errorf("len=%d want 1", len(facts))
	}
}

func TestLLMFactExtractor_DegradesOnMalformedOutput(t *testing.T) {
	stub := &fakeFactCompletion{response: "I cannot extract any facts here."}
	ex := NewLLMFactExtractor(stub)
	facts, err := ex.Extract(Turn{Speaker: "Bob", Text: "...", TurnID: "t1"})
	if err != nil {
		t.Errorf("malformed output should NOT error, got %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("malformed output should yield 0 facts, got %d", len(facts))
	}
}

func TestLLMFactExtractor_PropagatesLLMError(t *testing.T) {
	stub := &fakeFactCompletion{err: errors.New("network down")}
	ex := NewLLMFactExtractor(stub)
	_, err := ex.Extract(Turn{Speaker: "Alice", Text: "Hello.", TurnID: "t1"})
	if err == nil {
		t.Errorf("expected error from LLM client to surface")
	}
}

func TestLLMFactExtractor_RespectsMaxFactsPerTurn(t *testing.T) {
	stub := &fakeFactCompletion{response: `[
        {"subject":"speaker","predicate":"likes","object":"a"},
        {"subject":"speaker","predicate":"likes","object":"b"},
        {"subject":"speaker","predicate":"likes","object":"c"},
        {"subject":"speaker","predicate":"likes","object":"d"},
        {"subject":"speaker","predicate":"likes","object":"e"},
        {"subject":"speaker","predicate":"likes","object":"f"}
    ]`}
	ex := NewLLMFactExtractor(stub)
	ex.MaxFactsPerTurn = 3
	facts, err := ex.Extract(Turn{Speaker: "Alice", Text: "...", TurnID: "t1"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts) != 3 {
		t.Errorf("MaxFactsPerTurn ignored: got %d, want 3", len(facts))
	}
}

func TestLLMFactExtractor_ClampsLongObject(t *testing.T) {
	long := "this object is way too long because the model copied half a paragraph instead of just the entity"
	stub := &fakeFactCompletion{response: `[{"subject":"speaker","predicate":"says","object":"` + long + `"}]`}
	ex := NewLLMFactExtractor(stub)
	ex.MaxObjectChars = 20
	facts, err := ex.Extract(Turn{Speaker: "Alice", Text: "...", TurnID: "t1"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts[0].Object) != 20 {
		t.Errorf("object len=%d want 20 (got %q)", len(facts[0].Object), facts[0].Object)
	}
}

func TestLLMFactExtractor_NewPanicsOnNilLLM(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewLLMFactExtractor(nil) must panic")
		}
	}()
	NewLLMFactExtractor(nil)
}
