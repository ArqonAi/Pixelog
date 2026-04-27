package bench

import (
	"strings"
	"unicode"
)

// tokenF1 computes per-token F1 between gold and predicted.
func tokenF1(gold, predicted string) float64 {
	g := tokenise(gold)
	p := tokenise(predicted)
	if len(g) == 0 && len(p) == 0 {
		return 1.0
	}
	if len(g) == 0 || len(p) == 0 {
		return 0
	}
	common := commonTokens(g, p)
	if common == 0 {
		return 0
	}
	prec := float64(common) / float64(len(p))
	rec := float64(common) / float64(len(g))
	return 2 * prec * rec / (prec + rec)
}

func tokenise(s string) []string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Fields(b.String())
}

func commonTokens(a, b []string) int {
	ca := count(a)
	cb := count(b)
	common := 0
	for tok, na := range ca {
		if nb, ok := cb[tok]; ok {
			if nb < na {
				common += nb
			} else {
				common += na
			}
		}
	}
	return common
}

func count(toks []string) map[string]int {
	m := make(map[string]int, len(toks))
	for _, t := range toks {
		m[t]++
	}
	return m
}

// Aggregate computes summary metrics from QA results.
func Aggregate(results []QAResult) Metrics {
	m := Metrics{Count: len(results)}
	if len(results) == 0 {
		return m
	}

	var sumJudge, sumF1, sumLatency float64
	var sumRecall, sumHit float64
	emCount := 0
	abstainCount := 0
	abstainCorrect := 0
	recallScored := 0

	for _, r := range results {
		if r.Error != "" {
			m.Errors++
			continue
		}
		sumJudge += r.JudgeScore
		sumF1 += r.F1
		sumLatency += float64(r.Latency.Milliseconds())
		if r.ExactMatch {
			emCount++
		}
		if r.QA.Abstain {
			abstainCount++
			if r.AbstainOK {
				abstainCorrect++
			}
		}
		if len(r.QA.Evidence) > 0 {
			recallScored++
			sumRecall += r.RecallAtK
			sumHit += r.HitAtK
		}
	}

	scored := float64(m.Count - m.Errors)
	if scored > 0 {
		m.JudgeMean = sumJudge / scored
		m.F1Mean = sumF1 / scored
		m.MeanLatencyMS = sumLatency / scored
		m.ExactMatchRate = float64(emCount) / scored
	}
	if abstainCount > 0 {
		m.AbstainAccuracy = float64(abstainCorrect) / float64(abstainCount)
	}
	if recallScored > 0 {
		m.NumScored = recallScored
		m.RecallAtK = sumRecall / float64(recallScored)
		m.HitAtK = sumHit / float64(recallScored)
	}
	return m
}

// AggregateByCategory groups QA results by their category and aggregates each.
func AggregateByCategory(results []QAResult) map[string]Metrics {
	groups := make(map[string][]QAResult)
	for _, r := range results {
		cat := r.QA.Category
		if cat == "" {
			cat = "uncategorised"
		}
		groups[cat] = append(groups[cat], r)
	}
	out := make(map[string]Metrics, len(groups))
	for cat, rs := range groups {
		out[cat] = Aggregate(rs)
	}
	return out
}
