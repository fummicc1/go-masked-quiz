// Package quiz defines the on-disk JSON model emitted by quizgen and consumed
// by the iOS app.
package quiz

import "time"

// SchemaVersion is the schema version written to every Bundle. The iOS client
// rejects payloads whose version it does not understand.
const SchemaVersion = 2

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

// BlockType labels one rendered fragment of a quiz. The v2 schema ships
// pre-parsed blocks so the iOS client can render a quiz by iterating the slice
// without re-parsing Markdown.
type BlockType string

const (
	// BlockText is literal prose text.
	BlockText BlockType = "text"
	// BlockInlineCode is an inline `code` span (rendered monospaced).
	BlockInlineCode BlockType = "inline_code"
	// BlockCodeBlock is a fragment of a fenced code block.
	BlockCodeBlock BlockType = "code_block"
	// BlockMask is the blank to fill in. Exactly one per quiz; carries no Value.
	BlockMask BlockType = "mask"
)

// Block is one fragment of a quiz's rendered body.
//
// Invariants (enforced by the generator and validated in tests):
//   - Type == BlockMask  => Value is empty (omitted from JSON).
//   - Type != BlockMask  => Value is non-empty.
type Block struct {
	Type  BlockType `json:"type"`
	Value string    `json:"value,omitempty"`
}

// Quiz is a single fill-in-the-blank question.
//
// Blocks reconstructs the displayed body; exactly one Block has Type
// BlockMask, marking where Answer was removed. Choices always contains Answer.
type Quiz struct {
	ID      string   `json:"id"`
	Kind    Kind     `json:"kind"`
	Index   int      `json:"index"`
	Blocks  []Block  `json:"blocks"`
	Answer  string   `json:"answer"`
	Choices []string `json:"choices"`
}

// MaskCount returns the number of BlockMask entries in the quiz. A valid quiz
// always returns 1.
func (q Quiz) MaskCount() int {
	n := 0
	for _, b := range q.Blocks {
		if b.Type == BlockMask {
			n++
		}
	}
	return n
}
