// Package convomem loads the Salesforce/ConvoMem benchmark and adapts
// it to the bench.Case format.
//
// ConvoMem covers six evidence categories, each split into N_evidence
// buckets (1..6 for most, 1..3 for abstention/implicit, 2..6 for
// changing). The on-disk layout is:
//
//	<root>/<category>/<N>_evidence/batched_*.json
//
// Each batched_*.json is a JSON array of test cases; each test case
// has the shape:
//
//	{
//	  "conversations": [
//	    {"id": "<uuid>", "containsEvidence": true|false,
//	     "messages": [{"speaker":"user|assistant", "text": "..."}, ...]}
//	  ],
//	  "evidenceItems": [
//	    {"question": "...", "answer": "...", "category": "...",
//	     "message_evidences": [{"speaker":"user", "text":"..."}, ...]}
//	  ],
//	  "contextSize": 4096
//	}
//
// One test case becomes one bench.Case. Each evidenceItem inside the
// case becomes one bench.QA. Evidence for retrieval-recall purposes is
// the list of conversation IDs whose containsEvidence flag is true.
package convomem

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ArqonAi/Pixelog/internal/bench"
)

// AllCategories returns the canonical list of ConvoMem evidence categories.
func AllCategories() []string {
	return []string{
		"user_evidence",
		"assistant_facts_evidence",
		"changing_evidence",
		"abstention_evidence",
		"preference_evidence",
		"implicit_connection_evidence",
	}
}

// rawTest matches one element of the on-disk JSON array.
type rawTest struct {
	Conversations []rawConv         `json:"conversations"`
	EvidenceItems []rawEvidenceItem `json:"evidenceItems"`
	ContextSize   int               `json:"contextSize,omitempty"`
}

type rawConv struct {
	ID               string       `json:"id"`
	ContainsEvidence bool         `json:"containsEvidence"`
	Messages         []rawMessage `json:"messages"`
	ModelName        string       `json:"model_name,omitempty"`
}

type rawMessage struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

type rawEvidenceItem struct {
	Question         string       `json:"question"`
	Answer           string       `json:"answer"`
	Category         string       `json:"category,omitempty"`
	MessageEvidences []rawMessage `json:"message_evidences"`
	PersonID         string       `json:"personId,omitempty"`
}

// LoadOpts controls dataset loading.
type LoadOpts struct {
	Categories     []string
	MaxPerCategory int
}

// Dataset is the in-memory ConvoMem dataset rooted at the user-supplied path.
type Dataset struct {
	root  string
	opts  LoadOpts
	cases []bench.Case
}

// Load walks root and parses all matching JSON files.
func Load(root string, opts LoadOpts) (*Dataset, error) {
	if root == "" {
		return nil, fmt.Errorf("convomem: empty root")
	}
	d := &Dataset{root: root, opts: opts}
	if err := d.scan(); err != nil {
		return nil, err
	}
	return d, nil
}

// Suite implements bench.Dataset.
func (d *Dataset) Suite() bench.Suite { return bench.SuiteConvoMem }

// Cases implements bench.Dataset.
func (d *Dataset) Cases(_ context.Context) ([]bench.Case, error) { return d.cases, nil }

func (d *Dataset) scan() error {
	allowed := map[string]bool{}
	for _, c := range d.opts.Categories {
		allowed[c] = true
	}
	if len(allowed) == 0 {
		for _, c := range AllCategories() {
			allowed[c] = true
		}
	}

	categoryCounts := map[string]int{}

	return filepath.WalkDir(d.root, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dirEntry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".json") {
			return nil
		}

		category, evidenceN := categoryFromPath(path)
		if category == "" || !allowed[category] {
			return nil
		}
		if d.opts.MaxPerCategory > 0 && categoryCounts[category] >= d.opts.MaxPerCategory {
			return nil
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		var arr []rawTest
		if err := json.Unmarshal(raw, &arr); err != nil {
			// Tolerate the legacy {"test_cases": [...]} envelope.
			var env struct {
				TestCases []rawTest `json:"test_cases"`
			}
			if err2 := json.Unmarshal(raw, &env); err2 != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			arr = env.TestCases
		}

		for i, tc := range arr {
			if d.opts.MaxPerCategory > 0 && categoryCounts[category] >= d.opts.MaxPerCategory {
				break
			}
			c, ok := convertTest(tc, category, evidenceN, path, i)
			if !ok {
				continue
			}
			d.cases = append(d.cases, c)
			categoryCounts[category]++
		}
		return nil
	})
}

