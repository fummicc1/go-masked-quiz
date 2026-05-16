// Package quiz defines the on-disk JSON model emitted by quizgen and consumed
// by the iOS app.
package quiz

import "time"

// Bundle is the top-level structure written to quizzes.json.
type Bundle struct {
	Version          int        `json:"version"`
	GeneratedAt      time.Time  `json:"generated_at"`
	SourceRepo       string     `json:"source_repo"`
	SourceFork       string     `json:"source_fork"`
	SourceCommit     string     `json:"source_commit,omitempty"`
	SourceLicense    string     `json:"source_license"`
	SourceLicenseURL string     `json:"source_license_url"`
	Proposals        []Proposal `json:"proposals"`
}

// Proposal groups the quizzes generated from one design/*.md file.
type Proposal struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Quizzes []Quiz `json:"quizzes"`
}

// Kind discriminates between prose-derived quizzes and code-block-derived quizzes.
type Kind string

const (
	KindProse Kind = "prose"
	KindCode  Kind = "code"
)

// Quiz is a single fill-in-the-blank question.
//
// ContextBefore + MaskedText + ContextAfter reconstructs the displayed text
// shown to the user, where MaskedText is the placeholder (e.g. "____").
type Quiz struct {
	ID            string   `json:"id"`
	Kind          Kind     `json:"kind"`
	Index         int      `json:"index"`
	ContextBefore string   `json:"context_before"`
	MaskedText    string   `json:"masked_text"`
	ContextAfter  string   `json:"context_after"`
	Answer        string   `json:"answer"`
	Choices       []string `json:"choices"`
}
