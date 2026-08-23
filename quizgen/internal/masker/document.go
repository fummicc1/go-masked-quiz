package masker

import (
	"sort"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
)

// DocSpan is one rendered run of a masked document: literal text, inline code,
// or a blank carrying the index of its entry in MaskedDocument.Blanks.
type DocSpan struct {
	Kind       string // "text" | "inline_code" | "mask"
	Text       string
	BlankIndex int // valid when Kind == "mask"
}

// DocBlock mirrors parser.DocBlock after masking.
type DocBlock struct {
	Kind  string
	Level int
	Lang  string
	Spans []DocSpan
}

// MaskedDocument is a whole proposal with blanks punched into it: the reader
// sees every block, and some tokens are replaced by answerable masks.
type MaskedDocument struct {
	Blocks []DocBlock
	Blanks []Blank
}

// MaskDocument selects up to maxPerBlock blanks in each block and returns the
// document split into renderable spans.
//
// Blanks are scoped to a block, not to the document: repeating a token inside
// one paragraph is a single blank, so its answer is never sitting in plain
// sight a line below; the same word in a later block is asked again.
func MaskDocument(rng *RNG, doc *parser.Document, maxPerBlock int) MaskedDocument {
	out := MaskedDocument{}
	for _, b := range doc.Blocks {
		mb := DocBlock{Kind: string(b.Kind), Level: b.Level, Lang: b.Lang}
		if b.Kind == parser.KindCode {
			mb.Spans = maskCodeBlock(rng, b, maxPerBlock, &out.Blanks)
		} else {
			mb.Spans = maskProseBlock(rng, b, maxPerBlock, &out.Blanks)
		}
		out.Blocks = append(out.Blocks, mb)
	}
	return out
}

// maskProseBlock turns picked inline-code spans into blanks, leaving the
// surrounding prose untouched.
func maskProseBlock(rng *RNG, b parser.DocBlock, maxBlanks int, blanks *[]Blank) []DocSpan {
	var order []string
	seen := map[string]bool{}
	for _, s := range b.Spans {
		if s.Kind != parser.SpanCode || !isMaskableWord(s.Text) || seen[s.Text] {
			continue
		}
		seen[s.Text] = true
		order = append(order, s.Text)
	}
	slot := allocBlanks(rng, order, maxBlanks, blanks)

	spans := make([]DocSpan, 0, len(b.Spans))
	for _, s := range b.Spans {
		switch {
		case s.Kind != parser.SpanCode:
			spans = append(spans, DocSpan{Kind: "text", Text: s.Text})
		default:
			if bi, ok := slot[s.Text]; ok {
				spans = append(spans, DocSpan{Kind: "mask", BlankIndex: bi})
			} else {
				spans = append(spans, DocSpan{Kind: "inline_code", Text: s.Text})
			}
		}
	}
	return spans
}

// maskCodeBlock scans the body for identifiers and distinctive keywords and
// splits it around each picked occurrence.
func maskCodeBlock(rng *RNG, b parser.DocBlock, maxBlanks int, blanks *[]Blank) []DocSpan {
	if len(b.Spans) == 0 {
		return nil
	}
	body := b.Spans[0].Text

	var order []string
	occ := map[string][]Span{}
	for _, id := range scanIdents([]byte(body)) {
		if len(id.name) <= 1 || id.name == "_" {
			continue
		}
		if _, ok := occ[id.name]; !ok {
			order = append(order, id.name)
		}
		occ[id.name] = append(occ[id.name], Span{Start: id.start, End: id.end})
	}
	slot := allocBlanks(rng, order, maxBlanks, blanks)
	if len(slot) == 0 {
		return []DocSpan{{Kind: "text", Text: body}}
	}

	type cut struct {
		span  Span
		blank int
	}
	var cuts []cut
	for tok, bi := range slot {
		for _, sp := range occ[tok] {
			cuts = append(cuts, cut{span: sp, blank: bi})
		}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].span.Start < cuts[j].span.Start })

	var spans []DocSpan
	pos := 0
	for _, c := range cuts {
		if c.span.Start > pos {
			spans = append(spans, DocSpan{Kind: "text", Text: body[pos:c.span.Start]})
		}
		spans = append(spans, DocSpan{Kind: "mask", BlankIndex: c.blank})
		pos = c.span.End
	}
	if pos < len(body) {
		spans = append(spans, DocSpan{Kind: "text", Text: body[pos:]})
	}
	return spans
}

// allocBlanks caps a block's candidate tokens, appends one document-wide blank
// per survivor, and maps each token to its blank index.
func allocBlanks(rng *RNG, order []string, maxBlanks int, blanks *[]Blank) map[string]int {
	if len(order) == 0 {
		return nil
	}
	pick := order
	if maxBlanks > 0 && len(order) > maxBlanks {
		idx := rng.Sample(len(order), maxBlanks)
		sort.Ints(idx)
		pick = make([]string, 0, maxBlanks)
		for _, i := range idx {
			pick = append(pick, order[i])
		}
	}
	out := make(map[string]int, len(pick))
	for _, tok := range pick {
		out[tok] = len(*blanks)
		*blanks = append(*blanks, Blank{Answer: tok})
	}
	return out
}
