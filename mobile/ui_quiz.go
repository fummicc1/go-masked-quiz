package main

import (
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/fummicc1/go-masked-quiz/quizgen/quiz"
)

// docView is the reader: one proposal shown as the document it is, with the
// blanks answered in place.
type docView struct {
	proposal quiz.Proposal
	answers  map[int]answer

	list  widget.List
	back  widget.Clickable
	reset widget.Clickable

	// taps is one clickable per mask occurrence, keyed by block and span so a
	// blank appearing twice can be tapped at either place.
	taps map[maskSite]*widget.Clickable

	// open is the blank whose choices are showing, or -1.
	open    int
	choices map[choiceRef]*widget.Clickable
	dismiss widget.Clickable
}

// maskSite locates one mask occurrence in the document.
type maskSite struct {
	Block int
	Span  int
}

type choiceRef struct {
	Blank  int
	Choice int
}

func (v *docView) open2(p quiz.Proposal, store *scoreStore) {
	v.proposal = p
	v.answers = store.answers(p.ID)
	v.list = widget.List{}
	v.list.Axis = layout.Vertical
	v.taps = map[maskSite]*widget.Clickable{}
	v.choices = map[choiceRef]*widget.Clickable{}
	v.open = -1
}

func (v *docView) tapAt(site maskSite) *widget.Clickable {
	c, ok := v.taps[site]
	if !ok {
		c = &widget.Clickable{}
		v.taps[site] = c
	}
	return c
}

func (v *docView) choiceAt(ref choiceRef) *widget.Clickable {
	c, ok := v.choices[ref]
	if !ok {
		c = &widget.Clickable{}
		v.choices[ref] = c
	}
	return c
}

func (u *UI) layoutDoc(gtx layout.Context, th *material.Theme) layout.Dimensions {
	v := &u.docV
	p := v.proposal

	if v.back.Clicked(gtx) {
		u.screen = screenList
		gtx.Execute(op.InvalidateCmd{})
	}
	if v.reset.Clicked(gtx) {
		u.store.reset(p.ID)
		v.answers = map[int]answer{}
		v.open = -1
		gtx.Execute(op.InvalidateCmd{})
	}

	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.docHeader(gtx, th, v, p)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &v.list).Layout(gtx, len(p.Document.Blocks),
				func(gtx layout.Context, i int) layout.Dimensions {
					return u.docBlock(gtx, th, v, i)
				})
		}),
	)

	// The sheet floats over the document, so it is drawn after it and consumes
	// the taps that would otherwise reach the text underneath.
	if v.open >= 0 {
		u.choiceSheet(gtx, th, v)
	}
	return dims
}

func (u *UI) docHeader(gtx layout.Context, th *material.Theme, v *docView, p quiz.Proposal) layout.Dimensions {
	answered, correct := u.store.progress(p.ID)
	total := len(p.Document.Blanks)

	return layout.Inset{Top: dp(12), Bottom: dp(10), Left: dp(16), Right: dp(16)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return v.back.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								l := material.Body1(th, "‹ Back")
								l.Color = colAccent
								return l.Layout(gtx)
							})
						}),
						layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return v.reset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								l := material.Caption(th, "Reset")
								l.Color = colFaint
								if answered == 0 {
									l.Color = colBorder
								}
								return l.Layout(gtx)
							})
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					var frac float32
					if total > 0 {
						frac = float32(answered) / float32(total)
					}
					return progressRail(gtx, frac, colAccent)
				}),
				layout.Rigid(layout.Spacer{Height: dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					s := fmt.Sprintf("%d/%d blanks", answered, total)
					if answered > 0 {
						s += fmt.Sprintf(" · %d%% correct", correct*100/answered)
					}
					l := material.Caption(th, s)
					l.Color = colFaint
					l.Font.Typeface = "monospace"
					return l.Layout(gtx)
				}),
			)
		})
}

// docBlock lays out one block and overlays a tap target on each of its masks.
func (u *UI) docBlock(gtx layout.Context, th *material.Theme, v *docView, i int) layout.Dimensions {
	b := v.proposal.Document.Blocks[i]
	bv := buildBlockView(th, b, v.proposal.Document, v.answers)

	return blockInset(b).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if b.Kind == quiz.BlockCode {
			return u.codeBlock(gtx, th, v, i, b, bv)
		}
		return u.textBlock(gtx, th, v, i, b, bv)
	})
}

