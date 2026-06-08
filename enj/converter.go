package enj

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ArqonAi/Pixelog/internal/crypto"
	"github.com/ArqonAi/Pixelog/internal/qr"
	"github.com/ArqonAi/Pixelog/internal/video"
	"github.com/ArqonAi/Pixelog/pkg/config"
)

// Converter creates and reads .enj files using the Pixelog pipeline.
// An .enj file is structurally identical to a .pixe file — an MP4 video
// where each frame is a QR code containing chunked, optionally encrypted data.
// The payload is a JSON-serialized AgentManifest.
type Converter struct {
	qrGen   *qr.Generator
	video   *video.Maker
	enc     *crypto.EncryptionService
	tempDir string
}

// NewConverter creates a new .enj converter.
func NewConverter(tempDir string) (*Converter, error) {
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	qrGen, err := qr.New(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create QR generator: %w", err)
	}

	videoMaker, err := video.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create video maker: %w", err)
	}

	return &Converter{
		qrGen:   qrGen,
		video:   videoMaker,
		enc:     crypto.NewEncryptionService(true),
		tempDir: tempDir,
	}, nil
}

// CreateENJ serializes an AgentManifest into a .enj file.
// If password is provided, the manifest (including the private key) is encrypted.
// The private key in the manifest is always encrypted separately with the password
// before the full manifest encryption, providing double protection.
func (c *Converter) CreateENJ(manifest *AgentManifest, outputPath string, password string) error {
	// Encrypt the private key within the manifest if password provided
	if password != "" && manifest.Identity.PrivateKey != "" {
		privKeyBytes, err := hex.DecodeString(manifest.Identity.PrivateKey)
		if err != nil {
			return fmt.Errorf("invalid private key hex: %w", err)
		}
		encPrivKey, err := c.enc.EncryptData(privKeyBytes, password)
		if err != nil {
			return fmt.Errorf("failed to encrypt private key: %w", err)
		}
		manifest.Identity.PrivateKey = hex.EncodeToString(encPrivKey)
	}

	manifest.UpdatedAt = time.Now()

	// Serialize manifest to JSON
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}

	// Optionally encrypt the full payload
	if password != "" {
		data, err = c.enc.EncryptData(data, password)
		if err != nil {
			return fmt.Errorf("failed to encrypt manifest: %w", err)
		}
	}

	// Chunk the data for QR encoding
	chunkSize := 2600 // leave room for QR metadata overhead
	var chunks []qr.Chunk
	encoded := hex.EncodeToString(data) // hex encode for safe QR transport

	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}

		chunks = append(chunks, qr.Chunk{
			ID:         fmt.Sprintf("enj_%s_%d", manifest.Identity.PublicKey[:8], len(chunks)),
			Index:      len(chunks),
			Data:       encoded[i:end],
			SourceFile: manifest.Identity.Name + ".enj",
			MimeType:   "application/enj+json",
			Hash:       manifest.Identity.PublicKey,
			Encrypted:  password != "",
			CreatedAt:  time.Now(),
		})
	}

	// Set total count
	for i := range chunks {
		chunks[i].Total = len(chunks)
	}

	// Generate QR frames
	framePaths, err := c.qrGen.GenerateFrames(chunks)
	if err != nil {
		return fmt.Errorf("failed to generate QR frames: %w", err)
	}
	defer func() {
		for _, p := range framePaths {
			os.Remove(p)
		}
	}()

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	// Create MP4 video
	cfg := &config.Config{
		FrameRate: 0.5,
		Quality:   23,
		ChunkSize: 2800,
	}

	metadata := map[string]interface{}{
		"format":       "enj",
		"version":      manifest.Version,
		"agent_name":   manifest.Identity.Name,
		"public_key":   manifest.Identity.PublicKey,
		"total_chunks": len(chunks),
		"encrypted":    password != "",
		"created_at":   manifest.CreatedAt,
	}

	if err := c.video.CreateVideo(framePaths, outputPath, metadata, cfg); err != nil {
		return fmt.Errorf("failed to create .enj video: %w", err)
	}

	return nil
}

