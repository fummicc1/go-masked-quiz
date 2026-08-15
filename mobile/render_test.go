package main

import (
	"image"
	"image/png"
	"os"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/fummicc1/go-masked-quiz/quizgen/quiz"
)

// renderUI lays the UI out into a headless window and returns the frame. This
// exercises the real layout and paint path — including the two-pass chip
// drawing in quizBody, whose bugs are only visible in pixels — without needing
// a device or emulator.
func renderUI(t *testing.T, u *UI, w, h int) *image.RGBA {
	t.Helper()

	win, err := headless.NewWindow(w, h)
	if err != nil {
		t.Skipf("headless window unavailable: %v", err)
	}
	defer win.Release()

	var ops op.Ops
	th := newTheme()
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: 2.75, PxPerSp: 2.75},
		Constraints: layout.Exact(image.Pt(w, h)),
	}
	u.layout(gtx, th)

	if err := win.Frame(&ops); err != nil {
		t.Fatalf("frame: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if err := win.Screenshot(img); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	return img
}

func writePNG(t *testing.T, img *image.RGBA, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	t.Logf("wrote %s", path)
}

// testUI builds a UI over the embedded bundle with persistence disabled.
func testUI(t *testing.T) *UI {
	t.Helper()
	b, err := decodeBundle(embeddedQuizzes)
	if err != nil {
		t.Fatalf("decode embedded bundle: %v", err)
	}
	u := &UI{bundle: b, source: SourceBundle, store: newScoreStore("")}
	u.list.init()
	return u
}

func TestRenderList(t *testing.T) {
	u := testUI(t)
	img := renderUI(t, u, 1080, 2000)
	writePNG(t, img, "testdata/screen-list.png")
}

// TestRenderQuiz renders a proposal that has both a prose quiz and a code quiz,
// with one blank answered correctly and one wrong, so every chip state appears
// in a single frame.
func TestRenderQuiz(t *testing.T) {
	u := testUI(t)

	idx := findProposalWithCode(u.bundle)
	if idx < 0 {
		t.Skip("no proposal with a code quiz in the embedded bundle")
	}
	p := u.bundle.Proposals[idx]

	u.selected = idx
	u.screen = screenQuiz
	u.quizV.open(p, u.store)

	// Answer the first blank correctly and the second one wrong, going through
	// the same path a tap takes so the header's progress reflects it.
	answerNth(u, p, 0, true)
	answerNth(u, p, 1, false)

	img := renderUI(t, u, 1080, 2000)
	writePNG(t, img, "testdata/screen-quiz.png")
}

func findProposalWithCode(b quiz.Bundle) int {
	for i, p := range b.Proposals {
		var hasCode, hasProse bool
		for _, q := range p.Quizzes {
			switch q.Kind {
			case quiz.KindCode:
				hasCode = true
			case quiz.KindProse:
				hasProse = true
			}
		}
		if hasCode && hasProse {
			return i
		}
	}
	return -1
}

// answerNth answers the nth blank of a proposal (in document order), updating
// both the view and the store exactly as choiceChip does on a tap.
func answerNth(u *UI, p quiz.Proposal, n int, correct bool) {
	seen := 0
	for _, q := range p.Quizzes {
		for bi, bl := range q.Blanks {
			if seen != n {
				seen++
				continue
			}
			k := blankKey{QuizIndex: q.Index, BlankIndex: bi}
			a := pick(bl, correct)
			u.quizV.answers[k] = a
			u.store.record(p.ID, k, a)
			return
		}
	}
}

// pick returns an answer for a blank: the right one, or the first choice that
// is not the right one.
func pick(bl quiz.Blank, correct bool) answer {
	if correct {
		return answer{Choice: bl.Answer, Correct: true}
	}
	for _, c := range bl.Choices {
		if c != bl.Answer {
			return answer{Choice: c, Correct: false}
		}
	}
	return answer{Choice: bl.Answer, Correct: true}
}