// convertTest turns one raw test case into a bench.Case. Returns
// (case, true) on success or (zero, false) when the case has no
// usable QAs or conversations.
func convertTest(tc rawTest, category string, evidenceN int, sourcePath string, idx int) (bench.Case, bool) {
	if len(tc.Conversations) == 0 || len(tc.EvidenceItems) == 0 {
		return bench.Case{}, false
	}

	id := fmt.Sprintf("%s-%s-%d-%s", category, evidenceLabel(evidenceN),
		idx, shortHash(sourcePath))

	turns := make([]bench.Turn, 0)
	evidenceConvIDs := make([]string, 0, len(tc.Conversations))
	for ci, conv := range tc.Conversations {
		sid := conv.ID
		if sid == "" {
			sid = fmt.Sprintf("conv_%d", ci)
		}
		if conv.ContainsEvidence {
			evidenceConvIDs = append(evidenceConvIDs, sid)
		}
		for mi, m := range conv.Messages {
			text := strings.TrimSpace(m.Text)
			if text == "" {
				continue
			}
			role := strings.ToLower(strings.TrimSpace(m.Speaker))
			if role == "" {
				role = "user"
			}
			speaker := role
			if role == "assistant" {
				speaker = "Assistant"
			} else if role == "user" {
				speaker = "User"
			}
			turns = append(turns, bench.Turn{
				Speaker:   speaker,
				Role:      role,
				Text:      text,
				SessionID: sid,
				TurnID:    fmt.Sprintf("%s:%d", sid, mi),
			})
		}
	}
	if len(turns) == 0 {
		return bench.Case{}, false
	}

	qa := make([]bench.QA, 0, len(tc.EvidenceItems))
	abstainCat := category == "abstention_evidence"
	for j, ei := range tc.EvidenceItems {
		cat := category // keep the on-disk category for grouping/aggregation
		if ei.Category != "" {
			// Surface the fine-grained ei.category as a hint via Rubric.
		}
		qa = append(qa, bench.QA{
			ID:         fmt.Sprintf("%s-q%d", id, j),
			Question:   ei.Question,
			GoldAnswer: ei.Answer,
			Category:   cat,
			Evidence:   evidenceConvIDs,
			EvidenceN:  evidenceN,
			Abstain:    abstainCat,
			Rubric:     ei.Category,
		})
	}

	return bench.Case{
		Suite:      bench.SuiteConvoMem,
		ID:         id,
		Namespace:  "convomem-" + id,
		Turns:      turns,
		QA:         qa,
		SourceFile: sourcePath,
	}, true
}

func evidenceLabel(n int) string {
	if n <= 0 {
		return "n0"
	}
	return fmt.Sprintf("n%d", n)
}

// shortHash deterministically derives a short tag from a path so case
// IDs are unique across batched_*.json files without dragging full
// paths into reports.
func shortHash(s string) string {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return fmt.Sprintf("%08x", h)
}

func categoryFromPath(path string) (string, int) {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, p := range parts {
		for _, c := range AllCategories() {
			if p == c {
				n := 0
				if i+1 < len(parts) {
					sub := parts[i+1]
					var x int
					if _, err := fmt.Sscanf(sub, "%d_evidence", &x); err == nil {
						n = x
					}
				}
				return c, n
			}
		}
	}
	return "", 0
}
