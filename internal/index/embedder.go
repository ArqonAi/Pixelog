package index

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SimpleEmbedder implements embedding service for multiple providers
// Supports: OpenAI, OpenRouter, Google Gemini, Anthropic, xAI Grok
// No mock embeddings - real semantic understanding only
type SimpleEmbedder struct {
	provider string
	apiKey   string
	model    string
	dim      int
	client   *http.Client
}

// GetDefaultModel returns the best embedding model for each provider
func GetDefaultModel(provider string) (string, int) {
	switch provider {
	case "openai":
		return "text-embedding-3-large", 3072 // Latest, best quality
	case "openrouter":
		return "openai/text-embedding-3-large", 3072 // OpenRouter proxy to OpenAI
	case "google", "gemini":
		return "models/text-embedding-004", 768 // Latest Gemini
	case "anthropic", "claude":
		// Anthropic doesn't have embeddings - proxy via OpenRouter
		return "openai/text-embedding-3-large", 3072
	case "xai", "grok":
		// xAI doesn't have embeddings yet - proxy via OpenRouter
		return "openai/text-embedding-3-large", 3072
	default:
		return "text-embedding-3-small", 1536 // Fallback
	}
}

// NewSimpleEmbedder creates a new embedder (requires API key)
func NewSimpleEmbedder(provider, apiKey, model string) *SimpleEmbedder {
	if apiKey == "" {
		panic("API key required for embeddings - no mock embeddings supported")
	}
	
	// Auto-select best model if not specified
	var dim int
	if model == "" || model == "auto" {
		model, dim = GetDefaultModel(provider)
	} else {
		// Infer dimension from model name
		if strings.Contains(model, "minilm") {
			dim = 384
		} else if strings.Contains(model, "3-small") {
			dim = 1536
		} else if strings.Contains(model, "3-large") {
			dim = 3072
		} else if strings.Contains(model, "embedding-004") {
			dim = 768
		} else {
			dim = 1536 // Default
		}
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
	case "google", "gemini":
		return e.embedGemini(text)
	case "anthropic", "claude":
		return e.embedAnthropic(text)
	case "xai", "grok":
		return e.embedXAI(text)
	default:
		return nil, fmt.Errorf("unsupported provider: %s (use 'openai', 'openrouter', 'gemini', 'anthropic', or 'xai')", e.provider)
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
		"model": e.model, // Uses model from GetDefaultModel or user-specified
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

// embedGemini uses Google Gemini embedding API
func (e *SimpleEmbedder) embedGemini(text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"content": map[string]interface{}{
			"parts": []map[string]string{
				{"text": text},
			},
		},
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	
	// Use embedding model
	model := "models/text-embedding-004"
	if e.model != "" && strings.Contains(e.model, "embedding") {
		model = e.model
	}
	
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s:embedContent?key=%s", model, e.apiKey)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	
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
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	if len(result.Embedding.Values) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	
	return result.Embedding.Values, nil
}

// embedAnthropic uses Anthropic's API (via OpenRouter proxy)
// Note: Anthropic doesn't have a direct embeddings API
func (e *SimpleEmbedder) embedAnthropic(text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"input": text,
		"model": e.model, // Uses OpenRouter proxy model
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

// embedXAI uses xAI Grok embedding API (via OpenRouter proxy)
// Note: xAI doesn't have embeddings API yet
func (e *SimpleEmbedder) embedXAI(text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"input": text,
		"model": e.model, // Uses OpenRouter proxy model
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
