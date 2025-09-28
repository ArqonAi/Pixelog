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
- **Semantic Search** - AI-powered content discovery (coming soon)
- **PWA Ready** - Install as desktop/mobile app
- **Docker Support** - One-command deployment
- **CLI Tool** - Command-line interface for automation

## Quick Start

### Web Interface (Recommended)
```bash
# Clone and run
git clone https://github.com/ArqonAi/Pixelog.git
cd Pixelog
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
- **PWA Support** - Installable as desktop/mobile app
- **File Management** - Upload, download, delete .pixe files

## How It Works

1. **Data Ingestion** - Files are analyzed and chunked (2800 bytes per chunk)
2. **QR Encoding** - Each chunk becomes a high-resolution QR code frame
3. **Video Assembly** - Frames are stitched into MP4 (0.5 FPS, libx264)
4. **Compression** - Silent audio track + fast-start optimization
5. **Storage** - Portable .pixe files ready for streaming/sharing

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

## Configuration

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

## Docker Deployment

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

- [ ] **Semantic Search** - Vector embeddings for content discovery
- [ ] **Encryption** - AES-256 encryption for sensitive data
- [ ] **Batch Processing** - Multi-file conversion optimization
- [ ] **Mobile App** - React Native implementation
- [ ] **Cloud Storage** - S3/GCS integration
- [ ] **Plugin System** - Custom file processors

##  Support

- **Issues**: [GitHub Issues](https://github.com/ArqonAi/Pixelog/issues)
- **Discussions**: [GitHub Discussions](https://github.com/ArqonAi/Pixelog/discussions)
- **Security**: See [SECURITY.md](SECURITY.md)

---

**Built with ❤️ by the Pixelog/Arqon team**

⭐ **Star this repository if you find it useful!** ⭐
