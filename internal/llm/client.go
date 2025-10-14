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
	
	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	if c.provider == "openai" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	} else if c.provider == "openrouter" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("HTTP-Referer", "https://github.com/ArqonAi/Pixelog")
		req.Header.Set("X-Title", "Pixe CLI")
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
