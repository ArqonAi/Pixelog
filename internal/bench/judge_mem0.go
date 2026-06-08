package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Mem0AccuracyPrompt is the verbatim LLM-judge prompt from
// mem0/evaluation/metrics/llm_judge.py (arXiv:2504.19413, MIT license).
//
// We use it unchanged so head-to-head comparisons against the published
// Mem0 / Zep / OpenAI numbers share an identical judge — vendors
// cannot dispute the comparison on prompt grounds.
//
// Source:
//
//	https://github.com/mem0ai/mem0/blob/main/evaluation/metrics/llm_judge.py
const Mem0AccuracyPrompt = `
Your task is to label an answer to a question as ’CORRECT’ or ’WRONG’. You will be given the following data:
    (1) a question (posed by one user to another user), 
    (2) a ’gold’ (ground truth) answer, 
    (3) a generated answer
which you will score as CORRECT/WRONG.

The point of the question is to ask about something one user should know about the other user based on their prior conversations.
The gold answer will usually be a concise and short answer that includes the referenced topic, for example:
Question: Do you remember what I got the last time I went to Hawaii?
Gold answer: A shell necklace
The generated answer might be much longer, but you should be generous with your grading - as long as it touches on the same topic as the gold answer, it should be counted as CORRECT. 

For time related questions, the gold answer will be a specific date, month, year, etc. The generated answer might be much longer or use relative time references (like "last Tuesday" or "next month"), but you should be generous with your grading - as long as it refers to the same date or time period as the gold answer, it should be counted as CORRECT. Even if the format differs (e.g., "May 7th" vs "7 May"), consider it CORRECT if it's the same date.

Now it's time for the real question:
Question: %s
Gold answer: %s
Generated answer: %s

First, provide a short (one sentence) explanation of your reasoning, then finish with CORRECT or WRONG. 
Do NOT include both CORRECT and WRONG in your response, or it will break the evaluation script.

Just return the label CORRECT or WRONG in a json format with the key as "label".
`

// Mem0Judge is a binary CORRECT/WRONG judge that ports the Mem0
// evaluation pipeline verbatim. Score is 1.0 for CORRECT, 0.0 for
// WRONG. Falls back to ExactMatchJudge on chat / parse errors.
type Mem0Judge struct {
	Chat     LLMChat
	Fallback Judge
}

// NewMem0Judge constructs a Mem0Judge with ExactMatchJudge fallback.
func NewMem0Judge(chat LLMChat) *Mem0Judge {
	return &Mem0Judge{Chat: chat, Fallback: ExactMatchJudge{}}
}

// Score implements Judge.
func (j *Mem0Judge) Score(ctx context.Context, qa QA, predicted string) (float64, string, error) {
	if j.Chat == nil {
		if j.Fallback != nil {
			return j.Fallback.Score(ctx, qa, predicted)
		}
		return 0, "", fmt.Errorf("Mem0Judge: no chat client configured")
	}

	// Abstain semantics: the mem0 prompt has no native abstain rubric,
	// so we apply ExactMatchJudge's abstain heuristic before invoking
	// the LLM. This preserves cross-suite scoring (LongMemEval has
	// abstention questions; LoCoMo does not).
	if qa.Abstain {
		return ExactMatchJudge{}.Score(ctx, qa, predicted)
	}

	prompt := fmt.Sprintf(Mem0AccuracyPrompt, qa.Question, qa.GoldAnswer, predicted)
	resp, err := j.Chat.Chat(prompt)
	if err != nil {
		if j.Fallback != nil {
			s, r, fbErr := j.Fallback.Score(ctx, qa, predicted)
			if fbErr == nil {
				return s, fmt.Sprintf("mem0 judge failed (%v); fallback: %s", err, r), nil
			}
		}
		return 0, "", fmt.Errorf("mem0 judge failed: %w", err)
	}

	label, reason := parseMem0JudgeResponse(resp)
	switch label {
	case "CORRECT":
		return 1.0, reason, nil
	case "WRONG":
		return 0.0, reason, nil
	default:
		// Unparseable — fall through to fallback for resilience.
		if j.Fallback != nil {
			s, r, fbErr := j.Fallback.Score(ctx, qa, predicted)
			if fbErr == nil {
				return s, fmt.Sprintf("mem0 judge unparseable; fallback: %s", r), nil
			}
		}
		return 0, "judge unparseable: " + truncate(resp, 200), nil
	}
}

// jsonLabelRE pulls "label": "CORRECT|WRONG" out of a JSON object.
var jsonLabelRE = regexp.MustCompile(`(?i)"label"\s*:\s*"(CORRECT|WRONG)"`)

// parseMem0JudgeResponse extracts the binary label. Tries strict JSON
// first; falls back to regex over the body, then to a final-token
// keyword scan ("CORRECT" / "WRONG") to match the prompt's relaxed
// output contract.
func parseMem0JudgeResponse(resp string) (string, string) {
	resp = strings.TrimSpace(resp)
	reason := truncate(resp, 240)

	// Strip code fences the model sometimes wraps JSON in.
	body := stripCodeFence(resp)

	// Try strict JSON.
	var obj struct {
		Label  string `json:"label"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(body), &obj); err == nil {
		l := strings.ToUpper(strings.TrimSpace(obj.Label))
		if l == "CORRECT" || l == "WRONG" {
			r := obj.Reason
			if r == "" {
				r = reason
			}
			return l, r
		}
	}

	// Regex over any embedded JSON-ish payload.
	if m := jsonLabelRE.FindStringSubmatch(body); len(m) == 2 {
		return strings.ToUpper(m[1]), reason
	}

	// Final fallback: keyword presence. Prompt explicitly forbids
	// emitting both labels, so first hit wins.
	upper := strings.ToUpper(resp)
	hasC := strings.Contains(upper, "CORRECT")
	hasW := strings.Contains(upper, "WRONG")
	switch {
	case hasC && !hasW:
		return "CORRECT", reason
	case hasW && !hasC:
		return "WRONG", reason
	}
	return "", reason
}

// stripCodeFence removes ```json ... ``` wrappers.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	// Drop opening fence line.
	if i := strings.IndexByte(s, '\n'); i > 0 {
		s = s[i+1:]
	}
	if j := strings.LastIndex(s, "```"); j > 0 {
		s = s[:j]
	}
	return strings.TrimSpace(s)
}
