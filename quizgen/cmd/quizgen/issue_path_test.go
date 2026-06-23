package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/source"
)

// A realistic issue body flows through the same buildQuizzes path as design docs
// and yields quizzes whose IDs carry the issue slug. This covers the github
// source without touching the network (buildQuizzes is source-agnostic).
func issueProposal(t *testing.T) *parser.Proposal {
	t.Helper()
	body := source.Normalize(
		"This proposal makes `slices.Collect` consume an `iter.Seq`. " +
			"The `range` keyword now accepts a `func` value, so `for x := range seq` works.\n\n" +
			"```\nfunc Collect[T any](seq iter.Seq[T]) []T {\n\tvar out []T\n\tfor v := range seq {\n\t\tout = append(out, v)\n\t}\n\treturn out\n}\n```\n",
	)
	return parser.ParseProposalWithOptions("issue-77273.md", []byte(body), parser.Options{AcceptBareGoFences: true})
}

func TestIssuePath_ProducesQuizzes(t *testing.T) {
	p := issueProposal(t)
	q := buildQuizzes(p, "issue-77273", 42, 5, 3, 4, nil, nil)
	if len(q) == 0 {
		t.Fatal("issue produced no quizzes")
	}
	for _, qz := range q {
		if !strings.HasPrefix(qz.ID, "issue-77273-q") {
			t.Errorf("quiz ID = %q, want issue-77273-q* prefix", qz.ID)
		}
	}
}

func TestIssuePath_Deterministic(t *testing.T) {
	p := issueProposal(t)
	a := buildQuizzes(p, "issue-77273", 42, 5, 3, 4, nil, nil)
	b := buildQuizzes(p, "issue-77273", 42, 5, 3, 4, nil, nil)
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	if string(ab) != string(bb) {
		t.Error("issue path is not deterministic")
	}
}
