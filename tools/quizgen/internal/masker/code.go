package masker

import (
	"go/scanner"
	"go/token"
	"sort"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
)

// CodeSeed marks one identifier inside a code block chosen to be blanked out.
// Start and End are byte offsets into that block's Source (block-local).
type CodeSeed struct {
	BlockIndex int
	Start      int
	End        int
	Answer     string
}

// ident is one token.IDENT occurrence found by the scanner.
type ident struct {
	name  string
	start int // block-local byte offset
	end   int
}

// CollectCodeSeeds scans every go code block for identifiers and picks up to
// maxSeeds of them. This is the heart of the talk: go/scanner tokenises code
// lexically, so it yields token.IDENT even for the incomplete or
// not-yet-valid-syntax snippets common in proposals, where go/parser would
// fail outright. Keywords, literals, and operators are returned as their own
// token kinds and so are excluded for free.
func CollectCodeSeeds(rng *RNG, p *parser.Proposal, maxSeeds int) []CodeSeed {
	var seeds []CodeSeed
	for bi, cb := range p.CodeBlocks {
		seenInBlock := map[string]bool{}
		for _, id := range scanIdents([]byte(cb.Source)) {
			if len(id.name) <= 1 || id.name == "_" {
				continue // single-char and blank identifiers make weak quizzes
			}
			if seenInBlock[id.name] {
				continue // mask a given identifier at most once per block
			}
			seenInBlock[id.name] = true
			seeds = append(seeds, CodeSeed{
				BlockIndex: bi,
				Start:      id.start,
				End:        id.end,
				Answer:     id.name,
			})
		}
	}

	rng.Shuffle(len(seeds), func(i, j int) { seeds[i], seeds[j] = seeds[j], seeds[i] })
	if maxSeeds >= 0 && len(seeds) > maxSeeds {
		seeds = seeds[:maxSeeds]
	}
	sort.Slice(seeds, func(i, j int) bool {
		if seeds[i].BlockIndex != seeds[j].BlockIndex {
			return seeds[i].BlockIndex < seeds[j].BlockIndex
		}
		return seeds[i].Start < seeds[j].Start
	})
	return seeds
}

// scanIdents returns every token.IDENT in src with its block-local byte range.
// A nil error handler is passed so lexing continues past malformed input
// instead of aborting — exactly what lets the tool survive proposal snippets.
func scanIdents(src []byte) []ident {
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	s.Init(file, src, nil, 0)

	var out []ident
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.IDENT {
			off := fset.Position(pos).Offset
			out = append(out, ident{name: lit, start: off, end: off + len(lit)})
		}
	}
	return out
}

// CodeTokens returns the distinct identifiers across all of a proposal's code
// blocks, used as the same-proposal distractor pool for code quizzes.
func CodeTokens(p *parser.Proposal) []string {
	seen := map[string]bool{}
	var out []string
	for _, cb := range p.CodeBlocks {
		for _, id := range scanIdents([]byte(cb.Source)) {
			if len(id.name) <= 1 || id.name == "_" || seen[id.name] {
				continue
			}
			seen[id.name] = true
			out = append(out, id.name)
		}
	}
	return out
}
