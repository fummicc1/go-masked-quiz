package main

import (
	"strings"
	"testing"
	"time"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/quiz"
)

func TestSplitSources(t *testing.T) {
	cases := []struct {
		in   string
		want []string
		err  bool
	}{
		{"design-docs", []string{"design-docs"}, false},
		{"design-docs,github-issues", []string{"design-docs", "github-issues"}, false},
		{" design-docs , github-issues ", []string{"design-docs", "github-issues"}, false},
		{"github-issues,github-issues", []string{"github-issues"}, false}, // deduped
		{"", nil, true},
		{" , ", nil, true},
	}
	for _, c := range cases {
		got, err := splitSources(c.in)
		if c.err {
			if err == nil {
				t.Errorf("splitSources(%q): want error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitSources(%q): unexpected error %v", c.in, err)
			continue
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("splitSources(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func designItem() genItem {
	p := parser.ParseProposal("12345-x.md", []byte("# Feature X\n\nUse `Foo` and `Bar` here in prose.\n"))
	return genItem{p: p, id: "design-12345-x", url: "https://example/design"}
}

func issueItem() genItem {
	p := parser.ParseProposalWithOptions("issue-77.md", []byte("Discuss `Baz` and `Qux` in this issue body.\n"), parser.Options{AcceptBareGoFences: true})
	p.Title = "spec: add Baz"
	return genItem{p: p, id: "issue-77", url: "https://example/issue/77"}
}

func designSrc() quiz.Source {
	return quiz.Source{Kind: "design-docs", Repo: "https://github.com/golang/proposal", License: "BSD-3-Clause", LicenseURL: "https://example/p-license"}
}

func issueSrc() quiz.Source {
	return quiz.Source{Kind: "github-issues", Repo: "https://github.com/golang/go", License: "BSD-3-Clause", LicenseURL: "https://go.dev/LICENSE"}
}

func TestBuildBundle_SingleSourceOmitsSources(t *testing.T) {
	b := buildBundle([]genItem{designItem()}, []quiz.Source{designSrc()}, time.Unix(0, 0).UTC(), 42, 5, 3, 4)
	if b.Sources != nil {
		t.Errorf("single source must omit Sources, got %v", b.Sources)
	}
	if b.SourceRepo != "https://github.com/golang/proposal" {
		t.Errorf("SourceRepo = %q", b.SourceRepo)
	}
	if b.SourceFork == "" {
		t.Errorf("design-docs primary should set SourceFork")
	}
}

func TestBuildBundle_MultiSourceMergesAndAttributes(t *testing.T) {
	items := []genItem{designItem(), issueItem()}
	srcs := []quiz.Source{designSrc(), issueSrc()}
	b := buildBundle(items, srcs, time.Unix(0, 0).UTC(), 42, 5, 3, 4)

	if len(b.Sources) != 2 {
		t.Fatalf("Sources = %d, want 2", len(b.Sources))
	}
	// legacy fields describe the first (design-docs) source for back-compat.
	if b.SourceRepo != "https://github.com/golang/proposal" {
		t.Errorf("legacy SourceRepo = %q, want design repo", b.SourceRepo)
	}
	// both proposals are present, keyed by their source-specific IDs.
	var haveDesign, haveIssue bool
	for _, p := range b.Proposals {
		switch {
		case strings.HasPrefix(p.ID, "design-"):
			haveDesign = true
		case strings.HasPrefix(p.ID, "issue-"):
			haveIssue = true
		}
	}
	if !haveDesign || !haveIssue {
		t.Errorf("merged bundle missing a source: design=%v issue=%v", haveDesign, haveIssue)
	}
}

func TestBuildBundle_Deterministic(t *testing.T) {
	items := []genItem{designItem(), issueItem()}
	srcs := []quiz.Source{designSrc(), issueSrc()}
	a := buildBundle(items, srcs, time.Unix(0, 0).UTC(), 42, 5, 3, 4)
	b := buildBundle(items, srcs, time.Unix(0, 0).UTC(), 42, 5, 3, 4)
	if countQuizzes(&a) != countQuizzes(&b) || len(a.Proposals) != len(b.Proposals) {
		t.Error("multi-source bundle is not deterministic")
	}
}
