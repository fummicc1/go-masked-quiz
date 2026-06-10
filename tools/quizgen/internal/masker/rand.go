// Package masker selects which tokens to blank out (the "seeds") and builds the
// multiple-choice distractors for each quiz. All randomness flows through RNG
// so that a given (seed, tag) pair always produces the same output — the
// determinism the golden tests and the CDN pipeline rely on.
package masker

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand/v2"
)

// RNG is a deterministic random source seeded from an int64 seed and a string
// tag. Different tags derived from the same seed yield independent streams, so
// seed selection and choice shuffling don't interfere with each other.
type RNG struct {
	r *rand.Rand
}

// NewRNG returns an RNG whose stream is fully determined by (seed, tag).
func NewRNG(seed int64, tag string) *RNG {
	return &RNG{r: rand.New(rand.NewChaCha8(deriveKey(seed, tag)))}
}

// deriveKey turns (seed, tag) into a 32-byte ChaCha8 key.
func deriveKey(seed int64, tag string) [32]byte {
	buf := binary.LittleEndian.AppendUint64(nil, uint64(seed))
	buf = append(buf, tag...)
	return sha256.Sum256(buf)
}

// Int64 returns a non-negative pseudo-random int64.
func (g *RNG) Int64() int64 { return int64(g.r.Uint64() >> 1) }

// Intn returns a pseudo-random int in [0, n). It panics if n <= 0.
func (g *RNG) Intn(n int) int { return g.r.IntN(n) }

// Shuffle permutes n elements using swap, via Fisher-Yates.
func (g *RNG) Shuffle(n int, swap func(i, j int)) { g.r.Shuffle(n, swap) }

// Sample returns k distinct indices in [0, n), in shuffled order. If k >= n it
// returns a full permutation of [0, n).
func (g *RNG) Sample(n, k int) []int {
	if k > n {
		k = n
	}
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	g.Shuffle(n, func(i, j int) { idx[i], idx[j] = idx[j], idx[i] })
	return idx[:k]
}
