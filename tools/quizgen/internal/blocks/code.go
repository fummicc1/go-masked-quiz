package blocks

import (
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/masker"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/quiz"
)

// BuildCodeBlocks expands a code seed into code_block / mask blocks. A context
// window of contextWidth bytes on each side of the masked identifier is taken
// (no line-boundary snapping — code context reads fine mid-line), producing
// [code_block, mask, code_block] with empty edges dropped.
func BuildCodeBlocks(p *parser.Proposal, seed masker.CodeSeed, contextWidth int) []quiz.Block {
	if seed.BlockIndex < 0 || seed.BlockIndex >= len(p.CodeBlocks) {
		return nil
	}
	body := p.CodeBlocks[seed.BlockIndex].Source
	if seed.Start < 0 || seed.End > len(body) || seed.Start > seed.End {
		return nil
	}
	wStart := clampLow(seed.Start - contextWidth)
	wEnd := clampHigh(seed.End+contextWidth, len(body))

	before := body[wStart:seed.Start]
	after := body[seed.End:wEnd]

	var out []quiz.Block
	if before != "" {
		out = append(out, quiz.Block{Type: quiz.BlockCodeBlock, Value: before})
	}
	out = append(out, quiz.Block{Type: quiz.BlockMask})
	if after != "" {
		out = append(out, quiz.Block{Type: quiz.BlockCodeBlock, Value: after})
	}
	return out
}
