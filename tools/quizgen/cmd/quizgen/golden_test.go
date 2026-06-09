package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/quiz"
)

var update = flag.Bool("update", false, "update golden files")

const (
	proposalsDir = "../../testdata/proposals"
	goldenPath   = "../../testdata/golden/quizzes-seed42.json"
	fixedNow     = "2026-05-18T00:00:00Z"
)

func generate(t *testing.T) []byte {
	t.Helper()
	outPath := filepath.Join(t.TempDir(), "out.json")
	if err := runGenerate([]string{
		"--proposals", proposalsDir, "--out", outPath,
		"--seed", "42", "--now", fixedNow,
	}); err != nil {
		t.Fatalf("runGenerate: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	return data
}

// TC-G-E-01: output matches the committed golden snapshot.
func TestGolden(t *testing.T) {
	got := generate(t)
	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated golden: %s", goldenPath)
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run `go test ./cmd/quizgen -update`): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from golden; run `go test ./cmd/quizgen -update`")
	}
}

// TC-G-E-02: two runs are byte-identical (determinism).
func TestDeterministic(t *testing.T) {
	if !bytes.Equal(generate(t), generate(t)) {
		t.Error("output is not deterministic")
	}
}

// v3 schema invariants for every generated quiz.
func TestSchemaInvariants(t *testing.T) {
	var b quiz.Bundle
	if err := json.Unmarshal(generate(t), &b); err != nil {
		t.Fatal(err)
	}
	if b.Version != quiz.SchemaVersion {
		t.Errorf("Version = %d, want %d", b.Version, quiz.SchemaVersion)
	}
	if len(b.Proposals) == 0 {
		t.Fatal("no proposals")
	}
	for _, p := range b.Proposals {
		for _, q := range p.Quizzes {
			if len(q.Blanks) == 0 {
				t.Errorf("%s: no blanks", q.ID)
			}
			if q.MaskCount() < 1 {
				t.Errorf("%s: no mask blocks", q.ID)
			}
			referenced := make([]bool, len(q.Blanks))
			for _, blk := range q.Blocks {
				switch blk.Type {
				case quiz.BlockMask:
					if blk.BlankIndex == nil {
						t.Errorf("%s: mask without blank_index", q.ID)
						continue
					}
					bi := *blk.BlankIndex
					if bi < 0 || bi >= len(q.Blanks) {
						t.Errorf("%s: blank_index %d out of range", q.ID, bi)
						continue
					}
					referenced[bi] = true
				default:
					if blk.Value == "" {
						t.Errorf("%s: non-mask block %q empty", q.ID, blk.Type)
					}
					if blk.BlankIndex != nil {
						t.Errorf("%s: non-mask block has blank_index", q.ID)
					}
					switch q.Kind {
					case quiz.KindProse:
						if blk.Type == quiz.BlockCodeBlock {
							t.Errorf("%s: prose has code_block", q.ID)
						}
					case quiz.KindCode:
						if blk.Type == quiz.BlockText || blk.Type == quiz.BlockInlineCode {
							t.Errorf("%s: code has %q", q.ID, blk.Type)
						}
					}
				}
			}
			for i, used := range referenced {
				if !used {
					t.Errorf("%s: blank %d (%q) referenced by no mask", q.ID, i, q.Blanks[i].Answer)
				}
			}
			for _, bl := range q.Blanks {
				if len(bl.Choices) != 4 {
					t.Errorf("%s: blank %q choices = %d, want 4", q.ID, bl.Answer, len(bl.Choices))
				}
				if !choicesContain(bl.Choices, bl.Answer) {
					t.Errorf("%s: blank %q choices missing answer", q.ID, bl.Answer)
				}
			}
		}
	}
}

func choicesContain(choices []string, answer string) bool {
	for _, c := range choices {
		if c == answer {
			return true
		}
	}
	return false
}
