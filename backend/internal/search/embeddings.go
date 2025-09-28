package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ArqonAi/Pixelog/backend/pkg/config"
)

type EmbeddingProvider interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GetDimensions() int
	GetProviderName() string
}

// OpenAI Provider
type OpenAIProvider struct {
	apiKey string
	model  string
	client *http.Client
}

type OpenAIRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

type OpenAIResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey: apiKey,
		model:  "text-embedding-3-small",
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *OpenAIProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if len(text) > 8000 {
		text = text[:8000]
	}

	reqBody := OpenAIRequest{Input: text, Model: p.model}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var openAIResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(openAIResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return openAIResp.Data[0].Embedding, nil
}

func (p *OpenAIProvider) GetDimensions() int { return 1536 }
func (p *OpenAIProvider) GetProviderName() string { return "OpenAI" }

// OpenRouter Provider
type OpenRouterProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewOpenRouterProvider(apiKey, model string) *OpenRouterProvider {
	return &OpenRouterProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *OpenRouterProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if len(text) > 8000 {
		text = text[:8000]
	}

	reqBody := OpenAIRequest{Input: text, Model: p.model}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/ArqonAi/Pixelog")
	req.Header.Set("X-Title", "Pixelog")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenRouter API error (status %d): %s", resp.StatusCode, string(body))
	}

	var openAIResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(openAIResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return openAIResp.Data[0].Embedding, nil
}

func (p *OpenRouterProvider) GetDimensions() int { return 1536 } // Most models use 1536
func (p *OpenRouterProvider) GetProviderName() string { return "OpenRouter" }

// Google Gemini Provider
type GeminiProvider struct {
	apiKey string
	client *http.Client
}

type GeminiRequest struct {
	Model string `json:"model"`
	Content struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"content"`
}

type GeminiResponse struct {
	Embeddings []struct {
		Values []float32 `json:"values"`
	} `json:"embeddings"`
}

func NewGeminiProvider(apiKey string) *GeminiProvider {
	return &GeminiProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *GeminiProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if len(text) > 8000 {
		text = text[:8000]
	}

	reqBody := GeminiRequest{
		Model: "models/embedding-001",
	}
	reqBody.Content.Parts = []struct {
		Text string `json:"text"`
	}{{Text: text}}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/embedding-001:embedContent?key=%s", p.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(body))
	}

	var geminiResp GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return geminiResp.Embeddings[0].Values, nil
}

func (p *GeminiProvider) GetDimensions() int { return 768 }
func (p *GeminiProvider) GetProviderName() string { return "Gemini" }

// xAI Grok Provider
type GrokProvider struct {
	apiKey string
	client *http.Client
}

func NewGrokProvider(apiKey string) *GrokProvider {
	return &GrokProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *GrokProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if len(text) > 8000 {
		text = text[:8000]
	}

	reqBody := OpenAIRequest{Input: text, Model: "text-embedding-grok"}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.x.ai/v1/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Grok API error (status %d): %s", resp.StatusCode, string(body))
	}

	var grokResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&grokResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(grokResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return grokResp.Data[0].Embedding, nil
}

func (p *GrokProvider) GetDimensions() int { return 1024 }
func (g *GrokProvider) GetProviderName() string {
	return "xAI Grok"
}


// Ollama Provider
type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaResponse struct {
	Embedding []float32 `json:"embedding"`
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	return &OllamaProvider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		model:   model,
		client:  &http.Client{Timeout: 60 * time.Second}, // Longer timeout for local models
	}
}

func (p *OllamaProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if len(text) > 32000 { // Ollama can handle more text
		text = text[:32000]
	}

	reqBody := OllamaRequest{Model: p.model, Prompt: text}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(ollamaResp.Embedding) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return ollamaResp.Embedding, nil
}

func (p *OllamaProvider) GetDimensions() int { return 768 } // nomic-embed-text default
func (p *OllamaProvider) GetProviderName() string { return "Ollama" }

// Factory function to create embedding provider based on config
func NewEmbeddingProvider(cfg *config.Config) (EmbeddingProvider, error) {
	providerName := strings.ToLower(cfg.EmbeddingProvider)

	// Auto-detect available providers
	if providerName == "auto" {
		if cfg.OpenAIAPIKey != "" {
			return NewOpenAIProvider(cfg.OpenAIAPIKey), nil
		}
		if cfg.OpenRouterAPIKey != "" {
			return NewOpenRouterProvider(cfg.OpenRouterAPIKey, cfg.OpenRouterModel), nil
		}
		if cfg.GoogleAPIKey != "" {
			return NewGeminiProvider(cfg.GoogleAPIKey), nil
		}
		if cfg.XAIAPIKey != "" {
			return NewGrokProvider(cfg.XAIAPIKey), nil
		}
		// Try Ollama as fallback
		if isOllamaAvailable(cfg.OllamaBaseURL) {
			return NewOllamaProvider(cfg.OllamaBaseURL, cfg.OllamaModel), nil
		}
		return nil, fmt.Errorf("no embedding providers available")
	}

	// Specific provider requested
	switch providerName {
	case "openai":
		if cfg.OpenAIAPIKey == "" {
			return nil, fmt.Errorf("OpenAI API key not configured")
		}
		return NewOpenAIProvider(cfg.OpenAIAPIKey), nil
	case "openrouter":
		if cfg.OpenRouterAPIKey == "" {
			return nil, fmt.Errorf("OpenRouter API key not configured")
		}
		return NewOpenRouterProvider(cfg.OpenRouterAPIKey, cfg.OpenRouterModel), nil
	case "google", "gemini":
		if cfg.GoogleAPIKey == "" {
			return nil, fmt.Errorf("Google API key not configured")
		}
		return NewGeminiProvider(cfg.GoogleAPIKey), nil
	case "xai", "grok":
		if cfg.XAIAPIKey == "" {
			return nil, fmt.Errorf("xAI API key not configured")
		}
		return NewGrokProvider(cfg.XAIAPIKey), nil
	case "ollama":
		if !isOllamaAvailable(cfg.OllamaBaseURL) {
			return nil, fmt.Errorf("Ollama not available at %s", cfg.OllamaBaseURL)
		}
		return NewOllamaProvider(cfg.OllamaBaseURL, cfg.OllamaModel), nil
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", providerName)
	}
}

func isOllamaAvailable(baseURL string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(strings.TrimSuffix(baseURL, "/") + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
