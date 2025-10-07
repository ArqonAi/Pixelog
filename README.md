# 🔐 Pixe-Core

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/ArqonAi/Pixelog)](https://goreportcard.com/report/github.com/ArqonAi/Pixelog)

**Pixe-Core** is the open-source core library for creating, encrypting, and managing `.pixe` files - a secure, portable file format for data archival and transfer.

## ✨ Features

- 🔒 **AES-256-GCM Encryption** - Military-grade encryption with PBKDF2 key derivation
- 📦 **File Conversion** - Convert any file type to secure `.pixe` format  
- 🔍 **Content Inspection** - List and analyze `.pixe` file contents without extraction
- 📱 **QR Code Support** - Generate QR codes for data chunks (future: video encoding)
- 🛡️ **Data Integrity** - SHA-256 hashing for tamper detection
- 🚀 **CLI Tool** - Command-line interface for all operations
- 📚 **Go Library** - Clean API for integration into other projects

## 🚀 Quick Start

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

### Basic Usage

```bash
# Convert a file to .pixe format
pixe -input document.txt

# Convert with encryption
pixe -input document.txt -encrypt mypassword

# Extract encrypted .pixe file  
pixe -input document.pixe -extract document.txt -decrypt mypassword

# List .pixe file contents
pixe -list document.pixe
```

## 📖 Library Usage

### Import the Package

```go
import "github.com/ArqonAi/Pixelog/pkg/converter"
```

### Convert Files

```go
package main

import (
    "log"
    "github.com/ArqonAi/Pixelog/pkg/converter"
)

func main() {
    // Create converter
    conv, err := converter.New("./output")
    if err != nil {
        log.Fatal(err)
    }

    // Convert file with encryption
    opts := &converter.ConvertOptions{
        EncryptionKey: "mypassword",
        OutputPath:    "document.pixe",
    }

    outputPath, err := conv.ConvertFile("document.txt", opts)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Created: %s", outputPath)
}
```

### Extract Files

```go
// Extract encrypted .pixe file
err = conv.ExtractFile("document.pixe", "document.txt", "mypassword")
if err != nil {
    log.Fatal(err)
}
```

### List Contents

```go
// List contents without extraction
contents, err := conv.ListContents("document.pixe")
if err != nil {
    log.Fatal(err)
}

for _, item := range contents {
    log.Printf("File: %s, Size: %s, Encrypted: %v", 
        item.Name, item.Size, item.Encrypted)
}
```

## 🔐 Encryption Details

Pixe-Core uses **AES-256-GCM** encryption with the following security features:

- **PBKDF2** key derivation (100,000 iterations, SHA-256)
- **32-byte random salt** per file
- **12-byte random nonce** per encryption
- **Authenticated encryption** prevents tampering
- **SHA-256 hashing** for integrity verification

### Encrypted File Structure

```
[32-byte salt][12-byte nonce][encrypted data + auth tag]
```

## 📋 .pixe File Format

The `.pixe` format is a JSON-based container with the following structure:

```json
{
  "metadata": {
    "version": "1.0",
    "created_at": "2025-10-07T10:30:00Z",
    "encrypted": true,
    "contents": [
      {
        "name": "document.txt",
        "type": "text/plain", 
        "size": "1.2 KB",
        "hash": "sha256_hash...",
        "encrypted": true,
        "created_at": "2025-10-07T10:30:00Z"
      }
    ]
  },
  "content": "base64_encoded_data..."
}
```

## 🛠️ CLI Reference

### Commands

| Operation | Command | Description |
|-----------|---------|-------------|
| **Convert** | `pixe -input file.txt` | Convert file to .pixe |
| **Encrypt** | `pixe -input file.txt -encrypt pass` | Convert with encryption |
| **Extract** | `pixe -input file.pixe -extract out.txt` | Extract .pixe file |
| **Decrypt** | `pixe -input file.pixe -extract out.txt -decrypt pass` | Extract encrypted file |
| **List** | `pixe -list file.pixe` | Show .pixe contents |

### Options

| Flag | Type | Description |
|------|------|-------------|
| `-input` | string | Input file path |
| `-output` | string | Output file path (optional) |
| `-extract` | string | Extract to specified path |
| `-list` | string | List contents of .pixe file |
| `-encrypt` | string | Encryption password |
| `-decrypt` | string | Decryption password |
| `-help` | bool | Show help message |

## 🏗️ Architecture

```
pixe-core/
├── pkg/
│   ├── converter/     # File conversion logic
│   ├── crypto/        # AES-256-GCM encryption
│   └── qr/           # QR code generation
├── cmd/pixe/         # CLI tool
├── examples/         # Usage examples  
└── docs/            # Documentation
```

## 🧪 Examples

See the [examples](examples/) directory for:

- Basic file conversion
- Encryption/decryption workflows
- Batch processing scripts
- Integration patterns

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🔗 Related Projects

- **[ArqonAi Platform](https://github.com/ArqonAi/Platform)** - Full-featured .pixe file manager with LLM integration (Private)
- **[Memvid](https://github.com/ArqonAi/memvid)** - Video encoding for .pixe data

## 📞 Support

- 📖 [Documentation](docs/)
- 🐛 [Issue Tracker](https://github.com/ArqonAi/Pixelog/issues)
- 💬 [Discussions](https://github.com/ArqonAi/Pixelog/discussions)

---

**Made with ❤️ by [ArqonAi](https://github.com/ArqonAi)**
