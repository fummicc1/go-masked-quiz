package masker

import (
	"fmt"
	"sort"
	"strings"
)

// goKeywords is the set of 25 Go keywords, used as last-resort distractors.
var goKeywords = []string{
	"break", "case", "chan", "const", "continue",
	"default", "defer", "else", "fallthrough", "for",
	"func", "go", "goto", "if", "import",
	"interface", "map", "package", "range", "return",
	"select", "struct", "switch", "type", "var",
}

// GoKeywords returns a copy of the 25 Go keywords.
func GoKeywords() []string {
	return append([]string(nil), goKeywords...)
}

// GenerateChoices builds exactly count choices for one blank. The result always
// contains answer, has no case-insensitive duplicates, and is shuffled.
//
// exclude lists tokens that must NOT appear as distractors — notably the
// answers of the other blanks in the same quiz, so one blank's options never
// reveal another blank's answer.
//
// Distractors are drawn, in priority order, from: same-proposal tokens ranked
// by edit distance to the answer (most plausible first), a shuffled
// cross-proposal pool, then Go keywords; synthetic tokens guarantee the count.
//
// At every tier, candidates are first restricted to the same category as
// answer (predeclared identifier / keyword / other identifier) — see
// category(). A predeclared type like "int" and an arbitrary sample-code
// variable like "value" are never plausible substitutes for one another, so
// they must never appear as each other's distractor.
func GenerateChoices(rng *RNG, answer string, proposalTokens, crossPoolTokens, exclude []string, count int) []string {
	if count < 1 {
		count = 1
	}
	answerCategory := category(answer)
	sameCategory := func(tokens []string) []string {
		out := make([]string, 0, len(tokens))
		for _, t := range tokens {
			if category(t) == answerCategory {
				out = append(out, t)
			}
		}
		return out
	}

	seen := map[string]bool{strings.ToLower(answer): true}
	for _, e := range exclude {
		if e != "" {
			seen[strings.ToLower(e)] = true
		}
	}
	choices := []string{answer}

	add := func(cands []string) {
		for _, c := range cands {
			if len(choices) >= count {
				return
			}
			lc := strings.ToLower(c)
			if lc == "" || seen[lc] {
				continue
			}
			seen[lc] = true
			choices = append(choices, c)
		}
	}

	add(rankByEdit(dedupeFold(sameCategory(proposalTokens), answer), answer))

	cross := dedupeFold(sameCategory(crossPoolTokens), answer)
	rng.Shuffle(len(cross), func(i, j int) { cross[i], cross[j] = cross[j], cross[i] })
	add(cross)

	kw := sameCategory(GoKeywords())
	rng.Shuffle(len(kw), func(i, j int) { kw[i], kw[j] = kw[j], kw[i] })
	add(kw)

	for i := 0; len(choices) < count; i++ {
		c := fmt.Sprintf("%s_%d", answer, i)
		if lc := strings.ToLower(c); !seen[lc] {
			seen[lc] = true
			choices = append(choices, c)
		}
	}

	rng.Shuffle(len(choices), func(i, j int) { choices[i], choices[j] = choices[j], choices[i] })
	return choices
}

// dedupeFold returns tokens with case-insensitive duplicates removed and any
// token equal to exclude dropped, preserving first-seen order.
func dedupeFold(tokens []string, exclude string) []string {
	seen := map[string]bool{strings.ToLower(exclude): true}
	var out []string
	for _, t := range tokens {
		lc := strings.ToLower(t)
		if t == "" || seen[lc] {
			continue
		}
		seen[lc] = true
		out = append(out, t)
	}
	return out
}

// rankByEdit stably orders candidates by ascending Levenshtein distance to
// target (ties keep input order).
func rankByEdit(candidates []string, target string) []string {
	out := append([]string(nil), candidates...)
	sort.SliceStable(out, func(i, j int) bool {
		return levenshtein(out[i], target) < levenshtein(out[j], target)
	})
	return out
}

// levenshtein returns the edit distance between a and b.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
