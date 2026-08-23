package main

import (
	"image"
	"runtime"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// maxBlockHeapMB is the ceiling one block may cost to lay out.
//
// A block is a single wrapped run, so its cost scales with its text, not with
// the screen. Proposals sometimes paste an entire file into one fence: the
// worst in the corpus is ~38k characters, which cost ~90MB before
// truncateSpans and got the app killed on iOS. This test fails if that
// regresses.
const maxBlockHeapMB = 25

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

	var m0, m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m0)

	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: 2.75, PxPerSp: 2.75},
		Constraints: layout.Exact(image.Pt(1080, 2000)),
	}
	u.docBlock(gtx, th, &u.docV, worstIdx)

	runtime.ReadMemStats(&m1)
	usedMB := float64(m1.HeapAlloc-m0.HeapAlloc) / 1048576
	t.Logf("largest block: %s #%d (%d chars) cost %.1f MB", p.ID, worstIdx, worstChars, usedMB)
	if usedMB > maxBlockHeapMB {
		t.Errorf("block layout used %.1f MB, budget is %d MB", usedMB, maxBlockHeapMB)
	}
}
