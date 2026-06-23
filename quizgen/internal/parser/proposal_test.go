package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func allInline(p *Proposal) []InlineCode {
	var out []InlineCode
	for _, u := range p.ProseUnits {
		out = append(out, u.InlineCodes...)
	}
	return out
}

// A realistic proposal parses into a slug, title, prose units, and a go block.
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
	if len(p.ProseUnits) < 1 {
		t.Fatalf("ProseUnits = %d, want >= 1", len(p.ProseUnits))
	}
	if len(p.CodeBlocks) < 1 {
		t.Fatalf("CodeBlocks = %d, want >= 1", len(p.CodeBlocks))
	}
}

// Inline-code spans are grouped into their containing paragraph, in order, and
// Source[Start:End] round-trips to Text.
func TestParseProposal_ProseUnitsPerParagraph(t *testing.T) {
	src := []byte("First para with `alpha`.\n\nSecond with `bravo` then `charlie`.\n")
	p := ParseProposal("x.md", src)

	if len(p.ProseUnits) != 2 {
		t.Fatalf("ProseUnits = %d, want 2", len(p.ProseUnits))
	}
	if len(p.ProseUnits[0].InlineCodes) != 1 {
		t.Errorf("unit0 inline = %d, want 1", len(p.ProseUnits[0].InlineCodes))
	}
	if len(p.ProseUnits[1].InlineCodes) != 2 {
		t.Errorf("unit1 inline = %d, want 2", len(p.ProseUnits[1].InlineCodes))
	}
	for _, ic := range allInline(p) {
		if got := string(p.Source[ic.Start:ic.End]); got != ic.Text {
			t.Errorf("Source[%d:%d] = %q, want %q", ic.Start, ic.End, got, ic.Text)
		}
	}
	// each unit's inline codes are inside its [Start,End)
	for _, u := range p.ProseUnits {
		for _, ic := range u.InlineCodes {
			if ic.Start < u.Start || ic.End > u.End {
				t.Errorf("inline %q outside unit [%d,%d)", ic.Text, u.Start, u.End)
			}
		}
	}
}

func TestParseProposal_MultipleCodeBlocks(t *testing.T) {
	src := []byte("```go\na := 1\n```\n\ntext\n\n```go\nb := 2\n```\n")
	p := ParseProposal("x.md", src)
	if got := len(p.CodeBlocks); got != 2 {
		t.Fatalf("CodeBlocks = %d, want 2", got)
	}
}

func TestParseProposal_IgnoresNonGoBlocks(t *testing.T) {
	src := []byte("```python\nprint('hi')\n```\n\n```go\nx := 1\n```\n")
	p := ParseProposal("x.md", src)
	if got := len(p.CodeBlocks); got != 1 {
		t.Fatalf("CodeBlocks = %d, want 1 (python ignored)", got)
	}
}

func TestParseProposal_Empty(t *testing.T) {
	p := ParseProposal("empty.md", []byte(""))
	if len(p.ProseUnits) != 0 || len(p.CodeBlocks) != 0 {
		t.Errorf("expected empty, got %d units / %d blocks", len(p.ProseUnits), len(p.CodeBlocks))
	}
	if p.Title != "empty" {
		t.Errorf("Title = %q, want slug", p.Title)
	}
}

func TestParseProposal_ParagraphWithoutInlineCodeIsSkipped(t *testing.T) {
	src := []byte("Just plain prose, no code at all.\n\nAnother `code` paragraph.\n")
	p := ParseProposal("x.md", src)
	// only the second paragraph has inline code → 1 unit
	if len(p.ProseUnits) != 1 {
		t.Fatalf("ProseUnits = %d, want 1 (code-less paragraph skipped)", len(p.ProseUnits))
	}
}

func TestParseProposal_NoHeadingTitleIsSlug(t *testing.T) {
	p := ParseProposal("99999-sample.md", []byte("Para with `code`.\n"))
	if p.Title != "99999-sample" {
		t.Errorf("Title = %q, want slug", p.Title)
	}
}

// By default a bare (untagged) fence is ignored, preserving design-doc behaviour.
func TestParseProposal_BareFenceIgnoredByDefault(t *testing.T) {
	src := []byte("```\nfunc f() { return }\n```\n")
	if got := len(ParseProposal("x.md", src).CodeBlocks); got != 0 {
		t.Fatalf("CodeBlocks = %d, want 0 (bare fence ignored by default)", got)
	}
}

// With AcceptBareGoFences, a bare fence that lexes like Go is admitted.
func TestParseProposal_BareGoFenceAccepted(t *testing.T) {
	src := []byte("```\nfunc f() (int, error) { return 0, nil }\n```\n")
	p := ParseProposalWithOptions("x.md", src, Options{AcceptBareGoFences: true})
	if got := len(p.CodeBlocks); got != 1 {
		t.Fatalf("CodeBlocks = %d, want 1 (bare Go fence accepted)", got)
	}
}

// A bare fence that is clearly not Go stays out even when bare fences are allowed.
func TestParseProposal_BareNonGoFenceRejected(t *testing.T) {
	src := []byte("```\n$ go build ./...\nhello world output\n```\n")
	p := ParseProposalWithOptions("x.md", src, Options{AcceptBareGoFences: true})
	if got := len(p.CodeBlocks); got != 0 {
		t.Fatalf("CodeBlocks = %d, want 0 (non-Go bare fence rejected)", got)
	}
}

func TestLooksLikeGo(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"func decl", "func f() error {\n\treturn nil\n}", true},
		{"type and var", "type T struct{}\nvar x T", true},
		{"shell", "$ ls -la\ncd /tmp && go build", false},
		{"prose", "this is just plain english text", false},
		{"empty", "   \n  ", false},
	}
	for _, c := range cases {
		if got := looksLikeGo(c.body); got != c.want {
			t.Errorf("looksLikeGo(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLoadProposal_MissingPath(t *testing.T) {
	if _, err := LoadProposal(filepath.Join(t.TempDir(), "nope.md")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

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
