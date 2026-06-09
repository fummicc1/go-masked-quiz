package blocks

import (
	"sort"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/masker"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/quiz"
)

// BuildCodeBlocks renders a whole code block into code_block / mask blocks.
// Every occurrence of each blank's identifier becomes a mask (carrying its
// BlankIndex); the gaps stay as code_block.
func BuildCodeBlocks(block parser.CodeBlock, blanks []masker.Blank) []quiz.Block {
	src := block.Source

	type mspan struct{ start, end, blankIdx int }
	var spans []mspan
	for bi, bl := range blanks {
		for _, sp := range bl.Occurrences {
			spans = append(spans, mspan{sp.Start, sp.End, bi})
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })

	var out []quiz.Block
	cur := 0
	for _, s := range spans {
		if s.start > cur {
			out = append(out, quiz.Block{Type: quiz.BlockCodeBlock, Value: src[cur:s.start]})
		}
		bi := s.blankIdx
		out = append(out, quiz.Block{Type: quiz.BlockMask, BlankIndex: &bi})
		cur = s.end
	}
	if cur < len(src) {
		out = append(out, quiz.Block{Type: quiz.BlockCodeBlock, Value: src[cur:]})
	}
	return optimize(out)
}
