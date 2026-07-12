package llm

import (
	"testing"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/quiz"
)

func TestCache_SaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := &Entry{
		IssueNumber:   77,
		BodyHash:      BodyHash("some body"),
		Model:         "qwen2.5-coder",
		PromptVersion: PromptVersion,
		Summary:       "a summary",
		Quizzes:       []quiz.Quiz{{Kind: quiz.KindLLM, GenMethod: "llm"}},
	}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir, 77)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.IssueNumber != 77 || got.Summary != "a summary" || len(got.Quizzes) != 1 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestCache_LoadMissingReturnsNil(t *testing.T) {
	got, err := Load(t.TempDir(), 999)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("want nil for missing entry, got %+v", got)
	}
}

func TestEntry_Fresh(t *testing.T) {
	body := "the body"
	e := &Entry{BodyHash: BodyHash(body), Model: "m", PromptVersion: PromptVersion}
	if !e.Fresh(body, "m") {
		t.Error("want fresh for matching body/model/prompt")
	}
	if e.Fresh("changed body", "m") {
		t.Error("want stale when body changed")
	}
	if e.Fresh(body, "other-model") {
		t.Error("want stale when model changed")
	}
	var nilEntry *Entry
	if nilEntry.Fresh(body, "m") {
		t.Error("nil entry must not be fresh")
	}
}
