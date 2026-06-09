// Package blocks turns a unit (a prose paragraph or a code block) plus its
// chosen blanks into the pre-parsed []quiz.Block body the JSON ships. The whole
// unit is rendered (no context window); every occurrence of a blank's token
// becomes a mask pointing at that blank. The transform is pure — deterministic.
package blocks

import (
	"sort"
	"strings"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/masker"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/quiz"
)

// BuildProseBlocks renders a paragraph into text / inline_code / mask blocks.
// Inline-code spans that were chosen as blanks become mask blocks (carrying
// their BlankIndex); other inline-code spans stay as inline_code; the gaps are
// text. src is the proposal's full Markdown.
func BuildProseBlocks(src []byte, unit parser.ProseUnit, blanks []masker.Blank) []quiz.Block {
	maskAt := map[int]int{} // span start offset -> blank index
	for bi, bl := range blanks {
		for _, sp := range bl.Occurrences {
			maskAt[sp.Start] = bi
		}
	}

	type seg struct {
		start, end, blankIdx int
		text                 string
		isMask               bool
	}
	var segs []seg
	for _, ic := range unit.InlineCodes {
		if bi, ok := maskAt[ic.Start]; ok {
			segs = append(segs, seg{start: ic.Start, end: ic.End, blankIdx: bi, isMask: true})
		} else {
			segs = append(segs, seg{start: ic.Start, end: ic.End, blankIdx: -1, text: ic.Text})
		}
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].start < segs[j].start })

	var out []quiz.Block
	cur := unit.Start
	for _, s := range segs {
		if s.start > cur {
			out = append(out, quiz.Block{Type: quiz.BlockText, Value: cleanText(string(src[cur:s.start]))})
		}
		if s.isMask {
			bi := s.blankIdx
			out = append(out, quiz.Block{Type: quiz.BlockMask, BlankIndex: &bi})
		} else {
			out = append(out, quiz.Block{Type: quiz.BlockInlineCode, Value: s.text})
		}
		cur = s.end
	}
	if cur < unit.End {
		out = append(out, quiz.Block{Type: quiz.BlockText, Value: cleanText(string(src[cur:unit.End]))})
	}
	return optimize(out)
}

// cleanText strips backticks bordering an inline-code span (the span's offsets
// sit just inside the backticks, so neighbouring text would keep a stray "`").
func cleanText(s string) string {
	return strings.Trim(s, "`")
}

// optimize drops empty non-mask blocks and merges adjacent same-type non-mask
// blocks. Mask blocks are never merged (each carries its own BlankIndex).
func optimize(blocks []quiz.Block) []quiz.Block {
	var out []quiz.Block
	for _, b := range blocks {
		if b.Type != quiz.BlockMask && b.Value == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1].Type == b.Type && b.Type != quiz.BlockMask {
			out[n-1].Value += b.Value
			continue
		}
		out = append(out, b)
	}
	return out
}
