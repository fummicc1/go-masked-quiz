package masker

import (
	"reflect"
	"testing"
)

// Semantic ranking must beat spelling: a vector close to the target wins even
// when its spelling is far, which is the whole point of the embeddings mode.
func TestRankBySimilarityPrefersMeaningOverSpelling(t *testing.T) {
	vecs := Vectors{
		"gcMark":    {1, 0},
		"collector": {0.9, 0.1}, // semantically close, lexically far
		"gcMars":    {0, 1},     // lexically close (distance 1), semantically far
	}
	got := rankBySimilarity([]string{"gcMars", "collector"}, "gcMark", vecs)
	want := []string{"collector", "gcMars"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rankBySimilarity = %v, want %v", got, want)
	}
}

// Tokens the cache doesn't know keep the old edit-distance order, after the
// ranked ones — an incomplete cache must degrade gracefully, not reorder
// arbitrarily.
func TestRankBySimilarityFallsBackForMissingVectors(t *testing.T) {
	vecs := Vectors{
		"gcMark": {1, 0},
		"far":    {0, 1},
	}
	got := rankBySimilarity([]string{"gcMarq", "far", "totallyDifferent"}, "gcMark", vecs)
	want := []string{"far", "gcMarq", "totallyDifferent"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rankBySimilarity = %v, want %v", got, want)
	}
}

func TestRankBySimilarityWithoutTargetVectorUsesEditDistance(t *testing.T) {
	got := rankBySimilarity([]string{"zzzz", "gcMarq"}, "gcMark", Vectors{"zzzz": {1}})
	want := []string{"gcMarq", "zzzz"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rankBySimilarity = %v, want %v", got, want)
	}
}

func TestCosineDegradesToZeroOnBadInput(t *testing.T) {
	if got := cosine([]float64{1, 0}, []float64{1}); got != 0 {
		t.Errorf("mismatched dims: cosine = %v, want 0", got)
	}
	if got := cosine([]float64{0, 0}, []float64{1, 0}); got != 0 {
		t.Errorf("zero vector: cosine = %v, want 0", got)
	}
}

// A nil Vectors must reproduce GenerateChoices exactly: the golden fixtures
// and the CDN pipeline rely on the default path being unchanged.
func TestGenerateChoicesSemanticNilMatchesDefault(t *testing.T) {
	pool := []string{"gcSweep", "scanblock", "forEachP", "worldsema"}
	cross := []string{"runtime", "netpoll"}
	a := GenerateChoices(NewRNG(42, "t"), "gcMark", pool, cross, nil, 4)
	b := GenerateChoicesSemantic(NewRNG(42, "t"), "gcMark", pool, cross, nil, 4, nil)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("nil-vecs semantic = %v, default = %v", b, a)
	}
}

// With more same-category candidates than slots, the semantic order decides
// who makes the cut, so the closest vectors must be the ones chosen.
func TestGenerateChoicesSemanticPicksClosestVectors(t *testing.T) {
	vecs := Vectors{
		"gcMark": {1, 0, 0},
		"near1":  {0.99, 0.1, 0},
		"near2":  {0.95, 0.2, 0},
		"far1":   {0, 1, 0},
		"far2":   {0, 0, 1},
	}
	pool := []string{"far1", "near1", "far2", "near2"}
	choices := GenerateChoicesSemantic(NewRNG(42, "t"), "gcMark", pool, nil, nil, 3, vecs)

	has := map[string]bool{}
	for _, c := range choices {
		has[c] = true
	}
	if !has["gcMark"] || !has["near1"] || !has["near2"] {
		t.Fatalf("choices = %v, want answer plus the two nearest vectors", choices)
	}
}
