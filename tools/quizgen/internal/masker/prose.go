package masker

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
)

// ProseSeed marks one inline-code span chosen to be blanked out. Start and End
// are byte offsets into the proposal's Markdown source.
type ProseSeed struct {
	Start  int
	End    int
	Answer string
}

// stopwords are common words skipped when choosing prose seeds: blanking them
// makes a poor quiz.
var stopwords = map[string]bool{
	"the": true, "is": true, "are": true, "was": true, "were": true,
	"a": true, "an": true, "of": true, "to": true, "in": true, "on": true,
	"and": true, "or": true, "for": true, "as": true, "at": true, "by": true,
	"it": true, "be": true, "if": true, "so": true, "we": true, "do": true,
}

// CollectProseSeeds picks up to maxSeeds inline-code spans worth masking.
// Spans that are too short, stopwords, or whitespace-containing are skipped,
// and repeated text is taken only once. Selection is shuffled deterministically
// (so the cap keeps a stable-but-varied subset), then emitted in source order.
func CollectProseSeeds(rng *RNG, p *parser.Proposal, maxSeeds int) []ProseSeed {
	seenText := map[string]bool{}
	var seeds []ProseSeed
	for _, ic := range p.InlineCodes {
		w := ic.Text
		if !isMaskableWord(w) {
			continue
		}
		lc := strings.ToLower(w)
		if seenText[lc] {
			continue
		}
		seenText[lc] = true
		seeds = append(seeds, ProseSeed{Start: ic.Start, End: ic.End, Answer: w})
	}

	rng.Shuffle(len(seeds), func(i, j int) { seeds[i], seeds[j] = seeds[j], seeds[i] })
	if maxSeeds >= 0 && len(seeds) > maxSeeds {
		seeds = seeds[:maxSeeds]
	}
	sort.Slice(seeds, func(i, j int) bool { return seeds[i].Start < seeds[j].Start })
	return seeds
}

// isMaskableWord reports whether an inline-code span is a good masking target:
// more than two characters, not a stopword, and a single whitespace-free token.
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
	return true
}

// ProposalTokens returns the distinct inline-code texts of a proposal, used as
// the same-proposal distractor pool.
func ProposalTokens(p *parser.Proposal) []string {
	seen := map[string]bool{}
	var out []string
	for _, ic := range p.InlineCodes {
		if ic.Text == "" || seen[strings.ToLower(ic.Text)] {
			continue
		}
		seen[strings.ToLower(ic.Text)] = true
		out = append(out, ic.Text)
	}
	return out
}
