package main

import (
	"image"
	"runtime"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// maxBlockAllocMB is the ceiling one block may allocate to lay out.
//
// A block is a single wrapped run, so its cost scales with its text, not with
// the screen. Proposals sometimes paste an entire file into one fence: the
// worst in the corpus is 38,371 characters, which allocated 309MB before
// truncateSpans and got the app killed on iOS. Truncated it allocates 0.5MB.
const maxBlockAllocMB = 25

func TestBlockLayoutStaysWithinMemoryBudget(t *testing.T) {
	u := testUI(t)
	th := newTheme()

	// The largest block is the only one that can breach the budget, so measure
	// that rather than paying for all 11k of them.
	var worstChars, worstProp, worstIdx int
	for pi, p := range u.bundle.Proposals {
		for bi, b := range p.Document.Blocks {
			n := 0
			for _, s := range b.Spans {
				n += len(s.Value)
			}
			if n > worstChars {
				worstChars, worstProp, worstIdx = n, pi, bi
			}
		}
	}

	p := u.bundle.Proposals[worstProp]
	u.selected = worstProp
	u.screen = screenQuiz
	u.docV.open2(p, u.store)

	layoutOnce := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 2.75, PxPerSp: 2.75},
			Constraints: layout.Exact(image.Pt(1080, 2000)),
		}
		u.docBlock(gtx, th, &u.docV, worstIdx)
	}

	// The first shape in a process initialises the font shaper, which allocates
	// ~84MB whatever the text — enough to swamp what this test is about. The
	// same block reads 83.8MB cold and 0.5MB warm, so measure warm.
	layoutOnce()

	var m0, m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m0)
	layoutOnce()
	runtime.ReadMemStats(&m1)
	// TotalAlloc is cumulative, so a GC landing mid-layout cannot move it. The
	// HeapAlloc reading of this same block swings between 40 and 64MB run to
	// run; TotalAlloc reads the same figure every time.
	usedMB := float64(m1.TotalAlloc-m0.TotalAlloc) / 1048576
	t.Logf("largest block: %s #%d (%d chars) allocated %.1f MB", p.ID, worstIdx, worstChars, usedMB)
	if usedMB > maxBlockAllocMB {
		t.Errorf("block layout allocated %.1f MB, budget is %d MB", usedMB, maxBlockAllocMB)
	}
}
