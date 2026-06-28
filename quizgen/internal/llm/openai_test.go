package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func stubOpenAI(apiKey, model string, status int, body string, captured *http.Request) *OpenAIClient {
	c := NewOpenAIClient("https://api.test/ai/v1", apiKey, model)
	c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if captured != nil {
			*captured = *r
		}
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{},
		}, nil
	})}
	return c
}

func TestOpenAIGenerate_ReturnsContentAndAuth(t *testing.T) {
	var req http.Request
	resp := `{"choices":[{"message":{"role":"assistant","content":"{\"summary\":\"x\"}"}}]}`
	got, err := stubOpenAI("tok123", "@cf/meta/llama-3.3-70b-instruct-fp8-fast", 200, resp, &req).
		Generate(context.Background(), "sys", "user", 42)
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"summary":"x"}` {
		t.Errorf("content = %q", got)
	}
	if req.Header.Get("Authorization") != "Bearer tok123" {
		t.Errorf("Authorization = %q", req.Header.Get("Authorization"))
	}
	if !strings.HasSuffix(req.URL.Path, "/chat/completions") {
		t.Errorf("path = %q, want .../chat/completions", req.URL.Path)
	}
}

func TestOpenAIGenerate_RequiresModelAndKey(t *testing.T) {
	if _, err := NewOpenAIClient("https://api.test/ai/v1", "tok", "").Generate(context.Background(), "s", "u", 1); err == nil {
		t.Fatal("want error without model")
	}
	if _, err := NewOpenAIClient("https://api.test/ai/v1", "", "m").Generate(context.Background(), "s", "u", 1); err == nil {
		t.Fatal("want error without API key")
	}
}

func TestOpenAIGenerate_HTTPError(t *testing.T) {
	if _, err := stubOpenAI("tok", "m", 401, `{"error":"bad token"}`, nil).Generate(context.Background(), "s", "u", 1); err == nil {
		t.Fatal("want error on HTTP 401")
	}
}

func TestOpenAIGenerate_EmptyChoices(t *testing.T) {
	if _, err := stubOpenAI("tok", "m", 200, `{"choices":[]}`, nil).Generate(context.Background(), "s", "u", 1); err == nil {
		t.Fatal("want error on empty choices")
	}
}

func TestWorkersAIBaseURL(t *testing.T) {
	got := WorkersAIBaseURL("acct123")
	want := "https://api.cloudflare.com/client/v4/accounts/acct123/ai/v1"
	if got != want {
		t.Errorf("WorkersAIBaseURL = %q, want %q", got, want)
	}
}
