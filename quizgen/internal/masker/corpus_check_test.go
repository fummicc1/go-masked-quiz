package masker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
)

// TestMaskDocument_RealCorpus masks a checkout of golang/proposal when
// GO_PROPOSAL_DIR points at one, checking the invariants that matter for
// rendering: every mask resolves to a blank, and putting the answers back
// reproduces the source text of every block.
func TestMaskDocument_RealCorpus(t *testing.T) {
	dir := os.Getenv("GO_PROPOSAL_DIR")
	if dir == "" {
		t.Skip("set GO_PROPOSAL_DIR to a golang/proposal design/ directory")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil || len(files) == 0 {
		t.Skipf("no markdown under %s", dir)
	}

	var docs, blocks, blanks, masks int
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		name := filepath.Base(f)
		parsed := parser.ParseDocument(name, src, parser.Options{AcceptBareGoFences: true})
		md := MaskDocument(NewRNG(42, name), parsed, 2)

		docs++
		blocks += len(md.Blocks)
		blanks += len(md.Blanks)

		if len(md.Blocks) != len(parsed.Blocks) {
			t.Errorf("%s: %d blocks after masking, want %d", name, len(md.Blocks), len(parsed.Blocks))
		}
		for i, b := range md.Blocks {
			var restored string
			for _, s := range b.Spans {
				switch s.Kind {
				case "mask":
					masks++
					if s.BlankIndex < 0 || s.BlankIndex >= len(md.Blanks) {
						t.Fatalf("%s block %d: blank index %d out of range", name, i, s.BlankIndex)
					}
					restored += md.Blanks[s.BlankIndex].Answer
				default:
					restored += s.Text
				}
			}
			var original string
			for _, s := range parsed.Blocks[i].Spans {
				original += s.Text
			}
			if restored != original {
				t.Errorf("%s block %d does not round trip\n got: %q\nwant: %q", name, i, restored, original)
			}
		}
	}
	t.Logf("docs=%d blocks=%d blanks=%d masks=%d", docs, blocks, blanks, masks)
}
