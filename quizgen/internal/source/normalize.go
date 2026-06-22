// Package source fetches Go proposals from the golang/go issue tracker and
// turns them into parser.Proposal values, so the existing masker/blocks
// pipeline can build quizzes from issues just as it does from design docs.
package source

import (
	"regexp"
	"strings"
)

var (
	// htmlComment matches <!-- ... --> spans (issue templates use these for
	// instructions that should never become quiz text).
	htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	// taskListItem matches GitHub task-list checkboxes ("- [ ]" / "- [x]").
	taskListItem = regexp.MustCompile(`(?m)^[ \t]*[-*][ \t]+\[[ xX]\].*$`)
	// blankRun matches three or more consecutive newlines.
	blankRun = regexp.MustCompile(`\n{3,}`)
)

// Normalize cleans a raw GitHub issue body into proposal-like Markdown:
// it normalises line endings, strips HTML comments and template checklists,
// and collapses runs of blank lines. It preserves headings, prose, inline
// code, and fenced code blocks (the spans the parser turns into quizzes).
func Normalize(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	body = htmlComment.ReplaceAllString(body, "")
	body = taskListItem.ReplaceAllString(body, "")
	body = blankRun.ReplaceAllString(body, "\n\n")
	return strings.TrimSpace(body) + "\n"
}
