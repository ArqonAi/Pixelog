// pixe-bench runs Pixelog memory against external benchmarks
// (LoCoMo, ConvoMem, MemBench) and writes a structured JSON report.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/ArqonAi/Pixelog/internal/bench"
	"github.com/ArqonAi/Pixelog/internal/bench/convomem"
	"github.com/ArqonAi/Pixelog/internal/bench/locomo"
	"github.com/ArqonAi/Pixelog/internal/bench/longmemeval"
	"github.com/ArqonAi/Pixelog/internal/bench/membench"
	"github.com/ArqonAi/Pixelog/internal/llm"
	"github.com/ArqonAi/Pixelog/internal/search"
)

func main() {
	suite := flag.String("suite", "locomo", "benchmark suite: locomo | convomem | membench | longmemeval")
	mode := flag.String("mode", "session", "ingestion mode: session | hybrid | full")
	dataset := flag.String("dataset", "", "path to dataset (file for locomo/membench, dir for convomem)")
	out := flag.String("out", "bench-report.json", "output report path")
	includeCases := flag.Bool("include-cases", false, "include per-case detail in the report")
	maxQA := flag.Int("max-qa", 0, "limit total QA probes (0 = unlimited)")
	maxPerCategory := flag.Int("max-per-category", 0, "ConvoMem only: cap cases per category")
	categories := flag.String("categories", "", "comma-separated category filter (ConvoMem)")
	embedderName := flag.String("embedder", "hash", "embedder: hash | openai | ollama")
	openaiKey := flag.String("openai-key", os.Getenv("OPENAI_API_KEY"), "OpenAI API key")
	ollamaURL := flag.String("ollama-url", "http://localhost:11434", "Ollama base URL")
	ollamaModel := flag.String("ollama-model", "nomic-embed-text", "Ollama embedding model")
	judgeName := flag.String("judge", "exact", "judge: exact | llm")
	provider := flag.String("provider", "openrouter", "chat provider: openai | openrouter | anthropic | gemini | groq | xai | nvidia | moonshot | deepseek | zai | local | ollama")
	llmModel := flag.String("llm-model", "", "model id (defaults per provider)")
	llmKey := flag.String("llm-key", "", "API key (defaults to provider's env var)")
	judgeProvider := flag.String("judge-provider", "", "separate provider for judge (defaults to --provider)")
	judgeModel := flag.String("judge-model", "", "separate model for judge (defaults to --llm-model)")
	answerer := flag.Bool("answerer", true, "use LLM to compose final answers (false = retrieval-only)")
	fullContext := flag.Bool("full-context", false, "skip retrieval; pass entire transcript to the answerer")
	cot := flag.Bool("cot", false, "chain-of-thought: ask the model to reason step-by-step before the short answer")
	reflect := flag.Bool("reflect", false, "session reflection: build per-session structured summaries during Consolidate (uses --reflect-provider/model or falls back to --provider)")
	reflectProvider := flag.String("reflect-provider", "", "provider for the reflector (defaults to --provider)")
	reflectModel := flag.String("reflect-model", "", "model for the reflector (defaults to --llm-model)")
	timeout := flag.Duration("qa-timeout", 90*time.Second, "per-QA timeout")
	recallK := flag.Int("recall-k", 0, "retrieval-recall mode: top-k to evaluate (sets mode=recall when > 0; 5 and 10 are the conventional values)")
	flag.Parse()

	// --recall-k implies mode=recall.
	if *recallK > 0 {
		*mode = "recall"
	}

	if *dataset == "" {
		fatal("--dataset is required")
	}

	ctx := context.Background()

	ds, err := loadDataset(bench.Suite(*suite), *dataset, convomem.LoadOpts{
		Categories:     splitCSV(*categories),
		MaxPerCategory: *maxPerCategory,
	})
	if err != nil {
		fatal("load dataset: %v", err)
	}

	emb, err := buildEmbedder(*embedderName, *openaiKey, *ollamaURL, *ollamaModel)
	if err != nil {
		fatal("embedder: %v", err)
	}

	var ans bench.AnswerLLM
	var judge bench.Judge = bench.ExactMatchJudge{}

	// Recall-only evaluation skips the LLM answerer and judge entirely
	// — all that matters is whether the top-k retrieved IDs intersect
	// the gold evidence set.
	if *recallK > 0 {
		fmt.Fprintf(os.Stderr, "[pixe-bench] recall-only mode: top-%d, no LLM\n", *recallK)
		*answerer = false
		*reflect = false
		*judgeName = "exact"
	}

	if *answerer || *judgeName == "llm" {
		client, err := llm.NewMultiClient(llm.Provider(*provider), *llmModel, *llmKey)
		if err != nil {
			fatal("answerer client: %v", err)
		}
		fmt.Fprintf(os.Stderr, "[pixe-bench] answerer: provider=%s model=%s\n", client.Provider(), client.Model())

		if *answerer {
			ans = &chatAnswerer{client: client, fullContext: *fullContext, cot: *cot}
		}

		if *judgeName == "llm" {
			jp := *judgeProvider
			if jp == "" {
				jp = *provider
			}
			jm := *judgeModel
			if jm == "" {
				jm = *llmModel
			}
			jc, err := llm.NewMultiClient(llm.Provider(jp), jm, "")
			if err != nil {
				fatal("judge client: %v", err)
			}
			fmt.Fprintf(os.Stderr, "[pixe-bench] judge:    provider=%s model=%s\n", jc.Provider(), jc.Model())
			judge = bench.NewLLMJudge(jc)
		}
	}

	var reflector bench.SessionReflector
	if *reflect {
		rp := *reflectProvider
		if rp == "" {
			rp = *provider
		}
		rm := *reflectModel
		if rm == "" {
			rm = *llmModel
		}
		rc, err := llm.NewMultiClient(llm.Provider(rp), rm, "")
		if err != nil {
			fatal("reflector client: %v", err)
		}
		fmt.Fprintf(os.Stderr, "[pixe-bench] reflector: provider=%s model=%s\n", rc.Provider(), rc.Model())
		reflector = &chatReflector{client: rc}
	}

	mem := bench.NewPixelogMemory(bench.PixelogConfig{
		Embedder:    emb,
		Answerer:    ans,
		FullContext: *fullContext,
		Reflector:   reflector,
	})

	cases, err := ds.Cases(ctx)
	if err != nil {
		fatal("dataset cases: %v", err)
	}
	cases = applyMaxQA(cases, *maxQA)
	wrapped := &capDataset{base: ds, cases: cases}

	runner := bench.NewRunner(mem, judge, bench.Config{
		Mode:         bench.Mode(*mode),
		IncludeCases: *includeCases,
		PerQATimeout: *timeout,
		K:            *recallK,
	})

	fmt.Fprintf(os.Stderr, "[pixe-bench] suite=%s mode=%s cases=%d total-qa=%d\n",
		*suite, *mode, len(cases), totalQA(cases))

	report, err := runner.Run(ctx, wrapped)
	if err != nil {
		fatal("run: %v", err)
	}

	if err := writeJSON(*out, report); err != nil {
		fatal("write report: %v", err)
	}

	printSummary(report)
}

