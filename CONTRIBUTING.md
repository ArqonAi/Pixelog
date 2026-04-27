# Contributing to Pixe-Core

We welcome contributions to Pixe-Core! This document provides guidelines for contributing to the project.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Code Style](#code-style)
- [Security](#security)

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/Pixelog.git
   cd Pixelog
   ```
3. **Add the upstream remote**:
   ```bash
   git remote add upstream https://github.com/ArqonAi/Pixelog.git
   ```

## Development Setup

### Prerequisites

- Go 1.21 or higher
- Git

### Build and Test

```bash
# Install dependencies
go mod tidy

# Build the CLI tool
go build ./cmd/pixe

# Run tests
go test ./...

# Run example
go run examples/basic_usage.go
```

## Making Changes

### Before You Start

1. **Check existing issues** - Look for related issues or discussions
2. **Create an issue** - For new features or significant changes
3. **Get feedback** - Discuss your approach before implementation

### Branch Naming

Use descriptive branch names:
- `feature/add-compression-algorithm`
- `fix/encryption-key-validation`
- `docs/update-cli-examples`
- `refactor/crypto-package-structure`

### Commit Messages

Follow conventional commit format:
```
type(scope): description

- feat(crypto): add ChaCha20-Poly1305 encryption support
- fix(cli): handle invalid file paths gracefully
- docs(readme): update installation instructions
- test(converter): add edge case tests for large files
```

## Testing

### Writing Tests

- Add tests for all new functionality
- Ensure tests cover edge cases and error conditions
- Use table-driven tests for multiple scenarios
- Mock external dependencies

### Test Categories

```bash
# Unit tests
go test ./pkg/...

# Integration tests
go test ./cmd/...

# Example tests
go test ./examples/...
```

### Test Coverage

Maintain high test coverage:
```bash
go test -cover ./...
```

## Submitting Changes

1. **Update your fork**:
   ```bash
   git fetch upstream
   git checkout main
   git merge upstream/main
   ```

2. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Make your changes** and commit them

4. **Push to your fork**:
   ```bash
   git push origin feature/your-feature-name
   ```

5. **Create a Pull Request** on GitHub

### Pull Request Guidelines

- **Clear title and description** - Explain what and why
- **Reference issues** - Use "Fixes #123" or "Closes #456"
- **Keep changes focused** - One feature/fix per PR
- **Update documentation** - Include relevant docs updates
- **Add tests** - Ensure new code is tested

## Code Style

### Go Guidelines

Follow standard Go conventions:
- Use `gofmt` for formatting
- Follow `golint` recommendations
- Use meaningful variable and function names
- Add comments for exported functions and packages
- Handle errors explicitly

### Package Structure

```go
// Package comment explaining purpose
package packagename

import (
    // Standard library first
    "fmt"
    "os"
    
    // Third-party packages
    "github.com/external/package"
    
    // Internal packages last
    "github.com/ArqonAi/Pixelog/pkg/crypto"
)
```

### Error Handling

```go
// Good: Descriptive error messages
if err != nil {
    return fmt.Errorf("failed to encrypt data: %w", err)
}

// Good: Check all errors
data, err := os.ReadFile(filename)
if err != nil {
    return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
}
```

## Security

### Reporting Security Issues

**DO NOT** report security vulnerabilities in public issues.

Instead:
1. Email security@arqon.ai with details
2. Include steps to reproduce
3. Wait for acknowledgment before public disclosure

### Security Guidelines

- Never hardcode secrets or API keys
- Use secure random number generation for cryptographic operations
- Validate all inputs
- Follow secure coding practices
- Keep dependencies updated

### Cryptographic Changes

For changes to encryption/decryption:
- Consult cryptographic experts
- Provide detailed security analysis
- Include test vectors
- Document breaking changes clearly

## Documentation

### Code Documentation

```go
// ConvertFile converts a single file to .pixe format with optional encryption.
// It returns the path to the created .pixe file or an error if conversion fails.
//
// Parameters:
//   - inputPath: Path to the input file
//   - opts: Conversion options including encryption key and output path
//
// Returns:
//   - string: Path to the created .pixe file
//   - error: Error if conversion fails
func (c *Converter) ConvertFile(inputPath string, opts *ConvertOptions) (string, error) {
    // Implementation...
}
```

### README Updates

When adding features:
- Update feature list
- Add usage examples
- Update CLI reference if applicable

## Release Process

### Versioning

We follow [Semantic Versioning](https://semver.org/):
- `MAJOR.MINOR.PATCH`
- Breaking changes increment MAJOR
- New features increment MINOR
- Bug fixes increment PATCH

### Changelog

Maintain CHANGELOG.md with:
- Added features
- Changed behavior
- Deprecated features
- Removed features
- Fixed bugs
- Security updates

## Questions?

- Open an issue for bugs or feature requests
- Start a discussion for questions or ideas
- Check existing issues and discussions first

## Recognition

Contributors will be recognized in:
- CHANGELOG.md for their contributions
- GitHub contributors list
- Release notes for significant contributions

Thank you for contributing to Pixe-Core! 🚀
