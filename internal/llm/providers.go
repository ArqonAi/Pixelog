package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Provider identifies an upstream LLM service.
type Provider string

const (
	ProviderOpenAI     Provider = "openai"
	ProviderOpenRouter Provider = "openrouter"
	ProviderAnthropic  Provider = "anthropic"
	ProviderGemini     Provider = "gemini"
	ProviderGroq       Provider = "groq"
	ProviderXAI        Provider = "xai"
	ProviderNVIDIA     Provider = "nvidia"
	ProviderMoonshot   Provider = "moonshot"
	ProviderDeepSeek   Provider = "deepseek"
	ProviderZAI        Provider = "zai"
	ProviderLocal      Provider = "local"
	ProviderOllama     Provider = "ollama"
)

// AllProviders returns every supported provider name.
func AllProviders() []Provider {
	return []Provider{
		ProviderOpenAI, ProviderOpenRouter, ProviderAnthropic, ProviderGemini,
		ProviderGroq, ProviderXAI, ProviderNVIDIA, ProviderMoonshot,
		ProviderDeepSeek, ProviderZAI, ProviderLocal, ProviderOllama,
	}
}

// providerSpec encodes the per-provider HTTP API surface.
type providerSpec struct {
	endpoint    string
	authHeader  string // "Authorization: Bearer X" (default) or "x-api-key: X" (anthropic)
	envVar      string
	defaultModel string
	dialect     string // "openai", "anthropic", "gemini", "ollama"
	extraHeaders map[string]string
}

func providerSpecs() map[Provider]providerSpec {
	return map[Provider]providerSpec{
		ProviderOpenAI: {
			endpoint:     "https://api.openai.com/v1/chat/completions",
			envVar:       "OPENAI_API_KEY",
			defaultModel: "gpt-4o-mini",
			dialect:      "openai",
		},
		ProviderOpenRouter: {
			endpoint:     "https://openrouter.ai/api/v1/chat/completions",
			envVar:       "OPENROUTER_API_KEY",
			defaultModel: "openai/gpt-4o-mini",
			dialect:      "openai",
			extraHeaders: map[string]string{
				"HTTP-Referer": "https://github.com/ArqonAi/Pixelog",
				"X-Title":      "Pixelog",
			},
		},
		ProviderAnthropic: {
			endpoint:     "https://api.anthropic.com/v1/messages",
			authHeader:   "x-api-key",
			envVar:       "ANTHROPIC_API_KEY",
			defaultModel: "claude-3-5-sonnet-latest",
			dialect:      "anthropic",
			extraHeaders: map[string]string{
				"anthropic-version": "2023-06-01",
			},
		},
		ProviderGemini: {
			// Gemini uses query-string auth: ?key=X — handled in dispatcher.
			endpoint:     "https://generativelanguage.googleapis.com/v1beta/models",
			envVar:       "GEMINI_API_KEY",
			defaultModel: "gemini-1.5-pro-latest",
			dialect:      "gemini",
		},
		ProviderGroq: {
			endpoint:     "https://api.groq.com/openai/v1/chat/completions",
			envVar:       "GROQ_API_KEY",
			defaultModel: "llama-3.3-70b-versatile",
			dialect:      "openai",
		},
		ProviderXAI: {
			endpoint:     "https://api.x.ai/v1/chat/completions",
			envVar:       "XAI_API_KEY",
			defaultModel: "grok-2-latest",
			dialect:      "openai",
		},
		ProviderNVIDIA: {
			endpoint:     "https://integrate.api.nvidia.com/v1/chat/completions",
			envVar:       "NVIDIA_API_KEY",
			defaultModel: "meta/llama-3.3-70b-instruct",
			dialect:      "openai",
		},
		ProviderMoonshot: {
			endpoint:     "https://api.moonshot.cn/v1/chat/completions",
			envVar:       "MOONSHOT_API_KEY",
			defaultModel: "moonshot-v1-8k",
			dialect:      "openai",
		},
		ProviderDeepSeek: {
			endpoint:     "https://api.deepseek.com/v1/chat/completions",
			envVar:       "DEEPSEEK_API_KEY",
			defaultModel: "deepseek-chat",
			dialect:      "openai",
		},
		ProviderZAI: {
			endpoint:     "https://api.z.ai/api/paas/v4/chat/completions",
			envVar:       "ZAI_API_KEY",
			defaultModel: "glm-4.6",
			dialect:      "openai",
		},
		ProviderLocal: {
			endpoint:     DefaultLocalURL,
			envVar:       "",
			defaultModel: "local",
			dialect:      "openai",
		},
		ProviderOllama: {
			endpoint:     "http://localhost:11434/api/chat",
			envVar:       "",
			defaultModel: "llama3.2",
			dialect:      "ollama",
		},
	}
}

