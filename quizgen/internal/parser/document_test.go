package parser

import (
	"strings"
	"testing"
)

const sampleDoc = `# Proposal: range over func

We propose that a ` + "`for`" + ` statement with ` + "`range`" + ` accept a ` + "`func`" + ` value.

This paragraph has no inline code at all, and must still survive.

## Details

- first item with ` + "`code`" + ` in it
- second item

> a quoted line

` + "```go" + `
func Count(n int) {}
` + "```" + `
`

func TestParseDocument_KeepsEveryBlockInOrder(t *testing.T) {
	d := ParseDocument("x.md", []byte(sampleDoc), Options{})

	if d.Title != "Proposal: range over func" {
		t.Errorf("title = %q", d.Title)
	}

	var got []BlockKind
	for _, b := range d.Blocks {
		got = append(got, b.Kind)
	}
	want := []BlockKind{
		KindHeading,   // # Proposal
		KindParagraph, // We propose...
		KindParagraph, // paragraph with no inline code
		KindHeading,   // ## Details
		KindListItem,  // - first item
		KindListItem,  // - second item
		KindQuote,     // > a quoted line
		KindCode,      // ```go
	}
	if len(got) != len(want) {
		t.Fatalf("blocks = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("block %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// A paragraph with no inline code carries no blanks but is still part of the
// document; dropping it is what made the old output unreadable.
func TestParseDocument_KeepsProseWithoutInlineCode(t *testing.T) {
	d := ParseDocument("x.md", []byte(sampleDoc), Options{})
	var found bool
	for _, b := range d.Blocks {
		for _, s := range b.Spans {
			if strings.Contains(s.Text, "must still survive") {
				found = true
			}
		}
	}
	if !found {
		t.Error("paragraph without inline code was dropped")
	}
}

func TestParseDocument_SplitsInlineCodeFromText(t *testing.T) {
	d := ParseDocument("x.md", []byte(sampleDoc), Options{})
	var para *DocBlock
	for i, b := range d.Blocks {
		if b.Kind == KindParagraph && len(b.Spans) > 1 {
			para = &d.Blocks[i]
			break
		}
	}
	if para == nil {
		t.Fatal("no paragraph with mixed spans")
	}
	var codes []string
	for _, s := range para.Spans {
		if s.Kind == SpanCode {
			codes = append(codes, s.Text)
		}
	}
	want := []string{"for", "range", "func"}
	if len(codes) != len(want) {
		t.Fatalf("inline codes = %v, want %v", codes, want)
	}
	for i := range want {
		if codes[i] != want[i] {
			t.Errorf("inline code %d = %q, want %q", i, codes[i], want[i])
		}
	}
}

// Links contribute their text but not their URL, and autolinks contribute the
// URL itself — the same result the regex-based cleanText produced for the old
// output, reached here through the AST instead.
func TestParseDocument_LinkHandling(t *testing.T) {
	src := "subscribe to [golang-announce](https://groups.google.com/x) for `news`\n\n" +
		"auto <https://go.dev/doc>\n"
	d := ParseDocument("x.md", []byte(src), Options{})

	var joined string
	for _, b := range d.Blocks {
		for _, s := range b.Spans {
			joined += s.Text
		}
	}
	if strings.Contains(joined, "groups.google.com") || strings.Contains(joined, "](") {
		t.Errorf("raw link syntax leaked: %q", joined)
	}
	if !strings.Contains(joined, "golang-announce") {
		t.Errorf("link text lost: %q", joined)
	}
	if !strings.Contains(joined, "https://go.dev/doc") {
		t.Errorf("autolink text lost: %q", joined)
	}
}

// The old pipeline had to strip stray backticks because it sliced the raw
// source around inline-code offsets. Spans come from the AST here, so the
// delimiters are never part of the text.
func TestParseDocument_NoStrayBackticks(t *testing.T) {
	d := ParseDocument("x.md", []byte("a `code` b\n"), Options{})
	for _, b := range d.Blocks {
		for _, s := range b.Spans {
			if strings.Contains(s.Text, "`") {
				t.Errorf("backtick leaked into span %+v", s)
			}
		}
	}
}

func TestParseDocument_CodeBlockKeepsBodyAndLanguage(t *testing.T) {
	d := ParseDocument("x.md", []byte(sampleDoc), Options{})
	for _, b := range d.Blocks {
		if b.Kind != KindCode {
			continue
		}
		if b.Lang != "go" {
			t.Errorf("lang = %q, want go", b.Lang)
		}
		if !strings.Contains(b.Spans[0].Text, "func Count(n int)") {
			t.Errorf("body = %q", b.Spans[0].Text)
		}
		return
	}
	t.Fatal("no code block")
}
