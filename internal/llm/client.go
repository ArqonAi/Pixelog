package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// Default local inference endpoint (OpenAI-compatible: llama.cpp / vLLM / LM Studio)
	DefaultLocalURL = "http://localhost:8089/v1/chat/completions"
	// OpenRouter fallback
	OpenRouterURL = "https://openrouter.ai/api/v1/chat/completions"
)

// Client for LLM inference — tries local inference first, falls back to OpenRouter.
type Client struct {
	model    string
	apiKey   string
	localURL string
	client   *http.Client
}

// NewClient creates a dual-mode client (local inference + OpenRouter fallback).
func NewClient(model, apiKey string) *Client {
	return &Client{
		model:    model,
		apiKey:   apiKey,
		localURL: DefaultLocalURL,
		client:   &http.Client{Timeout: 90 * time.Second},
	}
}

// NewLocalClient creates a client that only uses the local endpoint (no cloud).
func NewLocalClient() *Client {
	return &Client{
		model:    "local",
		localURL: DefaultLocalURL,
		client:   &http.Client{Timeout: 90 * time.Second},
	}
}

// SetLocalURL overrides the default local inference URL.
func (c *Client) SetLocalURL(url string) {
	c.localURL = url
}

// Chat sends a prompt and returns the response.
// Priority: local inference → OpenRouter fallback.
func (c *Client) Chat(prompt string) (string, error) {
	// Try local endpoint first
	if c.localURL != "" {
		result, err := c.chatLocal(prompt)
		if err == nil {
			return result, nil
		}
		// Local unavailable — fall through to OpenRouter if API key exists
		if c.apiKey == "" {
			return "", fmt.Errorf("local inference failed and no OpenRouter API key: %w", err)
		}
	}

	return c.chatOpenRouter(prompt)
}

// chatLocal sends a request to the local endpoint with a fast connect
// timeout (3s) so the OpenRouter fallback kicks in promptly when the
// local server isn't running.
func (c *Client) chatLocal(prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": 2048,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.localURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	localClient := &http.Client{Timeout: 3 * time.Second}
	resp, err := localClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("local inference unavailable: %w", err)
	}
	defer resp.Body.Close()

	return c.parseOpenAIResponse(resp)
}

// chatOpenRouter sends a request to OpenRouter as fallback.
func (c *Client) chatOpenRouter(prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", OpenRouterURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/ArqonAi/Pixelog")
	req.Header.Set("X-Title", "Pixe CLI")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenRouter request failed: %w", err)
	}
	defer resp.Body.Close()

	return c.parseOpenAIResponse(resp)
}

// parseOpenAIResponse parses a standard OpenAI-compatible chat completion response.
func (c *Client) parseOpenAIResponse(resp *http.Response) (string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

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
		return "", fmt.Errorf("no response from model")
	}

	return result.Choices[0].Message.Content, nil
}
