package main

import (
	"strings"
	"testing"
	"time"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/llm"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/quiz"
)

var epoch = time.Unix(0, 0).UTC()

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
	return genItem{p: p, id: "design-12345-x", url: "https://example/design", sourceKind: "design-docs"}
}

func issueItem() genItem {
	p := parser.ParseProposalWithOptions("issue-77.md", []byte("Discuss `Baz` and `Qux` in this issue body.\n"), parser.Options{AcceptBareGoFences: true})
	p.Title = "spec: add Baz"
	return genItem{p: p, id: "issue-77", url: "https://example/issue/77", sourceKind: "github-issues", status: "accepted", issueNumber: 77}
}

func designSrc() quiz.Source {
	return quiz.Source{Kind: "design-docs", Repo: "https://github.com/golang/proposal", License: "BSD-3-Clause", LicenseURL: "https://example/p-license"}
}

func issueSrc() quiz.Source {
	return quiz.Source{Kind: "github-issues", Repo: "https://github.com/golang/go", License: "BSD-3-Clause", LicenseURL: "https://go.dev/LICENSE"}
}

func TestBuildBundle_SingleSourceOmitsSources(t *testing.T) {
	b, _ := buildBundle([]genItem{designItem()}, []quiz.Source{designSrc()}, epoch, 42, 5, 3, 4, "")
	if b.Version != 1 {
		t.Errorf("Version = %d, want 1", b.Version)
	}
	if b.Sources != nil {
		t.Errorf("single source must omit Sources, got %v", b.Sources)
	}
	if b.SourceRepo != "https://github.com/golang/proposal" || b.SourceFork == "" {
		t.Errorf("legacy source fields missing: repo=%q fork=%q", b.SourceRepo, b.SourceFork)
	}
	// Every proposal/quiz carries its source metadata even without LLM content.
	for _, p := range b.Proposals {
		if p.SourceKind != "design-docs" {
			t.Errorf("proposal %s source_kind = %q", p.ID, p.SourceKind)
		}
		for _, q := range p.Quizzes {
			if q.GenMethod != "mechanical" {
				t.Errorf("quiz %s gen_method = %q, want mechanical", q.ID, q.GenMethod)
			}
		}
	}
}

func TestBuildBundle_MultiSourceMergesAndAttributes(t *testing.T) {
	items := []genItem{designItem(), issueItem()}
	srcs := []quiz.Source{designSrc(), issueSrc()}
	b, _ := buildBundle(items, srcs, epoch, 42, 5, 3, 4, "")

	if len(b.Sources) != 2 {
		t.Fatalf("Sources = %d, want 2", len(b.Sources))
	}
	if b.SourceRepo != "https://github.com/golang/proposal" {
		t.Errorf("legacy SourceRepo = %q, want design repo", b.SourceRepo)
	}
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
	a, _ := buildBundle(items, srcs, epoch, 42, 5, 3, 4, "")
	b, _ := buildBundle(items, srcs, epoch, 42, 5, 3, 4, "")
	if countQuizzes(&a) != countQuizzes(&b) || len(a.Proposals) != len(b.Proposals) {
		t.Error("multi-source bundle is not deterministic")
	}
}

func TestBuildBundle_MergesLLMCache(t *testing.T) {
	dir := t.TempDir()
	it := issueItem()
	body := string(it.p.Source)
	bi := 0
	if err := llm.Save(dir, &llm.Entry{
		IssueNumber:   77,
		BodyHash:      llm.BodyHash(body),
		Model:         "test-model",
		PromptVersion: llm.PromptVersion,
		Summary:       "A concise summary.",
		Quizzes: []quiz.Quiz{{
			Kind:      quiz.KindLLM,
			GenMethod: "llm",
			Blocks: []quiz.Block{
				{Type: quiz.BlockText, Value: "Use "},
				{Type: quiz.BlockMask, BlankIndex: &bi},
				{Type: quiz.BlockText, Value: " here."},
			},
			Blanks: []quiz.Blank{{Answer: "Baz", Choices: []string{"Baz", "Qux", "Foo", "Bar"}}},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	b, notes := buildBundle([]genItem{it}, []quiz.Source{issueSrc()}, epoch, 42, 5, 3, 4, dir)
	if len(notes) != 0 {
		t.Errorf("unexpected notes: %v", notes)
	}
	if b.Version != 1 {
		t.Fatalf("Version = %d, want 1", b.Version)
	}
	p := b.Proposals[0]
	if p.Summary != "A concise summary." {
		t.Errorf("summary not merged: %q", p.Summary)
	}
	if p.SourceKind != "github-issues" || p.IssueNumber != 77 || p.Status != "accepted" {
		t.Errorf("source metadata missing: %+v", p)
	}
	var llmQ *quiz.Quiz
	var mechCount int
	for i := range p.Quizzes {
		if p.Quizzes[i].Kind == quiz.KindLLM {
			llmQ = &p.Quizzes[i]
		} else {
			mechCount++
			if p.Quizzes[i].GenMethod != "mechanical" {
				t.Errorf("mechanical quiz %s gen_method = %q", p.Quizzes[i].ID, p.Quizzes[i].GenMethod)
			}
		}
	}
	if mechCount == 0 {
		t.Error("expected at least one mechanical quiz")
	}
	if llmQ == nil {
		t.Fatal("llm quiz not merged")
	}
	if llmQ.GenMethod != "llm" || !strings.HasPrefix(llmQ.ID, "issue-77-q") {
		t.Errorf("llm quiz id=%q gen_method=%q", llmQ.ID, llmQ.GenMethod)
	}
	if llmQ.Index != mechCount {
		t.Errorf("llm quiz Index = %d, want %d (after mechanical)", llmQ.Index, mechCount)
	}
}

func TestBuildBundle_StaleCacheReportsNote(t *testing.T) {
	dir := t.TempDir()
	it := issueItem()
	if err := llm.Save(dir, &llm.Entry{
		IssueNumber:   77,
		BodyHash:      llm.BodyHash("a different body"),
		Model:         "m",
		PromptVersion: llm.PromptVersion,
		Summary:       "stale summary",
	}); err != nil {
		t.Fatal(err)
	}
	b, notes := buildBundle([]genItem{it}, []quiz.Source{issueSrc()}, epoch, 42, 5, 3, 4, dir)
	if b.Proposals[0].Summary != "" {
		t.Error("stale cache must not merge summary")
	}
	if len(notes) == 0 {
		t.Error("expected a stale-cache note")
	}
}
