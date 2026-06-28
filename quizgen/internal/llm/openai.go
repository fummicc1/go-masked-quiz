package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIClient calls an OpenAI-compatible /chat/completions endpoint. It works
// with Cloudflare Workers AI (via WorkersAIBaseURL) and any other provider that
// speaks the OpenAI schema (Together, Fireworks, Groq, OpenRouter, …).
type OpenAIClient struct {
	HTTP    *http.Client
	BaseURL string // e.g. https://api.cloudflare.com/client/v4/accounts/<id>/ai/v1
	APIKey  string
	Model   string
}

// NewOpenAIClient returns a client for model at an OpenAI-compatible baseURL.
func NewOpenAIClient(baseURL, apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		HTTP:    &http.Client{Timeout: 10 * time.Minute},
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
	}
}

// WorkersAIBaseURL builds the OpenAI-compatible base URL for a Cloudflare
// account's Workers AI.
func WorkersAIBaseURL(accountID string) string {
	return fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/v1", accountID)
}

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaiRequest struct {
	Model          string         `json:"model"`
	Messages       []oaiMessage   `json:"messages"`
	Temperature    int            `json:"temperature"`
	Seed           int64          `json:"seed"`
	Stream         bool           `json:"stream"`
	ResponseFormat map[string]any `json:"response_format,omitempty"`
}

type oaiResponse struct {
	Choices []struct {
		Message oaiMessage `json:"message"`
	} `json:"choices"`
	Error any `json:"error"`
}

// Generate sends a system + user prompt and returns the assistant's content.
// temperature 0 + a fixed seed + JSON response format make the output strict
// JSON and as reproducible as the provider allows.
func (c *OpenAIClient) Generate(ctx context.Context, system, user string, seed int64) (string, error) {
	if c.Model == "" {
		return "", fmt.Errorf("llm: model is required")
	}
	if c.APIKey == "" {
		return "", fmt.Errorf("llm: API key is required")
	}
	payload, err := json.Marshal(oaiRequest{
		Model: c.Model,
		Messages: []oaiMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature:    0,
		Seed:           seed,
		Stream:         false,
		ResponseFormat: map[string]any{"type": "json_object"},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: call %s: %w", c.BaseURL, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm: HTTP %d: %s", res.StatusCode, truncate(string(body), 300))
	}
	var parsed oaiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("llm: decode response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("llm: provider error: %v", parsed.Error)
	}
	if len(parsed.Choices) == 0 || parsed.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("llm: empty response")
	}
	return parsed.Choices[0].Message.Content, nil
}
