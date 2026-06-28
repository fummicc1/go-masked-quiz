package llm

import (
	"strings"
	"testing"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/quiz"
)

const sampleBody = "The range keyword now accepts a func value. iter.Seq is a new type."

func TestBuild_ConvertsValidQuizAndDropsInvalid(t *testing.T) {
	content := `{
	  "summary": "This proposal lets range accept function values.",
	  "quizzes": [
	    {"stem": "The [[BLANK]] keyword now accepts a [[BLANK]] value.",
	     "blanks": [
	       {"answer": "range", "distractors": ["for", "go", "map"]},
	       {"answer": "func",  "distractors": ["chan", "int", "var"]}
	     ]},
	    {"stem": "A [[BLANK]] is invented.", "blanks": [{"answer": "NotInBody", "distractors": ["a","b","c"]}]},
	    {"stem": "no markers here", "blanks": []}
	  ]
	}`
	res, dropped, err := Build(content, sampleBody, "issue-1", 42, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary == "" {
		t.Error("summary lost")
	}
	if len(res.Quizzes) != 1 {
		t.Fatalf("valid quizzes = %d, want 1", len(res.Quizzes))
	}
	if len(dropped) != 2 {
		t.Errorf("dropped = %d (%v), want 2", len(dropped), dropped)
	}

	q := res.Quizzes[0]
	if q.Kind != quiz.KindLLM || q.GenMethod != "llm" {
		t.Errorf("kind=%q gen_method=%q, want llm/llm", q.Kind, q.GenMethod)
	}
	if q.MaskCount() != 2 {
		t.Errorf("mask count = %d, want 2", q.MaskCount())
	}
	if len(q.Blanks) != 2 {
		t.Fatalf("blanks = %d, want 2", len(q.Blanks))
	}
	for _, b := range q.Blanks {
		if len(b.Choices) != 4 {
			t.Errorf("blank %q choices = %d, want 4", b.Answer, len(b.Choices))
		}
		if !contains(b.Choices, b.Answer) {
			t.Errorf("blank %q choices missing answer", b.Answer)
		}
	}
	// one blank's options must not reveal the other's answer
	if contains(q.Blanks[0].Choices, "func") {
		t.Error("blank 0 choices leak blank 1 answer")
	}
}

func TestBuild_DropsMarkerBlankMismatch(t *testing.T) {
	content := `{"summary":"s","quizzes":[{"stem":"one [[BLANK]] two [[BLANK]]","blanks":[{"answer":"range","distractors":["a","b","c"]}]}]}`
	res, dropped, err := Build(content, sampleBody, "issue-1", 42, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Quizzes) != 0 || len(dropped) != 1 {
		t.Errorf("want 0 quizzes / 1 dropped, got %d / %d", len(res.Quizzes), len(dropped))
	}
}

func TestBuild_DropsTooManyBlanks(t *testing.T) {
	content := `{"summary":"s","quizzes":[{"stem":"[[BLANK]] [[BLANK]]","blanks":[{"answer":"range","distractors":["a"]},{"answer":"func","distractors":["b"]}]}]}`
	res, _, err := Build(content, sampleBody, "issue-1", 42, 4, 1) // maxBlanks=1
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Quizzes) != 0 {
		t.Errorf("want 0 quizzes (exceeds maxBlanks), got %d", len(res.Quizzes))
	}
}

func TestBuild_EmptySummaryIsError(t *testing.T) {
	if _, _, err := Build(`{"summary":"  ","quizzes":[]}`, sampleBody, "issue-1", 42, 4, 3); err == nil {
		t.Fatal("want error for empty summary")
	}
}

func TestBuild_InvalidJSONIsError(t *testing.T) {
	if _, _, err := Build(`not json`, sampleBody, "issue-1", 42, 4, 3); err == nil {
		t.Fatal("want error for invalid JSON")
	}
}

func TestBuild_Deterministic(t *testing.T) {
	content := `{"summary":"s","quizzes":[{"stem":"The [[BLANK]] keyword","blanks":[{"answer":"range","distractors":["for","go","map"]}]}]}`
	a, _, _ := Build(content, sampleBody, "issue-1", 42, 4, 3)
	b, _, _ := Build(content, sampleBody, "issue-1", 42, 4, 3)
	if strings.Join(a.Quizzes[0].Blanks[0].Choices, ",") != strings.Join(b.Quizzes[0].Blanks[0].Choices, ",") {
		t.Error("choices are not deterministic")
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
