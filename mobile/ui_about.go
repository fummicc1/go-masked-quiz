package main

import (
	"fmt"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// aboutView owns the About screen's persistent widget state.
type aboutView struct {
	list widget.List
	back widget.Clickable
}

func (v *aboutView) init() {
	v.list.Axis = layout.Vertical
}

// appRepoURL and appLicenseURL describe this binary's own source, which
// carries no runtime metadata about itself the way quiz.Bundle does for the
// upstream proposals it renders.
const (
	appRepoURL    = "https://github.com/fummicc1/go-masked-quiz"
	appLicenseURL = appRepoURL + "/blob/main/LICENSE"
)

// bsdConditions is the BSD 3-Clause boilerplate shared by this project's own
// LICENSE and every upstream source quizzed on below: every Go-project repo
// uses the same template, differing only in the copyright line above it.
const bsdConditions = `Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.`

func (u *UI) layoutAbout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	v := &u.about
	if v.back.Clicked(gtx) {
		u.screen = screenList
		gtx.Execute(op.InvalidateCmd{})
	}

	sections := u.aboutSections(th)

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.aboutHeader(gtx, th)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &v.list).LayoutWidgets(gtx, sections...)
		}),
	)
}

func (u *UI) aboutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	v := &u.about
	return layout.Inset{Top: dp(12), Bottom: dp(8), Left: dp(16), Right: dp(16)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return v.back.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						l := material.Body1(th, "‹ Back")
						l.Color = colAccent
						return l.Layout(gtx)
					})
				}),
				layout.Rigid(layout.Spacer{Height: dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.H6(th, "About & Licenses")
					l.Color = colText
					return l.Layout(gtx)
				}),
			)
		})
}

// aboutSections builds the About screen's content: this app's own attribution,
// the upstream quiz content's attribution (read from the loaded bundle, so it
// stays correct if the source repo or license ever changes), and the shared
// BSD 3-Clause text both are licensed under.
func (u *UI) aboutSections(th *material.Theme) []layout.Widget {
	return []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return aboutCard(gtx, th, "go-masked-quiz",
				"Copyright © 2026 Fumiya Tanaka. Licensed under the BSD 3-Clause "+
					"License (full text below).\n"+appRepoURL)
		},
		func(gtx layout.Context) layout.Dimensions {
			return aboutCard(gtx, th, "Quiz content", u.quizContentAttribution())
		},
		func(gtx layout.Context) layout.Dimensions {
			return aboutCard(gtx, th, "BSD 3-Clause License",
				"Copyright © The Go Authors. All rights reserved.\n"+
					"Copyright © 2026 Fumiya Tanaka. All rights reserved.\n\n"+bsdConditions)
		},
	}
}

// quizContentAttribution describes where the currently loaded proposals came
// from. It reads quiz.Bundle rather than hardcoding golang/proposal, so a
// future multi-source bundle (Bundle.Sources) is reflected without a code
// change here.
func (u *UI) quizContentAttribution() string {
	b := u.bundle
	if len(b.Sources) == 0 && b.SourceRepo == "" {
		return "Attribution details appear once quiz data has loaded."
	}

	var lines []string
	if len(b.Sources) > 0 {
		for _, s := range b.Sources {
			lines = append(lines, fmt.Sprintf("%s: %s (%s, full text: %s)",
				s.Kind, s.Repo, s.License, s.LicenseURL))
		}
	} else {
		lines = append(lines, fmt.Sprintf("%s (fork: %s)", b.SourceRepo, b.SourceFork))
		lines = append(lines, fmt.Sprintf("%s, full text: %s", b.SourceLicense, b.SourceLicenseURL))
	}
	lines = append(lines, "Copyright © The Go Authors. Used under the license above; see the BSD 3-Clause text below.")
	return strings.Join(lines, "\n")
}

// aboutCard is one titled block of attribution text.
func aboutCard(gtx layout.Context, th *material.Theme, title, body string) layout.Dimensions {
	return layout.Inset{Left: dp(16), Right: dp(16), Bottom: dp(10)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return card(gtx, colSurface, colBorder, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Caption(th, title)
							l.Color = colFaint
							l.Font.Typeface = "monospace"
							return l.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Body2(th, body)
							l.Color = colText
							return l.Layout(gtx)
						}),
					)
				})
			})
		})
}
