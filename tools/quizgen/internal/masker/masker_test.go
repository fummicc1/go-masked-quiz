package masker

import (
	"strings"
	"testing"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
)

func TestRNG_Deterministic(t *testing.T) {
	a, b := NewRNG(42, "x"), NewRNG(42, "x")
	for i := 0; i < 100; i++ {
		if a.Int64() != b.Int64() {
			t.Fatalf("diverged at %d", i)
		}
	}
}

func TestRNG_TagsIndependent(t *testing.T) {
	a, b := NewRNG(42, "x"), NewRNG(42, "y")
	same := 0
	for i := 0; i < 100; i++ {
		if a.Int64() == b.Int64() {
			same++
		}
	}
	if same > 5 {
		t.Fatalf("too correlated: %d/100", same)
	}
}

func TestGoKeywords_Count(t *testing.T) {
	if got := len(GoKeywords()); got != 25 {
		t.Fatalf("GoKeywords = %d, want 25", got)
	}
}

func TestGenerateChoices_Invariants(t *testing.T) {
	ch := GenerateChoices(NewRNG(1, "c"), "comparable",
		[]string{"comparable", "constraints", "ordered"},
		[]string{"iterator", "yield"}, nil, 4)
	if len(ch) != 4 {
		t.Fatalf("len = %d, want 4", len(ch))
	}
	if !contains(ch, "comparable") {
		t.Errorf("missing answer: %v", ch)
	}
	seen := map[string]bool{}
	for _, c := range ch {
		if seen[strings.ToLower(c)] {
			t.Errorf("dup %q in %v", c, ch)
		}
		seen[strings.ToLower(c)] = true
	}
}

// Other blanks' answers must not appear as distractors.
func TestGenerateChoices_ExcludesOtherAnswers(t *testing.T) {
	ch := GenerateChoices(NewRNG(1, "c"), "range",
		[]string{"func", "yield", "chan"}, nil, []string{"func", "yield"}, 4)
	for _, c := range ch {
		if c == "func" || c == "yield" {
			t.Errorf("excluded answer %q leaked into %v", c, ch)
		}
	}
	if !contains(ch, "range") {
		t.Errorf("answer missing: %v", ch)
	}
}

func TestGenerateChoices_Deterministic(t *testing.T) {
	mk := func() []string {
		return GenerateChoices(NewRNG(7, "c"), "yield", []string{"next", "stop"}, []string{"range"}, nil, 4)
	}
	if strings.Join(mk(), ",") != strings.Join(mk(), ",") {
		t.Error("not deterministic")
	}
}

func TestIsMaskableWord(t *testing.T) {
	cases := map[string]bool{
		"the": false, "is": false, "go": false, "x": false,
		"comparable": true, "Seq": true, "a b": false,
	}
	for w, want := range cases {
		if got := isMaskableWord(w); got != want {
			t.Errorf("isMaskableWord(%q) = %v, want %v", w, got, want)
		}
	}
}

// A token occurring multiple times in a paragraph forms ONE blank with all
// occurrences; distinct tokens form distinct blanks.
func TestSelectProseBlanks_GroupsAllOccurrences(t *testing.T) {
	p := parser.ParseProposal("x.md", []byte("use `range` then `range` and `func`\n"))
	blanks := SelectProseBlanks(NewRNG(42, "t"), p.ProseUnits[0], 3)

	byAns := map[string][]Span{}
	for _, b := range blanks {
		byAns[b.Answer] = b.Occurrences
	}
	if len(byAns["range"]) != 2 {
		t.Errorf("range occurrences = %d, want 2", len(byAns["range"]))
	}
	if len(byAns["func"]) != 1 {
		t.Errorf("func occurrences = %d, want 1", len(byAns["func"]))
	}
}

func TestSelectProseBlanks_CapsAtMax(t *testing.T) {
	p := parser.ParseProposal("x.md", []byte("`alpha` `bravo` `charlie` `delta`\n"))
	blanks := SelectProseBlanks(NewRNG(1, "t"), p.ProseUnits[0], 2)
	if len(blanks) != 2 {
		t.Fatalf("blanks = %d, want 2 (capped)", len(blanks))
	}
	// emitted in first-occurrence order
	if blanks[0].Occurrences[0].Start > blanks[1].Occurrences[0].Start {
		t.Error("blanks not ordered by first occurrence")
	}
}

// go/scanner-based code blanks: identifiers grouped, keywords excluded.
func TestSelectCodeBlanks(t *testing.T) {
	p := parser.ParseProposal("x.md", []byte("```go\nfunc Hello() { Hello(); fmt.Println(x) }\n```\n"))
	blanks := SelectCodeBlanks(NewRNG(42, "t"), p.CodeBlocks[0], 5)

	byAns := map[string][]Span{}
	for _, b := range blanks {
		byAns[b.Answer] = b.Occurrences
	}
	if len(byAns["Hello"]) != 2 {
		t.Errorf("Hello occurrences = %d, want 2", len(byAns["Hello"]))
	}
	if _, ok := byAns["func"]; ok {
		t.Error("keyword 'func' must not be a blank")
	}
}

// Incomplete / future-syntax snippets still yield blanks (no panic).
func TestSelectCodeBlanks_SurvivesInvalidSyntax(t *testing.T) {
	p := parser.ParseProposal("x.md", []byte("```go\nfor v := range slices.All(xs {  // missing )\n  use(v)\n```\n"))
	blanks := SelectCodeBlanks(NewRNG(1, "t"), p.CodeBlocks[0], 5)
	names := map[string]bool{}
	for _, b := range blanks {
		names[b.Answer] = true
	}
	if !names["slices"] || !names["use"] {
		t.Errorf("expected slices/use among blanks, got %v", names)
	}
}

func TestLevenshtein(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{{"foo", "for", 1}, {"kitten", "sitting", 3}, {"same", "same", 0}, {"", "abc", 3}} {
		if got := levenshtein(c.a, c.b); got != c.want {
			t.Errorf("levenshtein(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestRankByEdit(t *testing.T) {
	got := rankByEdit([]string{"qux", "bar", "for"}, "foo")
	if got[0] != "for" {
		t.Errorf("ranked %v; want 'for' first", got)
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