func (u *UI) textBlock(gtx layout.Context, th *material.Theme, v *docView, i int, b quiz.DocBlock, bv blockView) layout.Dimensions {
	rec := recordMacro(gtx, func(gtx layout.Context) layout.Dimensions {
		return u.layoutBlock(gtx, th, b, bv)
	})
	if b.Kind == quiz.BlockQuote {
		bar := image.Point{X: gtx.Dp(dp(3)), Y: rec.dims.Size.Y}
		fillRRect(gtx, colBorder, dp(2), bar)
	}
	rec.call.Add(gtx.Ops)
	u.maskTargets(gtx, v, i, b, bv, rec.dims)
	return rec.dims
}

func (u *UI) codeBlock(gtx layout.Context, th *material.Theme, v *docView, i int, b quiz.DocBlock, bv blockView) layout.Dimensions {
	return card(gtx, colBg, colBorder, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			dims := u.layoutBlock(gtx, th, b, bv)
			u.maskTargets(gtx, v, i, b, bv, dims)
			return dims
		})
	})
}

// maskTargets covers the whole block with one clickable per mask.
//
// styledtext gives no way to hit-test a span, so instead of trying to place a
// target over each chip, the block gets one target per mask stacked over it and
// the topmost unanswered one wins. That is enough while a block holds few
// blanks, which the generator enforces.
func (u *UI) maskTargets(gtx layout.Context, v *docView, blockIdx int, b quiz.DocBlock, bv blockView, dims layout.Dimensions) {
	for si, isMask := range bv.mask {
		if !isMask {
			continue
		}
		bi := bv.blank[si]
		if _, done := v.answers[bi]; done {
			continue
		}
		site := maskSite{Block: blockIdx, Span: si}
		c := v.tapAt(site)
		if c.Clicked(gtx) {
			v.open = bi
			gtx.Execute(op.InvalidateCmd{})
		}
		c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: dims.Size}
		})
		return // one target per block is enough; see the comment above
	}
}

// choiceSheet is the panel that opens when a mask is tapped.
func (u *UI) choiceSheet(gtx layout.Context, th *material.Theme, v *docView) {
	d := v.proposal.Document
	if v.open < 0 || v.open >= len(d.Blanks) {
		v.open = -1
		return
	}
	blank := d.Blanks[v.open]

	if v.dismiss.Clicked(gtx) {
		v.open = -1
		gtx.Execute(op.InvalidateCmd{})
	}

	// Scrim: dims the document and swallows taps meant for the sheet.
	v.dismiss.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
		paint.ColorOp{Color: colScrim}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})

	layout.S.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return card(gtx, colElevated, colBorder, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Caption(th, fmt.Sprintf("%s  what goes here?", blankMarker(v.open)))
						l.Color = colSecondary
						return l.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: dp(12)}.Layout),
				}
				for ci, choice := range blank.Choices {
					ci, choice := ci, choice
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return u.sheetChoice(gtx, th, v, ci, choice, blank)
						}),
						layout.Rigid(layout.Spacer{Height: dp(8)}.Layout),
					)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
	})
}

func (u *UI) sheetChoice(gtx layout.Context, th *material.Theme, v *docView, ci int, choice string, blank quiz.Blank) layout.Dimensions {
	c := v.choiceAt(choiceRef{Blank: v.open, Choice: ci})
	if c.Clicked(gtx) {
		a := answer{Choice: choice, Correct: choice == blank.Answer}
		v.answers[v.open] = a
		u.store.record(v.proposal.ID, blankKey{BlankIndex: v.open}, a)
		v.open = -1
		gtx.Execute(op.InvalidateCmd{})
		return layout.Dimensions{}
	}
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return card(gtx, colAccentDim, colAccentDim, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Body1(th, choice)
				l.Color = colText
				l.Font.Typeface = "monospace"
				l.MaxLines = 1
				return l.Layout(gtx)
			})
		})
	})
}

// blankMarker labels a blank. Circled digits exist in the system CJK font Gio
// falls back to on Android, so they render without bundling anything.
func blankMarker(i int) string {
	circled := []string{"①", "②", "③", "④", "⑤", "⑥", "⑦", "⑧", "⑨"}
	if i < len(circled) {
		return circled[i]
	}
	return fmt.Sprintf("(%d)", i+1)
}
