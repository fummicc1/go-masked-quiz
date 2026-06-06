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

func findInline(p *parser.Proposal, text string) parser.InlineCode {
	for _, ic := range p.InlineCodes {
		if ic.Text == text {
			return ic
		}
	}
	panic("inline code not found: " + text)
}

// TC-G-B-01: "hello `world`!" with seed=world → [text, mask, text].
func TestBuildProseBlocks_Simple(t *testing.T) {
	p := parser.ParseProposal("x.md", []byte("hello `world`!\n"))
	ic := findInline(p, "world")
	bs := BuildProseBlocks(p, masker.ProseSeed{Start: ic.Start, End: ic.End, Answer: "world"}, 80)

	if len(bs) != 3 {
		t.Fatalf("blocks = %#v, want 3", bs)
	}
	if bs[0].Type != quiz.BlockText || bs[0].Value != "hello " {
		t.Errorf("block0 = %#v, want text 'hello '", bs[0])
	}
	if bs[1].Type != quiz.BlockMask {
		t.Errorf("block1 = %#v, want mask", bs[1])
	}
	if bs[2].Type != quiz.BlockText || bs[2].Value != "!" {
		t.Errorf("block2 = %#v, want text '!'", bs[2])
	}
}

// TC-G-B-02: another inline code inside the window survives as inline_code.
func TestBuildProseBlocks_KeepsOtherInlineCode(t *testing.T) {
	p := parser.ParseProposal("x.md", []byte("the `quizgen` tool parses `proposals` nicely\n"))
	ic := findInline(p, "quizgen")
	bs := BuildProseBlocks(p, masker.ProseSeed{Start: ic.Start, End: ic.End, Answer: "quizgen"}, 80)

	if maskCount(bs) != 1 {
		t.Fatalf("mask count = %d, want 1", maskCount(bs))
	}
	var sawInline bool
	for _, b := range bs {
		if b.Type == quiz.BlockInlineCode && b.Value == "proposals" {
			sawInline = true
		}
	}
	if !sawInline {
		t.Errorf("expected inline_code 'proposals' in %#v", bs)
	}
}

// TC-G-B-03: text blocks never cross a line boundary (window is line-snapped).
func TestBuildProseBlocks_LineSnapped(t *testing.T) {
	src := []byte("first line unrelated\nsecond `target` line here\nthird line unrelated\n")
	p := parser.ParseProposal("x.md", src)
	ic := findInline(p, "target")
	bs := BuildProseBlocks(p, masker.ProseSeed{Start: ic.Start, End: ic.End, Answer: "target"}, 80)
	for _, b := range bs {
		if b.Type == quiz.BlockText {
			if got := b.Value; containsNewline(got) {
				t.Errorf("text block crosses newline: %q", got)
			}
		}
	}
	// The masked line's text should be present, the neighbouring lines absent.
	joined := joinText(bs)
	if !contains(joined, "second ") || !contains(joined, " line here") {
		t.Errorf("masked line text missing: %q", joined)
	}
	if contains(joined, "first line") || contains(joined, "third line") {
		t.Errorf("neighbour lines leaked into window: %q", joined)
	}
}

// TC-G-B-04: code seed → [code_block, mask, code_block] around the identifier.
func TestBuildCodeBlocks_Splits(t *testing.T) {
	p := parser.ParseProposal("x.md", []byte("```go\nfunc Hello() {}\n```\n"))
	if len(p.CodeBlocks) != 1 {
		t.Fatalf("want 1 code block, got %d", len(p.CodeBlocks))
	}
	body := p.CodeBlocks[0].Source
	// locate "Hello" within the block body
	start := indexOf(body, "Hello")
	seed := masker.CodeSeed{BlockIndex: 0, Start: start, End: start + len("Hello"), Answer: "Hello"}
	bs := BuildCodeBlocks(p, seed, 120)

	if maskCount(bs) != 1 {
		t.Fatalf("mask count = %d, want 1", maskCount(bs))
	}
	if bs[0].Type != quiz.BlockCodeBlock || !contains(bs[0].Value, "func ") {
		t.Errorf("block0 = %#v, want code_block containing 'func '", bs[0])
	}
	if bs[len(bs)-1].Type != quiz.BlockCodeBlock || !contains(bs[len(bs)-1].Value, "()") {
		t.Errorf("last block = %#v, want code_block containing '()'", bs[len(bs)-1])
	}
}

// TC-G-B-05: the window bounds how much code context is included.
func TestBuildCodeBlocks_WindowBounded(t *testing.T) {
	var sb []byte
	sb = append(sb, "```go\n"...)
	for i := 0; i < 50; i++ {
		sb = append(sb, "x := 1\n"...)
	}
	sb = append(sb, "target := 2\n"...)
	for i := 0; i < 50; i++ {
		sb = append(sb, "y := 3\n"...)
	}
	sb = append(sb, "```\n"...)
	p := parser.ParseProposal("x.md", sb)
	body := p.CodeBlocks[0].Source
	start := indexOf(body, "target")
	seed := masker.CodeSeed{BlockIndex: 0, Start: start, End: start + len("target"), Answer: "target"}
	bs := BuildCodeBlocks(p, seed, 20)

	total := 0
	for _, b := range bs {
		total += len(b.Value)
	}
	if total > 40 {
		t.Errorf("context too wide: %d bytes, want <= 40", total)
	}
}

// TC-G-B-06: every prose quiz body has exactly one mask.
func TestBuildProseBlocks_ExactlyOneMask(t *testing.T) {
	p := parser.ParseProposal("x.md", []byte("a `one` b `two` c `three` d\n"))
	for _, txt := range []string{"one", "two", "three"} {
		ic := findInline(p, txt)
		bs := BuildProseBlocks(p, masker.ProseSeed{Start: ic.Start, End: ic.End, Answer: txt}, 80)
		if maskCount(bs) != 1 {
			t.Errorf("seed %q: mask count = %d, want 1", txt, maskCount(bs))
		}
	}
}

// TC-G-B-07 / 08: empty edges dropped and adjacent text merged.
func TestOptimize_MergesAndDrops(t *testing.T) {
	in := []quiz.Block{
		{Type: quiz.BlockText, Value: ""},      // dropped
		{Type: quiz.BlockText, Value: "a"},     // merged...
		{Type: quiz.BlockText, Value: "b"},     // ...with this
		{Type: quiz.BlockMask},                 // kept
		{Type: quiz.BlockText, Value: "c"},     // kept
	}
	out := optimize(in)
	if len(out) != 3 {
		t.Fatalf("optimize len = %d (%#v), want 3", len(out), out)
	}
	if out[0].Type != quiz.BlockText || out[0].Value != "ab" {
		t.Errorf("out[0] = %#v, want text 'ab'", out[0])
	}
	if out[1].Type != quiz.BlockMask {
		t.Errorf("out[1] = %#v, want mask", out[1])
	}
	if out[2].Type != quiz.BlockText || out[2].Value != "c" {
		t.Errorf("out[2] = %#v, want text 'c'", out[2])
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func contains(s, sub string) bool { return indexOf(s, sub) >= 0 }

func containsNewline(s string) bool {
	for _, r := range s {
		if r == '\n' {
			return true
		}
	}
	return false
}

func joinText(bs []quiz.Block) string {
	var s string
	for _, b := range bs {
		s += b.Value
	}
	return s
}
