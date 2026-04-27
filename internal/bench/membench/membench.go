// Package membench loads the MemBench (ACL 2025 Findings, Tan et al.)
// benchmark and adapts it to bench.Case.
//
// MemBench partitions data along two axes:
//
//	Perspective: Participation (FirstAgent) | Observation (ThirdAgent)
//	Question type (one file per type): simple, conditional, comparative,
//	  aggregative, knowledge_update, post_processing, noisy
//
// Layout on disk (cloned from import-myself/Membench):
//
//	MemData/FirstAgent/{simple,conditional,...}.json
//	MemData/ThirdAgent/{simple,conditional,...}.json
//
// Each file is:
//
//	{
//	  "roles": [
//	    {
//	      "tid": 0,
//	      "message_list": [
//	        [ { "sid": 0, "user_message": "...", "assistant_message": "...",
//	            "time": "...", "place": "..." }, ... ],   // session 0
//	        [ ... ],                                       // session 1
//	        ...
//	      ],
//	      "QA": {
//	        "qid": 0, "question": "...", "answer": "...",
//	        "target_step_id": [[<session_idx>, <msg_idx>], ...],
//	        "choices": { "A": "...", "B": "...", "C": "...", "D": "..." }
//	      }
//	    }
//	  ],
//	  "events": [...]
//	}
//
// One role becomes one bench.Case with a single bench.QA. The QA's
// Evidence is the list of session IDs that contain target steps.
package membench

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ArqonAi/Pixelog/internal/bench"
)

// Perspective is the user-vs-third-party split.
type Perspective string

const (
	Participation Perspective = "participation" // FirstAgent
	Observation   Perspective = "observation"   // ThirdAgent
)

// PerspectiveFromPath looks at the directory path to derive Perspective.
func PerspectiveFromPath(p string) Perspective {
	lp := strings.ToLower(filepath.ToSlash(p))
	switch {
	case strings.Contains(lp, "/firstagent/"):
		return Participation
	case strings.Contains(lp, "/thirdagent/"):
		return Observation
	}
	return ""
}

// QuestionTypeFromPath returns the basename without extension.
func QuestionTypeFromPath(p string) string {
	base := filepath.Base(p)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// rawFile is the on-disk schema.
type rawFile struct {
	Roles  []rawRole `json:"roles"`
	Events any       `json:"events,omitempty"`
}

type rawRole struct {
	TID int `json:"tid"`
	// MessageList may be either:
	//   FirstAgent (Participation): [][]rawDialogueMessage  (session, msg)
	//   ThirdAgent (Observation):   []rawObservationMessage (flat)
	MessageList json.RawMessage `json:"message_list"`
	QA          rawQA           `json:"QA"`
}

type rawDialogueMessage struct {
	SID              int    `json:"sid"`
	UserMessage      string `json:"user_message"`
	AssistantMessage string `json:"assistant_message"`
	Time             string `json:"time,omitempty"`
	Place            string `json:"place,omitempty"`
}

type rawObservationMessage struct {
	MID     int    `json:"mid"`
	Message string `json:"message"`
	Time    string `json:"time,omitempty"`
	Place   string `json:"place,omitempty"`
	Rel     string `json:"rel,omitempty"`
	Attr    string `json:"attr,omitempty"`
	Value   string `json:"value,omitempty"`
}

type rawQA struct {
	QID          int               `json:"qid"`
	Question     string            `json:"question"`
	Answer       json.RawMessage   `json:"answer"`
	TargetStepID json.RawMessage   `json:"target_step_id"`
	Choices      map[string]string `json:"choices,omitempty"`
	GroundTruth  string            `json:"ground_truth,omitempty"`
}

// observationChunkSize groups N flat ThirdAgent messages into one
// "session" for retrieval purposes. 10 is a sane default — small
// enough that a top-5 query has meaningful selectivity over the 100
// or so messages in a typical role, large enough to keep recall
// scoring stable.
const observationChunkSize = 10

// LoadOpts controls the loader.
type LoadOpts struct {
	Max int // 0 = unlimited
}

// Dataset is one or more MemBench JSON files loaded together.
type Dataset struct {
	cases []bench.Case
}

// Load parses a single MemBench JSON file.
func Load(path string, opts LoadOpts) (*Dataset, error) {
	cases, err := loadOne(path, opts)
	if err != nil {
		return nil, err
	}
	return &Dataset{cases: cases}, nil
}

// LoadMany loads several files (typically a whole MemData/* tree).
func LoadMany(paths []string, opts LoadOpts) (*Dataset, error) {
	all := make([]bench.Case, 0)
	for _, p := range paths {
		cases, err := loadOne(p, opts)
		if err != nil {
			return nil, err
		}
		all = append(all, cases...)
	}
	return &Dataset{cases: all}, nil
}

// LoadAllInDir recursively loads every *.json file under dir. Files
// without a `roles` array (e.g. graphs.json) are skipped.
func LoadAllInDir(dir string, opts LoadOpts) (*Dataset, error) {
	var paths []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".json") {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("membench: walk %s: %w", dir, err)
	}
	return LoadMany(paths, opts)
}

