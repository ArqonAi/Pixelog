package bench

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// HashEmbedder is a deterministic, local-only embedder useful for
// reproducible CI benchmark runs and unit tests. It is NOT a quality
// substitute for OpenAI/Ollama embeddings — for real benchmark numbers
// configure a real EmbeddingProvider.
//
// The implementation uses SHA-256 token hashing into a fixed-dimension
// bag-of-words vector with TF normalisation.
type HashEmbedder struct {
	Dim int
}

// NewHashEmbedder constructs an embedder with the given dimension.
// Default dimension is 384 (matches all-MiniLM-L6-v2).
func NewHashEmbedder(dim int) *HashEmbedder {
	if dim <= 0 {
		dim = 384
	}
	return &HashEmbedder{Dim: dim}
}

// GenerateEmbedding implements memory.Embedder.
func (e *HashEmbedder) GenerateEmbedding(_ context.Context, text string) ([]float32, error) {
	vec := make([]float32, e.Dim)
	if text == "" {
		return vec, nil
	}

	tokens := tokeniseEmbed(text)
	if len(tokens) == 0 {
		return vec, nil
	}

	for _, tok := range tokens {
		// Hash to two slots with opposite signs to balance the vector.
		h1 := fnv.New32a()
		h1.Write([]byte(tok))
		idx1 := int(h1.Sum32()) % e.Dim
		if idx1 < 0 {
			idx1 += e.Dim
		}

		sum := sha256.Sum256([]byte(tok))
		idx2 := int(binary.BigEndian.Uint32(sum[:4])) % e.Dim
		if idx2 < 0 {
			idx2 += e.Dim
		}
		sign := float32(1)
		if sum[4]&1 == 0 {
			sign = -1
		}

		vec[idx1] += 1
		vec[idx2] += sign
	}

	// L2 normalise
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm == 0 {
		return vec, nil
	}
	for i := range vec {
		vec[i] /= norm
	}
	return vec, nil
}

// GetDimensions implements search.EmbeddingProvider.
func (e *HashEmbedder) GetDimensions() int { return e.Dim }

// GetProviderName implements search.EmbeddingProvider.
func (e *HashEmbedder) GetProviderName() string { return "hash" }

func tokeniseEmbed(s string) []string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Fields(b.String())
}
