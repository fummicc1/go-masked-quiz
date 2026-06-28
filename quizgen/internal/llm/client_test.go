package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func stubClient(model string, status int, body string) *OllamaClient {
	c := NewOllamaClient("http://ollama.test", model)
	c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{},
		}, nil
	})}
	return c
}

func TestGenerate_ReturnsContent(t *testing.T) {
	// ollama wraps the model output (itself JSON) as a string in message.content.
	resp := `{"message":{"role":"assistant","content":"{\"summary\":\"x\"}"},"done":true}`
	got, err := stubClient("qwen2.5-coder", 200, resp).Generate(context.Background(), "sys", "user", 42)
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"summary":"x"}` {
		t.Errorf("content = %q", got)
	}
}

func TestGenerate_RequiresModel(t *testing.T) {
	if _, err := NewOllamaClient("", "").Generate(context.Background(), "s", "u", 1); err == nil {
		t.Fatal("want error without model")
	}
}

func TestGenerate_HTTPError(t *testing.T) {
	if _, err := stubClient("m", 500, "boom").Generate(context.Background(), "s", "u", 1); err == nil {
		t.Fatal("want error on HTTP 500")
	}
}

func TestGenerate_OllamaErrorField(t *testing.T) {
	resp := `{"error":"model not found"}`
	_, err := stubClient("m", 200, resp).Generate(context.Background(), "s", "u", 1)
	if err == nil || !strings.Contains(err.Error(), "model not found") {
		t.Fatalf("err = %v, want ollama error", err)
	}
}

func TestGenerate_EmptyContent(t *testing.T) {
	resp := `{"message":{"role":"assistant","content":""},"done":true}`
	if _, err := stubClient("m", 200, resp).Generate(context.Background(), "s", "u", 1); err == nil {
		t.Fatal("want error on empty content")
	}
}
