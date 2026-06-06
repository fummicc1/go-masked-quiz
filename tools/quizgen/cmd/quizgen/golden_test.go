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

// testdata lives at the module root (tools/quizgen/testdata); this package is
// two levels down at cmd/quizgen.
const (
	proposalsDir = "../../testdata/proposals"
	goldenPath   = "../../testdata/golden/quizzes-seed42.json"
	fixedNow     = "2026-05-18T00:00:00Z"
)

func generate(t *testing.T) []byte {
	t.Helper()
	outPath := filepath.Join(t.TempDir(), "out.json")
	err := runGenerate([]string{
		"--proposals", proposalsDir,
		"--out", outPath,
		"--seed", "42",
		"--now", fixedNow,
	})
	if err != nil {
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
		t.Errorf("output differs from golden; run `go test ./cmd/quizgen -update` to refresh")
	}
}

// TC-G-E-02: two runs with identical inputs are byte-identical (determinism).
func TestDeterministic(t *testing.T) {
	if !bytes.Equal(generate(t), generate(t)) {
		t.Error("output is not deterministic across runs")
	}
}

// TC-G-S-*: schema invariants hold for every generated quiz.
func TestSchemaInvariants(t *testing.T) {
	var b quiz.Bundle
	if err := json.Unmarshal(generate(t), &b); err != nil {
		t.Fatal(err)
	}
	if b.Version != quiz.SchemaVersion {
		t.Errorf("Version = %d, want %d", b.Version, quiz.SchemaVersion)
	}
	if len(b.Proposals) == 0 {
		t.Fatal("no proposals generated")
	}
	for _, p := range b.Proposals {
		for _, q := range p.Quizzes {
			if q.MaskCount() != 1 {
				t.Errorf("%s: mask count = %d, want 1", q.ID, q.MaskCount())
			}
			if len(q.Choices) != 4 {
				t.Errorf("%s: choices = %d, want 4", q.ID, len(q.Choices))
			}
			if !choicesContain(q.Choices, q.Answer) {
				t.Errorf("%s: choices %v missing answer %q", q.ID, q.Choices, q.Answer)
			}
			for _, blk := range q.Blocks {
				if blk.Type == quiz.BlockMask {
					if blk.Value != "" {
						t.Errorf("%s: mask block has value %q", q.ID, blk.Value)
					}
					continue
				}
				if blk.Value == "" {
					t.Errorf("%s: non-mask block %q has empty value", q.ID, blk.Type)
				}
				switch q.Kind {
				case quiz.KindProse:
					if blk.Type == quiz.BlockCodeBlock {
						t.Errorf("%s: prose quiz must not contain code_block", q.ID)
					}
				case quiz.KindCode:
					if blk.Type == quiz.BlockText || blk.Type == quiz.BlockInlineCode {
						t.Errorf("%s: code quiz must not contain %q", q.ID, blk.Type)
					}
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
