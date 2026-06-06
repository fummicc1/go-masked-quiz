package masker

import (
	"strings"
	"testing"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
)

// TC-G-M-01: the same (seed, tag) yields an identical stream.
func TestRNG_Deterministic(t *testing.T) {
	a := NewRNG(42, "x")
	b := NewRNG(42, "x")
	for i := 0; i < 100; i++ {
		if a.Int64() != b.Int64() {
			t.Fatalf("stream diverged at i=%d", i)
		}
	}
}

// TC-G-M-02: different tags give independent streams.
func TestRNG_TagsIndependent(t *testing.T) {
	a := NewRNG(42, "x")
	b := NewRNG(42, "y")
	same := 0
	for i := 0; i < 100; i++ {
		if a.Int64() == b.Int64() {
			same++
		}
	}
	if same > 5 {
		t.Fatalf("streams too correlated: %d/100 equal", same)
	}
}

// TC-G-M-03: there are 25 Go keywords.
func TestGoKeywords_Count(t *testing.T) {
	if got := len(GoKeywords()); got != 25 {
		t.Fatalf("GoKeywords() = %d, want 25", got)
	}
}

// TC-G-M-04: GenerateChoices satisfies its invariants.
func TestGenerateChoices_Invariants(t *testing.T) {
	rng := NewRNG(1, "choices")
	choices := GenerateChoices(rng, "comparable",
		[]string{"comparable", "constraints", "ordered"},
		[]string{"iterator", "yield", "Seq"}, 4)

	if len(choices) != 4 {
		t.Fatalf("len = %d, want 4", len(choices))
	}
	if !contains(choices, "comparable") {
		t.Errorf("choices %v must contain the answer", choices)
	}
	seen := map[string]bool{}
	for _, c := range choices {
		lc := strings.ToLower(c)
		if seen[lc] {
			t.Errorf("duplicate (case-insensitive) choice %q in %v", c, choices)
		}
		seen[lc] = true
	}
}

// TC-G-M-05: choices are deterministic for the same seed/tag and inputs.
func TestGenerateChoices_Deterministic(t *testing.T) {
	mk := func() []string {
		return GenerateChoices(NewRNG(7, "c"), "yield",
			[]string{"yield", "next", "stop"}, []string{"range", "func"}, 4)
	}
	a, b := mk(), mk()
	if strings.Join(a, ",") != strings.Join(b, ",") {
		t.Errorf("not deterministic: %v vs %v", a, b)
	}
}

// TC-G-M-04b: tiny pools still reach the requested count via fallback.
func TestGenerateChoices_FallbackFillsCount(t *testing.T) {
	choices := GenerateChoices(NewRNG(1, "c"), "x", nil, nil, 4)
	if len(choices) != 4 {
		t.Fatalf("len = %d, want 4", len(choices))
	}
	if !contains(choices, "x") {
		t.Errorf("answer missing from %v", choices)
	}
}

// TC-G-M-06 / 07: stopwords and short spans are not maskable.
func TestIsMaskableWord(t *testing.T) {
	cases := map[string]bool{
		"the":        false, // stopword
		"is":         false, // stopword + short
		"go":         false, // 2 chars
		"x":          false, // 1 char
		"comparable": true,
		"Seq":        true,
		"a b":        false, // whitespace
	}
	for w, want := range cases {
		if got := isMaskableWord(w); got != want {
			t.Errorf("isMaskableWord(%q) = %v, want %v", w, got, want)
		}
	}
}

// TC-G-M-08: repeated inline-code text is collected only once.
func TestCollectProseSeeds_Dedup(t *testing.T) {
	p := parser.ParseProposal("x.md", []byte("Use `comparable` and again `comparable` plus `ordered`.\n"))
	seeds := CollectProseSeeds(NewRNG(42, "prose"), p, 10)
	if len(seeds) != 2 {
		t.Fatalf("seeds = %d, want 2 (comparable deduped)", len(seeds))
	}
	// emitted in source order
	if seeds[0].Start > seeds[1].Start {
		t.Error("seeds not in source order")
	}
}

// TC-G-M-09: go/scanner extracts identifiers from a snippet, ignoring keywords.
func TestScanIdents_ExtractsIdentifiers(t *testing.T) {
	idents := scanIdents([]byte("func Hello() { fmt.Println(name) }"))
	names := map[string]bool{}
	for _, id := range idents {
		names[id.name] = true
		// offsets must round-trip
		got := "func Hello() { fmt.Println(name) }"[id.start:id.end]
		if got != id.name {
			t.Errorf("offset mismatch: src[%d:%d]=%q != %q", id.start, id.end, got, id.name)
		}
	}
	for _, want := range []string{"Hello", "fmt", "Println", "name"} {
		if !names[want] {
			t.Errorf("expected identifier %q in %v", want, names)
		}
	}
	if names["func"] {
		t.Error("keyword 'func' must not be an IDENT")
	}
}

// TC-G-M-10 / 11: a not-yet-valid-syntax / broken snippet still lexes (no panic,
// identifiers still extracted) — the property that makes the scanner approach work.
func TestScanIdents_SurvivesInvalidSyntax(t *testing.T) {
	// Incomplete + uses a hypothetical future construct; go/parser would fail.
	idents := scanIdents([]byte("for v := range slices.All(xs {  // missing )\n  use(v)"))
	names := map[string]bool{}
	for _, id := range idents {
		names[id.name] = true
	}
	if !names["slices"] || !names["xs"] || !names["use"] {
		t.Errorf("expected slices/xs/use among identifiers, got %v", names)
	}
}

// TC-G-M-12: Levenshtein distance.
func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"foo", "for", 1},
		{"kitten", "sitting", 3},
		{"same", "same", 0},
		{"", "abc", 3},
	}
	for _, c := range cases {
		if got := levenshtein(c.a, c.b); got != c.want {
			t.Errorf("levenshtein(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TC-G-M-13: rankByEdit orders by ascending distance to the target.
func TestRankByEdit(t *testing.T) {
	got := rankByEdit([]string{"qux", "bar", "for"}, "foo")
	if got[0] != "for" {
		t.Errorf("rankByEdit ranked %v; want 'for' (distance 1) first", got)
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
