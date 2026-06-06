// Package parser turns a single golang/proposal design/*.md file into a
// structured Proposal: its title plus the byte offsets of every inline `code`
// span and every fenced ```go code block.
//
// Offsets are byte positions into Proposal.Source. Inline-code offsets are used
// to mask identifiers in prose; code-block bodies are scanned separately with
// go/scanner. Markdown is parsed with goldmark.
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

// InlineCode is a single inline `code` span.
//
// Start and End are byte offsets into Proposal.Source: Start points just after
// the opening backtick, End just before the closing backtick, so
// Source[Start:End] == Text.
type InlineCode struct {
	Start int
	End   int
	Text  string
}

// CodeBlock is a fenced code block whose info string names a language.
//
// Source is the block body (without the fences). LineStart is the 1-based line
// number of the body's first line within the Markdown file, for diagnostics.
type CodeBlock struct {
	Language  string
	Source    string
	LineStart int
}

// Proposal is the parsed view of one design/*.md file.
type Proposal struct {
	Slug        string // file name without extension, e.g. "61405-range-func"
	Title       string // first H1, or Slug if the file has none
	Source      []byte // raw Markdown bytes
	InlineCodes []InlineCode
	CodeBlocks  []CodeBlock // only blocks whose language is "go"
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
		case *ast.CodeSpan:
			if ic, ok := inlineCode(node, src); ok {
				p.InlineCodes = append(p.InlineCodes, ic)
			}
		case *ast.FencedCodeBlock:
			lang := string(node.Language(src))
			if lang != "go" {
				return ast.WalkContinue, nil
			}
			body, lineStart := blockBody(node, src)
			p.CodeBlocks = append(p.CodeBlocks, CodeBlock{
				Language:  lang,
				Source:    body,
				LineStart: lineStart,
			})
		}
		return ast.WalkContinue, nil
	})

	return p
}

// inlineCode extracts an inline code span's text and byte range. goldmark
// represents the span's content as one or more child text segments; we span
// from the first segment's start to the last segment's stop.
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

// blockBody concatenates a fenced block's line segments into its body and
// computes the 1-based line number of the body's first line.
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
	lineStart = 1 + strings.Count(string(src[:first.Start]), "\n")
	return sb.String(), lineStart
}

// nodeText concatenates the raw text segments under an inline-container node
// (used for headings).
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
