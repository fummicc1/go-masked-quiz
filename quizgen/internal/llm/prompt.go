package llm

import "fmt"

// PromptVersion is part of every cache key: bump it whenever the prompts or the
// expected output shape change, so stale cached generations are regenerated.
const PromptVersion = "v1"

// BlankMarker is the literal token the model must place in a stem for each blank.
const BlankMarker = "[[BLANK]]"

const systemPrompt = `You generate educational fill-in-the-blank quizzes about Go language design proposals, for developers learning Go.

Return ONLY a JSON object (no surrounding prose) of exactly this shape:
{
  "summary": "2-4 sentence plain-English summary of what the proposal changes and why",
  "quizzes": [
    {
      "stem": "one self-contained sentence teaching a key concept, with each blank written literally as [[BLANK]]",
      "blanks": [
        {"answer": "the exact term or identifier from the proposal that fills this blank", "distractors": ["plausible wrong option", "another", "another"]}
      ]
    }
  ]
}

Rules:
- Every "answer" MUST appear verbatim somewhere in the proposal text provided. Never invent terms.
- The count of [[BLANK]] markers in "stem" MUST equal the number of "blanks", in the same order.
- Mask the most important term(s); use 1 blank per stem, at most %d.
- Provide exactly 3 distractors per blank: plausible but wrong, the same kind of thing as the answer.
- Stems must be self-contained. Do not write "the proposal", "above", or "this document".`

// SystemPrompt returns the system prompt for the given max blanks per quiz.
func SystemPrompt(maxBlanks int) string {
	return fmt.Sprintf(systemPrompt, maxBlanks)
}

// UserPrompt builds the per-proposal user message. The body is clipped so it
// fits a local model's context window.
func UserPrompt(title, body string, nQuizzes int) string {
	return fmt.Sprintf("Proposal title: %s\n\nProposal text:\n%s\n\nGenerate a summary and up to %d quizzes.",
		title, clip(body, 8000), nQuizzes)
}

func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