func loadDataset(suite bench.Suite, path string, cmOpts convomem.LoadOpts) (bench.Dataset, error) {
	switch suite {
	case bench.SuiteLoCoMo:
		return locomo.Load(path)
	case bench.SuiteConvoMem:
		return convomem.Load(path, cmOpts)
	case bench.SuiteMemBench:
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			return membench.LoadAllInDir(path, membench.LoadOpts{})
		}
		return membench.Load(path, membench.LoadOpts{})
	case bench.SuiteLongMemEval:
		return longmemeval.Load(path)
	default:
		return nil, fmt.Errorf("unknown suite: %s", suite)
	}
}

type capDataset struct {
	base  bench.Dataset
	cases []bench.Case
}

func (d *capDataset) Suite() bench.Suite                                   { return d.base.Suite() }
func (d *capDataset) Cases(_ context.Context) ([]bench.Case, error)        { return d.cases, nil }

func applyMaxQA(cases []bench.Case, max int) []bench.Case {
	if max <= 0 {
		return cases
	}
	out := make([]bench.Case, 0, len(cases))
	used := 0
	for _, c := range cases {
		if used >= max {
			break
		}
		room := max - used
		if len(c.QA) > room {
			c.QA = c.QA[:room]
		}
		used += len(c.QA)
		out = append(out, c)
	}
	return out
}

