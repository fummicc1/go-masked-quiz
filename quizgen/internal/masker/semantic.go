package masker

import (
	"math"
	"sort"
)

// Vectors maps a token to its embedding. It is loaded from the committed
// embed-generate cache; a nil map means semantic ranking is unavailable and
// callers fall back to edit distance.
type Vectors map[string][]float64

// rankBySimilarity stably orders candidates by descending cosine similarity to
// target. Candidates without a vector cannot be compared semantically, so they
// follow the ranked ones, ordered by edit distance — the ranking the tool used
// before embeddings existed. A missing target vector disables semantic ranking
// for the same reason.
func rankBySimilarity(candidates []string, target string, vecs Vectors) []string {
	tv := vecs[target]
	if tv == nil {
		return rankByEdit(candidates, target)
	}
	var withVec, without []string
	for _, c := range candidates {
		if vecs[c] != nil {
			withVec = append(withVec, c)
		} else {
			without = append(without, c)
		}
	}
	sort.SliceStable(withVec, func(i, j int) bool {
		return cosine(vecs[withVec[i]], tv) > cosine(vecs[withVec[j]], tv)
	})
	return append(withVec, rankByEdit(without, target)...)
}

// cosine returns the cosine similarity of a and b. Mismatched dimensions or a
// zero-magnitude vector yield 0 ("no signal") rather than an error, so a
// malformed cache entry degrades the ranking instead of failing generation.
func cosine(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
