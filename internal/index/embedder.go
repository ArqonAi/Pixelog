package index

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SimpleEmbedder implements embedding service using OpenAI API
// No mock embeddings - real semantic understanding only
type SimpleEmbedder struct {
	provider string
	apiKey   string
	model    string
	dim      int
	client   *http.Client
}

// NewSimpleEmbedder creates a new embedder (requires API key)
func NewSimpleEmbedder(provider, apiKey, model string) *SimpleEmbedder {
	if apiKey == "" {
		panic("API key required for embeddings - no mock embeddings supported")
	}
	
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
		return nil, fmt.Errorf("unsupported provider: %s (use 'openai' or 'openrouter')", e.provider)
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

// embedOpenRouter uses OpenRouter embedding API
func (e *SimpleEmbedder) embedOpenRouter(text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"input": text,
		"model": "openai/text-embedding-3-small", // OpenRouter proxies to OpenAI
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/ArqonAi/Pixelog")
	
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
