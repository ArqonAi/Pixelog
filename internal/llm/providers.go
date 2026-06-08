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

// ChatCtx is Chat with an explicit context. Wraps the per-dialect
// transport with bounded exponential-backoff retry on transient
// upstream failures (429 rate limit, 5xx overload, network errors).
//
// Retry policy is intentionally conservative — no retries on 4xx
// other than 429 (those are deterministic auth/billing/format
// errors that won't resolve), and capped attempts so a permanently
// degraded provider doesn't hang a benchmark for hours.
func (c *MultiClient) ChatCtx(ctx context.Context, prompt string) (string, error) {
	const maxAttempts = 5
	// Base backoff 2s; doubled per attempt: 2, 4, 8, 16. Total
	// worst-case wait ≈30s before the final attempt. Production
	// rate-limit windows on Anthropic / OpenAI are typically 10-60s,
	// so this fits the recovery window without hanging callers.
	backoff := 2 * time.Second

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		out, err := c.chatOnce(ctx, prompt)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !isTransientLLMError(err) {
			return "", err
		}
		// Exponential backoff with jitter-free deterministic
		// schedule. Sleep is context-cancellable so a parent
		// timeout still aborts promptly.
		if attempt == maxAttempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return "", fmt.Errorf("llm: exhausted retries: %w", lastErr)
}

// chatOnce dispatches a single request without retry. Extracted so
// ChatCtx can wrap it in a retry loop without duplicating the
// dialect switch.
func (c *MultiClient) chatOnce(ctx context.Context, prompt string) (string, error) {
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

// isTransientLLMError classifies an upstream error as worth
// retrying. Matches the status-code substrings emitted by every
// dialect's error path:
//   - 429: rate limit (any provider)
//   - 5xx: server-side overload (Anthropic emits 529 specifically
//     for "Overloaded"; OpenAI / others emit 500/502/503)
//
// Network-level errors (connection reset, EOF, i/o timeout) also
// match. Auth errors (401/403), billing (402), and bad-request
// (400) are NOT retried because they're deterministic.
func isTransientLLMError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, " 429"),
		strings.Contains(msg, " 500"),
		strings.Contains(msg, " 502"),
		strings.Contains(msg, " 503"),
		strings.Contains(msg, " 529"),
		strings.Contains(msg, "overloaded"),
		strings.Contains(msg, "Overloaded"),
		strings.Contains(msg, "rate_limit"),
		strings.Contains(msg, "rate limit"),
		strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "EOF"),
		strings.Contains(msg, "i/o timeout"):
		return true
	}
	return false
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
	// think:false disables the reasoning trace on R1-class models
	// (deepseek-r1, qwq, etc.) so the response lands in
	// message.content instead of being trapped in message.thinking.
	// num_predict:4096 gives long-form answers room to fit; the
	// Ollama default (~128) truncates LoCoMo answers mid-sentence.
	body := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
		"think":  false,
		"options": map[string]interface{}{
			"num_predict": 4096,
		},
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
			Content  string `json:"content"`
			Thinking string `json:"thinking"`
		} `json:"message"`
	}
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return "", fmt.Errorf("ollama parse: %w", err)
	}
	// Belt-and-braces: if a model still emits its answer in the
	// thinking field (older Ollama versions ignore think:false),
	// fall back to that so the bench doesn't see an empty string.
	if out.Message.Content == "" && out.Message.Thinking != "" {
		return out.Message.Thinking, nil
	}
	return out.Message.Content, nil
}

func truncResp(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