// Suite implements bench.Dataset.
func (d *Dataset) Suite() bench.Suite { return bench.SuiteMemBench }

// Cases implements bench.Dataset.
func (d *Dataset) Cases(_ context.Context) ([]bench.Case, error) { return d.cases, nil }

func loadOne(path string, opts LoadOpts) ([]bench.Case, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("membench: read %s: %w", path, err)
	}

	var f rawFile
	if err := json.Unmarshal(raw, &f); err != nil {
		// Tolerate files that are flat arrays of roles.
		var roles []rawRole
		if err2 := json.Unmarshal(raw, &roles); err2 != nil {
			return nil, fmt.Errorf("membench: parse %s: %w", path, err)
		}
		f.Roles = roles
	}
	if len(f.Roles) == 0 {
		return nil, nil
	}

	perspective := PerspectiveFromPath(path)
	qtype := QuestionTypeFromPath(path)
	category := qtype // primary grouping is question type
	if perspective != "" {
		category = string(perspective) + "_" + qtype
	}

	out := make([]bench.Case, 0, len(f.Roles))
	for i, r := range f.Roles {
		if opts.Max > 0 && len(out) >= opts.Max {
			break
		}
		c, ok := convertRole(r, category, qtype, string(perspective), path, i)
		if !ok {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func convertRole(r rawRole, category, qtype, perspective, srcPath string, idx int) (bench.Case, bool) {
	if r.QA.Question == "" || len(r.MessageList) == 0 {
		return bench.Case{}, false
	}

	turns, evidence, ok := convertMessages(r.MessageList, r.QA.TargetStepID)
	if !ok {
		return bench.Case{}, false
	}

	id := fmt.Sprintf("%s-%d", category, r.TID)
	if r.TID == 0 && idx > 0 {
		id = fmt.Sprintf("%s-%d", category, idx)
	}

	gold := strings.TrimSpace(membenchAnswer(r.QA.Answer, r.QA.Choices))
	if gold == "" && r.QA.GroundTruth != "" {
		gold = membenchAnswer(json.RawMessage(`"`+r.QA.GroundTruth+`"`), r.QA.Choices)
	}

	return bench.Case{
		Suite:     bench.SuiteMemBench,
		ID:        id,
		Namespace: "membench-" + id,
		Turns:     turns,
		QA: []bench.QA{{
			ID:         fmt.Sprintf("%s-q%d", id, r.QA.QID),
			Question:   r.QA.Question,
			GoldAnswer: gold,
			Category:   category,
			Evidence:   evidence,
			Rubric:     qtype,
		}},
		SourceFile: srcPath,
	}, true
}

// looksLikeDialogueShape returns true when at least one inner message
// has either UserMessage or AssistantMessage set, distinguishing the
// FirstAgent dialogue schema from a ThirdAgent flat list that
// happens to decode without erroring.
func looksLikeDialogueShape(d [][]rawDialogueMessage) bool {
	for _, sess := range d {
		for _, m := range sess {
			if m.UserMessage != "" || m.AssistantMessage != "" {
				return true
			}
		}
	}
	return false
}

// convertMessages decodes message_list and target_step_id under either
// the FirstAgent (Participation) dialogue schema or the ThirdAgent
// (Observation) flat-message schema, producing turns and evidence
// IDs in the canonical bench shape.
func convertMessages(msgListRaw, targetRaw json.RawMessage) (turns []bench.Turn, evidence []string, ok bool) {
	// Try FirstAgent first: [][]rawDialogueMessage with target_step_id
	// of [[sid, x], ...] where sid is a GLOBAL message id incrementing
	// across all sessions (NOT the array index of message_list).
	var dialogue [][]rawDialogueMessage
	if err := json.Unmarshal(msgListRaw, &dialogue); err == nil && len(dialogue) > 0 {
		// Heuristic: dialogue[0][0] should have the dialogue field
		// shape — peek at SID (always present) and UserMessage/Assistant.
		if !looksLikeDialogueShape(dialogue) {
			// fall through to ThirdAgent decode below
		} else {
			// Build the sid → session-index map as we flatten turns.
			sidToSessionIdx := map[int]int{}
			turns = make([]bench.Turn, 0, 8)
			for sIdx, sess := range dialogue {
				sessID := fmt.Sprintf("session_%d", sIdx)
				for mIdx, m := range sess {
					sidToSessionIdx[m.SID] = sIdx
					if u := strings.TrimSpace(m.UserMessage); u != "" {
						turns = append(turns, bench.Turn{
							Speaker:   "User",
							Role:      "user",
							Text:      u,
							SessionID: sessID,
							TurnID:    fmt.Sprintf("%s:%d:user", sessID, mIdx),
						})
					}
					if a := strings.TrimSpace(m.AssistantMessage); a != "" {
						turns = append(turns, bench.Turn{
							Speaker:   "Assistant",
							Role:      "assistant",
							Text:      a,
							SessionID: sessID,
							TurnID:    fmt.Sprintf("%s:%d:assistant", sessID, mIdx),
						})
					}
				}
			}
			if len(turns) > 0 {
				var targets [][]int
				_ = json.Unmarshal(targetRaw, &targets)
				seen := make(map[string]bool)
				for _, ts := range targets {
					if len(ts) == 0 {
						continue
					}
					sIdx, ok := sidToSessionIdx[ts[0]]
					if !ok {
						continue
					}
					sessID := fmt.Sprintf("session_%d", sIdx)
					if !seen[sessID] {
						seen[sessID] = true
						evidence = append(evidence, sessID)
					}
					// We don't have a stable mapping from global sid to
					// the per-session message index without re-walking,
					// so we anchor evidence at the session level — the
					// same granularity used by retrieval-recall benchmarks.
				}
				return turns, evidence, true
			}
		}
	}

	// Fall through to ThirdAgent: []rawObservationMessage with
	// target_step_id [int, ...].
	var obs []rawObservationMessage
	if err := json.Unmarshal(msgListRaw, &obs); err != nil || len(obs) == 0 {
		return nil, nil, false
	}
	turns = make([]bench.Turn, 0, len(obs))
	for _, o := range obs {
		text := strings.TrimSpace(o.Message)
		if text == "" {
			continue
		}
		sid := fmt.Sprintf("session_%d", o.MID/observationChunkSize)
		turns = append(turns, bench.Turn{
			Speaker:   "Observer",
			Role:      "observation",
			Text:      text,
			SessionID: sid,
			TurnID:    fmt.Sprintf("%s:m%d", sid, o.MID),
		})
	}
	if len(turns) == 0 {
		return nil, nil, false
	}

	var mids []int
	_ = json.Unmarshal(targetRaw, &mids)
	seen := make(map[string]bool)
	for _, mid := range mids {
		sid := fmt.Sprintf("session_%d", mid/observationChunkSize)
		if !seen[sid] {
			seen[sid] = true
			evidence = append(evidence, sid)
		}
		evidence = append(evidence, fmt.Sprintf("%s:m%d", sid, mid))
	}
	return turns, evidence, true
}

// membenchAnswer flattens the QA.answer field, optionally resolving
// multiple-choice letters via choices map.
func membenchAnswer(raw json.RawMessage, choices map[string]string) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		// If the answer is a single letter and matches a choice, expand.
		if len(s) == 1 && choices != nil {
			if v, ok := choices[strings.ToUpper(s)]; ok {
				return s + ". " + v
			}
		}
		return s
	}
	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		if num == float64(int64(num)) {
			return fmt.Sprintf("%d", int64(num))
		}
		return fmt.Sprintf("%g", num)
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil {
		parts := make([]string, 0, len(list))
		for _, item := range list {
			parts = append(parts, membenchAnswer(item, choices))
		}
		return strings.Join(parts, "; ")
	}
	return strings.TrimSpace(string(raw))
}
