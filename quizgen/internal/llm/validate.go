package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/masker"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/quiz"
)

// rawOutput is the JSON shape the model is asked to return.
type rawOutput struct {
	Summary string    `json:"summary"`
	Quizzes []rawQuiz `json:"quizzes"`
}

type rawQuiz struct {
	Stem   string     `json:"stem"`
	Blanks []rawBlank `json:"blanks"`
}

type rawBlank struct {
	Answer      string   `json:"answer"`
	Distractors []string `json:"distractors"`
}

// Result is the validated content for one proposal. Quizzes carry Kind=llm and
// GenMethod=llm; their ID and Index are assigned later, when merged after the
// proposal's mechanical quizzes.
type Result struct {
	Summary string
	Quizzes []quiz.Quiz
}

// Build parses model output and converts the valid quizzes, dropping invalid
// ones (returning their reasons). The body is the proposal text the answers must
// appear in (anti-hallucination). idPrefix/seed make the choices deterministic;
// nChoices/maxBlanks mirror the mechanical pipeline.
func Build(content, body, idPrefix string, seed int64, nChoices, maxBlanks int) (Result, []string, error) {
	var raw rawOutput
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return Result{}, nil, fmt.Errorf("parse model JSON: %w", err)
	}
	res := Result{Summary: strings.TrimSpace(raw.Summary)}
	if res.Summary == "" {
		return Result{}, nil, fmt.Errorf("empty summary")
	}
	lowerBody := strings.ToLower(body)
	var dropped []string
	for i, rq := range raw.Quizzes {
		q, reason := buildQuiz(rq, lowerBody, idPrefix, seed, i, nChoices, maxBlanks)
		if reason != "" {
			dropped = append(dropped, fmt.Sprintf("quiz %d: %s", i, reason))
			continue
		}
		res.Quizzes = append(res.Quizzes, q)
	}
	return res, dropped, nil
}

// buildQuiz validates one raw quiz and converts it to a quiz.Quiz, or returns a
// non-empty reason if it must be dropped.
func buildQuiz(rq rawQuiz, lowerBody, idPrefix string, seed int64, ordinal, nChoices, maxBlanks int) (quiz.Quiz, string) {
	stem := strings.TrimSpace(rq.Stem)
	if stem == "" {
		return quiz.Quiz{}, "empty stem"
	}
	markers := strings.Count(stem, BlankMarker)
	switch {
	case markers == 0:
		return quiz.Quiz{}, "no blank markers"
	case markers != len(rq.Blanks):
		return quiz.Quiz{}, fmt.Sprintf("%d markers but %d blanks", markers, len(rq.Blanks))
	case markers > maxBlanks:
		return quiz.Quiz{}, fmt.Sprintf("too many blanks (%d > %d)", markers, maxBlanks)
	}

	answers := make([]string, len(rq.Blanks))
	for i, b := range rq.Blanks {
		a := strings.TrimSpace(b.Answer)
		if a == "" {
			return quiz.Quiz{}, fmt.Sprintf("blank %d: empty answer", i)
		}
		if !strings.Contains(lowerBody, strings.ToLower(a)) {
			return quiz.Quiz{}, fmt.Sprintf("blank %d: answer %q not found in proposal", i, a)
		}
		answers[i] = a
	}

	// Split the stem on the marker into text/mask blocks, in order.
	segments := strings.Split(stem, BlankMarker) // len == markers+1
	var blocks []quiz.Block
	for i, seg := range segments {
		if seg != "" {
			blocks = append(blocks, quiz.Block{Type: quiz.BlockText, Value: seg})
		}
		if i < len(answers) {
			bi := i
			blocks = append(blocks, quiz.Block{Type: quiz.BlockMask, BlankIndex: &bi})
		}
	}

	// Build choices, excluding the other blanks' answers so options never reveal
	// another blank. Reuse the mechanical choice generator (distractors as the
	// ranked pool; it guarantees exactly nChoices including the answer).
	blanks := make([]quiz.Blank, len(answers))
	for i, a := range answers {
		exclude := make([]string, 0, len(answers)-1)
		for j, other := range answers {
			if j != i {
				exclude = append(exclude, other)
			}
		}
		tag := fmt.Sprintf("llm-choice:%s:%d:%s", idPrefix, ordinal, a)
		rng := masker.NewRNG(seed, tag)
		blanks[i] = quiz.Blank{
			Answer:  a,
			Choices: masker.GenerateChoices(rng, a, rq.Blanks[i].Distractors, nil, exclude, nChoices),
		}
	}

	return quiz.Quiz{
		Kind:      quiz.KindLLM,
		GenMethod: "llm",
		Blocks:    blocks,
		Blanks:    blanks,
	}, ""
}
