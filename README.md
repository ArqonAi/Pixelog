# Pixelog - Knowledge Storage Platform

**Pixelog v1.0.0** - SQLite-meets-YouTube for LLM memories. Convert diverse knowledge sources into portable, encrypted .pixe files.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/ArqonAi/Pixelog)](https://goreportcard.com/report/github.com/ArqonAi/Pixelog)
[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org/dl/)
[![React](https://img.shields.io/badge/React-18+-61DAFB.svg?logo=react&logoColor=white)](https://reactjs.org/)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg?logo=docker&logoColor=white)](https://hub.docker.com/)

[![GitHub Actions](https://github.com/ArqonAi/Pixelog/workflows/CI/badge.svg)](https://github.com/ArqonAi/Pixelog/actions)
[![GitHub release](https://img.shields.io/github/v/release/ArqonAi/Pixelog?include_prereleases)](https://github.com/ArqonAi/Pixelog/releases)
[![GitHub issues](https://img.shields.io/github/issues/ArqonAi/Pixelog)](https://github.com/ArqonAi/Pixelog/issues)
[![GitHub stars](https://img.shields.io/github/stars/ArqonAi/Pixelog?style=social)](https://github.com/ArqonAi/Pixelog/stargazers)

## Features

- **Encrypted & Compressed** - Store data as QR-encoded video streams
- **Portable & Streamable** - Access your knowledge anywhere
- **Drag & Drop Interface** - Intuitive web-based GUI
- **Real-time Progress** - WebSocket-powered conversion tracking
- **LLM Chat Interface** - Chat with your memories using multiple AI providers
- **Multi-Provider AI Support** - OpenAI, Anthropic, Google Gemini, OpenRouter, xAI Grok, Ollama
- **Semantic Search** - AI-powered content discovery with multiple embedding providers
- **PWA Ready** - Install as desktop/mobile app
- **Docker Support** - One-command deployment
- **CLI Tool** - Command-line interface for automation

## Quick Start

### Web Interface (Recommended)
```bash
# Clone and run
git clone https://github.com/ArqonAi/Pixelog.git
cd Pixelog

# Optional: Enable AI features (choose your provider)
export OPENAI_API_KEY="your-openai-key-here"
# OR export ANTHROPIC_API_KEY="your-anthropic-key-here"  
# OR export GOOGLE_API_KEY="your-gemini-key-here"
# OR export OPENROUTER_API_KEY="your-openrouter-key-here"
# OR export XAI_API_KEY="your-grok-key-here"
# OR run Ollama locally (no API key needed)

go run backend/cmd/server/main.go -dev

# Open http://localhost:8080
```

### Docker (Production)
```bash
# Build and run with Docker Compose
docker-compose up --build

# Or run individual containers
docker build -t pixelog .
docker run -p 8080:8080 pixelog
```

### CLI Tool
```bash
# Build
go build -o pixelog backend/cmd/pixelog/main.go

# Convert files
./pixelog -input document.txt -output knowledge.pixe
./pixelog -input /path/to/files/ -output collection.pixe

# Extract data
./pixelog -extract -input knowledge.pixe -output ./extracted/

# List contents
./pixelog -list -input knowledge.pixe
```

## Architecture

### Backend (Go)
- **CLI Tool** - Command-line interface for batch processing
- **Web API** - REST API with WebSocket support for real-time updates
- **QR Generation** - High-resolution QR code frames (512x512)
- **Video Assembly** - FFmpeg integration for MP4 creation
- **Security** - Rate limiting, CORS, XSS protection, file validation

### Frontend (Web)
- **Modern UI** - Beautiful, responsive interface with Tailwind CSS
- **Drag & Drop** - Intuitive file upload with visual feedback
- **Real-time Updates** - WebSocket progress tracking
- **Semantic Search** - AI-powered content discovery modal
- **PWA Support** - Installable as desktop/mobile app
- **File Management** - Upload, download, delete .pixe files

## 📖 How It Works

1. **Data Ingestion** - Files are analyzed and chunked (2800 bytes per chunk)
2. **QR Encoding** - Each chunk becomes a high-resolution QR code frame
3. **Video Assembly** - Frames are stitched into MP4 (0.5 FPS, libx264)
4. **Compression** - Silent audio track + fast-start optimization
5. **Storage** - Portable .pixe files ready for streaming/sharing
6. **Search Indexing** - Text extracted and vectorized for semantic search

## 💬 LLM Chat Interface

Pixelog features a powerful chat interface that lets you converse with your stored memories using state-of-the-art language models.

### Supported AI Providers

**🤖 OpenAI**
- GPT-4o, GPT-4o-mini, GPT-4-turbo, GPT-4, GPT-3.5-turbo
- Industry-leading performance and reasoning capabilities

**🎭 Anthropic**  
- Claude 3.5 Sonnet, Claude 3.5 Haiku, Claude 3 Opus, Claude 3 Haiku
- Advanced reasoning with strong safety focus

**🌟 Google Gemini**
- Gemini 2.5 Flash, Gemini 2.0 Flash, Gemini 1.5 Pro, Gemini 1.5 Flash  
- Multimodal capabilities and fast inference

**🌐 OpenRouter**
- Access 30+ models through unified API: DeepSeek, Moonshot, Mistral, and more
- Cost-effective access to multiple providers

**🤖 xAI Grok**
- Grok-2, Grok-1.5V, Grok-1 models with real-time data access
- Witty and conversational AI with up-to-date knowledge

**🦙 Ollama (Local)**
- Run models locally without API keys: Llama, Mistral, CodeLlama, and more
- Privacy-focused with no external API calls required

### Chat Features

- **Memory Integration** - Chat with uploaded .pixe files as context
- **Export Conversations** - Save chats as .pixe files or text
- **Right-aligned UI** - Modern chat interface with user messages on right
- **Clickable Send** - Both Enter key and button click support
- **Tab-based Interface** - Seamless switching between chat, upload, and create

## 🔍 Semantic Search

Pixelog includes AI-powered semantic search to help you discover content using natural language queries.

### Features
- **Natural Language Queries** - Ask questions or describe what you're looking for
- **Multiple AI Providers** - OpenAI, Gemini, Grok, OpenRouter, or local Ollama
- **Similarity Matching** - Finds content based on meaning, not just keywords
- **Real-time Results** - Instant search with similarity scores
- **Content Preview** - See matching content snippets with highlighted relevance

### AI Provider Support

**Cloud Providers:**
- **OpenAI**: `text-embedding-3-small` (1536 dimensions)
- **Google Gemini**: `embedding-001` (768 dimensions) 
- **xAI Grok**: `text-embedding-grok` (1024 dimensions)
- **OpenRouter**: Multiple models via unified API

**Local/Privacy:**
- **Ollama**: Run models locally (`nomic-embed-text`, `all-minilm`, etc.)

### Setup

**Option 1: OpenAI (Recommended)**
```bash
export OPENAI_API_KEY="sk-your-api-key-here"
go run backend/cmd/server/main.go
```

**Option 2: Google Gemini**
```bash
export GOOGLE_API_KEY="your-gemini-key-here"
export EMBEDDING_PROVIDER="gemini"
go run backend/cmd/server/main.go
```

**Option 3: xAI Grok**
```bash
export XAI_API_KEY="xai-your-grok-key-here"
export EMBEDDING_PROVIDER="grok"
go run backend/cmd/server/main.go
```

**Option 4: OpenRouter (Access Multiple Models)**
```bash
export OPENROUTER_API_KEY="sk-or-your-key-here"
export OPENROUTER_MODEL="text-embedding-3-small"  # or any supported model
export EMBEDDING_PROVIDER="openrouter"
go run backend/cmd/server/main.go
```

**Option 5: Local Ollama (No API Key Required)**
```bash
# Install and start Ollama
curl -fsSL https://ollama.com/install.sh | sh
ollama pull nomic-embed-text

# Configure Pixelog
export EMBEDDING_PROVIDER="ollama"
export OLLAMA_MODEL="nomic-embed-text"
go run backend/cmd/server/main.go
```

**Auto-Detection (Default)**
```bash
# Set any API key - Pixelog will auto-detect the first available provider
export OPENAI_API_KEY="sk-..." # or any other provider
# EMBEDDING_PROVIDER="auto" is the default
go run backend/cmd/server/main.go
```

### Usage
1. **Upload Files** - Text files (.txt, .md, .csv, .json, .yaml) are automatically indexed
2. **Click Search** - Use the search icon in the header
3. **Ask Questions** - Try queries like:
   - "What are the main findings about climate change?"
   - "Show me configuration examples"
   - "Find discussions about performance optimization"
4. **Adjust Filters** - Set similarity threshold (50%-90%) and result limits

### API Endpoints
```bash
# Search for content
POST /api/search/query
{
  "query": "machine learning algorithms",
  "limit": 10,
  "threshold": 0.7
}

# Get similar documents
GET /api/search/similar/:documentId?limit=5

# List all indexed documents
GET /api/search/documents?limit=20&offset=0
```

### Supported File Types
- **Plain Text**: .txt, .md, .csv, .json, .yaml, .yml, .log
- **Future**: PDF, DOCX, images (OCR) - coming soon

## 🔒 Encryption & Security

Pixelog supports AES-256-GCM encryption for sensitive data:

```bash
# Enable encryption
export ENCRYPTION_ENABLED=true
export DEFAULT_PASSWORD="your-secure-password"  # Optional

# Files will be encrypted before conversion to .pixe format
go run backend/cmd/server/main.go
```

**Features:**
- **AES-256-GCM** - Industry-standard encryption
- **PBKDF2 Key Derivation** - 100,000 iterations with random salt
- **Password Protection** - User-defined or auto-generated passwords
- **Secure Storage** - Encrypted at rest and in transit

## ☁️ Cloud Storage Integration

Upload your .pixe files directly to cloud storage:

**AWS S3:**
```bash
export S3_BUCKET="my-pixelog-bucket"
export S3_REGION="us-east-1"
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
```

**Google Cloud Storage:**
```bash
export GCS_BUCKET="my-pixelog-bucket"
export GOOGLE_PROJECT_ID="your-project-id"
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"
```

**Azure Blob Storage:**
```bash
export AZURE_STORAGE_ACCOUNT="your-storage-account"
export AZURE_CONTAINER="pixelog"
export AZURE_STORAGE_KEY="your-azure-key"
```

## 📱 Progressive Web App

Pixelog is a full PWA with:
- **Offline Support** - Works without internet connection
- **Install Prompt** - Add to home screen on mobile/desktop
- **Background Sync** - Queue uploads when offline
- **Native Sharing** - Receive files from other apps
- **Push Notifications** - Conversion status updates
- **File Association** - Open supported files directly in Pixelog

**Install as App:**
1. Visit Pixelog in your browser
2. Click the "Install" prompt or "Add to Home Screen"
3. Use like a native app with full functionality

## Development

### Prerequisites
- Go 1.21+
- Node.js 20+ (for frontend development)
- FFmpeg
- Docker (optional)

### Setup
```bash
# Backend
go mod download
go test ./...

# Frontend (for development)
cd frontend
npm install
npm run dev

# Build everything
go build -o pixelog-server backend/cmd/server/main.go
go build -o pixelog-cli backend/cmd/pixelog/main.go
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/convert` | Upload and convert files |
| `GET` | `/api/v1/progress/:id` | Check conversion progress |
| `WS` | `/api/v1/ws/:id` | Real-time progress updates |
| `GET` | `/api/v1/pixefiles` | List all .pixe files |
| `POST` | `/api/v1/extract` | Extract data from .pixe |
| `GET` | `/api/v1/download/:id` | Download .pixe file |
| `DELETE` | `/api/v1/pixefile/:id` | Delete .pixe file |
| `GET` | `/api/v1/health` | Health check |

## 🔧 Configuration

### Backend Config
```go
type Config struct {
    ChunkSize int     // QR chunk size (default: 2800)
    Quality   int     // Video quality CRF (default: 23)
    FrameRate float64 // Video FPS (default: 0.5)
    Verbose   bool    // Debug logging
}
```

### Video Settings
- **Codec**: libx264 with yuv420p pixel format
- **Audio**: Silent stereo track (48kHz)
- **Container**: MP4 with fast-start optimization
- **Compression**: CRF 23 for optimal quality/size balance

## 🐳 Docker Deployment

### Docker Compose
```yaml
version: '3.8'
services:
  pixelog:
    image: ghcr.io/arqonai/pixelog:latest
    ports:
      - "8080:8080"
    volumes:
      - pixelog_data:/app/output
    environment:
      - PORT=8080
      - GIN_MODE=release
volumes:
  pixelog_data:
```

### Build from Source
```bash
docker build -t pixelog .
docker run -p 8080:8080 pixelog
```

## Security

- **Rate limiting** - 60 requests per minute per IP
- **File size limits** - 100MB per file maximum
- **CORS protection** - Configured for production domains
- **XSS protection** - Security headers enabled
- **Input validation** - All user inputs sanitized
- **Non-root container** - Docker runs as unprivileged user

## Testing

```bash
# Backend tests
go test ./...

# Load testing
curl -X POST http://localhost:8080/api/v1/health

# Docker health check
docker ps --format "table {{.Names}}\t{{.Status}}"
```

## File Format (.pixe)

```
.pixe file structure:
├── Video Stream (QR frames)
│   ├── Metadata chunks (JSON)
│   ├── File data chunks (Base64/Text)
│   └── Index chunks (File mapping)
├── Audio Stream (silent, compatibility)
└── Container metadata (MP4 headers)
```

## Performance

- **Compression**: ~60% size reduction vs raw files
- **Speed**: 1-10MB/sec conversion (depends on file type)
- **Memory**: <100MB RAM usage during conversion
- **Storage**: Minimal disk space (temporary files cleaned)

## Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- **FFmpeg** - Powerful multimedia framework
- **Go Community** - Excellent libraries and tools
- **React Community** - Modern frontend ecosystem
- **QR Code Pioneers** - Data encoding innovation

## Roadmap

- [x] **Semantic Search** - Vector embeddings for content discovery ✨
- [x] **Batch Processing** - Multi-file conversion optimization
- [x] **Encryption** - AES-256 encryption for sensitive data ✨
- [x] **Mobile/PWA** - Progressive Web App with offline support ✨  
- [x] **Cloud Storage** - S3/GCS/Azure integration framework ✨
- [ ] **Plugin System** - Custom file processors
- [ ] **Real-time Collaboration** - Multi-user workspace sharing
- [ ] **Advanced OCR** - PDF and image text extraction [SECURITY.md](SECURITY.md)

##  Support

- **Issues**: [GitHub Issues](https://github.com/ArqonAi/Pixelog/issues)
- **Discussions**: [GitHub Discussions](https://github.com/ArqonAi/Pixelog/discussions)
- **Security**: See [SECURITY.md](SECURITY.md)

---

**Built with ❤️ by the Pixelog/Arqon team**

⭐ **Star this repository if you find it useful!** ⭐
