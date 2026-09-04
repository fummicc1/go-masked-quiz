// Command mobile is the go-masked-quiz client: a quiz about Go, built entirely
// in Go, UI included, via Gio. It reads the published quizzes.json and imports
// the same schema types quizgen writes.
package main

import (
	"context"
	"log"
	"os"
	"sync"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/fummicc1/go-masked-quiz/quizgen/quiz"
)

func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("Go Masked Quiz"))
		if err := run(w); err != nil {
			log.Println(err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}

// screen is which view the single window is currently showing. Two screens is
// few enough that an explicit variable beats a navigation framework.
type screen int

const (
	screenList screen = iota
	screenQuiz
	screenAbout
)

// loadState is how far the one network load has got. Without an embedded
// fallback the app can genuinely end up with nothing to show, so "empty" and
// "failed" have to be distinguishable states rather than both rendering as an
// endless spinner.
type loadState int

const (
	loadLoading loadState = iota
	loadReady
	loadFailed
)

// pending hands a load result from the loader goroutine to the UI goroutine.
// The layout code reads the bundle every frame from the UI goroutine, so the
// result is parked here and applied at a frame boundary instead of being
// written across goroutines.
type pending struct {
	mu     sync.Mutex
	set    bool
	bundle quiz.Bundle
	source Source
	err    error
}

func (p *pending) put(b quiz.Bundle, src Source, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bundle, p.source, p.err, p.set = b, src, err, true
}

func (p *pending) take() (quiz.Bundle, Source, error, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.set {
		return quiz.Bundle{}, "", nil, false
	}
	p.set = false
	return p.bundle, p.source, p.err, true
}

// UI holds everything that must survive across frames. In immediate mode the
// layout code runs every frame, so widget state (clickables, list positions)
// has to live outside it or clicks are never observed.
type UI struct {
	bundle  quiz.Bundle
	source  Source
	state   loadState
	loadErr error
	store   *scoreStore

	win      *app.Window
	pend     pending
	inflight bool
	retry    widget.Clickable

	screen   screen
	selected int // index into bundle.Proposals when screen == screenQuiz

	list  listView
	docV  docView
	about aboutView

	aboutBtn widget.Clickable
}

// load fetches in the background. Taps on retry while a load is still in flight
// are ignored rather than stacking up goroutines racing to publish a result.
func (u *UI) load() {
	if u.inflight {
		return
	}
	u.inflight = true
	u.state = loadLoading
	w := u.win
	go func() {
		b, src, err := loadBundle(context.Background(), cacheFilePath())
		u.pend.put(b, src, err)
		if w != nil {
			w.Invalidate()
		}
	}()
}

// sync applies a finished load. It runs at the top of a frame, on the UI
// goroutine, so everything downstream can read the bundle without locking.
func (u *UI) sync() {
	b, src, err, ok := u.pend.take()
	if !ok {
		return
	}
	u.inflight = false
	if err != nil {
		u.loadErr, u.state = err, loadFailed
		return
	}
	u.bundle, u.source, u.loadErr, u.state = b, src, nil, loadReady
}

func run(w *app.Window) error {
	th := newTheme()

	dataDir, err := app.DataDir()
	if err != nil {
		log.Println("data dir unavailable, running without persistence:", err)
		dataDir = ""
	}

	ui := &UI{store: newScoreStore(dataDir), win: w}
	ui.list.init()
	ui.about.init()
	ui.load()

	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			gtx.Metric.PxPerDp = e.Metric.PxPerDp
			ui.sync()
			ui.layout(gtx, th)
			e.Frame(gtx.Ops)
		}
	}
}

// dp is shorthand for device-independent pixels.
func dp(v float32) unit.Dp { return unit.Dp(v) }
