package main

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/styledtext"
	"github.com/fummicc1/go-masked-quiz/quizgen/quiz"
)

// blockView is the per-frame rendering input for one document block: the spans
// to draw, and which of them are masks needing a filled chip behind them.
type blockView struct {
	spans []styledtext.SpanStyle
	mask  []bool
	fill  []color.NRGBA
	blank []int // blank index per span, -1 when not a mask
}

// layoutBlock draws one block of the proposal.
//
// Masks are drawn as filled chips inside the flowing text. Gio has no per-span
// background — the trick is that styledtext reports each span's rectangle to a
// callback *after* drawing it, once per line fragment when a span wraps. So the
// text is laid out twice: the first pass paints the chip backgrounds from those
// rectangles, the second draws the glyphs on top, because Gio paints in op
// order and a background recorded later would cover the text.
func (u *UI) layoutBlock(gtx layout.Context, th *material.Theme, b quiz.DocBlock, bv blockView) layout.Dimensions {
	if len(bv.spans) == 0 {
		return layout.Dimensions{}
	}
	st := styledtext.Text(th.Shaper, bv.spans...)
	// Code has long unbroken runs that would overflow the screen; prose reads
	// better broken at words.
	if b.Kind == quiz.BlockCode {
		st.WrapPolicy = styledtext.WrapGraphemes
	}

	dims := st.Layout(gtx, func(gtx layout.Context, idx int, d layout.Dimensions) {
		if idx >= len(bv.mask) || !bv.mask[idx] || d.Size.X == 0 {
			return
		}
		pad := gtx.Dp(dp(1))
		r := image.Rect(-pad, -1, d.Size.X+pad, d.Size.Y+1)
		rr := clip.RRect{Rect: r, SE: gtx.Dp(dp(4)), SW: gtx.Dp(dp(4)), NE: gtx.Dp(dp(4)), NW: gtx.Dp(dp(4))}
		defer rr.Push(gtx.Ops).Pop()
		paint.ColorOp{Color: bv.fill[idx]}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	})
	st.Layout(gtx, nil)
	return dims
}

// maxBlockChars caps how much text one block renders.
//
// A block is laid out as a single wrapped run, so its cost is not bounded by
// the screen: the largest code block in the corpus is ~38k characters and costs
// ~90MB to lay out, which iOS kills the app for. Proposals occasionally paste
// whole files, and no one reads those on a phone anyway.
const maxBlockChars = 4000

// truncateSpans limits a block's text to maxBlockChars, keeping whole spans and
// appending a marker so the elision is visible rather than silent.
func truncateSpans(spans []quiz.Span) ([]quiz.Span, bool) {
	total := 0
	for _, s := range spans {
		total += len(s.Value)
	}
	if total <= maxBlockChars {
		return spans, false
	}

	out := make([]quiz.Span, 0, len(spans))
	used := 0
	for _, s := range spans {
		if used >= maxBlockChars {
			break
		}
		if s.Kind == quiz.SpanMask {
			out = append(out, s)
			continue
		}
		if room := maxBlockChars - used; len(s.Value) > room {
			s.Value = s.Value[:room]
		}
		used += len(s.Value)
		out = append(out, s)
	}
	return out, true
}

// buildBlockView converts a block's spans into styled runs, resolving each mask
// to its current label and colour: a number while unanswered, the answer once
// it has been picked.
func buildBlockView(th *material.Theme, b quiz.DocBlock, d quiz.Document, answers map[int]answer) blockView {
	bv := blockView{}
	add := func(s styledtext.SpanStyle, isMask bool, fill color.NRGBA, blank int) {
		bv.spans = append(bv.spans, s)
		bv.mask = append(bv.mask, isMask)
		bv.fill = append(bv.fill, fill)
		bv.blank = append(bv.blank, blank)
	}

	size, weight, col := blockTextStyle(b)
	mono := font.Font{Typeface: "monospace"}

	spans, truncated := truncateSpans(b.Spans)
	for _, s := range spans {
		switch s.Kind {
		case quiz.SpanText:
			f := font.Font{Weight: weight}
			if b.Kind == quiz.BlockCode {
				f = mono
			}
			add(styledtext.SpanStyle{Content: s.Value, Size: size, Color: col, Font: f}, false, color.NRGBA{}, -1)

		case quiz.SpanInlineCode:
			add(styledtext.SpanStyle{
				Content: s.Value, Size: unit.Sp(14), Color: colAccent, Font: mono,
			}, false, color.NRGBA{}, -1)

		case quiz.SpanMask:
			if s.BlankIndex == nil || *s.BlankIndex >= len(d.Blanks) {
				continue
			}
			bi := *s.BlankIndex
			label, fill := blankMarker(bi), colAccent
			if a, ok := answers[bi]; ok {
				label = d.Blanks[bi].Answer
				fill = colSuccess
				if !a.Correct {
					fill = colDanger
				}
			}
			// Padding spaces keep the filled chip from butting against the
			// neighbouring words; the fill covers them.
			add(styledtext.SpanStyle{
				Content: " " + label + " ", Size: unit.Sp(14), Color: colOnAccent,
				Font: font.Font{Typeface: "monospace", Weight: font.Bold},
			}, true, fill, bi)
		}
	}
	if truncated {
		bv.spans = append(bv.spans, styledtext.SpanStyle{
			Content: "\n… (truncated)", Size: unit.Sp(13), Color: colFaint,
			Font: font.Font{Typeface: "monospace"},
		})
		bv.mask = append(bv.mask, false)
		bv.fill = append(bv.fill, color.NRGBA{})
		bv.blank = append(bv.blank, -1)
	}
	return bv
}

// blockTextStyle is the base type treatment for a block kind.
func blockTextStyle(b quiz.DocBlock) (unit.Sp, font.Weight, color.NRGBA) {
	switch b.Kind {
	case quiz.BlockHeading:
		switch b.Level {
		case 1:
			return unit.Sp(24), font.Bold, colText
		case 2:
			return unit.Sp(20), font.Bold, colText
		default:
			return unit.Sp(17), font.Bold, colText
		}
	case quiz.BlockCode:
		return unit.Sp(13), font.Normal, colText
	case quiz.BlockQuote:
		return unit.Sp(15), font.Normal, colSecondary
	default:
		return unit.Sp(16), font.Normal, colText
	}
}

// blockInset is the surrounding space for a block, giving headings room and
// indenting nested list items and quotes.
func blockInset(b quiz.DocBlock) layout.Inset {
	in := layout.Inset{Top: dp(6), Bottom: dp(6), Left: dp(16), Right: dp(16)}
	switch b.Kind {
	case quiz.BlockHeading:
		in.Top, in.Bottom = dp(18), dp(6)
	case quiz.BlockListItem:
		in.Left = dp(16) + dp(float32(14*max(b.Level, 1)))
	case quiz.BlockQuote:
		in.Left = dp(28)
	case quiz.BlockCode:
		in.Top, in.Bottom = dp(8), dp(8)
	}
	return in
}
