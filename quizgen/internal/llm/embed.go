package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Embed returns one embedding per input text via the ollama /api/embed
// endpoint. Like Generate, it only ever talks to a local server: embeddings
// are computed on demand, cached on disk, and never requested from CI.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	if c.Model == "" {
		return nil, fmt.Errorf("llm: model is required")
	}
	if len(texts) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(embedRequest{Model: c.Model, Input: texts})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/api/embed", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm: call ollama (is it running at %s?): %w", c.URL, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm: ollama HTTP %d: %s", res.StatusCode, truncate(string(body), 200))
	}
	var parsed embedResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("llm: decode ollama response: %w", err)
	}
	if parsed.Error != "" {
		return nil, fmt.Errorf("llm: ollama error: %s", parsed.Error)
	}
	if len(parsed.Embeddings) != len(texts) {
		return nil, fmt.Errorf("llm: got %d embeddings for %d inputs", len(parsed.Embeddings), len(texts))
	}
	return parsed.Embeddings, nil
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
	Error      string      `json:"error"`
}

// EmbedCache is the on-disk store of token embeddings produced by
// "quizgen embed-generate" and consumed by "generate --embeddings". The model
// name is recorded so a cache built with one model is never silently mixed
// with vectors from another (vector spaces are not comparable across models).
type EmbedCache struct {
	Model   string               `json:"model"`
	Vectors map[string][]float64 `json:"vectors"`
}

// LoadEmbedCache reads the cache at path. The error is os.IsNotExist-able so
// embed-generate can treat a missing cache as "start fresh" while generate
// treats it as a hard error (a mistyped path must not silently disable
// semantic ranking).
func LoadEmbedCache(path string) (*EmbedCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c EmbedCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("llm: decode embed cache %s: %w", path, err)
	}
	if c.Vectors == nil {
		c.Vectors = map[string][]float64{}
	}
	return &c, nil
}

// SaveEmbedCache writes c to path. The JSON is not indented: with thousands of
// tokens the vectors dominate the file size and nobody reads them by eye.
func SaveEmbedCache(path string, c *EmbedCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("llm: mkdir embed cache dir: %w", err)
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
