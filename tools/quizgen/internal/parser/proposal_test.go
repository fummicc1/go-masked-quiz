package parser

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TC-G-P-01: a realistic proposal parses into a slug, title, and at least one
// inline code span and one go code block.
func TestParseProposal_Basic(t *testing.T) {
	src := []byte("# Range Over Func\n\n" +
		"The `range` keyword now accepts a `func` value.\n\n" +
		"```go\nfunc f() { for x := range seq { _ = x } }\n```\n")
	p := ParseProposal("61405-range-func.md", src)

	if p.Slug != "61405-range-func" {
		t.Errorf("Slug = %q, want %q", p.Slug, "61405-range-func")
	}
	if p.Title != "Range Over Func" {
		t.Errorf("Title = %q, want %q", p.Title, "Range Over Func")
	}
	if len(p.InlineCodes) < 1 {
		t.Fatalf("InlineCodes = %d, want >= 1", len(p.InlineCodes))
	}
	if len(p.CodeBlocks) < 1 {
		t.Fatalf("CodeBlocks = %d, want >= 1", len(p.CodeBlocks))
	}
}

// TC-G-P-02: every inline code span is captured, in ascending offset order,
// and Source[Start:End] round-trips to Text.
func TestParseProposal_InlineCodesOrderedAndExact(t *testing.T) {
	src := []byte("Use `alpha`, then `bravo`, then `charlie` in order.\n")
	p := ParseProposal("x.md", src)

	if got := len(p.InlineCodes); got != 3 {
		t.Fatalf("InlineCodes = %d, want 3", got)
	}
	want := []string{"alpha", "bravo", "charlie"}
	for i, ic := range p.InlineCodes {
		if ic.Text != want[i] {
			t.Errorf("InlineCodes[%d].Text = %q, want %q", i, ic.Text, want[i])
		}
		if got := string(p.Source[ic.Start:ic.End]); got != ic.Text {
			t.Errorf("Source[%d:%d] = %q, want %q", ic.Start, ic.End, got, ic.Text)
		}
	}
	if !sort.SliceIsSorted(p.InlineCodes, func(i, j int) bool {
		return p.InlineCodes[i].Start < p.InlineCodes[j].Start
	}) {
		t.Error("InlineCodes not sorted by Start")
	}
}

// TC-G-P-03: multiple go blocks are all captured.
func TestParseProposal_MultipleCodeBlocks(t *testing.T) {
	src := []byte("```go\na := 1\n```\n\ntext\n\n```go\nb := 2\n```\n")
	p := ParseProposal("x.md", src)
	if got := len(p.CodeBlocks); got != 2 {
		t.Fatalf("CodeBlocks = %d, want 2", got)
	}
	if !strings.Contains(p.CodeBlocks[0].Source, "a := 1") {
		t.Errorf("block 0 = %q, want to contain %q", p.CodeBlocks[0].Source, "a := 1")
	}
	if !strings.Contains(p.CodeBlocks[1].Source, "b := 2") {
		t.Errorf("block 1 = %q, want to contain %q", p.CodeBlocks[1].Source, "b := 2")
	}
}

// TC-G-P-04: non-go fenced blocks are ignored.
func TestParseProposal_IgnoresNonGoBlocks(t *testing.T) {
	src := []byte("```python\nprint('hi')\n```\n\n```go\nx := 1\n```\n")
	p := ParseProposal("x.md", src)
	if got := len(p.CodeBlocks); got != 1 {
		t.Fatalf("CodeBlocks = %d, want 1 (python ignored)", got)
	}
	if p.CodeBlocks[0].Language != "go" {
		t.Errorf("Language = %q, want go", p.CodeBlocks[0].Language)
	}
}

// TC-G-P-05: an empty file yields no panic and empty slices.
func TestParseProposal_Empty(t *testing.T) {
	p := ParseProposal("empty.md", []byte(""))
	if len(p.InlineCodes) != 0 || len(p.CodeBlocks) != 0 {
		t.Errorf("expected empty, got %d inline / %d blocks", len(p.InlineCodes), len(p.CodeBlocks))
	}
	if p.Title != "empty" {
		t.Errorf("Title = %q, want slug %q", p.Title, "empty")
	}
}

// TC-G-P-06: a file with no H1 falls back to the slug as title.
func TestParseProposal_NoHeadingTitleIsSlug(t *testing.T) {
	src := []byte("Just a paragraph with `code` and nothing else.\n")
	p := ParseProposal("99999-sample.md", src)
	if p.Title != "99999-sample" {
		t.Errorf("Title = %q, want %q", p.Title, "99999-sample")
	}
}

// TC-G-P-07: a large document parses without error.
func TestParseProposal_Large(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Big\n\n")
	for i := 0; i < 2000; i++ {
		b.WriteString("Paragraph with `ident` text.\n\n")
	}
	p := ParseProposal("big.md", []byte(b.String()))
	if len(p.InlineCodes) != 2000 {
		t.Errorf("InlineCodes = %d, want 2000", len(p.InlineCodes))
	}
}

// TC-G-P-09: a missing path returns an error (no panic).
func TestLoadProposal_MissingPath(t *testing.T) {
	_, err := LoadProposal(filepath.Join(t.TempDir(), "does-not-exist.md"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// LoadProposal reads a real file and derives the slug from its base name.
func TestLoadProposal_ReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "12345-feature.md")
	if err := os.WriteFile(path, []byte("# Title\n\n`code`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProposal(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Slug != "12345-feature" {
		t.Errorf("Slug = %q, want %q", p.Slug, "12345-feature")
	}
}
