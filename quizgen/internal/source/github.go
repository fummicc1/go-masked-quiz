package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultGraphQLEndpoint is the GitHub GraphQL API endpoint.
const DefaultGraphQLEndpoint = "https://api.github.com/graphql"

// Issue is a single proposal issue fetched from the tracker.
type Issue struct {
	Number    int
	Title     string
	Body      string
	State     string // OPEN | CLOSED
	URL       string
	UpdatedAt time.Time
	Labels    []string
}

// Client queries the GitHub GraphQL API for proposal issues. The zero value is
// not usable; build one with NewClient.
type Client struct {
	HTTP     *http.Client
	Token    string
	Endpoint string
}

// NewClient returns a Client authenticated with token (a GitHub PAT or
// Actions token). An empty token yields an unauthenticated client, which the
// GraphQL API rejects — callers should require GITHUB_TOKEN.
func NewClient(token string) *Client {
	return &Client{
		HTTP:     &http.Client{Timeout: 30 * time.Second},
		Token:    token,
		Endpoint: DefaultGraphQLEndpoint,
	}
}

// searchQuery returns proposal issues plus paging and rate-limit info in one
// request. Sorting by updated-desc keeps the freshest proposals first.
const searchQuery = `
query($q: String!, $first: Int!, $after: String) {
  search(query: $q, type: ISSUE, first: $first, after: $after) {
    issueCount
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on Issue {
        number
        title
        url
        state
        updatedAt
        body
        labels(first: 25) { nodes { name } }
      }
    }
  }
  rateLimit { remaining resetAt cost }
}`

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphQLResponse struct {
	Data struct {
		Search struct {
			IssueCount int `json:"issueCount"`
			PageInfo   struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []struct {
				Number    int       `json:"number"`
				Title     string    `json:"title"`
				URL       string    `json:"url"`
				State     string    `json:"state"`
				UpdatedAt time.Time `json:"updatedAt"`
				Body      string    `json:"body"`
				Labels    struct {
					Nodes []struct {
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"labels"`
			} `json:"nodes"`
		} `json:"search"`
		RateLimit struct {
			Remaining int       `json:"remaining"`
			ResetAt   time.Time `json:"resetAt"`
			Cost      int       `json:"cost"`
		} `json:"rateLimit"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Search runs a GitHub issue-search query (e.g.
// "repo:golang/go label:Proposal-Accepted sort:updated-desc") and returns up to
// limit issues across as many pages as needed (limit <= 0 means all results).
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Issue, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("github: GITHUB_TOKEN is required for the GraphQL API")
	}
	var out []Issue
	cursor := ""
	for {
		page := 50
		if limit > 0 {
			if remaining := limit - len(out); remaining < page {
				page = remaining
			}
		}
		resp, err := c.query(ctx, query, page, cursor)
		if err != nil {
			return nil, err
		}
		for _, n := range resp.Data.Search.Nodes {
			labels := make([]string, 0, len(n.Labels.Nodes))
			for _, l := range n.Labels.Nodes {
				labels = append(labels, l.Name)
			}
			out = append(out, Issue{
				Number:    n.Number,
				Title:     n.Title,
				Body:      n.Body,
				State:     n.State,
				URL:       n.URL,
				UpdatedAt: n.UpdatedAt,
				Labels:    labels,
			})
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
		pi := resp.Data.Search.PageInfo
		if !pi.HasNextPage || pi.EndCursor == "" || len(resp.Data.Search.Nodes) == 0 {
			return out, nil
		}
		cursor = pi.EndCursor
	}
}

func (c *Client) query(ctx context.Context, q string, first int, after string) (*graphQLResponse, error) {
	vars := map[string]any{"q": q, "first": first}
	if after != "" {
		vars["after"] = after
	}
	payload, err := json.Marshal(graphQLRequest{Query: searchQuery, Variables: vars})
	if err != nil {
		return nil, err
	}
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = DefaultGraphQLEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: HTTP %d: %s", res.StatusCode, truncate(string(data), 200))
	}
	var parsed graphQLResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("github: decode response: %w", err)
	}
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("github: graphql error: %s", parsed.Errors[0].Message)
	}
	return &parsed, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
