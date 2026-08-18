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
// drawing, whose bugs are only visible in pixels — without a device.
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

// TestRenderDocument opens a proposal that mixes prose and code, answers the
// first two blanks (one right, one wrong) so every chip state appears, and
// renders the reader.
func TestRenderDocument(t *testing.T) {
	u := testUI(t)

	idx := findRichProposal(u.bundle)
	if idx < 0 {
		t.Skip("no proposal with both prose and code in the embedded bundle")
	}
	p := u.bundle.Proposals[idx]

	u.selected = idx
	u.screen = screenQuiz
	u.docV.open2(p, u.store)

	answerNth(u, p, 0, true)
	answerNth(u, p, 1, false)

	img := renderUI(t, u, 1080, 2000)
	writePNG(t, img, "testdata/screen-document.png")
}

// TestRenderChoiceSheet renders the reader with a blank's choices open, which
// is the state a tap on a mask produces.
func TestRenderChoiceSheet(t *testing.T) {
	u := testUI(t)

	idx := findRichProposal(u.bundle)
	if idx < 0 {
		t.Skip("no suitable proposal in the embedded bundle")
	}
	p := u.bundle.Proposals[idx]

	u.selected = idx
	u.screen = screenQuiz
	u.docV.open2(p, u.store)
	if len(p.Document.Blanks) == 0 {
		t.Skip("proposal has no blanks")
	}
	u.docV.open = 0

	img := renderUI(t, u, 1080, 2000)
	writePNG(t, img, "testdata/screen-sheet.png")
}

// findRichProposal returns a proposal whose document has both prose and code
// blocks plus at least two blanks, so one frame shows every rendering path.
func findRichProposal(b quiz.Bundle) int {
	for i, p := range b.Proposals {
		var hasCode, hasProse bool
		for _, blk := range p.Document.Blocks {
			switch blk.Kind {
			case quiz.BlockCode:
				hasCode = true
			case quiz.BlockParagraph:
				hasProse = true
			}
		}
		if hasCode && hasProse && len(p.Document.Blanks) >= 2 {
			return i
		}
	}
	return -1
}

// answerNth answers the nth blank of a proposal's document, going through the
// same path a tap takes so the header's progress reflects it.
func answerNth(u *UI, p quiz.Proposal, n int, correct bool) {
	if n >= len(p.Document.Blanks) {
		return
	}
	bl := p.Document.Blanks[n]
	a := pick(bl, correct)
	u.docV.answers[n] = a
	u.store.record(p.ID, blankKey{BlankIndex: n}, a)
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
