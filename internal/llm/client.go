package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client for OpenRouter API (supports all LLM providers)
type Client struct {
	model   string
	apiKey  string
	client  *http.Client
}

// NewClient creates OpenRouter client
func NewClient(model, apiKey string) *Client {
	return &Client{
		model:  model,
		apiKey: apiKey,
		client: &http.Client{},
	}
}

// Chat sends a prompt to OpenRouter and returns the response
func (c *Client) Chat(prompt string) (string, error) {
	// OpenRouter standard format (works for all models)
	reqBody := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	// OpenRouter headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/ArqonAi/Pixelog")
	req.Header.Set("X-Title", "Pixe CLI")
	
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
		return "", fmt.Errorf("OpenRouter API error %d: %s", resp.StatusCode, string(body))
	}
	
	// OpenRouter standard response format
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
		return "", fmt.Errorf("no response from OpenRouter")
	}
	
	return result.Choices[0].Message.Content, nil
}
