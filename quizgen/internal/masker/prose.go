package masker

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
)

// Span is a byte range of one mask occurrence within its unit.
type Span struct {
	Start int
	End   int
}

// Blank is one fill-in target chosen from a unit: the answer token, every place
// it occurs in the unit (so all are masked together), and its choices. Choices
// is filled in later by the caller (GenerateChoices).
type Blank struct {
	Answer      string
	Occurrences []Span
	Choices     []string
}

// stopwords are common words skipped when choosing prose blanks.
var stopwords = map[string]bool{
	"the": true, "is": true, "are": true, "was": true, "were": true,
	"a": true, "an": true, "of": true, "to": true, "in": true, "on": true,
	"and": true, "or": true, "for": true, "as": true, "at": true, "by": true,
	"it": true, "be": true, "if": true, "so": true, "we": true, "do": true,
}

// SelectProseBlanks groups a paragraph's maskable inline-code spans by token,
// picks up to maxBlanks of them deterministically, and records every occurrence
// of each picked token (so repeats mask together and never leak the answer).
func SelectProseBlanks(rng *RNG, unit parser.ProseUnit, maxBlanks int) []Blank {
	var order []string
	occ := map[string][]Span{}
	for _, ic := range unit.InlineCodes {
		if !isMaskableWord(ic.Text) {
			continue
		}
		if _, ok := occ[ic.Text]; !ok {
			order = append(order, ic.Text)
		}
		occ[ic.Text] = append(occ[ic.Text], Span{ic.Start, ic.End})
	}
	return buildBlanks(rng, order, occ, maxBlanks)
}

// buildBlanks picks up to maxBlanks tokens (deterministically when capping) and
// returns blanks ordered by first occurrence.
func buildBlanks(rng *RNG, order []string, occ map[string][]Span, maxBlanks int) []Blank {
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
	blanks := make([]Blank, 0, len(pick))
	for _, w := range pick {
		spans := append([]Span(nil), occ[w]...)
		sort.Slice(spans, func(i, j int) bool { return spans[i].Start < spans[j].Start })
		blanks = append(blanks, Blank{Answer: w, Occurrences: spans})
	}
	sort.Slice(blanks, func(i, j int) bool {
		return blanks[i].Occurrences[0].Start < blanks[j].Occurrences[0].Start
	})
	return blanks
}

// isMaskableWord reports whether an inline-code span is a good masking
// target: more than two characters, not a stopword, a single whitespace-free
// token, and not a boilerplate Go keyword. The boilerplate-keyword exclusion
// mirrors code.go's scanIdents, so prose and code apply the same standard:
// package/import/func/var/if/else/return/for carry no proposal-specific
// meaning in either place, while a distinctive keyword like "range" is a
// fine masking target — it may well be the concept the proposal is about.
func isMaskableWord(w string) bool {
	if utf8.RuneCountInString(w) <= 2 {
		return false
	}
	if strings.ContainsAny(w, " \t\n\r") {
		return false
	}
	if stopwords[strings.ToLower(w)] {
		return false
	}
	if boilerplateKeywords[w] {
		return false
	}
	return true
}

// ProposalTokens returns the distinct inline-code texts across a proposal's
// prose units (the same-proposal distractor pool for prose quizzes).
func ProposalTokens(p *parser.Proposal) []string {
	seen := map[string]bool{}
	var out []string
	for _, u := range p.ProseUnits {
		for _, ic := range u.InlineCodes {
			lc := strings.ToLower(ic.Text)
			if ic.Text == "" || seen[lc] {
				continue
			}
			seen[lc] = true
			out = append(out, ic.Text)
		}
	}
	return out
}
