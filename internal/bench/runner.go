package bench

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Config controls runner behaviour.
type Config struct {
	Mode             Mode
	IncludeCases     bool          // include per-case results in Report.Cases
	PerQATimeout     time.Duration // 0 = no timeout
	StopOnFirstError bool
	// K sets the top-k used for retrieval-recall evaluation. When
	// > 0 the runner will truncate each QA's RetrievedIDs to K and
	// compute RecallAtK / HitAtK. 0 means "score everything retrieved".
	K int
}

// DefaultConfig returns sensible runner defaults.
func DefaultConfig() Config {
	return Config{
		Mode:         ModeSession,
		IncludeCases: false,
		PerQATimeout: 60 * time.Second,
	}
}

// Runner executes a Dataset against a MemorySystem and scores via Judge.
type Runner struct {
	mem    MemorySystem
	judge  Judge
	cfg    Config
}

// NewRunner constructs a Runner.
func NewRunner(mem MemorySystem, judge Judge, cfg Config) *Runner {
	if cfg.Mode == "" {
		cfg.Mode = ModeSession
	}
	return &Runner{mem: mem, judge: judge, cfg: cfg}
}

// Run executes the full benchmark.
func (r *Runner) Run(ctx context.Context, ds Dataset) (*Report, error) {
	cases, err := ds.Cases(ctx)
	if err != nil {
		return nil, fmt.Errorf("load dataset: %w", err)
	}

	report := &Report{
		Suite:     ds.Suite(),
		Mode:      r.cfg.Mode,
		K:         r.cfg.K,
		StartedAt: time.Now(),
		NumCases:  len(cases),
	}

	var allResults []QAResult
	for _, c := range cases {
		caseRes, err := r.runCase(ctx, c)
		if err != nil && r.cfg.StopOnFirstError {
			return report, err
		}
		allResults = append(allResults, caseRes.QAResults...)
		if r.cfg.IncludeCases {
			report.Cases = append(report.Cases, caseRes)
		}
	}

	report.NumQA = len(allResults)
	report.Aggregate = Aggregate(allResults)
	report.ByCategory = AggregateByCategory(allResults)
	report.FinishedAt = time.Now()
	return report, nil
}

func (r *Runner) runCase(ctx context.Context, c Case) (CaseResult, error) {
	start := time.Now()
	res := CaseResult{Case: c}

	if err := r.mem.Reset(ctx, c.Namespace); err != nil {
		res.Error = fmt.Sprintf("reset: %v", err)
		res.Duration = time.Since(start)
		return res, fmt.Errorf("reset namespace %q: %w", c.Namespace, err)
	}

	if err := r.ingest(ctx, c); err != nil {
		res.Error = fmt.Sprintf("ingest: %v", err)
		res.Duration = time.Since(start)
		return res, fmt.Errorf("ingest case %q: %w", c.ID, err)
	}

	for _, qa := range c.QA {
		qaRes := r.runQA(ctx, c.Namespace, qa)
		res.QAResults = append(res.QAResults, qaRes)
	}

	res.Duration = time.Since(start)
	return res, nil
}

func (r *Runner) ingest(ctx context.Context, c Case) error {
	switch r.cfg.Mode {
	case ModeFull, ModeRecall:
		if err := r.mem.Ingest(ctx, c.Namespace, c.Turns); err != nil {
			return err
		}
		return r.mem.Consolidate(ctx, c.Namespace)
	case ModeSession, ModeHybrid:
		// Group turns by SessionID, in order of first appearance.
		var order []string
		groups := make(map[string][]Turn)
		for _, t := range c.Turns {
			sid := t.SessionID
			if sid == "" {
				sid = "default"
			}
			if _, ok := groups[sid]; !ok {
				order = append(order, sid)
			}
			groups[sid] = append(groups[sid], t)
		}
		for _, sid := range order {
			if err := r.mem.Ingest(ctx, c.Namespace, groups[sid]); err != nil {
				return err
			}
			if err := r.mem.Consolidate(ctx, c.Namespace); err != nil {
				return err
			}
		}
		// Hybrid mode does an additional final consolidate. Real "hybrid"
		// retrieval combines session memory with full-context fallback at
		// answer time; that lives in the MemorySystem implementation.
		if r.cfg.Mode == ModeHybrid {
			return r.mem.Consolidate(ctx, c.Namespace)
		}
		return nil
	default:
		return fmt.Errorf("unknown mode: %q", r.cfg.Mode)
	}
}

