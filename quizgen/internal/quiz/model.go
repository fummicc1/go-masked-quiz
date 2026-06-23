// Package quiz defines the on-disk JSON model emitted by quizgen and consumed
// by the iOS app.
package quiz

import "time"

// SchemaVersion is the schema version written to every Bundle. The iOS client
// rejects payloads whose version it does not understand.
const SchemaVersion = 3

// Bundle is the top-level structure written to quizzes.json.
//
// The single-valued Source* fields describe the primary upstream (kept for
// backward compatibility with v3 clients). When the bundle draws from more than
// one upstream, Sources lists every contributing source; it is omitted for
// single-source bundles, so their output is byte-identical to before and v3
// clients (which ignore unknown keys) are unaffected.
type Bundle struct {
	Version          int        `json:"version"`
	GeneratedAt      time.Time  `json:"generated_at"`
	SourceRepo       string     `json:"source_repo"`
	SourceFork       string     `json:"source_fork"`
	SourceCommit     string     `json:"source_commit,omitempty"`
	SourceLicense    string     `json:"source_license"`
	SourceLicenseURL string     `json:"source_license_url"`
	Sources          []Source   `json:"sources,omitempty"`
	Proposals        []Proposal `json:"proposals"`
}

// Source describes one upstream that contributed proposals to a bundle, for
// attribution and reproducibility.
type Source struct {
	Kind       string `json:"kind"` // "design-docs" | "github-issues"
	Repo       string `json:"repo"`
	Commit     string `json:"commit,omitempty"`
	License    string `json:"license"`
	LicenseURL string `json:"license_url"`
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

// BlockType labels one rendered fragment of a quiz. The schema ships pre-parsed
// blocks so the iOS client renders a quiz by iterating the slice without
// re-parsing Markdown.
type BlockType string

const (
	// BlockText is literal prose text.
	BlockText BlockType = "text"
	// BlockInlineCode is an inline `code` span (rendered monospaced).
	BlockInlineCode BlockType = "inline_code"
	// BlockCodeBlock is a fragment of a fenced code block.
	BlockCodeBlock BlockType = "code_block"
	// BlockMask is a blank to fill in. Carries BlankIndex (no Value).
	BlockMask BlockType = "mask"
)

// Block is one fragment of a quiz's rendered body.
//
// Invariants (enforced by the generator and validated in tests):
//   - Type == BlockMask  => Value empty, BlankIndex set (∈ [0, len(Blanks))).
//   - Type != BlockMask  => Value non-empty, BlankIndex nil.
type Block struct {
	Type       BlockType `json:"type"`
	Value      string    `json:"value,omitempty"`
	BlankIndex *int      `json:"blank_index,omitempty"`
}

// Blank is one fill-in target: the answer plus its multiple-choice options.
// A single Blank may be referenced by several mask blocks (every occurrence of
// the same token in the unit), so answering once fills them all.
type Blank struct {
	Answer  string   `json:"answer"`
	Choices []string `json:"choices"`
}

// Quiz is one fill-in-the-blank question built from a single unit (a prose
// paragraph or a code block). Blocks reconstructs the displayed body; each
// BlockMask points into Blanks. There is at least one Blank.
type Quiz struct {
	ID     string  `json:"id"`
	Kind   Kind    `json:"kind"`
	Index  int     `json:"index"`
	Blocks []Block `json:"blocks"`
	Blanks []Blank `json:"blanks"`
}

// MaskCount returns the number of BlockMask entries (mask occurrences, which
// may exceed the number of blanks).
func (q Quiz) MaskCount() int {
	n := 0
	for _, b := range q.Blocks {
		if b.Type == BlockMask {
			n++
		}
	}
	return n
}
