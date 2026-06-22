package source

import (
	"context"
	"fmt"
	"strings"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
)

// Proposal pairs a parsed issue body with the metadata the bundle needs.
type Proposal struct {
	Number int
	Slug   string // "issue-<number>", also the parser slug (stable RNG tag)
	Title  string // human title (issue title with the "proposal:" prefix trimmed)
	URL    string
	Status string // "accepted" | "active"
	Parsed *parser.Proposal
}

// FetchOptions tunes which issues become proposals.
type FetchOptions struct {
	// Query is the GitHub issue-search query. Empty means accepted proposals.
	Query string
	// Max caps the number of issues fetched (0 = no cap).
	Max int
	// MinBodyChars is the prose floor below which an issue is kept only if it
	// has a Go code block. Zero uses defaultMinBodyChars.
	MinBodyChars int
}

const (
	defaultQuery        = "repo:golang/go label:Proposal-Accepted sort:updated-desc"
	defaultMinBodyChars = 400
)

// FetchProposals fetches proposal issues, normalises each body, parses it, and
// returns those with enough content to make at least one quiz.
func FetchProposals(ctx context.Context, c *Client, opts FetchOptions) ([]Proposal, error) {
	query := opts.Query
	if query == "" {
		query = defaultQuery
	}
	minChars := opts.MinBodyChars
	if minChars == 0 {
		minChars = defaultMinBodyChars
	}
	issues, err := c.Search(ctx, query, opts.Max)
	if err != nil {
		return nil, err
	}
	out := make([]Proposal, 0, len(issues))
	for _, iss := range issues {
		body := Normalize(iss.Body)
		slug := fmt.Sprintf("issue-%d", iss.Number)
		parsed := parser.ParseProposalWithOptions(slug+".md", []byte(body), parser.Options{AcceptBareGoFences: true})
		// The issue title is authoritative; the body rarely has its own H1.
		parsed.Title = cleanTitle(iss.Title)
		if !hasQuizMaterial(parsed, body, minChars) {
			continue
		}
		out = append(out, Proposal{
			Number: iss.Number,
			Slug:   slug,
			Title:  parsed.Title,
			URL:    iss.URL,
			Status: statusOf(iss.Labels),
			Parsed: parsed,
		})
	}
	return out, nil
}

// hasQuizMaterial reports whether a parsed issue can yield any quiz: it needs an
// inline-code paragraph or a Go code block, and enough prose to be worth it.
func hasQuizMaterial(p *parser.Proposal, body string, minChars int) bool {
	if len(p.CodeBlocks) > 0 {
		return true
	}
	return len(p.ProseUnits) > 0 && len(body) >= minChars
}

// cleanTitle strips the conventional "proposal:" / "Proposal:" prefix so the
// displayed title reads naturally.
func cleanTitle(title string) string {
	t := strings.TrimSpace(title)
	for _, prefix := range []string{"proposal:", "Proposal:"} {
		if strings.HasPrefix(t, prefix) {
			return strings.TrimSpace(t[len(prefix):])
		}
	}
	return t
}

// statusOf derives a coarse status from the issue's labels.
func statusOf(labels []string) string {
	for _, l := range labels {
		if l == "Proposal-Accepted" {
			return "accepted"
		}
	}
	return "active"
}
