package parser

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestParseDocument_RealCorpus runs the document parser over a checkout of
// golang/proposal when one is pointed at by GO_PROPOSAL_DIR. It is a sanity
// check against real input rather than an assertion-heavy test: the sample in
// document_test.go cannot cover the shapes 100+ real proposals use.
func TestParseDocument_RealCorpus(t *testing.T) {
	dir := os.Getenv("GO_PROPOSAL_DIR")
	if dir == "" {
		t.Skip("set GO_PROPOSAL_DIR to a golang/proposal design/ directory")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil || len(files) == 0 {
		t.Skipf("no markdown under %s", dir)
	}

	counts := map[BlockKind]int{}
	var totalBlocks, totalBytes, empty int
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		d := ParseDocument(filepath.Base(f), src, Options{AcceptBareGoFences: true})
		if len(d.Blocks) == 0 {
			empty++
			t.Errorf("%s produced no blocks", filepath.Base(f))
		}
		totalBlocks += len(d.Blocks)
		for _, b := range d.Blocks {
			counts[b.Kind]++
			for _, s := range b.Spans {
				totalBytes += len(s.Text)
			}
		}
	}

	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, string(k))
	}
	sort.Strings(kinds)
	t.Logf("files=%d blocks=%d text_bytes=%d empty=%d", len(files), totalBlocks, totalBytes, empty)
	for _, k := range kinds {
		t.Logf("  %-10s %d", k, counts[BlockKind(k)])
	}
}
