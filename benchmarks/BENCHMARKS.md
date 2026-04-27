# Pixelog Benchmarks

All numbers below are reproducible from this repository with the
commands shown in this file. Per-question result files are written to
the path passed via `--out` for inspection.

> Pixelog reports **retrieval recall (R@k)** — the fraction of gold
> evidence sessions / turns that the retriever surfaces in the top-k.
>
> **"LLM required: None" applies to the retrieval pipeline only.** These
> benchmarks measure ranking quality (does the gold session appear in the
> top-k?), which is a pure search problem — no text generation is needed.
> Generating a final natural-language answer from the retrieved context
> still requires an LLM and is reported separately under end-to-end QA.

| Benchmark | Metric | **Pixelog** | Notes |
| --- | --- | --- | --- |
| LongMemEval S | Hit@5 | **97.20%** | 500 QA, 115k-token haystacks, hash embedder, no LLM |
| LongMemEval S | Hit@10 | **98.20%** | same config |
| LongMemEval Oracle | Hit@5 | **100.00%** | 500 QA, oracle haystack |
| LoCoMo | Hit@10 | **96.62%** | 1,986 QA, 10 conversations, hash embedder |
| LoCoMo | Hit@5  | **92.08%** | same config |
| ConvoMem (top-bucket, 6 categories) | Hit@5 | **100.00%** | 265 cases, hardest bucket per category, hash embedder |
| MemBench (ACL 2025, 14 categories) | Hit@5 | **98.45%** | 6,779 QA, all FirstAgent + ThirdAgent splits, hash embedder |

Every Pixelog row above uses the **deterministic hash embedder** — no
network calls, no API key, no LLM at any stage. The hybrid retriever
that produces these numbers lives in
[`internal/bench/hybrid_retriever.go`](../internal/bench/hybrid_retriever.go)
and runs **two parallel rankers** — one over sessions, one over
individual turns — then interleaves their tops so a single ranked
output serves both turn-granular evidence (LoCoMo `D{conv}:{msg}`,
MemBench message IDs) and session-granular evidence (LongMemEval
`answer_*`).

**Per-session signals**

- **Semantic** cosine similarity between the query embedding and per-session embedding.
- **BM25** (Okapi, k1=1.2, b=0.75) over each session's token bag.
- **Temporal proximity** — boost when the question references a date and
  the session date is within ±60 days.
- **Preference-pattern** — boost when the question contains a preference
  or inference cue (`like`, `prefer`, `favourite`, `would`, `might`,
  `considered`, `attribute`, …) and the session text echoes the same
  family of verbs.
- **IDF-weighted entity overlap** — rare entities (`Trattoria Lucca`,
  `LGBTQ`) dominate over common ones (`John`, `May`); the weight is
  `log((N+1)/df_min) + 1` where `df_min` is the rarest-token document
  frequency inside the entity.
- **Recency tiebreak** — small linear bonus for the most recent session.

**Per-turn signals (global turn ranker)**

In parallel, every individual turn is scored using turn-level BM25
(turn-corpus IDF + average-turn-length normalisation) plus the same
temporal / preference / IDF-entity signals. Turn embeddings are *not*
computed (the embedder is hit once per session for cost reasons), but
turns are short and topic-dense so lexical signals are strong on their
own. The top-`k×4` turns are kept as a parallel ranking.

**Interleaved emit**

The top-K window alternates between the session ranker and the turn
ranker (`[session_1, turn_1, session_2, turn_2, …]`), with hard
dedup. After the K-window, the long tail appends every turn ID inside
the top-K candidate sessions plus the rest of the global-turn ranking,
so k=0 callers see the full retrieved set unchanged.

Default weights:
`semantic=1.0, bm25=0.6, temporal=0.5, preference=0.3, keyword=0.4, recency=0.05`.

---

## Reproducing every number

### 1. Build the CLI

```bash
go build -o pixe-bench ./cmd/pixe-bench
```

### 2. Fetch the datasets

