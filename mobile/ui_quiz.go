package main

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/styledtext"
	"github.com/fummicc1/go-masked-quiz/quizgen/quiz"
)

// quizView owns the per-proposal state for the quiz screen: which blanks are
// answered, and the widget state for every tappable choice.
type quizView struct {
	proposal quiz.Proposal
	answers  map[blankKey]answer

	list    widget.List
	back    widget.Clickable
	reset   widget.Clickable
	choices map[choiceKey]*widget.Clickable
}

// choiceKey addresses one tappable option: which blank, and which of its
// choices.
type choiceKey struct {
	blankKey
	Choice int
}

// open re-seeds the view for a proposal, discarding the previous proposal's
// widget state so scroll position and choice clickables start fresh.
func (v *quizView) open(p quiz.Proposal, store *scoreStore) {
	v.proposal = p
	v.answers = store.answers(p.ID)
	v.list = widget.List{}
	v.list.Axis = layout.Vertical
	v.choices = map[choiceKey]*widget.Clickable{}
}

func (v *quizView) clickable(k choiceKey) *widget.Clickable {
	c, ok := v.choices[k]
	if !ok {
		c = &widget.Clickable{}
		v.choices[k] = c
	}
	return c
}

func (u *UI) layoutQuiz(gtx layout.Context, th *material.Theme) layout.Dimensions {
	v := &u.quizV
	p := v.proposal

	if v.back.Clicked(gtx) {
		u.screen = screenList
	}
	if v.reset.Clicked(gtx) {
		u.store.reset(p.ID)
		v.answers = map[blankKey]answer{}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.quizHeader(gtx, th, v, p)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &v.list).Layout(gtx, len(p.Quizzes),
				func(gtx layout.Context, i int) layout.Dimensions {
					return layout.Inset{Left: dp(16), Right: dp(16), Bottom: dp(12)}.Layout(gtx,
						func(gtx layout.Context) layout.Dimensions {
							return u.quizCard(gtx, th, v, p.Quizzes[i])
						})
				})
		}),
	)
}

func (u *UI) quizHeader(gtx layout.Context, th *material.Theme, v *quizView, p quiz.Proposal) layout.Dimensions {
	answered, correct := u.store.progress(p.ID)
	total := blankCount(p)

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
					l := material.Body2(th, displayTitle(p))
					l.Color = colText
					l.MaxLines = 2
					return l.Layout(gtx)
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

func (u *UI) quizCard(gtx layout.Context, th *material.Theme, v *quizView, q quiz.Quiz) layout.Dimensions {
	return card(gtx, colSurface, colBorder, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Caption(th, fmt.Sprintf("Q%d", q.Index+1))
							l.Color = colSecondary
							l.Font.Typeface = "monospace"
							return l.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return badge(gtx, th, string(q.Kind))
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.quizBody(gtx, th, v, q)
				}),
			}
			for bi := range q.Blanks {
				bi := bi
				children = append(children,
					layout.Rigid(layout.Spacer{Height: dp(12)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.blankRow(gtx, th, v, q, bi)
					}),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

// blankRow renders one blank's marker, result line, and its choice chips.
func (u *UI) blankRow(gtx layout.Context, th *material.Theme, v *quizView, q quiz.Quiz, bi int) layout.Dimensions {
	blank := q.Blanks[bi]
	key := blankKey{QuizIndex: q.Index, BlankIndex: bi}
	ans, answered := v.answers[key]

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(th, blankMarker(bi))
					l.Color = colAccent
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !answered {
						return layout.Dimensions{}
					}
					text, col := "correct", colSuccess
					if !ans.Correct {
						text, col = "answer: "+blank.Answer, colDanger
					}
					l := material.Caption(th, text)
					l.Color = col
					l.Font.Typeface = "monospace"
					return l.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.choiceGrid(gtx, th, v, q, bi)
		}),
	)
}

// choiceGrid lays the four options out in two columns.
func (u *UI) choiceGrid(gtx layout.Context, th *material.Theme, v *quizView, q quiz.Quiz, bi int) layout.Dimensions {
	blank := q.Blanks[bi]
	key := blankKey{QuizIndex: q.Index, BlankIndex: bi}
	ans, answered := v.answers[key]

	gap := gtx.Dp(dp(8))
	colW := (gtx.Constraints.Max.X - gap) / 2

	var rows []layout.FlexChild
	for i := 0; i < len(blank.Choices); i += 2 {
		i := i
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Max.X = colW
					gtx.Constraints.Min.X = colW
					return u.choiceChip(gtx, th, v, q, bi, i, ans, answered)
				}),
				layout.Rigid(layout.Spacer{Width: dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if i+1 >= len(blank.Choices) {
						return layout.Dimensions{Size: image.Point{X: colW}}
					}
					gtx.Constraints.Max.X = colW
					gtx.Constraints.Min.X = colW
					return u.choiceChip(gtx, th, v, q, bi, i+1, ans, answered)
				}),
			)
		}))
		if i+2 < len(blank.Choices) {
			rows = append(rows, layout.Rigid(layout.Spacer{Height: dp(8)}.Layout))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

func (u *UI) choiceChip(gtx layout.Context, th *material.Theme, v *quizView, q quiz.Quiz, bi, ci int, ans answer, answered bool) layout.Dimensions {
	blank := q.Blanks[bi]
	choice := blank.Choices[ci]
	key := blankKey{QuizIndex: q.Index, BlankIndex: bi}

	click := v.clickable(choiceKey{blankKey: key, Choice: ci})
	if !answered && click.Clicked(gtx) {
		a := answer{Choice: choice, Correct: choice == blank.Answer}
		v.answers[key] = a
		u.store.record(v.proposal.ID, key, a)
		ans, answered = a, true
	}

	bg, fg := colAccentDim, colText
	if answered {
		switch {
		case choice == blank.Answer:
			bg, fg = colSuccess, colOnAccent
		case choice == ans.Choice:
			bg, fg = colDanger, colOnAccent
		default:
			bg, fg = colElevated, colFaint
		}
	}

	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return card(gtx, bg, bg, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(th, choice)
				l.Color = fg
				l.Font.Typeface = "monospace"
				l.MaxLines = 1
				return l.Layout(gtx)
			})
		})
	})
}