// ReadENJ extracts and deserializes an AgentManifest from a .enj file.
// If the file is encrypted, password must be provided.
func (c *Converter) ReadENJ(enjPath string, password string) (*AgentManifest, error) {
	// Extract frames from .enj video using ffmpeg
	tempDir, err := os.MkdirTemp("", "enj-extract-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	framePattern := filepath.Join(tempDir, "frame_%05d.png")
	cmd := exec.Command("ffmpeg", "-y", "-i", enjPath, "-vf", "fps=1", framePattern)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to extract frames: %w", err)
	}

	frameFiles, err := filepath.Glob(filepath.Join(tempDir, "frame_*.png"))
	if err != nil {
		return nil, fmt.Errorf("failed to find frames: %w", err)
	}
	if len(frameFiles) == 0 {
		return nil, fmt.Errorf("no frames extracted from .enj file")
	}

	sort.Strings(frameFiles)

	// Decode QR codes from each frame
	var allChunks []qr.Chunk
	for _, frameFile := range frameFiles {
		chunk, err := qr.DecodeFrame(frameFile)
		if err != nil {
			continue
		}
		allChunks = append(allChunks, *chunk)
	}

	if len(allChunks) == 0 {
		return nil, fmt.Errorf("no valid QR codes found in .enj frames")
	}

	// Sort by index and reassemble
	sort.Slice(allChunks, func(i, j int) bool {
		return allChunks[i].Index < allChunks[j].Index
	})

	var reassembled strings.Builder
	for _, chunk := range allChunks {
		reassembled.WriteString(chunk.Data)
	}

	// Decode hex payload
	decoded, err := hex.DecodeString(reassembled.String())
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex data: %w", err)
	}

	// Decrypt if password provided
	if password != "" {
		decoded, err = c.enc.DecryptData(decoded, password)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt manifest: %w", err)
		}
	}

	// Deserialize
	var manifest AgentManifest
	if err := json.Unmarshal(decoded, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Decrypt private key if password provided
	if password != "" && manifest.Identity.PrivateKey != "" {
		encPrivKey, err := hex.DecodeString(manifest.Identity.PrivateKey)
		if err == nil {
			decPrivKey, err := c.enc.DecryptData(encPrivKey, password)
			if err == nil {
				manifest.Identity.PrivateKey = hex.EncodeToString(decPrivKey)
			}
		}
	}

	return &manifest, nil
}

// SignManifest signs the manifest with the agent's private key.
// Returns the hex-encoded signature.
func SignManifest(manifest *AgentManifest, privKeyHex string) (string, error) {
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}

	if len(privKeyBytes) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("invalid private key size: %d", len(privKeyBytes))
	}

	// Serialize without the private key for signing
	temp := *manifest
	temp.Identity.PrivateKey = ""
	data, err := json.Marshal(temp)
	if err != nil {
		return "", fmt.Errorf("failed to serialize for signing: %w", err)
	}

	sig := ed25519.Sign(ed25519.PrivateKey(privKeyBytes), data)
	return hex.EncodeToString(sig), nil
}

// VerifyManifest verifies a manifest signature using the agent's public key.
func VerifyManifest(manifest *AgentManifest, signatureHex string) (bool, error) {
	pubKeyBytes, err := hex.DecodeString(manifest.Identity.PublicKey)
	if err != nil {
		return false, fmt.Errorf("invalid public key: %w", err)
	}

	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("invalid signature: %w", err)
	}

	// Serialize without private key for verification
	temp := *manifest
	temp.Identity.PrivateKey = ""
	data, err := json.Marshal(temp)
	if err != nil {
		return false, fmt.Errorf("failed to serialize for verification: %w", err)
	}

	return ed25519.Verify(ed25519.PublicKey(pubKeyBytes), data, sigBytes), nil
}
