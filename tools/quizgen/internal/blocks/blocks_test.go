package blocks

import (
	"testing"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/masker"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/quiz"
)

func maskCount(bs []quiz.Block) int {
	n := 0
	for _, b := range bs {
		if b.Type == quiz.BlockMask {
			n++
		}
	}
	return n
}

// A single masked token in a paragraph → [text, mask, text].
func TestBuildProseBlocks_Simple(t *testing.T) {
	src := []byte("hello `world`!\n")
	p := parser.ParseProposal("x.md", src)
	unit := p.ProseUnits[0]
	blanks := masker.SelectProseBlanks(masker.NewRNG(1, "t"), unit, 3)
	bs := BuildProseBlocks(src, unit, blanks)

	if maskCount(bs) != 1 {
		t.Fatalf("mask count = %d, want 1 (%#v)", maskCount(bs), bs)
	}
	if bs[0].Type != quiz.BlockText || bs[0].Value != "hello " {
		t.Errorf("block0 = %#v, want text 'hello '", bs[0])
	}
	if bs[1].Type != quiz.BlockMask || bs[1].BlankIndex == nil || *bs[1].BlankIndex != 0 {
		t.Errorf("block1 = %#v, want mask -> blank 0", bs[1])
	}
}

// Every occurrence of a blank token is masked and points at the same blank.
func TestBuildProseBlocks_AllOccurrencesSameBlank(t *testing.T) {
	src := []byte("use `range` and `range` here\n")
	p := parser.ParseProposal("x.md", src)
	unit := p.ProseUnits[0]
	blanks := masker.SelectProseBlanks(masker.NewRNG(1, "t"), unit, 3)
	bs := BuildProseBlocks(src, unit, blanks)

	if maskCount(bs) != 2 {
		t.Fatalf("mask count = %d, want 2", maskCount(bs))
	}
	for _, b := range bs {
		if b.Type == quiz.BlockMask {
			if b.BlankIndex == nil || *b.BlankIndex != 0 {
				t.Errorf("mask points at %v, want blank 0", b.BlankIndex)
			}
		}
		// the answer text must never appear verbatim
		if b.Type == quiz.BlockText && contains(b.Value, "range") {
			t.Errorf("answer leaked into text: %q", b.Value)
		}
	}
}

// A non-masked inline code (stopword/short or not picked) stays as inline_code.
func TestBuildProseBlocks_KeepsUnpickedInlineCode(t *testing.T) {
	// "for" is a stopword → not a blank, but should render as inline_code.
	src := []byte("the `for` loop uses `range` here\n")
	p := parser.ParseProposal("x.md", src)
	unit := p.ProseUnits[0]
	blanks := masker.SelectProseBlanks(masker.NewRNG(1, "t"), unit, 3)
	bs := BuildProseBlocks(src, unit, blanks)

	var sawForInline bool
	for _, b := range bs {
		if b.Type == quiz.BlockInlineCode && b.Value == "for" {
			sawForInline = true
		}
	}
	if !sawForInline {
		t.Errorf("expected inline_code 'for' in %#v", bs)
	}
}

func TestBuildCodeBlocks_MasksIdentifiers(t *testing.T) {
	p := parser.ParseProposal("x.md", []byte("```go\nfunc Hello() { Hello() }\n```\n"))
	block := p.CodeBlocks[0]
	blanks := masker.SelectCodeBlanks(masker.NewRNG(1, "t"), block, 3)
	bs := BuildCodeBlocks(block, blanks)

	if maskCount(bs) < 1 {
		t.Fatalf("expected masks, got %#v", bs)
	}
	for _, b := range bs {
		if b.Type == quiz.BlockMask && b.BlankIndex == nil {
			t.Error("code mask without BlankIndex")
		}
		if b.Type != quiz.BlockCodeBlock && b.Type != quiz.BlockMask {
			t.Errorf("code quiz unexpected block type %q", b.Type)
		}
	}
}

func TestOptimize_MergesAndDrops(t *testing.T) {
	zero := 0
	in := []quiz.Block{
		{Type: quiz.BlockText, Value: ""},
		{Type: quiz.BlockText, Value: "a"},
		{Type: quiz.BlockText, Value: "b"},
		{Type: quiz.BlockMask, BlankIndex: &zero},
		{Type: quiz.BlockText, Value: "c"},
	}
	out := optimize(in)
	if len(out) != 3 {
		t.Fatalf("len = %d (%#v), want 3", len(out), out)
	}
	if out[0].Type != quiz.BlockText || out[0].Value != "ab" {
		t.Errorf("out[0] = %#v, want text 'ab'", out[0])
	}
	if out[1].Type != quiz.BlockMask {
		t.Errorf("out[1] = %#v, want mask", out[1])
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// cleanText turns Markdown links into their text and strips inline-code backticks.
func TestCleanText_StripsMarkdownLinks(t *testing.T) {
	cases := map[string]string{
		"see [golang-announce](https://groups.google.com/forum/#!forum/x) now": "see golang-announce now",
		"auto <https://go.dev/doc>":  "auto https://go.dev/doc",
		"plain text only":            "plain text only",
		"`code`":                     "code",
	}
	for in, want := range cases {
		if got := cleanText(in); got != want {
			t.Errorf("cleanText(%q) = %q, want %q", in, got, want)
		}
	}
}

// A paragraph with a Markdown link renders without the raw URL, keeping the text.
func TestBuildProseBlocks_StripsRawURL(t *testing.T) {
	src := []byte("subscribe to [golang-announce](https://groups.google.com/x) for `news`\n")
	p := parser.ParseProposal("x.md", src)
	unit := p.ProseUnits[0]
	blanks := masker.SelectProseBlanks(masker.NewRNG(1, "t"), unit, 3)
	bs := BuildProseBlocks(src, unit, blanks)

	var joined string
	for _, b := range bs {
		joined += b.Value
	}
	if contains(joined, "https://") || contains(joined, "](") {
		t.Errorf("raw link syntax leaked: %q", joined)
	}
	if !contains(joined, "golang-announce") {
		t.Errorf("link text lost: %q", joined)
	}
}
