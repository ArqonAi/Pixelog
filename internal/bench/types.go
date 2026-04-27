// Package bench provides a unified harness for evaluating Pixelog memory
// systems against external conversational-memory benchmarks (LoCoMo,
// ConvoMem, MemBench, etc.).
//
// The package is deliberately decoupled from any specific dataset: each
// benchmark adapter produces a stream of Case values which the Runner
// feeds into a MemorySystem and scores via a Judge.
package bench

import (
	"context"
	"time"
)

// Suite identifies a benchmark family.
type Suite string

const (
	SuiteLoCoMo       Suite = "locomo"
	SuiteConvoMem     Suite = "convomem"
	SuiteMemBench     Suite = "membench"
	SuiteLongMemEval  Suite = "longmemeval"
)

// Mode controls how conversation history is staged into the memory
// system before queries are answered.
type Mode string

const (
	// ModeSession ingests messages session-by-session, calling Consolidate
	// between sessions. Models the "live" use case.
	ModeSession Mode = "session"

	// ModeHybrid mixes session ingestion with periodic full-context
	// retrieval, as in LoCoMo's RAG-on-summaries baseline.
	ModeHybrid Mode = "hybrid"

	// ModeFull dumps the full transcript into memory in a single batch.
	// Useful for benchmarks like MemBench which provide pre-built flows.
	ModeFull Mode = "full"

	// ModeRecall runs retrieval-only evaluation: ingest all turns, then
	// for each QA call Retrieve(k) and compute Recall@k against
	// qa.Evidence. No LLM answerer or judge is invoked. R@5 / R@10
	// are the canonical k values for this mode.
	ModeRecall Mode = "recall"
)

// Turn is a single utterance in a benchmark conversation.
type Turn struct {
	Speaker   string    `json:"speaker"`
	Role      string    `json:"role"` // "user" | "assistant" | "system"
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	TurnID    string    `json:"turn_id,omitempty"`
}

// QA is a question-answer probe attached to a conversation.
type QA struct {
	ID         string   `json:"id"`
	Question   string   `json:"question"`
	GoldAnswer string   `json:"gold_answer"`
	Category   string   `json:"category,omitempty"`
	Evidence   []string `json:"evidence,omitempty"`   // turn IDs supporting the answer
	Abstain    bool     `json:"abstain,omitempty"`    // expected "I don't know"
	Rubric     string   `json:"rubric,omitempty"`     // for preference-style scoring
	EvidenceN  int      `json:"evidence_n,omitempty"` // ConvoMem evidence-count bucket
}

// Case is a single benchmark item: one conversation + its QA probes.
type Case struct {
	Suite        Suite  `json:"suite"`
	ID           string `json:"id"`
	Namespace    string `json:"namespace"`
	Turns        []Turn `json:"turns"`
	QA           []QA   `json:"qa"`
	SourceFile   string `json:"source_file,omitempty"`
	TokenEstimate int   `json:"token_estimate,omitempty"`
}

// Dataset is a finite stream of benchmark cases.
type Dataset interface {
	Suite() Suite
	Cases(ctx context.Context) ([]Case, error)
}

// MemorySystem is the surface a memory implementation must implement
// to be benchmarked. Pixelog's own implementation lives in adapters.go.
type MemorySystem interface {
	// Reset wipes all state for the given namespace.
	Reset(ctx context.Context, namespace string) error

	// Ingest stores a batch of turns under the namespace. The harness calls
	// this once per session in ModeSession, once total in ModeFull.
	Ingest(ctx context.Context, namespace string, turns []Turn) error

	// Consolidate signals that the current session has ended and the
	// memory system should perform any compression / archival work.
	// Called between sessions in ModeSession and ModeHybrid.
	Consolidate(ctx context.Context, namespace string) error

	// Answer the question using whatever retrieval the memory system
	// chooses, returning the predicted answer and supporting context.
	Answer(ctx context.Context, namespace string, question string) (Answer, error)
}

// Retriever is an optional companion interface. Systems that implement
// it can be evaluated in ModeRecall without invoking an LLM answerer.
// Returned IDs should include both the turn ID and the owning session
// ID (or equivalent evidence granularity) so a dataset that anchors
// evidence at either level will match.
type Retriever interface {
	Retrieve(ctx context.Context, namespace string, question string, k int) ([]string, error)
}

// Answer is the predicted response with retrieval metadata.
type Answer struct {
	Text         string        `json:"text"`
	Context      []string      `json:"context,omitempty"`
	Latency      time.Duration `json:"latency"`
	TokensIn     int           `json:"tokens_in,omitempty"`
	TokensOut    int           `json:"tokens_out,omitempty"`
	RetrievedIDs []string      `json:"retrieved_ids,omitempty"`
}

// QAResult captures the scoring of a single QA probe.
type QAResult struct {
	QA           QA            `json:"qa"`
	Predicted    string        `json:"predicted"`
	JudgeScore   float64       `json:"judge_score"`   // 0-1 from LLM judge
	ExactMatch   bool          `json:"exact_match"`
	F1           float64       `json:"f1"`
	AbstainOK    bool          `json:"abstain_ok"`    // for abstention category
	JudgeReason  string        `json:"judge_reason,omitempty"`
	Latency      time.Duration `json:"latency"`
	RetrievedIDs []string      `json:"retrieved_ids,omitempty"`
	// RecallAtK reports |retrieved_ids[:K] ∩ evidence| / |evidence|.
	// HitAtK is 1.0 if the intersection is non-empty, 0.0 otherwise.
	// K is carried separately in the enclosing Report.
	RecallAtK float64 `json:"recall_at_k,omitempty"`
	HitAtK    float64 `json:"hit_at_k,omitempty"`
	Error     string  `json:"error,omitempty"`
}

// CaseResult is the aggregated score for one case.
type CaseResult struct {
	Case      Case       `json:"case"`
	QAResults []QAResult `json:"qa_results"`
	Duration  time.Duration `json:"duration"`
	Error     string     `json:"error,omitempty"`
}

// Report is the full benchmark output across all cases.
type Report struct {
	Suite      Suite              `json:"suite"`
	Mode       Mode               `json:"mode"`
	K          int                `json:"k,omitempty"` // top-k used for recall eval
	StartedAt  time.Time          `json:"started_at"`
	FinishedAt time.Time          `json:"finished_at"`
	NumCases   int                `json:"num_cases"`
	NumQA      int                `json:"num_qa"`
	Aggregate  Metrics            `json:"aggregate"`
	ByCategory map[string]Metrics `json:"by_category"`
	Cases      []CaseResult       `json:"cases,omitempty"`
}

// Metrics holds aggregate scores for a slice of QA results.
type Metrics struct {
	Count           int     `json:"count"`
	JudgeMean       float64 `json:"judge_mean"`
	ExactMatchRate  float64 `json:"exact_match_rate"`
	F1Mean          float64 `json:"f1_mean"`
	AbstainAccuracy float64 `json:"abstain_accuracy,omitempty"` // 0 if no abstain cases
	MeanLatencyMS   float64 `json:"mean_latency_ms"`
	Errors          int     `json:"errors"`
	// Retrieval-recall metrics (populated when evidence is available).
	RecallAtK float64 `json:"recall_at_k,omitempty"`
	HitAtK    float64 `json:"hit_at_k,omitempty"`
	NumScored int     `json:"num_scored_recall,omitempty"` // count with non-empty evidence
}
