package main

import (
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/fummicc1/go-masked-quiz/quizgen/quiz"
)

// listView owns the proposal list's persistent widget state. Clickables must
// outlive a frame: recreating them inside the layout function would reset the
// gesture state every frame and no tap would ever register.
type listView struct {
	list  widget.List
	rows  []widget.Clickable
	ready bool
}

func (v *listView) init() {
	v.list.Axis = layout.Vertical
	v.ready = true
}

// ensureRows grows the clickable slice to match the data. Rows are addressed by
// index, so the slice only ever needs to grow.
func (v *listView) ensureRows(n int) {
	for len(v.rows) < n {
		v.rows = append(v.rows, widget.Clickable{})
	}
}

func (u *UI) layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	paint.Fill(gtx.Ops, colBg)
	switch u.screen {
	case screenQuiz:
		return u.layoutDoc(gtx, th)
	default:
		return u.layoutList(gtx, th)
	}
}

func (u *UI) layoutList(gtx layout.Context, th *material.Theme) layout.Dimensions {
	props := u.bundle.Proposals
	u.list.ensureRows(len(props))

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: dp(16), Bottom: dp(8), Left: dp(16), Right: dp(16)}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					l := material.H6(th, "Go Proposals")
					l.Color = colText
					return l.Layout(gtx)
				})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(props) == 0 {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Body1(th, "Loading…")
					l.Color = colFaint
					return l.Layout(gtx)
				})
			}
			return material.List(th, &u.list.list).Layout(gtx, len(props),
				func(gtx layout.Context, i int) layout.Dimensions {
					return layout.Inset{Left: dp(16), Right: dp(16), Bottom: dp(10)}.Layout(gtx,
						func(gtx layout.Context) layout.Dimensions {
							return u.proposalRow(gtx, th, i)
						})
				})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: dp(6), Bottom: dp(10)}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						l := material.Caption(th, fmt.Sprintf("source · %s", u.source))
						l.Color = colFaint
						l.Font.Typeface = "monospace"
						return l.Layout(gtx)
					})
				})
		}),
	)
}

func (u *UI) proposalRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	p := u.bundle.Proposals[i]
	total := blankCount(p)
	answered, _ := u.store.progress(p.ID)

	click := &u.list.rows[i]
	if click.Clicked(gtx) {
		u.selected = i
		u.screen = screenQuiz
		u.docV.open2(p, u.store)
		// The screen changes while this frame is already laying out the list,
		// so the new screen only appears on the next frame — and without this
		// there is no next frame until some other input arrives, making the
		// tap look ignored.
		gtx.Execute(op.InvalidateCmd{})
	}

	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return card(gtx, colSurface, colBorder, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.rowHeader(gtx, th, p, total, answered)
					}),
					layout.Rigid(layout.Spacer{Height: dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Body1(th, displayTitle(p))
						l.Color = colText
						l.MaxLines = 2
						return l.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						var frac float32
						if total > 0 {
							frac = float32(answered) / float32(total)
						}
						fill := colAccent
						if total > 0 && answered == total {
							fill = colSuccess
						}
						return progressRail(gtx, frac, fill)
					}),
				)
			})
		})
	})
}

func (u *UI) rowHeader(gtx layout.Context, th *material.Theme, p quiz.Proposal, total, answered int) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return badge(gtx, th, displayNumber(p))
		}),
		layout.Flexed(1, layout.Spacer{}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := fmt.Sprintf("%d blanks", total)
			if answered > 0 {
				label = fmt.Sprintf("%d/%d", answered, total)
			}
			l := material.Caption(th, label)
			l.Color = colFaint
			l.Font.Typeface = "monospace"
			return l.Layout(gtx)
		}),
	)
}

// badge draws the small capsule holding a proposal number. The label is laid
// out first to measure it, then the capsule is painted underneath and the label
// replayed on top, offset by the padding.
func badge(gtx layout.Context, th *material.Theme, s string) layout.Dimensions {
	l := material.Caption(th, s)
	l.Color = colAccent
	l.Font.Typeface = "monospace"

	inner := gtx
	inner.Constraints.Min = image.Point{}
	rec := recordMacro(inner, l.Layout)

	padX, padY := gtx.Dp(dp(9)), gtx.Dp(dp(3))
	size := image.Point{X: rec.dims.Size.X + padX*2, Y: rec.dims.Size.Y + padY*2}

	fillRRect(gtx, colAccentDim, dp(9), size)
	defer op.Offset(image.Point{X: padX, Y: padY}).Push(gtx.Ops).Pop()
	rec.call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}
