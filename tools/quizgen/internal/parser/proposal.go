// Package parser turns a single golang/proposal design/*.md file into a
// structured Proposal: its title, the prose paragraphs (each with the inline
// `code` spans it contains), and the fenced ```go code blocks.
//
// Quizzes are built per "unit" — one prose paragraph or one code block — so the
// parser groups inline-code spans by their containing paragraph. Offsets are
// byte positions into Proposal.Source. Markdown is parsed with goldmark.
package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// InlineCode is a single inline `code` span. Start/End are byte offsets into
// Proposal.Source: Start just after the opening backtick, End just before the
// closing one, so Source[Start:End] == Text.
type InlineCode struct {
	Start int
	End   int
	Text  string
}

// ProseUnit is one paragraph and the inline-code spans inside it. Start/End are
// byte offsets bounding the paragraph in Proposal.Source.
type ProseUnit struct {
	Start       int
	End         int
	InlineCodes []InlineCode
}

// CodeBlock is a fenced code block whose info string names a language. Source
// is the block body (without fences). LineStart is the 1-based line of the
// body's first line, for diagnostics.
type CodeBlock struct {
	Language  string
	Source    string
	LineStart int
}

// Proposal is the parsed view of one design/*.md file.
type Proposal struct {
	Slug       string      // file name without extension, e.g. "61405-range-func"
	Title      string      // first H1, or Slug if none
	Source     []byte      // raw Markdown bytes
	ProseUnits []ProseUnit // paragraphs containing at least one inline-code span
	CodeBlocks []CodeBlock // blocks whose language is "go"
}

// LoadProposal reads and parses the file at path.
func LoadProposal(path string) (*Proposal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read proposal %q: %w", path, err)
	}
	return ParseProposal(filepath.Base(path), data), nil
}

// ParseProposal parses raw Markdown. filename is used only to derive the slug.
func ParseProposal(filename string, src []byte) *Proposal {
	slug := strings.TrimSuffix(filename, filepath.Ext(filename))
	p := &Proposal{Slug: slug, Title: slug, Source: src}

	doc := goldmark.New().Parser().Parse(text.NewReader(src))
	titleSet := false
	var allInline []InlineCode
	var paraRanges [][2]int

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Heading:
			if node.Level == 1 && !titleSet {
				if t := nodeText(node, src); t != "" {
					p.Title = t
					titleSet = true
				}
			}
		case *ast.Paragraph:
			if r, ok := paragraphRange(node); ok {
				paraRanges = append(paraRanges, r)
			}
		case *ast.CodeSpan:
			if ic, ok := inlineCode(node, src); ok {
				allInline = append(allInline, ic)
			}
		case *ast.FencedCodeBlock:
			lang := string(node.Language(src))
			if lang != "go" {
				return ast.WalkContinue, nil
			}
			body, lineStart := blockBody(node, src)
			p.CodeBlocks = append(p.CodeBlocks, CodeBlock{Language: lang, Source: body, LineStart: lineStart})
		}
		return ast.WalkContinue, nil
	})

	// Group inline-code spans into their containing paragraph. Spans outside any
	// paragraph (e.g. inside a heading) are dropped — only paragraph prose is
	// quizzed.
	for _, r := range paraRanges {
		var ics []InlineCode
		for _, ic := range allInline {
			if ic.Start >= r[0] && ic.End <= r[1] {
				ics = append(ics, ic)
			}
		}
		if len(ics) > 0 {
			p.ProseUnits = append(p.ProseUnits, ProseUnit{Start: r[0], End: r[1], InlineCodes: ics})
		}
	}

	return p
}

// paragraphRange returns the byte range [start, end) spanned by a paragraph.
func paragraphRange(n *ast.Paragraph) ([2]int, bool) {
	lines := n.Lines()
	if lines.Len() == 0 {
		return [2]int{}, false
	}
	return [2]int{lines.At(0).Start, lines.At(lines.Len() - 1).Stop}, true
}

// inlineCode extracts an inline code span's text and byte range from its child
// text segments.
func inlineCode(n *ast.CodeSpan, src []byte) (InlineCode, bool) {
	start, end := -1, -1
	var sb strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		t, ok := c.(*ast.Text)
		if !ok {
			continue
		}
		seg := t.Segment
		if start < 0 {
			start = seg.Start
		}
		end = seg.Stop
		sb.Write(seg.Value(src))
	}
	if start < 0 || end < start {
		return InlineCode{}, false
	}
	return InlineCode{Start: start, End: end, Text: sb.String()}, true
}

// blockBody concatenates a fenced block's line segments and computes the 1-based
// line of its first line.
func blockBody(n *ast.FencedCodeBlock, src []byte) (body string, lineStart int) {
	lines := n.Lines()
	if lines.Len() == 0 {
		return "", 0
	}
	var sb strings.Builder
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		sb.Write(seg.Value(src))
	}
	first := lines.At(0)
	return sb.String(), 1 + strings.Count(string(src[:first.Start]), "\n")
}

// nodeText concatenates the text under an inline-container node (for headings).
func nodeText(n ast.Node, src []byte) string {
	var sb strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch t := c.(type) {
		case *ast.Text:
			sb.Write(t.Segment.Value(src))
		case *ast.CodeSpan:
			if ic, ok := inlineCode(t, src); ok {
				sb.WriteString(ic.Text)
			}
		}
	}
	return strings.TrimSpace(sb.String())
}
