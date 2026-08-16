package quiz

// The v2 shape: a proposal is a document you read, with blanks punched into it,
// rather than a list of detached quiz cards. It exists so the client can show
// the proposal the way its author wrote it — headings, prose, lists and code in
// order — and ask about a token where that token actually appears.

// BlockKind is how a document block is laid out.
type BlockKind string

const (
	BlockParagraph BlockKind = "paragraph"
	BlockHeading   BlockKind = "heading"
	BlockCode      BlockKind = "code"
	BlockListItem  BlockKind = "list_item"
	BlockQuote     BlockKind = "quote"
)

// SpanKind labels one inline run within a block.
type SpanKind string

const (
	SpanText       SpanKind = "text"
	SpanInlineCode SpanKind = "inline_code"
	SpanMask       SpanKind = "mask"
)

// Span is one inline run.
//
// Invariants (enforced by the generator, checked in tests):
//   - Kind == SpanMask => Value empty, BlankIndex set and in range.
//   - Kind != SpanMask => BlankIndex nil.
type Span struct {
	Kind       SpanKind `json:"kind"`
	Value      string   `json:"value,omitempty"`
	BlankIndex *int     `json:"blank_index,omitempty"`
}

// DocBlock is one block of a proposal in reading order.
type DocBlock struct {
	Kind  BlockKind `json:"kind"`
	Level int       `json:"level,omitempty"` // heading level, or list nesting depth
	Lang  string    `json:"lang,omitempty"`  // fenced code language
	Spans []Span    `json:"spans"`
}

// Document is a proposal rendered as blocks with its blanks.
type Document struct {
	Blocks []DocBlock `json:"blocks"`
	Blanks []Blank    `json:"blanks"`
}

// MaskCount returns how many mask spans the document contains. It can exceed
// len(Blanks): a token repeated in one block is masked at every occurrence but
// answered once.
func (d Document) MaskCount() int {
	n := 0
	for _, b := range d.Blocks {
		for _, s := range b.Spans {
			if s.Kind == SpanMask {
				n++
			}
		}
	}
	return n
}