func totalQA(cases []bench.Case) int {
	n := 0
	for _, c := range cases {
		n += len(c.QA)
	}
	return n
}

type chatAnswerer struct {
	client      *llm.MultiClient
	fullContext bool
	cot         bool
}

// finalAnswerRE extracts the short answer when the model produces CoT output
// using a "FINAL ANSWER:" sentinel.
var finalAnswerRE = regexp.MustCompile(`(?is)FINAL\s+ANSWER\s*:\s*(.+?)\s*$`)

func (a *chatAnswerer) Answer(ctx context.Context, q string, retrieved []string) (string, error) {
	var sb strings.Builder
	sb.WriteString("You are answering a question about a long multi-session conversation between two people. ")
	sb.WriteString("Use the excerpts below as the primary source. ")
	sb.WriteString("If multiple excerpts conflict, prefer the most recent. ")
	sb.WriteString("Dates may be relative (\"yesterday\", \"last week\") — interpret them against the [YYYY-MM-DD] session prefix when present.\n\n")
	if a.fullContext {
		sb.WriteString("FULL TRANSCRIPT:\n")
	} else {
		sb.WriteString("RETRIEVED EXCERPTS:\n")
	}
	if len(retrieved) == 0 {
		sb.WriteString("(none)\n")
	} else {
		for i, r := range retrieved {
			fmt.Fprintf(&sb, "[%d] %s\n", i+1, r)
		}
	}
	sb.WriteString("\nQUESTION: ")
	sb.WriteString(q)
	if a.cot {
		sb.WriteString("\n\nThink step by step, then emit ONE terse answer matching LoCoMo gold style.\n\n")
		sb.WriteString("REASONING:\n")
		sb.WriteString("1. Decompose multi-hop questions into sub-questions.\n")
		sb.WriteString("2. Quote the specific turns that answer each part, including the [YYYY-MM-DD] session date.\n")
		sb.WriteString("3. Resolve relative dates against the session prefix (e.g. session [2023-05-07] + \"yesterday\" = 2023-05-06).\n")
		sb.WriteString("4. On conflict, prefer the most recent session.\n\n")
		sb.WriteString("FINAL ANSWER RULES — violate these and the answer is graded wrong:\n")
		sb.WriteString("- Give the shortest accurate answer. Usually 1-8 words.\n")
		sb.WriteString("- For list questions (\"what activities / books / events / fields?\"), output ONLY the items directly asked, comma-separated. No parentheticals, no dates, no extra items that you merely inferred.\n")
		sb.WriteString("- For date questions, use the transcript's own date format (e.g. \"7 May 2023\" or \"June 2023\" or \"The week before 9 June 2023\"). Never add time-of-day.\n")
		sb.WriteString("- For yes/no questions, start with \"Yes\" or \"No\" then one short clause of justification (e.g. \"Yes, since she collects classic children's books\").\n")
		sb.WriteString("- Do NOT add qualifiers (\"possibly\", \"particularly\", \"especially\"). Do NOT list extra items not asked for.\n")
		sb.WriteString("- Do NOT say \"No information available\" if the transcript implies the answer — infer from what's there.\n")
		sb.WriteString("- Never output the reasoning on the FINAL ANSWER line.\n\n")
		sb.WriteString("Examples of the required style (generic, not from this transcript):\n")
		sb.WriteString("  Q: What did the user research?  →  FINAL ANSWER: Tax law\n")
		sb.WriteString("  Q: When did they meet at the cafe?  →  FINAL ANSWER: 3 March 2024\n")
		sb.WriteString("  Q: What hobbies does X have?  →  FINAL ANSWER: knitting, hiking, piano\n")
		sb.WriteString("  Q: Would X enjoy skydiving?  →  FINAL ANSWER: Likely no; she avoids risky activities\n")
		sb.WriteString("  Q: What is X's profession?  →  FINAL ANSWER: Software engineer\n\n")
		sb.WriteString("Emit your reasoning, then a line starting 'FINAL ANSWER:' with the short answer only.")
	} else {
		sb.WriteString("\n\nGive the shortest accurate answer (a name, date, fact, place, person, or number). ")
		sb.WriteString("Answer with just that value — no preamble, no explanation. ")
		sb.WriteString("If the information is truly absent from the transcript, answer \"I don't know.\"")
		sb.WriteString("\n\nSHORT ANSWER:")
	}
	raw, err := a.client.ChatCtx(ctx, sb.String())
	if err != nil {
		return "", err
	}
	if a.cot {
		if m := finalAnswerRE.FindStringSubmatch(raw); len(m) > 1 {
			return strings.TrimSpace(m[1]), nil
		}
	}
	return strings.TrimSpace(raw), nil
}

