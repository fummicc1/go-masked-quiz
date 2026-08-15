package main

import (
	"image/color"

	"gioui.org/widget/material"
)

// Palette mirrors the iOS app's dark theme so both clients look like the same
// product.
var (
	colBg        = color.NRGBA{R: 0x0F, G: 0x14, B: 0x1A, A: 0xFF}
	colSurface   = color.NRGBA{R: 0x18, G: 0x20, B: 0x28, A: 0xFF}
	colElevated  = color.NRGBA{R: 0x1F, G: 0x29, B: 0x33, A: 0xFF}
	colBorder    = color.NRGBA{R: 0x2C, G: 0x38, B: 0x44, A: 0xFF}
	colText      = color.NRGBA{R: 0xE6, G: 0xED, B: 0xF3, A: 0xFF}
	colSecondary = color.NRGBA{R: 0x9F, G: 0xB0, B: 0xC0, A: 0xFF}
	colFaint     = color.NRGBA{R: 0x6B, G: 0x7C, B: 0x8C, A: 0xFF}
	colAccent    = color.NRGBA{R: 0x2E, G: 0xC4, B: 0xD6, A: 0xFF}
	colAccentDim = color.NRGBA{R: 0x1B, G: 0x3A, B: 0x42, A: 0xFF}
	colSuccess   = color.NRGBA{R: 0x3F, G: 0xB9, B: 0x50, A: 0xFF}
	colDanger    = color.NRGBA{R: 0xE5, G: 0x53, B: 0x4B, A: 0xFF}
	colOnAccent  = color.NRGBA{R: 0x0B, G: 0x10, B: 0x14, A: 0xFF}
)

// newTheme returns the app theme.
//
// It deliberately leaves Shaper at its zero value: an empty text.Shaper enables
// system font discovery, which is what finds the CJK font on the device. Passing
// gofont.Collection() here — the obvious-looking thing to do — replaces that
// fallback with a Latin-only collection and silently breaks Japanese text.
func newTheme() *material.Theme {
	th := material.NewTheme()
	th.Palette.Bg = colBg
	th.Palette.Fg = colText
	th.Palette.ContrastBg = colAccent
	th.Palette.ContrastFg = colOnAccent
	return th
}