// blankMarker labels a blank. Circled digits exist in the system CJK font that
// Gio falls back to on Android, so they render without bundling anything.
func blankMarker(i int) string {
	circled := []string{"①", "②", "③", "④", "⑤", "⑥", "⑦", "⑧", "⑨"}
	if i < len(circled) {
		return circled[i]
	}
	return fmt.Sprintf("(%d)", i+1)
}

// quizBody renders a quiz's blocks as one wrapped paragraph with the masks
// drawn as filled chips.
//
// This is the part SwiftUI gives away for free via AttributedString's
// backgroundColor. Gio has no per-span background, so the text is laid out with
// styledtext, whose span callback reports each span's rectangle *after* it is
// drawn — including once per line fragment when a span wraps. The body is laid
// out twice: once to paint the chip backgrounds from those rectangles, then
// again to draw the glyphs on top, because Gio paints in op order and a
// background recorded later would cover the text.
func (u *UI) quizBody(gtx layout.Context, th *material.Theme, v *quizView, q quiz.Quiz) layout.Dimensions {
	spans, isMask, maskColor := u.buildSpans(th, v, q)
	if len(spans) == 0 {
		return layout.Dimensions{}
	}

	st := styledtext.Text(th.Shaper, spans...)
	// Prose wraps at word boundaries; code blocks have long unbroken runs that
	// would otherwise overflow the card, so they wrap anywhere.
	if q.Kind == quiz.KindCode {
		st.WrapPolicy = styledtext.WrapGraphemes
	}

	dims := st.Layout(gtx, func(gtx layout.Context, idx int, d layout.Dimensions) {
		if idx >= len(isMask) || !isMask[idx] || d.Size.X == 0 {
			return
		}
		pad := gtx.Dp(dp(1))
		r := image.Rect(-pad, -1, d.Size.X+pad, d.Size.Y+1)
		rr := clip.RRect{Rect: r, SE: gtx.Dp(dp(4)), SW: gtx.Dp(dp(4)), NE: gtx.Dp(dp(4)), NW: gtx.Dp(dp(4))}
		defer rr.Push(gtx.Ops).Pop()
		paint.ColorOp{Color: maskColor[idx]}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	})
	st.Layout(gtx, nil)
	return dims
}

// buildSpans converts a quiz's blocks into styled spans, returning parallel
// slices marking which spans are masks and what colour each mask should be.
func (u *UI) buildSpans(th *material.Theme, v *quizView, q quiz.Quiz) ([]styledtext.SpanStyle, []bool, []color.NRGBA) {
	var (
		spans  []styledtext.SpanStyle
		isMask []bool
		cols   []color.NRGBA
	)
	body, mono := unit.Sp(16), unit.Sp(14)
	if q.Kind == quiz.KindCode {
		body = mono
	}

	add := func(s styledtext.SpanStyle, mask bool, c color.NRGBA) {
		spans = append(spans, s)
		isMask = append(isMask, mask)
		cols = append(cols, c)
	}

	for _, b := range q.Blocks {
		switch b.Type {
		case quiz.BlockText:
			add(styledtext.SpanStyle{Content: b.Value, Size: body, Color: colText}, false, color.NRGBA{})
		case quiz.BlockInlineCode:
			add(styledtext.SpanStyle{
				Content: b.Value, Size: mono, Color: colAccent,
				Font: font.Font{Typeface: "monospace"},
			}, false, color.NRGBA{})
		case quiz.BlockCodeBlock:
			add(styledtext.SpanStyle{
				Content: b.Value, Size: mono, Color: colText,
				Font: font.Font{Typeface: "monospace"},
			}, false, color.NRGBA{})
		case quiz.BlockMask:
			if b.BlankIndex == nil || *b.BlankIndex >= len(q.Blanks) {
				continue
			}
			bi := *b.BlankIndex
			key := blankKey{QuizIndex: q.Index, BlankIndex: bi}
			ans, answered := v.answers[key]

			label, bg := blankMarker(bi), colAccent
			if answered {
				label = q.Blanks[bi].Answer
				bg = colSuccess
				if !ans.Correct {
					bg = colDanger
				}
			}
			// The label is padded with spaces so the filled chip does not butt
			// straight up against the surrounding words; the fill covers the
			// padding, which is what gives it breathing room.
			add(styledtext.SpanStyle{
				Content: " " + label + " ", Size: mono, Color: colOnAccent,
				Font: font.Font{Typeface: "monospace", Weight: font.Bold},
			}, true, bg)
		}
	}
	return spans, isMask, cols
}
