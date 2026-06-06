// Package blocks turns a seed (a chosen identifier and its location) plus the
// surrounding source into the pre-parsed []quiz.Block body the JSON ships. The
// transform is a pure function of its inputs — no randomness — so output is
// deterministic.
package blocks

import (
	"sort"
	"strings"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/masker"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/quiz"
)

// BuildProseBlocks expands a prose seed into text / inline_code / mask blocks.
// A context window of contextWidth bytes on each side is taken, snapped to line
// boundaries; the seed becomes the single mask, and any other inline-code spans
// inside the window are preserved as inline_code blocks.
func BuildProseBlocks(p *parser.Proposal, seed masker.ProseSeed, contextWidth int) []quiz.Block {
	src := p.Source
	// Confine the window to the seed's own line, then trim it to contextWidth
	// on each side. This keeps text blocks free of newlines and never pulls in
	// neighbouring lines.
	lineStart := lineStartAt(src, seed.Start)
	lineEnd := lineEndAt(src, seed.End)
	wStart := clampLow(seed.Start - contextWidth)
	if wStart < lineStart {
		wStart = lineStart
	}
	wEnd := clampHigh(seed.End+contextWidth, len(src))
	if wEnd > lineEnd {
		wEnd = lineEnd
	}

	// Collect the spans that interrupt the plain text: the mask, plus other
	// inline-code spans fully inside the window.
	type span struct {
		start, end int
		typ        quiz.BlockType
		val        string
	}
	spans := []span{{seed.Start, seed.End, quiz.BlockMask, ""}}
	for _, ic := range p.InlineCodes {
		if ic.Start == seed.Start && ic.End == seed.End {
			continue
		}
		if ic.Start >= wStart && ic.End <= wEnd {
			spans = append(spans, span{ic.Start, ic.End, quiz.BlockInlineCode, ic.Text})
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })

	var out []quiz.Block
	cur := wStart
	for _, s := range spans {
		if s.start > cur {
			out = append(out, quiz.Block{Type: quiz.BlockText, Value: cleanText(string(src[cur:s.start]))})
		}
		if s.typ == quiz.BlockMask {
			out = append(out, quiz.Block{Type: quiz.BlockMask})
		} else {
			out = append(out, quiz.Block{Type: s.typ, Value: s.val})
		}
		cur = s.end
	}
	if cur < wEnd {
		out = append(out, quiz.Block{Type: quiz.BlockText, Value: cleanText(string(src[cur:wEnd]))})
	}
	return optimize(out)
}

// cleanText strips the backticks that border an inline-code span (the span's
// own offsets sit just inside the backticks, so the neighbouring text would
// otherwise keep a stray "`").
func cleanText(s string) string {
	return strings.Trim(s, "`")
}

// optimize drops empty non-mask blocks and merges adjacent same-type blocks.
func optimize(blocks []quiz.Block) []quiz.Block {
	var out []quiz.Block
	for _, b := range blocks {
		if b.Type != quiz.BlockMask && b.Value == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1].Type == b.Type && b.Type != quiz.BlockMask {
			out[n-1].Value += b.Value
			continue
		}
		out = append(out, b)
	}
	return out
}

func clampLow(x int) int {
	if x < 0 {
		return 0
	}
	return x
}

func clampHigh(x, hi int) int {
	if x > hi {
		return hi
	}
	return x
}

// lineStartAt returns the offset just after the newline preceding pos (or 0).
func lineStartAt(src []byte, pos int) int {
	for i := pos - 1; i >= 0; i-- {
		if src[i] == '\n' {
			return i + 1
		}
	}
	return 0
}

// lineEndAt returns the offset of the newline at or after pos (or len(src)).
func lineEndAt(src []byte, pos int) int {
	for i := pos; i < len(src); i++ {
		if src[i] == '\n' {
			return i
		}
	}
	return len(src)
}