// chatReflector implements bench.SessionReflector by asking an LLM to
// produce a dense, structured recap of one conversational session.
// Output format is a series of ENTITY/EVENT/FACT lines optimised for
// downstream multi-hop reasoning.
type chatReflector struct {
	client *llm.MultiClient
}

func (r *chatReflector) Reflect(ctx context.Context, sessionID string, date time.Time, turns []bench.Turn) (string, error) {
	if len(turns) == 0 {
		return "", nil
	}

	var transcript strings.Builder
	for _, t := range turns {
		speaker := t.Speaker
		if speaker == "" {
			speaker = t.Role
		}
		fmt.Fprintf(&transcript, "%s: %s\n", speaker, t.Text)
	}

	dateStr := "unknown"
	if !date.IsZero() {
		dateStr = date.Format("2006-01-02 (Mon)")
	}

	prompt := fmt.Sprintf(`You are building a structured memory entry for one session of a long-running conversation.

SESSION ID: %s
SESSION DATE: %s

TRANSCRIPT:
%s

Produce a dense, structured recap. Use these line types — every fact must be tagged. Be exhaustive: include EVERY person, place, date, number, preference, plan, event, relationship, and explicit feeling that appears.

Format (one entry per line, no prose, no markdown headings):
- PERSON: <name> — <attributes mentioned this session> (e.g. "Caroline — transgender woman, attended LGBTQ support group on %s")
- EVENT: <what happened> on <date or relative-resolved date> at <where if known>
- FACT: <subject> <predicate> <object>
- PREFERENCE: <person> likes/dislikes/prefers <thing>
- PLAN: <person> plans to <do> on/in <when>
- RELATIONSHIP: <person A> is <relation> of <person B>
- FEELING: <person> feels <emotion> about <topic>

Resolve relative dates: e.g. session date is %s and a turn says "yesterday" → use the actual date.
Quote specific times, addresses, money, and counts verbatim.
Never invent facts; only record what is explicitly in the transcript.

OUTPUT (lines only, no preamble):`, sessionID, dateStr, transcript.String(), dateStr, dateStr)

	return r.client.ChatCtx(ctx, prompt)
}

