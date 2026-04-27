package bench

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Judge scores a predicted answer against a gold answer.
// Returns score in [0, 1] plus an optional natural-language reason.
type Judge interface {
	Score(ctx context.Context, qa QA, predicted string) (score float64, reason string, err error)
}

// LLMChat is the minimal LLM interface required by LLMJudge. It is
// satisfied by *llm.Client (Chat method).
type LLMChat interface {
	Chat(prompt string) (string, error)
}

// LLMJudge uses a chat model to score predictions on a 0-5 rubric and
// normalises to [0, 1]. Falls back to ExactMatchJudge on chat errors.
type LLMJudge struct {
	Chat     LLMChat
	Fallback Judge
}

// NewLLMJudge creates an LLM-as-judge with ExactMatchJudge fallback.
func NewLLMJudge(chat LLMChat) *LLMJudge {
	return &LLMJudge{Chat: chat, Fallback: ExactMatchJudge{}}
}

// Score implements Judge.
func (j *LLMJudge) Score(ctx context.Context, qa QA, predicted string) (float64, string, error) {
	if j.Chat == nil {
		if j.Fallback != nil {
			return j.Fallback.Score(ctx, qa, predicted)
		}
		return 0, "", fmt.Errorf("LLMJudge: no chat client configured")
	}

	prompt := buildJudgePrompt(qa, predicted)
	resp, err := j.Chat.Chat(prompt)
	if err != nil {
		if j.Fallback != nil {
			s, r, fbErr := j.Fallback.Score(ctx, qa, predicted)
			if fbErr == nil {
				return s, fmt.Sprintf("llm judge failed (%v); fallback: %s", err, r), nil
			}
		}
		return 0, "", fmt.Errorf("LLM judge failed: %w", err)
	}

	score, reason := parseJudgeResponse(resp)
	return score, reason, nil
}

func buildJudgePrompt(qa QA, predicted string) string {
	var sb strings.Builder
	sb.WriteString("You are a strict evaluator scoring predicted answers against gold answers.\n")
	sb.WriteString("Score 0-5 where:\n")
	sb.WriteString("  5 = semantically identical, all key facts present\n")
	sb.WriteString("  4 = essentially correct, minor omissions\n")
	sb.WriteString("  3 = partially correct, key fact present but incomplete\n")
	sb.WriteString("  2 = mostly wrong, only loosely related\n")
	sb.WriteString("  1 = wrong but on-topic\n")
	sb.WriteString("  0 = completely incorrect or empty\n\n")
	if qa.Abstain {
		sb.WriteString("NOTE: This question has no answer in the conversation. ")
		sb.WriteString("Score 5 ONLY if the prediction explicitly declines (\"I don't know\", ")
		sb.WriteString("\"not mentioned\", etc.). Otherwise score 0.\n\n")
	}
	if qa.Rubric != "" {
		sb.WriteString("Additional rubric: ")
		sb.WriteString(qa.Rubric)
		sb.WriteString("\n\n")
	}
	sb.WriteString("Question: ")
	sb.WriteString(qa.Question)
	sb.WriteString("\nGold answer: ")
	sb.WriteString(qa.GoldAnswer)
	sb.WriteString("\nPredicted answer: ")
	sb.WriteString(predicted)
	sb.WriteString("\n\nRespond with ONLY one line: SCORE: <0-5> | REASON: <one sentence>\n")
	return sb.String()
}

func parseJudgeResponse(resp string) (float64, string) {
	resp = strings.TrimSpace(resp)
	scoreStr := ""
	reason := ""

	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "SCORE:") {
			body := strings.TrimSpace(line[len("SCORE:"):])
			parts := strings.SplitN(body, "|", 2)
			scoreStr = strings.TrimSpace(parts[0])
			if len(parts) == 2 {
				r := strings.TrimSpace(parts[1])
				if strings.HasPrefix(strings.ToUpper(r), "REASON:") {
					reason = strings.TrimSpace(r[len("REASON:"):])
				} else {
					reason = r
				}
			}
			break
		}
	}

	// Tolerant fallback: find first digit 0-5
	if scoreStr == "" {
		for _, r := range resp {
			if r >= '0' && r <= '5' {
				scoreStr = string(r)
				break
			}
		}
	}

	score, err := strconv.ParseFloat(scoreStr, 64)
	if err != nil {
		return 0, "judge response unparseable: " + truncate(resp, 200)
	}
	if score < 0 {
		score = 0
	}
	if score > 5 {
		score = 5
	}
	return score / 5.0, reason
}

// ExactMatchJudge scores 1.0 if predicted exactly matches gold (case-insensitive,
// whitespace-normalised), else uses token F1 as a soft fallback. Useful for
// CI runs without an LLM.
type ExactMatchJudge struct{}

// Score implements Judge.
func (ExactMatchJudge) Score(ctx context.Context, qa QA, predicted string) (float64, string, error) {
	if qa.Abstain {
		// Heuristic: predicted indicates abstention?
		lp := strings.ToLower(predicted)
		abst := strings.Contains(lp, "i don't know") ||
			strings.Contains(lp, "i do not know") ||
			strings.Contains(lp, "not mentioned") ||
			strings.Contains(lp, "no information") ||
			strings.Contains(lp, "cannot determine") ||
			strings.Contains(lp, "unknown")
		if abst {
			return 1.0, "abstention detected", nil
		}
		return 0.0, "abstention expected, prediction asserted an answer", nil
	}

	if normalise(predicted) == normalise(qa.GoldAnswer) {
		return 1.0, "exact match", nil
	}
	f1 := tokenF1(qa.GoldAnswer, predicted)
	return f1, fmt.Sprintf("token F1=%.2f", f1), nil
}

// normalise lowercases and collapses whitespace.
func normalise(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
