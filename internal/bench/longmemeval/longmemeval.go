// Package longmemeval loads the LongMemEval dataset (Wu et al., ICLR 2025)
// and adapts it to the bench package case format.
//
// LongMemEval ships three sized splits — `longmemeval_oracle.json` (gold
// evidence only, ~few sessions), `longmemeval_s.json` (~115k token
// haystacks) and `longmemeval_m.json` (~1.5M token haystacks). All three
// share the same JSON schema, so this adapter handles any of them.
//
// One example object:
//
//	{
//	  "question_id": "...",
//	  "question_type": "multi-session",
//	  "question": "...",
//	  "answer": "...",
//	  "question_date": "2023-05-21",
//	  "haystack_session_ids": ["sid_0", "sid_1", ...],
//	  "haystack_dates":       ["2023-05-01", "2023-05-03", ...],
//	  "haystack_sessions":    [[{"role":"user","content":"..."}, ...], ...],
//	  "answer_session_ids":   ["sid_5"]
//	}
//
// The benchmark groups questions by `question_type`:
//   - single-session-user
//   - single-session-assistant
//   - single-session-preference
//   - multi-session
//   - knowledge-update
//   - temporal-reasoning
//
// Each top-level object becomes its own bench.Case. Because LongMemEval
// haystacks already encode the "long history" property, every QA in a
// case has access to all of its sessions.
package longmemeval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ArqonAi/Pixelog/internal/bench"
)

// Sample matches one LongMemEval example. Answer is decoded as
// json.RawMessage because the dataset mixes string, number and list
// answer values across question types.
type Sample struct {
	QuestionID         string          `json:"question_id"`
	QuestionType       string          `json:"question_type"`
	Question           string          `json:"question"`
	Answer             json.RawMessage `json:"answer"`
	QuestionDate       string          `json:"question_date"`
	HaystackSessionIDs []string        `json:"haystack_session_ids"`
	HaystackDates      []string        `json:"haystack_dates"`
	HaystackSessions   [][]rawTurn     `json:"haystack_sessions"`
	AnswerSessionIDs   []string        `json:"answer_session_ids"`
}

// answerString flattens the raw JSON answer into a single string, no
// matter whether it was a scalar, list, or nested object.
func answerString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		// Integer-valued floats render without trailing decimals.
		if num == float64(int64(num)) {
			return strings.TrimSpace(fmt.Sprintf("%d", int64(num)))
		}
		return strings.TrimSpace(fmt.Sprintf("%g", num))
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil {
		parts := make([]string, 0, len(list))
		for _, item := range list {
			parts = append(parts, answerString(item))
		}
		return strings.Join(parts, "; ")
	}
	return strings.TrimSpace(string(raw))
}

type rawTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	// Some splits include `has_answer` flags; ignore.
}

// Dataset is a loaded LongMemEval JSON file.
type Dataset struct {
	path    string
	samples []Sample
}

// Load reads any longmemeval_*.json file from disk.
func Load(path string) (*Dataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("longmemeval: read %s: %w", path, err)
	}
	var samples []Sample
	if err := json.Unmarshal(data, &samples); err != nil {
		return nil, fmt.Errorf("longmemeval: parse %s: %w", path, err)
	}
	return &Dataset{path: path, samples: samples}, nil
}

// Suite implements bench.Dataset.
func (d *Dataset) Suite() bench.Suite { return bench.SuiteLongMemEval }

// Cases implements bench.Dataset. One bench.Case per LongMemEval example.
func (d *Dataset) Cases(_ context.Context) ([]bench.Case, error) {
	cases := make([]bench.Case, 0, len(d.samples))
	for i, s := range d.samples {
		c, err := convert(s, i)
		if err != nil {
			return nil, fmt.Errorf("longmemeval sample %d: %w", i, err)
		}
		cases = append(cases, c)
	}
	return cases, nil
}

func convert(s Sample, idx int) (bench.Case, error) {
	id := s.QuestionID
	if id == "" {
		id = fmt.Sprintf("longmemeval-%d", idx)
	}

	turns := flattenTurns(s)

	cat := categoryFromType(s.QuestionType)
	gold := answerString(s.Answer)
	abstain := strings.HasPrefix(strings.ToLower(gold), "i don't know") ||
		strings.Contains(strings.ToLower(s.QuestionType), "abstention")

	qa := []bench.QA{{
		ID:         id,
		Question:   s.Question,
		GoldAnswer: gold,
		Category:   cat,
		Evidence:   s.AnswerSessionIDs,
		Abstain:    abstain,
	}}

	return bench.Case{
		Suite:     bench.SuiteLongMemEval,
		ID:        id,
		Namespace: "longmemeval-" + id,
		Turns:     turns,
		QA:        qa,
	}, nil
}

// flattenTurns produces a single ordered slice of bench.Turn values, one
// entry per role/content pair, tagged with the session_id and parsed
// session date.
func flattenTurns(s Sample) []bench.Turn {
	var out []bench.Turn
	for i, sess := range s.HaystackSessions {
		var sid string
		if i < len(s.HaystackSessionIDs) {
			sid = s.HaystackSessionIDs[i]
		} else {
			sid = fmt.Sprintf("sid_%d", i)
		}
		var ts time.Time
		if i < len(s.HaystackDates) {
			ts = parseDate(s.HaystackDates[i])
		}
		for j, t := range sess {
			text := strings.TrimSpace(t.Content)
			if text == "" {
				continue
			}
			role := strings.ToLower(strings.TrimSpace(t.Role))
			if role == "" {
				role = "user"
			}
			speaker := role
			if role == "assistant" {
				speaker = "Assistant"
			} else if role == "user" {
				speaker = "User"
			}
			out = append(out, bench.Turn{
				Speaker:   speaker,
				Role:      role,
				Text:      text,
				Timestamp: ts,
				SessionID: sid,
				TurnID:    fmt.Sprintf("%s:%d", sid, j),
			})
		}
	}
	return out
}

// categoryFromType normalises the LongMemEval `question_type` strings
// onto the snake_case style used elsewhere in the harness.
func categoryFromType(t string) string {
	t = strings.TrimSpace(t)
	if t == "" {
		return "unknown"
	}
	return strings.ReplaceAll(strings.ToLower(t), "-", "_")
}

func parseDate(v string) time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}
	}
	// Real LongMemEval splits use "2023/04/10 (Mon) 17:50". Strip the
	// weekday in parens before parsing.
	if i := strings.Index(v, "("); i >= 0 {
		if j := strings.Index(v[i:], ")"); j >= 0 {
			v = strings.TrimSpace(v[:i] + v[i+j+1:])
		}
	}
	// Collapse internal double spaces left behind by the "(Mon) " strip.
	for strings.Contains(v, "  ") {
		v = strings.ReplaceAll(v, "  ", " ")
	}
	for _, layout := range []string{
		"2006/01/02 15:04",
		"2006/01/02",
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"January 2, 2006",
		"2 January 2006",
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, v); err == nil {
			return t
		}
	}
	return time.Time{}
}
