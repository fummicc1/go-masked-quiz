// Package llm generates quiz content (summaries and concept fill-in-the-blank
// quizzes) from proposal text using a local ollama server. Generation runs
// locally and on demand; its validated output is cached to disk and committed,
// so CI never needs to call a model.
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

// DefaultOllamaURL is the local ollama server address.
const DefaultOllamaURL = "http://localhost:11434"

// OllamaClient talks to a local ollama server's /api/chat endpoint.
type OllamaClient struct {
	HTTP  *http.Client
	URL   string
	Model string
}

// NewOllamaClient returns an OllamaClient for model at url (DefaultOllamaURL if empty).
func NewOllamaClient(url, model string) *OllamaClient {
	if url == "" {
		url = DefaultOllamaURL
	}
	return &OllamaClient{
		HTTP:  &http.Client{Timeout: 10 * time.Minute},
		URL:   url,
		Model: model,
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string         `json:"model"`
	Messages []chatMessage  `json:"messages"`
	Stream   bool           `json:"stream"`
	Format   string         `json:"format"`
	Options  map[string]any `json:"options"`
}

type chatResponse struct {
	Message chatMessage `json:"message"`
	Done    bool        `json:"done"`
	Error   string      `json:"error"`
}

// Generate sends a system and user prompt and returns the assistant's response
// content. format=json + temperature 0 + a fixed seed make the output strict
// JSON and as reproducible as the local model allows.
func (c *OllamaClient) Generate(ctx context.Context, system, user string, seed int64) (string, error) {
	if c.Model == "" {
		return "", fmt.Errorf("llm: model is required")
	}
	payload, err := json.Marshal(chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Stream:  false,
		Format:  "json",
		Options: map[string]any{"temperature": 0, "seed": seed},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: call ollama (is it running at %s?): %w", c.URL, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm: ollama HTTP %d: %s", res.StatusCode, truncate(string(body), 200))
	}
	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("llm: decode ollama response: %w", err)
	}
	if parsed.Error != "" {
		return "", fmt.Errorf("llm: ollama error: %s", parsed.Error)
	}
	if parsed.Message.Content == "" {
		return "", fmt.Errorf("llm: empty response from ollama")
	}
	return parsed.Message.Content, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
