package source

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc adapts a function into an http.RoundTripper, so tests can stub
// GitHub responses without binding a port.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// stubClient returns a Client whose transport replays handler's response.
func stubClient(token string, handler func(*http.Request) *http.Response) *Client {
	c := NewClient(token)
	c.Endpoint = "https://api.test/graphql"
	c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return handler(r), nil
	})}
	return c
}

const (
	page1 = `{"data":{"search":{"issueCount":2,"pageInfo":{"hasNextPage":true,"endCursor":"CUR1"},
	  "nodes":[{"number":111,"title":"proposal: add X","url":"https://github.com/golang/go/issues/111",
	    "state":"CLOSED","updatedAt":"2026-06-01T00:00:00Z","body":"body one",
	    "labels":{"nodes":[{"name":"Proposal"},{"name":"Proposal-Accepted"}]}}]},
	  "rateLimit":{"remaining":4999,"resetAt":"2026-06-01T01:00:00Z","cost":1}}}`
	page2 = `{"data":{"search":{"issueCount":2,"pageInfo":{"hasNextPage":false,"endCursor":""},
	  "nodes":[{"number":222,"title":"runtime: do Y","url":"https://github.com/golang/go/issues/222",
	    "state":"OPEN","updatedAt":"2026-05-01T00:00:00Z","body":"body two",
	    "labels":{"nodes":[{"name":"Proposal"}]}}]},
	  "rateLimit":{"remaining":4998,"resetAt":"2026-06-01T01:00:00Z","cost":1}}}`
)

func TestClientSearch_PaginationAndParsing(t *testing.T) {
	var lastAuth string
	c := stubClient("tok123", func(r *http.Request) *http.Response {
		lastAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "CUR1") {
			return jsonResponse(200, page2)
		}
		return jsonResponse(200, page1)
	})

	got, err := c.Search(context.Background(), "repo:golang/go label:Proposal-Accepted", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("issues = %d, want 2 (both pages)", len(got))
	}
	if got[0].Number != 111 || got[1].Number != 222 {
		t.Errorf("numbers = %d,%d want 111,222", got[0].Number, got[1].Number)
	}
	if lastAuth != "Bearer tok123" {
		t.Errorf("Authorization = %q, want Bearer tok123", lastAuth)
	}
	if len(got[0].Labels) != 2 {
		t.Errorf("labels = %v, want 2", got[0].Labels)
	}
}

func TestClientSearch_RespectsLimit(t *testing.T) {
	c := stubClient("tok", func(r *http.Request) *http.Response { return jsonResponse(200, page1) })
	got, err := c.Search(context.Background(), "q", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("issues = %d, want 1 (limit)", len(got))
	}
}

func TestClientSearch_RequiresToken(t *testing.T) {
	if _, err := NewClient("").Search(context.Background(), "q", 0); err == nil {
		t.Fatal("expected error without token")
	}
}

func TestClientSearch_GraphQLError(t *testing.T) {
	c := stubClient("tok", func(r *http.Request) *http.Response {
		return jsonResponse(200, `{"errors":[{"message":"bad query"}]}`)
	})
	_, err := c.Search(context.Background(), "q", 0)
	if err == nil || !strings.Contains(err.Error(), "bad query") {
		t.Fatalf("err = %v, want graphql error", err)
	}
}

func TestClientSearch_HTTPError(t *testing.T) {
	c := stubClient("tok", func(r *http.Request) *http.Response {
		return jsonResponse(401, `Bad credentials`)
	})
	if _, err := c.Search(context.Background(), "q", 0); err == nil {
		t.Fatal("expected error on HTTP 401")
	}
}

// guard against accidental schema drift in the request payload.
func TestClientSearch_SendsVariables(t *testing.T) {
	var body map[string]any
	c := stubClient("tok", func(r *http.Request) *http.Response {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		return jsonResponse(200, `{"data":{"search":{"issueCount":0,"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}`)
	})
	if _, err := c.Search(context.Background(), "myquery", 0); err != nil {
		t.Fatal(err)
	}
	vars, _ := body["variables"].(map[string]any)
	if vars["q"] != "myquery" {
		t.Errorf("variables.q = %v, want myquery", vars["q"])
	}
}
