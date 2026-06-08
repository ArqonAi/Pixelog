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
	openaiEmbedModel := flag.String("openai-embed-model", "text-embedding-3-small", "OpenAI embedding model: text-embedding-3-small | text-embedding-3-large")
	ollamaURL := flag.String("ollama-url", "http://localhost:11434", "Ollama base URL")
	ollamaModel := flag.String("ollama-model", "nomic-embed-text", "Ollama embedding model")
	judgeName := flag.String("judge", "exact", "judge: exact | llm | mem0 (mem0 = verbatim arXiv:2504.19413 binary CORRECT/WRONG prompt)")
	mem0Prompts := flag.Bool("mem0-prompts", false, "use the verbatim Mem0 ANSWER_PROMPT (arXiv:2504.19413) for the answerer; pairs with --judge=mem0 for full parity")
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
	retrievalK := flag.Int("retrieval-k", 30, "QA-mode: top-k turns fed to the answerer as context (Mem0 paper default: 30)")
	llmFacts := flag.Bool("llm-facts", false, "ingest-time LLM fact extraction: runs the answerer's provider/model over every turn to produce a high-recall fact micro-graph (in addition to the rule-based one). Adds per-turn API cost; use only when measuring cat-3 / counterfactual lift.")
	llmFactsModel := flag.String("llm-facts-model", "", "override model id for fact extraction (defaults to --llm-model). Use a cheap fast model — fact extraction is volume-heavy.")
	factEvidence := flag.Bool("fact-evidence", false, "surface matched fact triples as a structured KNOWN FACTS preamble in the answerer context (mirrors atomic-fact distillation à la Mem0). Closes the answerer-extraction gap on direct-lookup and counterfactual-inference questions.")
	entityProfiles := flag.Bool("entity-profiles", false, "Lever-2 symbolic recursion: surface a per-entity time-anchored chronology (KNOWN TIMELINES) as a preamble for questions whose subject has a profile. Converts temporal-diff and counterfactual reasoning from multi-hop turn-walking into a direct lookup over a dated projection.")
	decompose := flag.Bool("decompose", false, "recursive question decomposition: ask the LLM to split compositional/temporal/multi-hop questions into atomic sub-questions, retrieve evidence for each, and merge before the final answer.")
	decomposeProvider := flag.String("decompose-provider", "", "provider for the decomposer (defaults to --provider)")
	decomposeModel := flag.String("decompose-model", "", "model id for the decomposer (defaults to --llm-model)")
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

	emb, err := buildEmbedder(*embedderName, *openaiKey, *openaiEmbedModel, *ollamaURL, *ollamaModel)
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

	useLLMJudge := *judgeName == "llm" || *judgeName == "mem0"
	if *answerer || useLLMJudge {
		client, err := llm.NewMultiClient(llm.Provider(*provider), *llmModel, *llmKey)
		if err != nil {
			fatal("answerer client: %v", err)
		}
		fmt.Fprintf(os.Stderr, "[pixe-bench] answerer: provider=%s model=%s mem0-prompts=%v\n", client.Provider(), client.Model(), *mem0Prompts)

		if *answerer {
			if *mem0Prompts {
				ans = &mem0Answerer{client: client, fullContext: *fullContext}
			} else {
				ans = &chatAnswerer{client: client, fullContext: *fullContext, cot: *cot}
			}
		}

		if useLLMJudge {
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
			fmt.Fprintf(os.Stderr, "[pixe-bench] judge:    provider=%s model=%s kind=%s\n", jc.Provider(), jc.Model(), *judgeName)
			if *judgeName == "mem0" {
				judge = bench.NewMem0Judge(jc)
			} else {
				judge = bench.NewLLMJudge(jc)
			}
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

	// Optional LLM fact extractor. Built behind --llm-facts so the
	// default path stays zero-API-cost. We share the answerer's
	// provider/key by default but allow a separate model override
	// via --llm-facts-model — fact extraction is volume-heavy and
	// usually wants a cheap fast model (gpt-4o-mini, sonnet-haiku).
	var factExtractor bench.FactExtractor
	if *llmFacts {
		fm := *llmFactsModel
		if fm == "" {
			fm = *llmModel
		}
		fc, ferr := llm.NewMultiClient(llm.Provider(*provider), fm, *llmKey)
		if ferr != nil {
			fatal("llm-facts client: %v", ferr)
		}
		fmt.Fprintf(os.Stderr, "[pixe-bench] llm-facts: provider=%s model=%s\n", fc.Provider(), fc.Model())
		factExtractor = bench.NewLLMFactExtractor(&multiClientFactCompletion{client: fc})
	}

	// Optional recursive question decomposer. Runs at QA time, not
	// ingest time — one short LLM call per question to split
	// compositional/temporal asks into atomic sub-questions. Each
	// sub-question gets its own retrieval pass; the union of evidence
	// is merged into the answerer context.
	var decomposer bench.QuestionDecomposer
	if *decompose {
		dp := *decomposeProvider
		if dp == "" {
			dp = *provider
		}
		dm := *decomposeModel
		if dm == "" {
			dm = *llmModel
		}
		dc, err := llm.NewMultiClient(llm.Provider(dp), dm, "")
		if err != nil {
			fatal("decomposer client: %v", err)
		}
		fmt.Fprintf(os.Stderr, "[pixe-bench] decomposer: provider=%s model=%s\n", dc.Provider(), dc.Model())
		decomposer = &chatDecomposer{client: dc}
	}

	mem := bench.NewPixelogMemory(bench.PixelogConfig{
		Embedder:      emb,
		Answerer:      ans,
		FullContext:   *fullContext,
		Reflector:     reflector,
		RetrievalK:    *retrievalK,
		FactExtractor:   factExtractor,
		FactEvidence:    *factEvidence,
		ProfilePreamble: *entityProfiles,
		Decomposer:      decomposer,
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

// multiClientFactCompletion adapts a llm.MultiClient (which exposes
// only a single-prompt ChatCtx call) to bench.FactCompletion (which
// takes separate system + user messages). The adapter prepends the
// system prompt to the user prompt with a clear delimiter, since the
// underlying providers' chat APIs reached via MultiClient.ChatCtx
// don't currently expose a typed system role.
//
// Concatenation is safe here because the system prompt is fixed-text
// instruction (see facts_llm.go llmFactSystemPrompt) and the user
// payload is wrapped in <<< / >>> delimiters that the system message
// instructs the model to treat as data — so prompt-injection from
// turn text can't escape into instruction territory.
type multiClientFactCompletion struct {
	client *llm.MultiClient
}

// Complete implements bench.FactCompletion. The system + user message
// are folded into a single prompt with a SYSTEM: prefix; production
// providers tolerate this format identically to a typed-role payload
// for the JSON-emission task we're driving here.
func (a *multiClientFactCompletion) Complete(ctx context.Context, system, user string) (string, error) {
	var sb strings.Builder
	sb.WriteString("SYSTEM:\n")
	sb.WriteString(system)
	sb.WriteString("\n\nUSER:\n")
	sb.WriteString(user)
	return a.client.ChatCtx(ctx, sb.String())
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

// mem0Answerer ports the verbatim ANSWER_PROMPT from
// mem0/evaluation/prompts.py (arXiv:2504.19413, MIT-licensed) — the
// "Zep" single-block memories variant, which matches our flat
// retrieved-context shape (turns are already prefixed
// "[YYYY-MM-DD] speaker: text" by the hybrid retriever).
//
// Using the published prompt verbatim means head-to-head LoCoMo
// numbers can't be disputed on prompt grounds.
type mem0Answerer struct {
	client      *llm.MultiClient
	fullContext bool
}

func (a *mem0Answerer) Answer(ctx context.Context, q string, retrieved []string) (string, error) {
	var memories strings.Builder
	if len(retrieved) == 0 {
		memories.WriteString("(no memories retrieved)")
	} else {
		for _, r := range retrieved {
			memories.WriteString(r)
			memories.WriteString("\n")
		}
	}

	// Verbatim port of ANSWER_PROMPT_ZEP from
	// github.com/mem0ai/mem0/blob/main/evaluation/prompts.py
	prompt := `
    You are an intelligent memory assistant tasked with retrieving accurate information from conversation memories.

    # CONTEXT:
    You have access to memories from a conversation. These memories contain
    timestamped information that may be relevant to answering the question.

    # INSTRUCTIONS:
    1. Carefully analyze all provided memories
    2. Pay special attention to the timestamps to determine the answer
    3. If the question asks about a specific event or fact, look for direct evidence in the memories
    4. If the memories contain contradictory information, prioritize the most recent memory
    5. If there is a question about time references (like "last year", "two months ago", etc.), 
       calculate the actual date based on the memory timestamp. For example, if a memory from 
       4 May 2022 mentions "went to India last year," then the trip occurred in 2021.
    6. Always convert relative time references to specific dates, months, or years. For example, 
       convert "last year" to "2022" or "two months ago" to "March 2023" based on the memory 
       timestamp. Ignore the reference while answering the question.
    7. Focus only on the content of the memories. Do not confuse character 
       names mentioned in memories with the actual users who created those memories.
    8. The answer should be less than 5-6 words.

    # APPROACH (Think step by step):
    1. First, examine all memories that contain information related to the question
    2. Examine the timestamps and content of these memories carefully
    3. Look for explicit mentions of dates, times, locations, or events that answer the question
    4. If the answer requires calculation (e.g., converting relative time references), show your work
    5. Formulate a precise, concise answer based solely on the evidence in the memories
    6. Double-check that your answer directly addresses the question asked
    7. Ensure your final answer is specific and avoids vague time references

    Memories:

` + memories.String() + `

    Question: ` + q + `
    Answer:
    `

	raw, err := a.client.ChatCtx(ctx, prompt)
	if err != nil {
		return "", err
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

// chatDecomposer implements bench.QuestionDecomposer by asking an LLM
// to split a compositional / temporal / multi-hop question into a
// short list of atomic sub-questions. Each sub-question must be
// independently answerable from the conversation history. Output is
// one sub-question per line; an empty / single-line output signals
// the question is already atomic and no decomposition is needed.
type chatDecomposer struct {
	client *llm.MultiClient
}

func (d *chatDecomposer) Decompose(ctx context.Context, question string) ([]string, error) {
	q := strings.TrimSpace(question)
	if q == "" {
		return nil, nil
	}

	prompt := fmt.Sprintf(`You are a query planner for a long-conversation memory system.

QUESTION:
%s

Decide whether this question requires looking up multiple independent facts (compositional, temporal-comparison, multi-hop, counterfactual). If so, break it into 2–4 ATOMIC sub-questions that can each be answered from a single piece of evidence in the conversation. If the question is already atomic, output exactly the single line: ATOMIC

Rules:
- Each sub-question must be self-contained (no pronouns, no "the previous one").
- Preserve named entities and explicit time references verbatim.
- Do NOT invent facts or constraints not in the original question.
- For temporal-difference questions ("how long ago", "before/after"), produce one sub-question per anchor date/event.
- For counterfactuals ("what would X have done if Y had not happened"), produce one sub-question for the actual event and one for the alternative.
- Output sub-questions only — one per line, no numbering, no bullets, no commentary.

OUTPUT:`, q)

	raw, err := d.client.ChatCtx(ctx, prompt)
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "ATOMIC") {
		return nil, nil
	}

	lines := strings.Split(raw, "\n")
	subs := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, ln := range lines {
		s := strings.TrimSpace(ln)
		// Strip common list prefixes the model may emit despite instructions.
		s = strings.TrimLeft(s, "-*•\t ")
		// "1. ", "1) ", "Q1: " etc.
		for i, r := range s {
			if r >= '0' && r <= '9' {
				continue
			}
			if i > 0 && (r == '.' || r == ')' || r == ':') {
				s = strings.TrimSpace(s[i+1:])
			}
			break
		}
		if s == "" || strings.EqualFold(s, "ATOMIC") {
			continue
		}
		key := strings.ToLower(s)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		subs = append(subs, s)
		if len(subs) >= 4 {
			break
		}
	}
	// If the model only restated the original question, treat as atomic.
	if len(subs) == 1 && strings.EqualFold(subs[0], q) {
		return nil, nil
	}
	return subs, nil
}

func buildEmbedder(name, openaiKey, openaiModel, ollamaURL, ollamaModel string) (interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}, error) {
	switch strings.ToLower(name) {
	case "hash", "":
		return bench.NewHashEmbedder(384), nil
	case "openai":
		if openaiKey == "" {
			return nil, fmt.Errorf("openai embedder requires --openai-key or OPENAI_API_KEY")
		}
		fmt.Fprintf(os.Stderr, "[pixe-bench] embedder: openai model=%s\n", openaiModel)
		return providerEmbedder{p: search.NewOpenAIProviderWithModel(openaiKey, openaiModel)}, nil
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
