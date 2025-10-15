package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	provider string
	model    string
	apiKey   string
	baseURL  string
	client   *http.Client
}

func NewClient(provider, model, apiKey string) *Client {
	baseURL := ""
	
	switch provider {
	case "openai":
		baseURL = "https://api.openai.com/v1/chat/completions"
	case "openrouter":
		baseURL = "https://openrouter.ai/api/v1/chat/completions"
	case "google", "gemini":
		baseURL = "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":generateContent?key=" + apiKey
	case "anthropic", "claude":
		baseURL = "https://api.anthropic.com/v1/messages"
	case "xai", "grok":
		baseURL = "https://api.x.ai/v1/chat/completions"
	default:
		baseURL = "https://openrouter.ai/api/v1/chat/completions"
	}
	
	return &Client{
		provider: provider,
		model:    model,
		apiKey:   apiKey,
		baseURL:  baseURL,
		client:   &http.Client{},
	}
}

func (c *Client) Chat(prompt string) (string, error) {
	var reqBody map[string]interface{}
	
	// Different request formats per provider
	switch c.provider {
	case "google", "gemini":
		// Gemini request format
		reqBody = map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"parts": []map[string]string{
						{"text": prompt},
					},
				},
			},
		}
	case "anthropic", "claude":
		// Anthropic request format
		reqBody = map[string]interface{}{
			"model": c.model,
			"max_tokens": 4096,
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": prompt,
				},
			},
		}
	default:
		// OpenAI/OpenRouter/xAI format (standard)
		reqBody = map[string]interface{}{
			"model": c.model,
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": prompt,
				},
			},
		}
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	switch c.provider {
	case "openai":
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	case "openrouter":
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("HTTP-Referer", "https://github.com/ArqonAi/Pixelog")
		req.Header.Set("X-Title", "Pixe CLI")
	case "google", "gemini":
		// API key already in URL
	case "anthropic", "claude":
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "xai", "grok":
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse response based on provider
	switch c.provider {
	case "google", "gemini":
		// Gemini response format
		var result struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return "", fmt.Errorf("failed to parse Gemini response: %w", err)
		}
		if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
			return "", fmt.Errorf("no response from Gemini API")
		}
		return result.Candidates[0].Content.Parts[0].Text, nil
	
	case "anthropic", "claude":
		// Anthropic response format
		var result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return "", fmt.Errorf("failed to parse Anthropic response: %w", err)
		}
		if len(result.Content) == 0 {
			return "", fmt.Errorf("no response from Anthropic API")
		}
		return result.Content[0].Text, nil
	
	default:
		// OpenAI/OpenRouter/xAI format (standard)
		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}
		if len(result.Choices) == 0 {
			return "", fmt.Errorf("no response from API")
		}
		return result.Choices[0].Message.Content, nil
	}
}