// MultiClient is a chat client supporting any of the registered providers.
type MultiClient struct {
	provider Provider
	model    string
	apiKey   string
	spec     providerSpec
	http     *http.Client
}

// NewMultiClient constructs a chat client for the given provider.
// If apiKey is empty, the provider's environment variable is consulted.
// If model is empty, the provider's default model is used.
func NewMultiClient(provider Provider, model, apiKey string) (*MultiClient, error) {
	specs := providerSpecs()
	spec, ok := specs[provider]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %q (supported: %v)", provider, AllProviders())
	}
	if apiKey == "" && spec.envVar != "" {
		apiKey = os.Getenv(spec.envVar)
	}
	if apiKey == "" && spec.envVar != "" && provider != ProviderLocal && provider != ProviderOllama {
		return nil, fmt.Errorf("provider %q requires API key (set %s)", provider, spec.envVar)
	}
	if model == "" {
		model = spec.defaultModel
	}
	return &MultiClient{
		provider: provider,
		model:    model,
		apiKey:   apiKey,
		spec:     spec,
		http:     &http.Client{Timeout: 120 * time.Second},
	}, nil
}

// Provider returns the configured provider.
func (c *MultiClient) Provider() Provider { return c.provider }

// Model returns the configured model.
func (c *MultiClient) Model() string { return c.model }

// Chat sends a single user prompt and returns the model's reply.
// Implements the bench.LLMChat interface.
func (c *MultiClient) Chat(prompt string) (string, error) {
	return c.ChatCtx(context.Background(), prompt)
}

// ChatCtx is Chat with an explicit context.
func (c *MultiClient) ChatCtx(ctx context.Context, prompt string) (string, error) {
	switch c.spec.dialect {
	case "openai":
		return c.chatOpenAILike(ctx, prompt)
	case "anthropic":
		return c.chatAnthropic(ctx, prompt)
	case "gemini":
		return c.chatGemini(ctx, prompt)
	case "ollama":
		return c.chatOllama(ctx, prompt)
	default:
		return "", fmt.Errorf("unsupported dialect: %q", c.spec.dialect)
	}
}

func (c *MultiClient) chatOpenAILike(ctx context.Context, prompt string) (string, error) {
	body := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.spec.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		header := c.spec.authHeader
		if header == "" {
			header = "Authorization"
			req.Header.Set(header, "Bearer "+c.apiKey)
		} else {
			req.Header.Set(header, c.apiKey)
		}
	}
	for k, v := range c.spec.extraHeaders {
		req.Header.Set(k, v)
	}
	return parseOpenAILike(c.http.Do(req))
}

func parseOpenAILike(resp *http.Response, err error) (string, error) {
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, truncResp(body, 500))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("no choices in response: %s", truncResp(body, 200))
	}
	return out.Choices[0].Message.Content, nil
}

func (c *MultiClient) chatAnthropic(ctx context.Context, prompt string) (string, error) {
	body := map[string]interface{}{
		"model":      c.model,
		"max_tokens": 4096,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.spec.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	for k, v := range c.spec.extraHeaders {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Anthropic %d: %s", resp.StatusCode, truncResp(bodyBytes, 500))
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return "", fmt.Errorf("anthropic parse: %w", err)
	}
	var sb strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return sb.String(), nil
}

func (c *MultiClient) chatGemini(ctx context.Context, prompt string) (string, error) {
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", c.spec.endpoint, c.model, c.apiKey)
	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{{"text": prompt}},
			},
		},
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Gemini %d: %s", resp.StatusCode, truncResp(bodyBytes, 500))
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return "", fmt.Errorf("gemini parse: %w", err)
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini empty response: %s", truncResp(bodyBytes, 200))
	}
	var sb strings.Builder
	for _, p := range out.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}
	return sb.String(), nil
}

func (c *MultiClient) chatOllama(ctx context.Context, prompt string) (string, error) {
	body := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.spec.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Ollama %d: %s", resp.StatusCode, truncResp(bodyBytes, 500))
	}
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return "", fmt.Errorf("ollama parse: %w", err)
	}
	return out.Message.Content, nil
}

func truncResp(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
