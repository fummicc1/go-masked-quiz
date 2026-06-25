package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/quiz"
)

// Entry is one validated generation for an issue, cached on disk and committed
// to the repo so CI can merge it without ever calling a model.
type Entry struct {
	IssueNumber   int         `json:"issue_number"`
	BodyHash      string      `json:"body_hash"`
	Model         string      `json:"model"`
	PromptVersion string      `json:"prompt_version"`
	Summary       string      `json:"summary"`
	Quizzes       []quiz.Quiz `json:"quizzes"`
}

// BodyHash hashes the normalized proposal body; a mismatch means the issue
// changed since its quizzes were generated (the cache is then stale).
func BodyHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func cachePath(dir string, issueNumber int) string {
	return filepath.Join(dir, fmt.Sprintf("issue-%d.json", issueNumber))
}

// Load reads the cache entry for an issue, returning (nil, nil) if absent.
func Load(dir string, issueNumber int) (*Entry, error) {
	data, err := os.ReadFile(cachePath(dir, issueNumber))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parse cache %s: %w", cachePath(dir, issueNumber), err)
	}
	return &e, nil
}

// Save writes a cache entry, pretty-printed for reviewable diffs.
func Save(dir string, e *Entry) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(dir, e.IssueNumber), append(data, '\n'), 0o644)
}

// Fresh reports whether the entry matches the current body, model, and prompt
// version, i.e. it does not need regenerating. Used by `llm-generate`.
func (e *Entry) Fresh(body, model string) bool {
	return e.MatchesBody(body) && e.Model == model
}

// MatchesBody reports whether the entry was generated from the current body
// under the current prompt version (regardless of which model produced it).
// Used by the merge step, which is model-agnostic.
func (e *Entry) MatchesBody(body string) bool {
	return e != nil &&
		e.BodyHash == BodyHash(body) &&
		e.PromptVersion == PromptVersion
}
