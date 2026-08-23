package masker

import (
	"strings"
	"testing"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
)

const doc = "# Title\n\n" +
	"We propose that `range` accept a `func` value, so `range` composes.\n\n" +
	"A paragraph with no inline code survives untouched.\n\n" +
	"```go\nfunc Count(n int) { process(n) }\n```\n"

func maskSample(t *testing.T, maxPerBlock int) MaskedDocument {
	t.Helper()
	d := parser.ParseDocument("x.md", []byte(doc), parser.Options{})
	return MaskDocument(NewRNG(42, "t"), d, maxPerBlock)
}

// Every block of the document survives masking, including the ones that carry
// no blanks — that is the whole point of reading the proposal in place.
func TestMaskDocument_KeepsAllBlocks(t *testing.T) {
	src := parser.ParseDocument("x.md", []byte(doc), parser.Options{})
	md := maskSample(t, 3)
	if len(md.Blocks) != len(src.Blocks) {
		t.Fatalf("blocks = %d, want %d", len(md.Blocks), len(src.Blocks))
	}
	var plain string
	for _, b := range md.Blocks {
		for _, s := range b.Spans {
			plain += s.Text
		}
	}
	if !strings.Contains(plain, "survives untouched") {
		t.Error("prose without blanks was lost")
	}
}

// Repeating a token inside one block yields one blank referenced twice, so the
// answer is not revealed a line below.
func TestMaskDocument_RepeatedTokenSharesOneBlank(t *testing.T) {
	md := maskSample(t, 3)
	counts := map[int]int{}
	for _, b := range md.Blocks {
		for _, s := range b.Spans {
			if s.Kind == "mask" {
				counts[s.BlankIndex]++
			}
		}
	}
	var shared bool
	for bi, n := range counts {
		if n > 1 {
			shared = true
			if md.Blanks[bi].Answer != "range" {
				t.Errorf("shared blank answer = %q, want range", md.Blanks[bi].Answer)
			}
		}
	}
	if !shared {
		t.Error("repeated `range` did not share a blank")
	}
}

func TestMaskDocument_MaskIndicesAreInRange(t *testing.T) {
	md := maskSample(t, 3)
	var masks int
	for _, b := range md.Blocks {
		for _, s := range b.Spans {
			if s.Kind != "mask" {
				continue
			}
			masks++
			if s.BlankIndex < 0 || s.BlankIndex >= len(md.Blanks) {
				t.Fatalf("blank index %d out of range (%d blanks)", s.BlankIndex, len(md.Blanks))
			}
		}
	}
	if masks == 0 {
		t.Fatal("no masks produced")
	}
}

// Code blocks are split around the masked tokens, and reassembling the spans
// with each blank's answer reproduces the original body exactly.
func TestMaskDocument_CodeBlockRoundTrips(t *testing.T) {
	src := parser.ParseDocument("x.md", []byte(doc), parser.Options{})
	md := maskSample(t, 2)

	var want, got string
	for _, b := range src.Blocks {
		if b.Kind == parser.KindCode {
			want = b.Spans[0].Text
		}
	}
	for _, b := range md.Blocks {
		if b.Kind != string(parser.KindCode) {
			continue
		}
		for _, s := range b.Spans {
			if s.Kind == "mask" {
				got += md.Blanks[s.BlankIndex].Answer
			} else {
				got += s.Text
			}
		}
	}
	if got != want {
		t.Errorf("round trip\n got: %q\nwant: %q", got, want)
	}
}

func TestMaskDocument_CapsBlanksPerBlock(t *testing.T) {
	md := maskSample(t, 1)
	for i, b := range md.Blocks {
		seen := map[int]bool{}
		for _, s := range b.Spans {
			if s.Kind == "mask" {
				seen[s.BlankIndex] = true
			}
		}
		if len(seen) > 1 {
			t.Errorf("block %d has %d blanks, want at most 1", i, len(seen))
		}
	}
}

func TestMaskDocument_Deterministic(t *testing.T) {
	a, b := maskSample(t, 2), maskSample(t, 2)
	if len(a.Blanks) != len(b.Blanks) {
		t.Fatalf("blank counts differ: %d vs %d", len(a.Blanks), len(b.Blanks))
	}
	for i := range a.Blanks {
		if a.Blanks[i].Answer != b.Blanks[i].Answer {
			t.Fatalf("blank %d differs: %q vs %q", i, a.Blanks[i].Answer, b.Blanks[i].Answer)
		}
	}
}
