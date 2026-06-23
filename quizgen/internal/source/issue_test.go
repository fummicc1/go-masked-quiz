package source

import (
	"context"
	"net/http"
	"testing"
)

// oneIssueClient returns a Client that serves a single-page result with one node.
func oneIssueClient(t *testing.T, nodeJSON string) *Client {
	t.Helper()
	resp := `{"data":{"search":{"issueCount":1,"pageInfo":{"hasNextPage":false,"endCursor":""},
	  "nodes":[` + nodeJSON + `]},"rateLimit":{"remaining":1,"resetAt":"2026-06-01T00:00:00Z","cost":1}}}`
	return stubClient("tok", func(r *http.Request) *http.Response { return jsonResponse(200, resp) })
}

func TestFetchProposals_ConvertsIssue(t *testing.T) {
	node := `{"number":77273,"title":"proposal: spec: allow X",
	  "url":"https://github.com/golang/go/issues/77273","state":"CLOSED",
	  "updatedAt":"2026-06-10T00:00:00Z",
	  "body":"This proposes that ` + "`Foo`" + ` becomes valid. Plenty of prose here to clear the floor.\n\n` + "```" + `\nfunc Foo() error { return nil }\n` + "```" + `\n",
	  "labels":{"nodes":[{"name":"Proposal"},{"name":"Proposal-Accepted"}]}}`
	got, err := FetchProposals(context.Background(), oneIssueClient(t, node), FetchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("proposals = %d, want 1", len(got))
	}
	p := got[0]
	if p.Slug != "issue-77273" {
		t.Errorf("Slug = %q, want issue-77273", p.Slug)
	}
	if p.Title != "spec: allow X" {
		t.Errorf("Title = %q, want %q (proposal: prefix trimmed)", p.Title, "spec: allow X")
	}
	if p.Status != "accepted" {
		t.Errorf("Status = %q, want accepted", p.Status)
	}
	if p.URL != "https://github.com/golang/go/issues/77273" {
		t.Errorf("URL = %q", p.URL)
	}
	if len(p.Parsed.ProseUnits) == 0 {
		t.Errorf("expected prose units from inline code")
	}
	if len(p.Parsed.CodeBlocks) == 0 {
		t.Errorf("expected the bare ``` fence to be detected as Go")
	}
}

func TestFetchProposals_DropsThinIssues(t *testing.T) {
	// No code block, no inline code, short body → no quiz material.
	node := `{"number":5,"title":"proposal: tiny","url":"u","state":"CLOSED",
	  "updatedAt":"2026-01-01T00:00:00Z","body":"too short","labels":{"nodes":[{"name":"Proposal"}]}}`
	got, err := FetchProposals(context.Background(), oneIssueClient(t, node), FetchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("proposals = %d, want 0 (thin issue dropped)", len(got))
	}
}

func TestFetchProposals_StatusActiveWithoutAcceptedLabel(t *testing.T) {
	node := `{"number":9,"title":"proposal: pending","url":"u","state":"OPEN",
	  "updatedAt":"2026-01-01T00:00:00Z",
	  "body":"A pending idea about ` + "`Bar`" + ` with enough surrounding prose text to pass the body floor easily here.",
	  "labels":{"nodes":[{"name":"Proposal"}]}}`
	got, err := FetchProposals(context.Background(), oneIssueClient(t, node), FetchOptions{MinBodyChars: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Status != "active" {
		t.Fatalf("got %d proposals, status check failed: %+v", len(got), got)
	}
}
