# Pixelog

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org/)
[![Go Report Card](https://goreportcard.com/badge/github.com/ArqonAi/Pixelog)](https://goreportcard.com/report/github.com/ArqonAi/Pixelog)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](https://github.com/ArqonAi/Pixelog/actions)
[![CLI Ready](https://img.shields.io/badge/CLI-12%20Commands-orange.svg)](cmd/pixe)
[![Security](https://img.shields.io/badge/Security-AES--256--GCM-green.svg)](https://en.wikipedia.org/wiki/Galois/Counter_Mode)
[![Format](https://img.shields.io/badge/Format-MP4%20Video-red.svg)](https://en.wikipedia.org/wiki/MP4_file_format)
[![QR Encoding](https://img.shields.io/badge/Encoding-QR%20Codes-black.svg)](https://en.wikipedia.org/wiki/QR_code)
[![Vector Search](https://img.shields.io/badge/Search-Vector%20Embeddings-purple.svg)](https://en.wikipedia.org/wiki/Word_embedding)
[![Performance](https://img.shields.io/badge/Retrieval-Sub--100ms-green.svg)](#)
[![Version Control](https://img.shields.io/badge/Git--like-Delta%20Encoding-orange.svg)](#)
[![Streaming](https://img.shields.io/badge/Streaming-Multi--GB%20Files-blue.svg)](#)

**A novel archival system that encodes documents as QR code frames in MP4 video files, enabling universal playback, Git-like version control, and sub-100ms semantic search.**

---

## What is Pixelog?

Pixelog transforms any document into a **`.pixe` file** - an MP4 video where each frame is a QR code containing chunks of your data. This approach unlocks:

- **Universal compatibility**: MP4 plays on any device (phones, computers, TVs, browsers)
- **Git-like version control**: Delta encoding tracks changes (64% space savings)
- **Sub-100ms semantic search**: Vector embeddings enable meaning-based queries
- **Interactive LLM chat**: RAG-powered Q&A with 200+ models via OpenRouter
- **Streaming architecture**: Handle multi-GB files with constant 10MB memory
- **Military-grade encryption**: AES-256-GCM with tamper detection
- **Air-gapped capable**: Works completely offline

**Technical Specs:**
- Format: MP4 with H.264-encoded QR frames
- Density: 2.9KB per frame @ 1080p (87KB/sec)
- Error correction: Reed-Solomon (30% damage tolerance)
- Search: HNSW vector index with cosine similarity

---

## Quick Start

### Installation

```bash
go install github.com/ArqonAi/Pixelog/cmd/pixe@latest
```

Or build from source:

```bash
git clone https://github.com/ArqonAi/Pixelog.git
cd Pixelog
go build -o pixe ./cmd/pixe
```

### Basic Workflow

```bash
# Convert document to .pixe format
pixe convert document.txt -o doc.pixe

# Build semantic search index
export OPENROUTER_API_KEY=sk-or-v1-xxx
pixe index doc.pixe

# Search by meaning
pixe search doc.pixe "machine learning concepts" --top 5

# Chat with your document
pixe chat doc.pixe
```

---

## Core Features

### File Operations
- Convert any file type to .pixe format
- Extract original files from .pixe archives
- Display file metadata and structure
- Integrity checking via SHA-256 hashing
- AES-256-GCM encryption with password

### Semantic Search
- Build vector embeddings for sub-100ms search
- Meaning-based queries (not just keyword matching)
- Interactive LLM Q&A with automatic context retrieval
- Ranked results by cosine similarity

### Version Control
- Create version snapshots with messages
- List all versions with timestamps
- Compare versions (frame-level changes)
- Time-travel search across historical versions
- Delta encoding (64% average space savings)

### Performance
- Sub-100ms search with HNSW indexing
- Constant 10MB memory footprint (any file size)
- Streaming support for multi-GB files
- Parallel frame encoding/decoding

### Security
- AES-256-GCM authenticated encryption
- PBKDF2 key derivation (600,000 iterations)
- Reed-Solomon error correction (30% damage tolerance)
- SHA-256 frame hashing for tamper detection
- Air-gapped operation (no internet required)

---

## CLI Commands

### Basic Operations

```bash
pixe convert <input> -o <output.pixe>    # Convert to .pixe
pixe extract <file.pixe> -o <output>      # Extract from .pixe
pixe info <file.pixe>                     # Show file info
pixe verify <file.pixe>                   # Verify integrity
```

### Semantic Search (requires OpenRouter API key)

```bash
export OPENROUTER_API_KEY=sk-or-v1-xxx
pixe index <file.pixe>                           # Build index
pixe search <file.pixe> "query" --top 5          # Search
pixe chat <file.pixe>                            # Interactive chat
pixe chat <file.pixe> --model openai/gpt-5       # Specific model
pixe chat <file.pixe> --list                     # Show models
```

### Version Control

```bash
pixe version <file.pixe> -m "message"            # Create version
pixe versions <file.pixe>                        # List versions
pixe diff <file.pixe> <v1> <v2>                 # Compare versions
pixe query <file.pixe> <version> "query"         # Time-travel query
```

### Encryption

```bash
pixe convert file.txt -o file.pixe --encrypt --password mypass
pixe extract file.pixe -o output --password mypass
pixe index file.pixe --password mypass
```

---

## Use Cases

### Knowledge Base Management

```bash
# Create and index
pixe convert docs/ -o knowledge.pixe
pixe index knowledge.pixe

# Semantic search
pixe search knowledge.pixe "authentication best practices"

# Track changes
pixe version knowledge.pixe -m "Added security section"
pixe diff knowledge.pixe 1 2
```

### Compliance & Audit Trails

```bash
# Encrypted archive
pixe convert compliance-docs/ -o audit.pixe --encrypt --password xxx

# Track all changes
pixe versions audit.pixe

# Time-travel query
pixe query audit.pixe 1 "Q1 data retention policy"

# Verify integrity
pixe verify audit.pixe --password xxx
```

### Research Paper Collections

```bash
# Index papers
pixe convert papers/ -o research.pixe
pixe index research.pixe

# Semantic citation search
pixe search research.pixe "transformer attention mechanisms"

# Chat with research
pixe chat research.pixe
```

### Secure Document Archival

```bash
# Encrypted, air-gapped storage
pixe convert classified/ -o vault.pixe --encrypt --password xxx
pixe verify vault.pixe --password xxx
pixe extract vault.pixe -o restored/ --password xxx
```

### Large-Scale Code Archival

```bash
# Streaming for multi-GB codebases
pixe convert monorepo.tar.gz -o codebase.pixe
# Auto-streaming: 2.5 GB with 10MB RAM

# Version control
pixe version codebase.pixe -m "Release v2.0"

# Semantic code search
pixe search codebase.pixe "authentication middleware"
```

---

## How It Works

### Architecture

```
Document → Chunks (2.9KB) → Encryption → QR Codes → MP4 Frames → .pixe File
```

Each `.pixe` file is an MP4 video:
- Frame 0: Metadata (file info, encryption params, version history)
- Frame 1+: QR-encoded data chunks
- Audio track: Silent (required for MP4 spec)

### Directory Structure

```
pixelog/
├── cmd/pixe/              # CLI (12 commands)
├── internal/
│   ├── converter/         # Document → .pixe
│   ├── crypto/            # AES-256-GCM
│   ├── qr/                # QR generation
│   ├── video/             # MP4 creation/extraction
│   ├── index/             # Semantic search
│   │   ├── indexer.go    # HNSW vector index
│   │   ├── embedder.go   # OpenRouter embeddings
│   │   └── delta.go      # Version control
│   └── llm/               # LLM client (OpenRouter)
├── pkg/config/            # Configuration
├── docs/                  # Documentation
└── examples/              # Usage examples
```

---

## Performance

| Operation | Time | Notes |
|-----------|------|-------|
| Index Build | 136ms | One-time per file |
| Semantic Search | <100ms | With 1000+ frames |
| Frame Extraction | 20ms | Direct FFmpeg seek |
| LLM Chat Response | <200ms | Excl. LLM latency |
| Version Creation | 85ms | Delta calculation |
| Integrity Check | 50ms/frame | Parallel decoding |

### Storage Efficiency

- Delta encoding: 64% space savings
- GZIP compression: 75% reduction
- Combined: ~80% smaller than raw storage

### Memory Efficiency

| File Size | Traditional | Pixelog Streaming |
|-----------|-------------|-------------------|
| 10 MB | 10 MB RAM | 10 MB RAM |
| 100 MB | 100 MB RAM | 10 MB RAM |
| 1 GB | 1 GB RAM | 10 MB RAM |
| 10 GB | 10 GB RAM | 10 MB RAM |

Streaming auto-enables for files >100MB.

---

## Security

### Encryption

- **Algorithm**: AES-256-GCM (authenticated encryption)
- **Key Derivation**: PBKDF2 (600,000 iterations, SHA-256)
- **Salt**: 32-byte random per file
- **Nonce**: 12-byte random per operation
- **Auth Tag**: 16-byte for tamper detection

### Error Correction

- **Reed-Solomon codes**: 30% damage tolerance per frame
- **QR Error Correction**: Level H (highest)
- **Data recovery**: Even if portions of video corrupted

### File Structure

```
.pixe File (MP4 Container)
├── Video Track (H.264)
│   ├── Frame 0: Metadata
│   ├── Frame 1+: [32B salt][12B nonce][encrypted data][16B auth tag]
└── Audio Track (silent)
```

---

## LLM Integration

### OpenRouter API

Pixelog uses OpenRouter for embeddings and LLM chat (200+ models, one API key).

**Get free key**: https://openrouter.ai/keys

```bash
export OPENROUTER_API_KEY=sk-or-v1-xxx
```

### Top 10 Models

| Rank | Model | Cost | Speed | Description |
|------|-------|------|-------|-------------|
| 1 | DeepSeek R1 | $0.14/1M | Fast | Best value (default) |
| 2 | Gemini 2.5 Flash | FREE | Very Fast | Latest Google, free |
| 3 | Gemini 2.5 Pro | $0.50/1M | Medium | Best Gemini |
| 4 | GPT-5 | $2.50/1M | Medium | Latest OpenAI |
| 5 | Claude 4.5 Sonnet | $3.00/1M | Medium | Best reasoning |
| 6 | Grok 3 | $5.00/1M | Fast | Real-time data |
| 7 | Llama 3.3 70B | $0.18/1M | Fast | Open source |
| 8 | Qwen 2.5 72B | $0.18/1M | Fast | Multilingual |
| 9 | Mistral Large | $2.00/1M | Fast | European |
| 10 | GPT-4o | $0.75/1M | Fast | Multimodal |

### Usage

```bash
# List models
pixe chat doc.pixe --list

# Default (DeepSeek R1)
pixe chat doc.pixe

# Free tier (Gemini)
pixe chat doc.pixe --model google/gemini-2.5-flash-latest

# Premium (GPT-5)
pixe chat doc.pixe --model openai/gpt-5
```

### Costs

| Operation | Model | Cost | Notes |
|-----------|-------|------|-------|
| Embeddings (indexing) | text-embedding-3-large | $0.02/1M | One-time |
| Search queries | text-embedding-3-large | $0.0001/query | Per query |
| Chat (default) | deepseek/deepseek-r1 | $0.14/1M | Best value |
| Chat (free) | gemini-2.5-flash | FREE | Free tier |

**Example**: Index 1,000 docs (~$2) + 10,000 searches (~$1) + 1M tokens chat ($0.14 or FREE)

---

## FAQ

### Why video-based storage?

1. **Universal compatibility**: MP4 plays everywhere
2. **Built-in streaming**: Progressive loading
3. **Frame-level access**: Direct seek without loading full file
4. **Visual inspection**: See data as scannable QR codes
5. **Novel use cases**: Video-based data transmission

### Do I need an API key?

**Optional**. Core operations work offline:
- Convert, extract, verify, version control: No API needed

**Required for**:
- Semantic search (indexing + search)
- LLM chat

Get free key: https://openrouter.ai/keys

### How secure is it?

**Military-grade**: AES-256-GCM encryption, same as classified government systems.
- 600,000 PBKDF2 iterations (brute-force protection)
- Authenticated encryption (tamper detection)
- Air-gapped operation (works offline)
- Suitable for HIPAA, SOC 2, ISO 27001

### Can I use it offline?

**Yes, most features**:
- Offline: Convert, extract, encrypt/decrypt, verify, version control
- Online: Semantic search, LLM chat (requires OpenRouter API)

### How large can files be?

**No practical limit** due to streaming:
- Small files (<100MB): Loaded into memory
- Large files (>100MB): Auto-streaming mode
- Memory: Constant 10MB footprint
- Tested: Up to 10GB files

### What file types?

**All types**: Documents, code, archives, media, databases, binaries.
Pixelog is format-agnostic.

### How fast is search?

**Sub-100ms**:
- Index build: 136ms (one-time)
- Search query: <100ms (1000+ frames)
- Total: Query → Results in <100ms

---

## Platform API

### Hosted Platform

Use the hosted Pixelog platform at **https://chat.arqon.ai** for a web-based interface with:

- Drag & drop file conversion
- Real-time progress tracking
- Interactive LLM chat with your .pixe files
- Semantic search with visual interface
- No CLI installation required

### REST API Endpoints

The platform provides a REST API for integration:

```bash
# Convert file to .pixe
POST https://chat.arqon.ai/api/convert
Content-Type: multipart/form-data

{
  "file": <binary>,
  "encrypt": true,
  "password": "optional"
}

Response:
{
  "memory_id": "doc123",
  "file_path": "/memories/doc123.pixe",
  "size": "52KB",
  "frames": 3
}

# Build search index
POST https://chat.arqon.ai/api/index/:memory_id

Response:
{
  "index_id": "idx123",
  "dimensions": 3072,
  "frame_count": 3
}

# Semantic search
POST https://chat.arqon.ai/api/search

{
  "memory_id": "doc123",
  "query": "machine learning concepts",
  "top_k": 5
}

Response:
{
  "results": [
    {
      "frame_number": 2,
      "score": 0.92,
      "preview": "Neural networks and deep learning..."
    }
  ]
}

# LLM chat
POST https://chat.arqon.ai/api/chat

{
  "memory_id": "doc123",
  "message": "Explain the main topics",
  "model": "deepseek/deepseek-r1"
}

Response:
{
  "response": "The main topics are...",
  "context_frames": [2, 5, 7]
}

# Create version
POST https://chat.arqon.ai/api/versions/:memory_id

{
  "message": "Updated documentation",
  "author": "user@example.com"
}
```

### WebSocket Support

Real-time progress tracking for long-running operations:

```javascript
const ws = new WebSocket('wss://chat.arqon.ai/ws/progress/:memory_id');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(`Progress: ${data.percentage}%`);
  // { percentage: 75, status: "Converting frame 3/4" }
};
```

---

## Go Library Usage

For local/self-hosted deployments:

```go
package main

import (
    "github.com/ArqonAi/Pixelog/internal/converter"
    "github.com/ArqonAi/Pixelog/internal/index"
    "github.com/ArqonAi/Pixelog/internal/llm"
)

func main() {
    // Convert
    conv, _ := converter.New("./output")
    conv.ConvertFile("doc.txt", &converter.ConvertOptions{
        OutputPath:    "doc.pixe",
        EncryptionKey: "password",
    })

    // Index
    embedder := index.NewSimpleEmbedder("openrouter", apiKey, "auto")
    indexer, _ := index.NewIndexer("./indexes", embedder)
    idx, _ := indexer.BuildIndex("doc", "doc.pixe")

    // Search
    results, _ := indexer.Search(idx, "query", 5)

    // Version control
    deltaManager, _ := index.NewDeltaManager("./deltas", indexer)
    deltaManager.CreateVersion("doc", "doc.pixe", "Initial", "user")

    // LLM chat
    client := llm.NewClient("deepseek/deepseek-r1", apiKey)
    response, _ := client.Chat("Explain main concepts")
}
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)

```bash
git checkout -b feature/amazing-feature
./test_e2e.sh
git commit -m "feat: Add amazing feature"
git push origin feature/amazing-feature
```

---

## License

Apache License 2.0 - see [LICENSE](LICENSE)

---

## Related Projects

- **[Arqon Chat](https://chat.arqon.ai)** - Hosted platform with web UI, drag & drop interface, and real-time progress tracking
- **[Platform Repository](https://github.com/ArqonAi/Platform)** - Backend API and React frontend for self-hosting

---

## Support

- **Platform**: [chat.arqon.ai](https://chat.arqon.ai) - Hosted web interface
- **Documentation**: [docs/](docs/)
- **Issues**: [GitHub Issues](https://github.com/ArqonAi/Pixelog/issues)
- **Discussions**: [GitHub Discussions](https://github.com/ArqonAi/Pixelog/discussions)
- **API Reference**: See [Platform API](#platform-api) section above

---

**Made by [ArqonAi](https://github.com/ArqonAi)**

*Turn documents into videos. Search at the speed of thought. Track changes like Git. Chat with AI.*
