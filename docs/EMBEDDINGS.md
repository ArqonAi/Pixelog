# Embeddings: Mock vs Real

## Why Two Embedder Types?

Pixelog supports two embedding strategies:

### 1. Mock Embedder (Default)
**Purpose:** Offline operation without API keys

**How it works:**
- Creates pseudo-embeddings based on text features (word frequency, length, position)
- 384 dimensions for fast computation
- Deterministic but NOT semantically accurate
- Free, no API calls

**Use cases:**
- Testing and development
- Air-gapped systems
- When you don't need semantic search
- File verification and integrity checks

**Limitations:**
- ⚠️ **Not semantically aware** - "car" and "automobile" are treated as different
- Search results based on word matching, not meaning
- Less accurate for complex queries

### 2. Real Embedder (OpenAI/OpenRouter)
**Purpose:** Semantic understanding and accurate search

**How it works:**
- Uses OpenAI's `text-embedding-3-small` model
- 1536 dimensions with true semantic understanding
- "car" and "automobile" are recognized as similar
- Costs ~$0.00002 per 1K tokens

**Use cases:**
- Production semantic search
- RAG (Retrieval Augmented Generation)
- When accuracy matters
- Multi-language support

**Advantages:**
- ✅ Understands meaning, not just words
- ✅ Finds synonyms and related concepts
- ✅ Better ranking of results
- ✅ Multi-language support

## When to Use Which?

| Use Case | Embedder Type | Reason |
|----------|---------------|---------|
| File verification | Mock | No semantic search needed |
| Local testing | Mock | Free, offline |
| Air-gapped deployment | Mock | No internet required |
| Production search | Real | Accuracy matters |
| RAG/LLM chat | Real | Semantic understanding critical |
| Multi-file search | Real | Need cross-document similarity |

## How to Switch

### CLI Usage

**Mock (default):**
```bash
pixe index doc.pixe
pixe search doc.pixe "query"
```

**Real embeddings:**
```bash
# OpenAI
export OPENAI_API_KEY=sk-xxx
pixe index doc.pixe --provider openai --api-key $OPENAI_API_KEY
pixe search doc.pixe "query"

# OpenRouter (cheaper)
export OPENROUTER_API_KEY=sk-or-xxx
pixe index doc.pixe --provider openrouter --api-key $OPENROUTER_API_KEY
pixe search doc.pixe "query"
```

### API Usage

**Mock:**
```go
embedder := index.NewMockEmbedder()
indexer, _ := index.NewIndexer("./indexes", embedder)
```

**Real:**
```go
embedder := index.NewSimpleEmbedder("openai", apiKey, "text-embedding-3-small")
indexer, _ := index.NewIndexer("./indexes", embedder)
```

## Performance Comparison

| Metric | Mock | Real (OpenAI) |
|--------|------|---------------|
| Index build time | 136ms | 500ms+ (API latency) |
| Search time | <100ms | <100ms (cached) |
| Accuracy | 40-60% | 85-95% |
| Cost | $0 | ~$0.00002/1K tokens |
| Offline | ✅ Yes | ❌ No |

## Example: Semantic vs Keyword

**Query:** "automobile safety features"

**Mock embedder finds:**
- Documents with exact words "automobile", "safety", "features"
- Misses: "car", "vehicle", "protection", "security"

**Real embedder finds:**
- All of the above PLUS:
  - "car" (synonym)
  - "vehicle" (related concept)
  - "protection" (semantic similarity)
  - "airbag" (related feature)
  - "crash test" (related context)

## Cost Analysis

**For 1000 documents (1M tokens total):**

| Operation | Mock | OpenAI | OpenRouter |
|-----------|------|--------|------------|
| Index build | $0 | $20 | $2 |
| Search (1000 queries) | $0 | $0.02 | $0.002 |
| **Total** | **$0** | **$20.02** | **$2.002** |

## Hybrid Approach

**Best practice for production:**

1. **Development:** Use mock embedder
2. **CI/CD:** Use mock embedder for tests
3. **Production indexing:** Use real embedder (one-time cost)
4. **Production search:** Use cached index (free)

This way you pay once for indexing but get accurate search forever!

## Future: Local Embeddings

Coming soon - run embeddings locally with models like:
- `BAAI/bge-small-en-v1.5` (133M parameters)
- `sentence-transformers/all-MiniLM-L6-v2` (22M parameters)

This will combine the benefits:
- ✅ Semantic accuracy (like OpenAI)
- ✅ Offline operation (like mock)
- ✅ Zero API costs
- ⚠️ Requires local model download (~50MB)

## Recommendations

### For Most Users
Start with **mock embedder** for testing, switch to **OpenRouter** for production (cheapest real embeddings).

### For Enterprise
Use **OpenAI embeddings** for best accuracy and reliability.

### For Air-Gapped
Use **mock embedder** - it's the only option, and works well for exact keyword matching.

### For Maximum Accuracy
Use **OpenAI text-embedding-3-large** (3072 dimensions) - most expensive but best results.
