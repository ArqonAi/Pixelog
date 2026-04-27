// Package locomo loads the LoCoMo (Snap Research, ACL 2024) dataset and
// adapts it to the bench package case format.
//
// Dataset layout (locomo10.json):
//
//	[
//	  {
//	    "sample_id": "...",
//	    "conversation": {
//	      "speaker_a": "Alice",
//	      "speaker_b": "Bob",
//	      "session_1": [ { "speaker": "Alice", "dia_id": "D1:1", "text": "..." }, ... ],
//	      "session_1_date_time": "...",
//	      "session_2": [...],
//	      ...
//	    },
//	    "qa": [
//	      { "question": "...", "answer": "...", "category": 1, "evidence": ["D1:3", ...] }
//	    ]
//	  }
//	]
package locomo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ArqonAi/Pixelog/internal/bench"
)

// Sample is one LoCoMo conversation with QA annotations.
type Sample struct {
	SampleID     string                 `json:"sample_id"`
	Conversation map[string]interface{} `json:"conversation"`
	QA           []rawQA                `json:"qa"`
}

type rawQA struct {
	Question string      `json:"question"`
	Answer   interface{} `json:"answer"` // sometimes string, sometimes list
	Category int         `json:"category"`
	Evidence []string    `json:"evidence,omitempty"`
}

type rawTurn struct {
	Speaker string `json:"speaker"`
	DiaID   string `json:"dia_id"`
	Text    string `json:"text"`
}

// LoCoMoCategoryName maps LoCoMo's integer category codes to human labels.
// Reference: Maharana et al. 2024 §3.2.
var LoCoMoCategoryName = map[int]string{
	1: "single_hop",
	2: "multi_hop",
	3: "temporal",
	4: "open_domain",
	5: "adversarial",
}

// Dataset loads locomo10.json (or any compatible file) and exposes it as a
// bench.Dataset.
type Dataset struct {
	path    string
	samples []Sample
}

// Load reads the LoCoMo JSON file from disk.
func Load(path string) (*Dataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("locomo: read %s: %w", path, err)
	}
	var samples []Sample
	if err := json.Unmarshal(data, &samples); err != nil {
		return nil, fmt.Errorf("locomo: parse %s: %w", path, err)
	}
	return &Dataset{path: path, samples: samples}, nil
}

// Suite implements bench.Dataset.
func (d *Dataset) Suite() bench.Suite { return bench.SuiteLoCoMo }

// Cases implements bench.Dataset.
func (d *Dataset) Cases(_ context.Context) ([]bench.Case, error) {
	cases := make([]bench.Case, 0, len(d.samples))
	for i, s := range d.samples {
		c, err := convert(s, i)
		if err != nil {
			return nil, fmt.Errorf("locomo sample %d: %w", i, err)
		}
		cases = append(cases, c)
	}
	return cases, nil
}

func convert(s Sample, idx int) (bench.Case, error) {
	id := s.SampleID
	if id == "" {
		id = fmt.Sprintf("locomo-%d", idx)
	}

	turns, err := flattenTurns(s.Conversation)
	if err != nil {
		return bench.Case{}, err
	}

	qa := make([]bench.QA, 0, len(s.QA))
	for j, raw := range s.QA {
		gold := stringify(raw.Answer)
		cat := LoCoMoCategoryName[raw.Category]
		if cat == "" {
			cat = fmt.Sprintf("category_%d", raw.Category)
		}
		qa = append(qa, bench.QA{
			ID:         fmt.Sprintf("%s-q%d", id, j),
			Question:   raw.Question,
			GoldAnswer: gold,
			Category:   cat,
			Evidence:   raw.Evidence,
			Abstain:    raw.Category == 5, // adversarial = abstention probe
		})
	}

	return bench.Case{
		Suite:     bench.SuiteLoCoMo,
		ID:        id,
		Namespace: "locomo-" + id,
		Turns:     turns,
		QA:        qa,
	}, nil
}

// flattenTurns walks session_<n> arrays in chronological order and produces
// a single ordered slice of bench.Turn values with SessionID set.
func flattenTurns(conv map[string]interface{}) ([]bench.Turn, error) {
	type sessionEntry struct {
		num      int
		key      string
		dateKey  string
	}
	var sessions []sessionEntry
	for k := range conv {
		if !strings.HasPrefix(k, "session_") || strings.HasSuffix(k, "_date_time") ||
			strings.HasSuffix(k, "_observation") || strings.HasSuffix(k, "_summary") ||
			strings.HasPrefix(k, "session_summary") {
			continue
		}
		// Some keys look like "session_1"; extract numeric suffix.
		numStr := strings.TrimPrefix(k, "session_")
		// Skip "session_summary" / "speaker_a" / etc.
		if numStr == "" {
			continue
		}
		n, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		sessions = append(sessions, sessionEntry{
			num:     n,
			key:     k,
			dateKey: k + "_date_time",
		})
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].num < sessions[j].num })

	var out []bench.Turn
	for _, s := range sessions {
		raw, ok := conv[s.key].([]interface{})
		if !ok {
			continue
		}
		ts, _ := parseDateTime(conv[s.dateKey])
		sid := fmt.Sprintf("session_%d", s.num)
		for _, t := range raw {
			tm, ok := t.(map[string]interface{})
			if !ok {
				continue
			}
			text, _ := tm["text"].(string)
			if text == "" {
				continue
			}
			speaker, _ := tm["speaker"].(string)
			dia, _ := tm["dia_id"].(string)
			out = append(out, bench.Turn{
				Speaker:   speaker,
				Role:      "user", // LoCoMo dyadic chat: both sides modeled as user-side input
				Text:      text,
				Timestamp: ts,
				SessionID: sid,
				TurnID:    dia,
			})
		}
	}
	return out, nil
}

func parseDateTime(v interface{}) (time.Time, error) {
	s, ok := v.(string)
	if !ok || s == "" {
		return time.Time{}, nil
	}
	// LoCoMo uses lowercase am/pm. Go's reference clock requires uppercase
	// PM, so normalise the meridiem before parsing.
	norm := s
	norm = strings.ReplaceAll(norm, " am ", " AM ")
	norm = strings.ReplaceAll(norm, " pm ", " PM ")
	norm = strings.ReplaceAll(norm, " am,", " AM,")
	norm = strings.ReplaceAll(norm, " pm,", " PM,")
	if strings.HasSuffix(norm, " am") {
		norm = strings.TrimSuffix(norm, " am") + " AM"
	}
	if strings.HasSuffix(norm, " pm") {
		norm = strings.TrimSuffix(norm, " pm") + " PM"
	}
	for _, layout := range []string{
		"3:04 PM on 2 January, 2006", // canonical LoCoMo
		"15:04 on 2 January, 2006",
		"3:04 PM on 2 January 2006",
		"2 January 2006, 15:04 PM",
		"2 January 2006, 3:04 PM",
		"January 2, 2006",
		"2 January 2006",
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, norm); err == nil {
			return t, nil
		}
	}
	return time.Time{}, nil
}

func stringify(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case []interface{}:
		var parts []string
		for _, e := range x {
			parts = append(parts, stringify(e))
		}
		return strings.Join(parts, "; ")
	case nil:
		return ""
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}