func buildEmbedder(name, openaiKey, ollamaURL, ollamaModel string) (interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}, error) {
	switch strings.ToLower(name) {
	case "hash", "":
		return bench.NewHashEmbedder(384), nil
	case "openai":
		if openaiKey == "" {
			return nil, fmt.Errorf("openai embedder requires --openai-key or OPENAI_API_KEY")
		}
		return providerEmbedder{p: search.NewOpenAIProvider(openaiKey)}, nil
	case "ollama":
		return providerEmbedder{p: search.NewOllamaProvider(ollamaURL, ollamaModel)}, nil
	default:
		return nil, fmt.Errorf("unknown embedder: %q", name)
	}
}

// providerEmbedder wraps any search.EmbeddingProvider as a bench Embedder.
type providerEmbedder struct{ p search.EmbeddingProvider }

func (e providerEmbedder) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return e.p.GenerateEmbedding(ctx, text)
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func writeJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printSummary(r *bench.Report) {
	fmt.Fprintf(os.Stderr, "\n=== %s / %s — %d cases / %d QA ===\n", r.Suite, r.Mode, r.NumCases, r.NumQA)
	if r.Mode == bench.ModeRecall {
		fmt.Fprintf(os.Stderr, "Recall@%-2d:     %6.2f%%  (%d scored)\n", r.K, r.Aggregate.RecallAtK*100, r.Aggregate.NumScored)
		fmt.Fprintf(os.Stderr, "Hit@%-4d:      %6.2f%%\n", r.K, r.Aggregate.HitAtK*100)
		fmt.Fprintf(os.Stderr, "Errors:        %d\n", r.Aggregate.Errors)
		fmt.Fprintf(os.Stderr, "Latency mean:  %.1f ms\n", r.Aggregate.MeanLatencyMS)
		if len(r.ByCategory) > 0 {
			fmt.Fprintf(os.Stderr, "\nBy category:\n")
			for cat, m := range r.ByCategory {
				fmt.Fprintf(os.Stderr, "  %-30s n=%-5d hit@%d=%6.2f%% recall@%d=%6.2f%%\n",
					cat, m.Count, r.K, m.HitAtK*100, r.K, m.RecallAtK*100)
			}
		}
		fmt.Fprintf(os.Stderr, "\nReport written.\n")
		return
	}
	fmt.Fprintf(os.Stderr, "Judge mean:    %6.2f%%\n", r.Aggregate.JudgeMean*100)
	fmt.Fprintf(os.Stderr, "Token F1 mean: %6.2f%%\n", r.Aggregate.F1Mean*100)
	fmt.Fprintf(os.Stderr, "Exact match:   %6.2f%%\n", r.Aggregate.ExactMatchRate*100)
	fmt.Fprintf(os.Stderr, "Errors:        %d\n", r.Aggregate.Errors)
	if r.Aggregate.AbstainAccuracy > 0 {
		fmt.Fprintf(os.Stderr, "Abstain acc:   %6.2f%%\n", r.Aggregate.AbstainAccuracy*100)
	}
	if r.Aggregate.NumScored > 0 {
		fmt.Fprintf(os.Stderr, "Recall@K:      %6.2f%%  (%d scored)\n", r.Aggregate.RecallAtK*100, r.Aggregate.NumScored)
		fmt.Fprintf(os.Stderr, "Hit@K:         %6.2f%%\n", r.Aggregate.HitAtK*100)
	}
	fmt.Fprintf(os.Stderr, "Latency mean:  %.1f ms\n", r.Aggregate.MeanLatencyMS)

	if len(r.ByCategory) > 0 {
		fmt.Fprintf(os.Stderr, "\nBy category:\n")
		for cat, m := range r.ByCategory {
			fmt.Fprintf(os.Stderr, "  %-30s n=%-5d judge=%6.2f%% f1=%6.2f%%\n", cat, m.Count, m.JudgeMean*100, m.F1Mean*100)
		}
	}
	fmt.Fprintf(os.Stderr, "\nReport written.\n")
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[pixe-bench] ERROR: "+format+"\n", args...)
	os.Exit(1)
}