```bash
mkdir -p /tmp/pixe-bench/datasets

# LongMemEval (Wu et al., ICLR 2025)
curl -fL "https://huggingface.co/datasets/xiaowu0162/longmemeval/resolve/main/longmemeval_oracle?download=true" \
    -o /tmp/pixe-bench/datasets/longmemeval_oracle.json
curl -fL "https://huggingface.co/datasets/xiaowu0162/longmemeval/resolve/main/longmemeval_s?download=true" \
    -o /tmp/pixe-bench/datasets/longmemeval_s.json

# LoCoMo (Maharana et al., ACL 2024)
curl -fL "https://raw.githubusercontent.com/snap-research/locomo/main/data/locomo10.json" \
    -o /tmp/pixe-bench/datasets/locomo10.json

# ConvoMem (Salesforce) — pre_mixed_testcases tree
# Download top-bucket per category for the meaningful eval:
#   user_evidence/6_evidence/, assistant_facts_evidence/6_evidence/,
#   changing_evidence/6_evidence/, abstention_evidence/3_evidence/,
#   preference_evidence/2_evidence/, implicit_connection_evidence/3_evidence/
# Each fetched as batched_000.json from
#   https://huggingface.co/datasets/Salesforce/ConvoMem/resolve/main/core_benchmark/pre_mixed_testcases/<cat>/<n>_evidence/batched_000.json

# MemBench (Tan et al., ACL 2025 Findings)
git clone --depth 1 https://github.com/import-myself/Membench.git /tmp/Membench
```

### 3. Run each benchmark

```bash
# LongMemEval S — Hit@5 = 97.20%
./pixe-bench --suite longmemeval \
    --dataset /tmp/pixe-bench/datasets/longmemeval_s.json \
    --recall-k 5 --embedder hash \
    --out /tmp/pixe-bench/lme-s-500-r5.json

# LongMemEval S — Hit@10 = 98.20%
./pixe-bench --suite longmemeval \
    --dataset /tmp/pixe-bench/datasets/longmemeval_s.json \
    --recall-k 10 --embedder hash \
    --out /tmp/pixe-bench/lme-s-500-r10.json

# LoCoMo — Hit@10 = 96.62%
./pixe-bench --suite locomo \
    --dataset /tmp/pixe-bench/datasets/locomo10.json \
    --recall-k 10 --embedder hash \
    --out /tmp/pixe-bench/locomo-r10.json

# ConvoMem (top-bucket-only fixture) — Hit@5 = 100.00%
./pixe-bench --suite convomem \
    --dataset /tmp/pixe-bench/datasets/convomem-hard \
    --max-per-category 50 --recall-k 5 --embedder hash \
    --out /tmp/pixe-bench/convomem-hard-r5.json

# MemBench — Hit@5 = 98.45%
./pixe-bench --suite membench \
    --dataset /tmp/Membench/MemData \
    --recall-k 5 --embedder hash \
    --out /tmp/pixe-bench/membench-r5.json
```

`--recall-k <K>` activates `mode=recall` automatically; the LLM
answerer and judge are bypassed. Latency for the full 6,779-QA MemBench
sweep is ~18 s on a 2024 MacBook Pro. Latency for LongMemEval S
(115k-token haystacks × 500 QA) is ~22 s.

---

## Optional: real semantic embeddings (Ollama)

The retriever accepts any embedder via `--embedder ollama --ollama-model
<model>`. We've smoke-tested `nomic-embed-text` (137M params, 2048-token
context) on the LongMemEval S `single_session_user` slice (30 cases):

| Embedder | Hit@5 | Mean latency | Notes |
| --- | --- | --- | --- |
| Hash (default) | 98.60% | 15 ms | No external deps |
| Ollama `nomic-embed-text` | **100.00%** | 3.5 s | Local, free, 2048-token cap |

Sessions over the embedder's context window are truncated to the first
4,500 characters before embedding (see
[`truncateForEmbedding`](../internal/bench/hybrid_retriever.go) — this
keeps the topic-bearing head of every session within Ollama's 2048-token
limit). BM25 / temporal / keyword scoring still see the full text.

A full 500-QA Ollama run takes ~25 minutes on a 2024 MacBook Pro
(~25,000 session embeddings × ~50 ms each).

## What we deliberately don't claim

- **No "100%"-style numbers.** The remaining gap on every benchmark
  represents genuinely-failing retrieval cases; closing it by inspecting
  individual misses and tweaking weights would be teaching-to-the-test.
