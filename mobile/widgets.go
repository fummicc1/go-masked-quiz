package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// fillRRect paints a rounded rectangle covering the whole constraint area.
func fillRRect(gtx layout.Context, col color.NRGBA, radius unit.Dp, size image.Point) {
	r := gtx.Dp(radius)
	rect := clip.RRect{Rect: image.Rectangle{Max: size}, SE: r, SW: r, NE: r, NW: r}
	defer rect.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// card draws w on a rounded surface with a hairline border, the repeated
// container shape of both screens.
func card(gtx layout.Context, bg, border color.NRGBA, w layout.Widget) layout.Dimensions {
	macro := recordMacro(gtx, w)
	size := macro.dims.Size
	fillRRect(gtx, bg, dp(14), size)
	strokeRRect(gtx, border, dp(14), size)
	macro.call.Add(gtx.Ops)
	return macro.dims
}

// strokeRRect outlines a rounded rectangle.
func strokeRRect(gtx layout.Context, col color.NRGBA, radius unit.Dp, size image.Point) {
	r := gtx.Dp(radius)
	width := float32(gtx.Dp(1))
	rect := clip.RRect{Rect: image.Rectangle{Max: size}, SE: r, SW: r, NE: r, NW: r}
	defer clip.Stroke{Path: rect.Path(gtx.Ops), Width: width}.Op().Push(gtx.Ops).Pop()
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// progressRail is the thin two-layer bar showing completion.
func progressRail(gtx layout.Context, fraction float32, fill color.NRGBA) layout.Dimensions {
	h := gtx.Dp(4)
	w := gtx.Constraints.Max.X
	fillRRect(gtx, colBorder, dp(2), image.Point{X: w, Y: h})
	if fraction > 0 {
		if fraction > 1 {
			fraction = 1
		}
		fw := int(float32(w) * fraction)
		if fw < h {
			fw = h
		}
		fillRRect(gtx, fill, dp(2), image.Point{X: fw, Y: h})
	}
	return layout.Dimensions{Size: image.Point{X: w, Y: h}}
}

// recorded is a laid-out widget captured for painting after its background.
type recorded struct {
	call op.CallOp
	dims layout.Dimensions
}

// recordMacro lays w out into a macro so the caller can paint a background
// first and replay the content on top. Gio paints in op order, so content
// recorded up front would otherwise be covered by its own background.
func recordMacro(gtx layout.Context, w layout.Widget) recorded {
	macro := op.Record(gtx.Ops)
	dims := w(gtx)
	return recorded{call: macro.Stop(), dims: dims}
}
