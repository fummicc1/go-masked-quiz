package masker

import (
	"go/scanner"
	"go/token"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
)

// ident is one maskable token occurrence found by the scanner: either a
// token.IDENT, or a non-boilerplate keyword (see boilerplateKeywords).
type ident struct {
	name  string
	start int // block-local byte offset
	end   int
}

// SelectCodeBlanks scans a code block for maskable tokens, groups them by
// name, and picks up to maxBlanks deterministically. This is the heart of the
// tool: go/scanner tokenises lexically, so tokens are found even in
// incomplete or not-yet-valid-syntax snippets where go/parser would fail.
// Literals and operators are their own token kinds and so are excluded for
// free; boilerplate keywords (package/import/func/var/if/else/return/for)
// are excluded deliberately since they carry no proposal-specific meaning.
// The remaining keywords (range, select, go, defer, chan, ...) ARE maskable
// alongside identifiers — a proposal's new syntax is often exactly one of
// these, so masking only identifiers would skip the concept the proposal is
// actually about.
func SelectCodeBlanks(rng *RNG, block parser.CodeBlock, maxBlanks int) []Blank {
	var order []string
	occ := map[string][]Span{}
	for _, id := range scanIdents([]byte(block.Source)) {
		if len(id.name) <= 1 || id.name == "_" {
			continue // single-char and blank identifiers make weak quizzes
		}
		if _, ok := occ[id.name]; !ok {
			order = append(order, id.name)
		}
		occ[id.name] = append(occ[id.name], Span{id.start, id.end})
	}
	return buildBlanks(rng, order, occ, maxBlanks)
}

// scanIdents returns every maskable token (identifier or non-boilerplate
// keyword) in src with its block-local byte range. A nil error handler lets
// lexing continue past malformed input — exactly what lets the tool survive
// proposal snippets.
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
		maskable := tok == token.IDENT || (tok.IsKeyword() && !boilerplateKeywords[lit])
		if maskable {
			off := fset.Position(pos).Offset
			out = append(out, ident{name: lit, start: off, end: off + len(lit)})
		}
	}
	return out
}

// CodeTokens returns the distinct identifiers across a proposal's code blocks
// (the same-proposal distractor pool for code quizzes).
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
