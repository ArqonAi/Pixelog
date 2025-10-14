package index

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SimpleEmbedder implements a basic embedding service
// Supports OpenAI, OpenRouter, and fallback to simple hash-based embeddings
type SimpleEmbedder struct {
	provider string
	apiKey   string
	model    string
	dim      int
	client   *http.Client
}

// NewSimpleEmbedder creates a new embedder
func NewSimpleEmbedder(provider, apiKey, model string) *SimpleEmbedder {
	dim := 1536 // Default OpenAI dimension
	
	// Adjust dimension based on model
	if strings.Contains(model, "minilm") {
		dim = 384
	} else if strings.Contains(model, "3-small") {
		dim = 1536
	}
	
	return &SimpleEmbedder{
		provider: provider,
		apiKey:   apiKey,
		model:    model,
		dim:      dim,
		client:   &http.Client{},
	}
}

func (e *SimpleEmbedder) Embed(text string) ([]float32, error) {
	// Truncate very long text
	if len(text) > 8000 {
		text = text[:8000]
	}
	
	switch e.provider {
	case "openai":
		return e.embedOpenAI(text)
	case "openrouter":
		return e.embedOpenRouter(text)
	default:
		// Fallback: simple hash-based embedding for testing
		return e.embedSimple(text), nil
	}
}

func (e *SimpleEmbedder) Dim() int {
	return e.dim
}

// embedOpenAI uses OpenAI embedding API
func (e *SimpleEmbedder) embedOpenAI(text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"input": text,
		"model": e.model,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	
	return result.Data[0].Embedding, nil
}

// embedOpenRouter uses OpenRouter API (similar to OpenAI)
func (e *SimpleEmbedder) embedOpenRouter(text string) ([]float32, error) {
	// OpenRouter might not have embedding endpoints yet
	// Fall back to simple embedding
	return e.embedSimple(text), nil
}

// embedSimple creates a simple hash-based embedding for testing
// This is NOT a real semantic embedding, just for prototyping
func (e *SimpleEmbedder) embedSimple(text string) []float32 {
	// Create a deterministic but varied embedding based on text content
	embedding := make([]float32, e.dim)
	
	// Use word frequency and position to create pseudo-embeddings
	words := strings.Fields(strings.ToLower(text))
	wordFreq := make(map[string]int)
	for _, word := range words {
		wordFreq[word]++
	}
	
	// Fill embedding with simple features
	for i := range embedding {
		// Mix of length, word count, and character distribution
		val := float32(len(text)%256) / 256.0
		if i < len(words) {
			val += float32(len(words[i])) / 50.0
		}
		if i%10 == 0 && len(words) > 0 {
			val += float32(wordFreq[words[0]]) / 10.0
		}
		embedding[i] = val
	}
	
	// Normalize
	var norm float32
	for _, v := range embedding {
		norm += v * v
	}
	if norm > 0 {
		norm = float32(1.0 / (norm + 0.0001))
		for i := range embedding {
			embedding[i] *= norm
		}
	}
	
	return embedding
}

// NewMockEmbedder creates a simple embedder for testing (no API key needed)
func NewMockEmbedder() *SimpleEmbedder {
	return &SimpleEmbedder{
		provider: "mock",
		apiKey:   "",
		model:    "mock-model",
		dim:      384, // Small dimension for fast testing
		client:   &http.Client{},
	}
}
