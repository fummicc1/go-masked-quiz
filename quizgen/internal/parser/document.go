package parser

import (
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// BlockKind is how a document block is laid out when rendered.
type BlockKind string

const (
	KindParagraph BlockKind = "paragraph"
	KindHeading   BlockKind = "heading"
	KindCode      BlockKind = "code"
	KindListItem  BlockKind = "list_item"
	KindQuote     BlockKind = "quote"
)

// SpanKind distinguishes prose from inline code within a block.
type SpanKind string

const (
	SpanText SpanKind = "text"
	SpanCode SpanKind = "inline_code"
)

// DocSpan is one run of text inside a block.
type DocSpan struct {
	Kind SpanKind
	Text string
}

// DocBlock is one renderable block of a proposal, in reading order.
type DocBlock struct {
	Kind  BlockKind
	Level int    // heading level, or list nesting depth
	Lang  string // fenced code language
	Spans []DocSpan
}

// Document is a proposal as a reader sees it: every block, in order, not just
// the ones that happen to be quizzable.
//
// This is deliberately a different view from Proposal, which keeps only the
// paragraphs and code blocks a quiz can be built from. Reading the proposal and
// answering blanks in place needs the parts in between as well.
type Document struct {
	Slug   string
	Title  string
	Blocks []DocBlock
}

// ParseDocument reads the whole Markdown document into ordered blocks.
func ParseDocument(filename string, src []byte, opts Options) *Document {
	slug := strings.TrimSuffix(filename, filepath.Ext(filename))
	d := &Document{Slug: slug, Title: slug}

	root := goldmark.New().Parser().Parse(text.NewReader(src))
	titleSet := false

	var walk func(n ast.Node, depth int)
	walk = func(n ast.Node, depth int) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch node := c.(type) {
			case *ast.Heading:
				t := nodeText(node, src)
				if node.Level == 1 && !titleSet && t != "" {
					d.Title = t
					titleSet = true
				}
				d.Blocks = append(d.Blocks, DocBlock{
					Kind:  KindHeading,
					Level: node.Level,
					Spans: inlineSpans(node, src),
				})

			case *ast.Paragraph:
				kind := KindParagraph
				if depth > 0 {
					kind = KindListItem
				}
				if spans := inlineSpans(node, src); len(spans) > 0 {
					d.Blocks = append(d.Blocks, DocBlock{Kind: kind, Level: depth, Spans: spans})
				}

			case *ast.TextBlock:
				// Paragraph content inside a tight list item.
				if spans := inlineSpans(node, src); len(spans) > 0 {
					d.Blocks = append(d.Blocks, DocBlock{Kind: KindListItem, Level: depth, Spans: spans})
				}

			case *ast.FencedCodeBlock:
				lang := string(node.Language(src))
				body, _ := blockBody(node, src)
				if lang == "" && opts.AcceptBareGoFences && looksLikeGo(body) {
					lang = "go"
				}
				d.Blocks = append(d.Blocks, DocBlock{
					Kind:  KindCode,
					Lang:  lang,
					Spans: []DocSpan{{Kind: SpanText, Text: body}},
				})

			case *ast.CodeBlock:
				d.Blocks = append(d.Blocks, DocBlock{
					Kind:  KindCode,
					Spans: []DocSpan{{Kind: SpanText, Text: rawLines(node, src)}},
				})

			case *ast.Blockquote:
				start := len(d.Blocks)
				walk(node, depth)
				for i := start; i < len(d.Blocks); i++ {
					d.Blocks[i].Kind = KindQuote
				}

			case *ast.List:
				walk(node, depth+1)

			case *ast.ListItem:
				walk(node, depth)

			default:
				walk(c, depth)
			}
		}
	}
	walk(root, 0)

	return d
}

// inlineSpans flattens a block's inline children into text and code runs,
// preserving order. Emphasis and links contribute their text; their markup is
// dropped because the client renders plain runs.
func inlineSpans(n ast.Node, src []byte) []DocSpan {
	var spans []DocSpan
	appendText := func(s string) {
		if s == "" {
			return
		}
		// Merge with the previous run so decorated text does not fragment the
		// line into dozens of spans.
		if k := len(spans) - 1; k >= 0 && spans[k].Kind == SpanText {
			spans[k].Text += s
			return
		}
		spans = append(spans, DocSpan{Kind: SpanText, Text: s})
	}

	var walk func(ast.Node)
	walk = func(node ast.Node) {
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			switch v := c.(type) {
			case *ast.CodeSpan:
				if t := nodeText(v, src); t != "" {
					spans = append(spans, DocSpan{Kind: SpanCode, Text: t})
				}
			case *ast.Text:
				appendText(string(v.Segment.Value(src)))
				if v.SoftLineBreak() || v.HardLineBreak() {
					appendText("\n")
				}
			case *ast.AutoLink:
				appendText(string(v.URL(src)))
			default:
				walk(c)
			}
		}
	}
	walk(n)
	return spans
}

// rawLines returns the literal text of an indented code block.
func rawLines(n ast.Node, src []byte) string {
	var b strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		s := lines.At(i)
		b.Write(s.Value(src))
	}
	return b.String()
}