func (r *Runner) runQA(ctx context.Context, namespace string, qa QA) QAResult {
	qaCtx := ctx
	var cancel context.CancelFunc
	if r.cfg.PerQATimeout > 0 {
		qaCtx, cancel = context.WithTimeout(ctx, r.cfg.PerQATimeout)
		defer cancel()
	}

	if r.cfg.Mode == ModeRecall {
		return r.runRecall(qaCtx, namespace, qa)
	}

	start := time.Now()
	ans, err := r.mem.Answer(qaCtx, namespace, qa.Question)
	if err != nil {
		return QAResult{
			QA:      qa,
			Latency: time.Since(start),
			Error:   err.Error(),
		}
	}

	score, reason, judgeErr := r.judge.Score(qaCtx, qa, ans.Text)
	res := QAResult{
		QA:           qa,
		Predicted:    ans.Text,
		JudgeScore:   score,
		JudgeReason:  reason,
		ExactMatch:   strings.EqualFold(strings.TrimSpace(ans.Text), strings.TrimSpace(qa.GoldAnswer)),
		F1:           tokenF1(qa.GoldAnswer, ans.Text),
		Latency:      ans.Latency,
		RetrievedIDs: ans.RetrievedIDs,
	}
	if qa.Abstain {
		res.AbstainOK = score >= 0.8
	}
	if judgeErr != nil {
		res.Error = "judge: " + judgeErr.Error()
	}
	// Always compute recall if evidence is present and we got retrievals.
	if len(qa.Evidence) > 0 && len(ans.RetrievedIDs) > 0 {
		res.RecallAtK, res.HitAtK = computeRecall(ans.RetrievedIDs, qa.Evidence, r.cfg.K)
	}
	return res
}

// runRecall is the retrieval-only evaluation path. It requires the
// MemorySystem to implement the Retriever interface. No LLM answerer
// or judge is invoked — the reported score is Recall@K / Hit@K against
// the QA evidence set.
func (r *Runner) runRecall(ctx context.Context, namespace string, qa QA) QAResult {
	start := time.Now()
	retriever, ok := r.mem.(Retriever)
	if !ok {
		return QAResult{
			QA:      qa,
			Latency: time.Since(start),
			Error:   "memory system does not implement bench.Retriever",
		}
	}
	k := r.cfg.K
	if k <= 0 {
		k = 10
	}
	ids, err := retriever.Retrieve(ctx, namespace, qa.Question, k)
	if err != nil {
		return QAResult{
			QA:      qa,
			Latency: time.Since(start),
			Error:   "retrieve: " + err.Error(),
		}
	}
	res := QAResult{
		QA:           qa,
		Latency:      time.Since(start),
		RetrievedIDs: ids,
	}
	if len(qa.Evidence) > 0 {
		// k=0: do not truncate. The Retriever is responsible for
		// emitting top-k sessions and their turn IDs in one slice,
		// so both session- and turn-granular evidence match cleanly.
		res.RecallAtK, res.HitAtK = computeRecall(ids, qa.Evidence, 0)
	}
	return res
}

// computeRecall returns (recall, hit) for a retrieved ID list against
// an evidence set. If k > 0 the retrieved list is truncated to the
// first k items before scoring. Case-insensitive matching.
func computeRecall(retrieved, evidence []string, k int) (float64, float64) {
	if len(evidence) == 0 {
		return 0, 0
	}
	if k > 0 && len(retrieved) > k {
		retrieved = retrieved[:k]
	}
	seen := make(map[string]struct{}, len(retrieved))
	for _, id := range retrieved {
		seen[strings.ToLower(strings.TrimSpace(id))] = struct{}{}
	}
	hit := 0
	for _, e := range evidence {
		if _, ok := seen[strings.ToLower(strings.TrimSpace(e))]; ok {
			hit++
		}
	}
	recall := float64(hit) / float64(len(evidence))
	hitAt := 0.0
	if hit > 0 {
		hitAt = 1.0
	}
	return recall, hitAt
}