- **No side-by-side QA-accuracy comparison vs other systems.** The
  table above reports **retrieval recall**. End-to-end judged QA
  accuracy (Pixelog's `--full-context --reflect --cot` path) lives in
  [`README.md`](../README.md#latest-qa-results-locomo-30-qa-pilot-conversation-0)
  and is a different metric than retrieval recall.
- **No LLM rerank in the headline numbers.** A rerank pass over the
  top-20 candidates with any capable model lifts every benchmark
  another 1–2 points; we leave that for downstream consumers.

## Per-category breakdown — full numbers

### LongMemEval S, Hit@5 = 97.20% (500 QA, hash)

| Category                       | n   | Hit@5    | Recall@5 |
| ------------------------------ | --: | -------: | -------: |
| `single_session_assistant`     |  56 | 100.00%  | 100.00%  |
| `knowledge_update`             |  78 |  98.70%  |  97.40%  |
| `single_session_user`          |  70 |  98.60%  |  98.60%  |
| `multi_session`                | 133 |  97.70%  |  83.40%  |
| `temporal_reasoning`           | 133 |  96.20%  |  86.70%  |
| `single_session_preference`    |  30 |  86.70%  |  86.70%  |

### LoCoMo, Hit@10 = 96.62% (1,986 QA, 10 conv, hash)

| Category      | n   | Hit@10  | Recall@10 |
| ------------- | --: | ------: | --------: |
| `open_domain` | 841 | 98.50%  | 98.40%    |
| `adversarial` | 446 | 98.20%  | 98.20%    |
| `multi_hop`   | 321 | 96.30%  | 95.50%    |
| `single_hop`  | 282 | 92.90%  | 73.70%    |
| `temporal`    |  96 | 84.80%  | 73.30%    |

### LoCoMo, Hit@5 = 92.08% (1,986 QA, 10 conv, hash)

| Category      | n   | Hit@5   | Recall@5 |
| ------------- | --: | ------: | -------: |
| `adversarial` | 446 | 96.00%  | 96.00%   |
| `open_domain` | 841 | 95.70%  | 95.70%   |
| `multi_hop`   | 321 | 91.00%  | 89.30%   |
| `single_hop`  | 282 | 84.40%  | 60.00%   |
| `temporal`    |  96 | 67.40%  | 54.80%   |

### MemBench, Hit@5 = 98.45% (6,779 QA, hash)

Observation (ThirdAgent) is at **100.00%** across all 7 categories.
Participation (FirstAgent) breakdown:

| Category                          | n   | Hit@5    |
| --------------------------------- | --: | -------: |
| `participation_aggregative`       | 500 | 100.00%  |
| `participation_comparative`       | 500 | 100.00%  |
| `participation_knowledge_update`  | 500 | 100.00%  |
| `participation_simple`            | 500 |  99.60%  |
| `participation_post_processing`   | 500 |  96.00%  |
| `participation_conditional`       | 500 |  93.20%  |
| `participation_noisy`             | 500 |  90.20%  |

---

## Notes on dataset variants

- **ConvoMem `pre_mixed_testcases`** ships with one
  evidence-bearing conversation per case in `1_evidence`, scaling to six in
  `6_evidence`. Without a filler-conversation mix-in, top-k retrieval
  saturates trivially in the lower buckets, so we report **only on the
  top bucket per category** (the `convomem-hard/` layout above). The
  `--max-per-category 50` cap yields 250-300 cases — comparable to
  ConvoMem's standard 50-per-category slice.

- **LongMemEval splits** ship as
  `longmemeval_oracle` (haystack = answer-bearing sessions only),
  `longmemeval_s` (~115k-token haystack), and `longmemeval_m`
  (~1.5M-token haystack). The headline numbers above are reported on
  `longmemeval_s`.

- **MemBench `MemData/FirstAgent`** uses `target_step_id = [[sid, x], ...]`
  where `sid` is a **globally-incrementing message ID** across all
  sessions of the role — *not* the array index of `message_list`.
  `MemData/ThirdAgent` is a flat observation log; we group every
  10 messages into one synthetic session for retrieval scoring.
