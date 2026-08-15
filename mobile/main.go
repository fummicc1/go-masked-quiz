// Command mobile is the go-masked-quiz client: a Go quiz built entirely in Go,
// UI included, via Gio. It reads the same quizzes.json the iOS app reads and
// imports the same schema types quizgen writes.
package main

import (
	"context"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"
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
)

// UI holds everything that must survive across frames. In immediate mode the
// layout code runs every frame, so widget state (clickables, list positions)
// has to live outside it or clicks are never observed.
type UI struct {
	bundle quiz.Bundle
	source Source
	store  *scoreStore

	screen   screen
	selected int // index into bundle.Proposals when screen == screenQuiz

	list  listView
	quizV quizView
}

func run(w *app.Window) error {
	th := newTheme()

	dataDir, err := app.DataDir()
	if err != nil {
		log.Println("data dir unavailable, running without persistence:", err)
		dataDir = ""
	}

	ui := &UI{store: newScoreStore(dataDir)}
	ui.list.init()

	// Load off the UI goroutine; the first frames render an empty list and the
	// window is invalidated once data arrives.
	go func() {
		b, src := loadBundle(context.Background(), cacheFilePath())
		ui.bundle, ui.source = b, src
		w.Invalidate()
	}()

	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			gtx.Metric.PxPerDp = e.Metric.PxPerDp
			ui.layout(gtx, th)
			e.Frame(gtx.Ops)
		}
	}
}

// dp is shorthand for device-independent pixels.
func dp(v float32) unit.Dp { return unit.Dp(v) }
