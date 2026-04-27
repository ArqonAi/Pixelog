# CI/CD for Pixelog CLI

## Overview
Automated workflows for testing, building, and releasing the Pixelog CLI tool.

## GitHub Actions Workflows

### 1. Test Workflow (`test.yml`)
**Trigger**: Every push and pull request to `main`

**What it does**:
- Runs tests on Linux, macOS, and Windows
- Checks code with `go vet` and `golangci-lint`
- Builds the CLI on all platforms
- Uploads code coverage to Codecov

**Status**: ✅ Runs automatically on every push

### 2. Release Workflow (`release.yml`)
**Trigger**: 
- Push to `main` branch (builds binaries)
- Push tags starting with `v*` (creates GitHub release)

**What it does**:
- Runs all tests
- Builds binaries for:
  - Linux (amd64, arm64)
  - macOS (amd64, arm64/Apple Silicon)
  - Windows (amd64)
- Creates checksums for all binaries
- Creates GitHub Release with all binaries (when tagged)

## Usage

### For Regular Development

Just push to `main`:
```bash
git add .
git commit -m "feat: your feature"
git push origin main
```

GitHub Actions will automatically:
- ✅ Run all tests
- ✅ Build binaries for all platforms
- ✅ Upload artifacts (available for 90 days)

### Creating a Release

When ready to release a new version:

```bash
# Tag the release
git tag v1.0.0

# Push the tag
git push origin v1.0.0
```

GitHub Actions will automatically:
- ✅ Run all tests
- ✅ Build all platform binaries
- ✅ Create a GitHub Release
- ✅ Attach all binaries to the release
- ✅ Generate release notes
- ✅ Create SHA256 checksums

### Download Released Binaries

Users can download from: `https://github.com/ArqonAi/Pixelog/releases`

Example:
```bash
# Linux (amd64)
wget https://github.com/ArqonAi/Pixelog/releases/download/v1.0.0/pixelog-linux-amd64
chmod +x pixelog-linux-amd64
sudo mv pixelog-linux-amd64 /usr/local/bin/pixelog

# macOS (Apple Silicon)
wget https://github.com/ArqonAi/Pixelog/releases/download/v1.0.0/pixelog-darwin-arm64
chmod +x pixelog-darwin-arm64
sudo mv pixelog-darwin-arm64 /usr/local/bin/pixelog

# Windows
# Download pixelog-windows-amd64.exe and run
```

## Versioning

Follow Semantic Versioning (semver):
- `v1.0.0` - Major release
- `v1.1.0` - Minor release (new features)
- `v1.1.1` - Patch release (bug fixes)

## Badges

Add these to your README.md:

```markdown
[![Tests](https://github.com/ArqonAi/Pixelog/actions/workflows/test.yml/badge.svg)](https://github.com/ArqonAi/Pixelog/actions/workflows/test.yml)
[![Release](https://github.com/ArqonAi/Pixelog/actions/workflows/release.yml/badge.svg)](https://github.com/ArqonAi/Pixelog/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ArqonAi/Pixelog)](https://goreportcard.com/report/github.com/ArqonAi/Pixelog)
```

## Installation Methods

### 1. Download Pre-built Binary (Recommended)
```bash
# From GitHub Releases
wget https://github.com/ArqonAi/Pixelog/releases/latest/download/pixelog-linux-amd64
chmod +x pixelog-linux-amd64
sudo mv pixelog-linux-amd64 /usr/local/bin/pixelog
```

### 2. Install with Go
```bash
go install github.com/ArqonAi/Pixelog/cmd/pixelog@latest
```

### 3. Build from Source
```bash
git clone https://github.com/ArqonAi/Pixelog.git
cd Pixelog
go build -o pixelog ./cmd/pixelog
```

## Continuous Deployment

The workflows are already set up! Just:

1. ✅ Push to `main` → Tests run + binaries build
2. ✅ Create tag → Release created with all binaries
3. ✅ Users download → Latest version available

## Monitoring

- **GitHub Actions**: Check [Actions tab](https://github.com/ArqonAi/Pixelog/actions)
- **Test Results**: View in GitHub Actions logs
- **Code Coverage**: Available on Codecov (if configured)
- **Build Status**: Visible with badges

## Next Steps

- [x] CI/CD workflows created
- [ ] Create first release with `git tag v1.0.0`
- [ ] Add badges to README.md
- [ ] Set up Codecov (optional)
- [ ] Add to package managers (homebrew, apt, etc.)

## Troubleshooting

### Tests Failing
- Check the Actions tab for detailed logs
- Run tests locally: `go test -v ./...`
- Fix issues and push again

### Build Failing
- Verify Go version compatibility
- Check for platform-specific code
- Test builds locally: `GOOS=linux GOARCH=amd64 go build ./cmd/pixelog`

### Release Not Creating
- Ensure tag starts with `v` (e.g., `v1.0.0`)
- Check you have write permissions
- Verify GITHUB_TOKEN has correct permissions

## Manual Release (if needed)

If GitHub Actions fails, release manually:

```bash
# Build for all platforms
./scripts/build-all.sh

# Create release on GitHub
gh release create v1.0.0 dist/* --generate-notes
```
