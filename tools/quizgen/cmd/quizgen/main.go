// Command quizgen generates a JSON file of fill-in-the-blank quizzes from a
// local clone of the Go proposals repository.
//
// Phase 1 scope: parse flags, walk the proposals directory, and emit a
// well-formed quizzes.json that contains a placeholder quiz for the first
// proposal found. Real masking logic lands in Phase 2.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/quiz"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "generate":
		if err := runGenerate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "quizgen:", err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "quizgen: unknown subcommand %q\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(2)
	}
}

func usage(w *os.File) {
	fmt.Fprintln(w, `usage: quizgen <subcommand> [flags]

subcommands:
  generate    Generate quizzes.json from a local clone of golang/proposal

Run "quizgen generate -h" for flag details.`)
}

func runGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	var (
		proposals      = fs.String("proposals", "", "path to the design/ directory of golang/proposal (required)")
		out            = fs.String("out", "output/quizzes.json", "output JSON path")
		seed           = fs.Int64("seed", 42, "deterministic RNG seed (used in Phase 2)")
		commit         = fs.String("commit", "", "source repo commit SHA (recorded in metadata)")
		maxPerProposal = fs.Int("max-per-proposal", 5, "maximum quizzes per proposal (used in Phase 2)")
		choices        = fs.Int("choices", 4, "number of choices per quiz (used in Phase 2)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Silence "declared and not used" until Phase 2 wires these in.
	_, _, _ = *seed, *maxPerProposal, *choices

	if *proposals == "" {
		return fmt.Errorf("--proposals is required")
	}
	info, err := os.Stat(*proposals)
	if err != nil {
		return fmt.Errorf("stat --proposals: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("--proposals %q is not a directory", *proposals)
	}

	bundle := quiz.Bundle{
		Version:          1,
		GeneratedAt:      time.Now().UTC(),
		SourceRepo:       "https://github.com/golang/proposal",
		SourceFork:       "https://github.com/fummicc1/golang-proposal",
		SourceCommit:     *commit,
		SourceLicense:    "BSD-3-Clause",
		SourceLicenseURL: "https://go.googlesource.com/proposal/+/refs/heads/master/LICENSE",
		Proposals:        []quiz.Proposal{},
	}

	mdFiles, err := listMarkdown(*proposals)
	if err != nil {
		return err
	}
	if len(mdFiles) == 0 {
		return fmt.Errorf("no *.md files found under %q", *proposals)
	}

	// Phase 1 placeholder: emit a single dummy quiz for the first proposal
	// so the end-to-end pipeline (CLI flag → directory walk → JSON write)
	// can be verified before Phase 2 lands the real masking logic.
	first := mdFiles[0]
	slug := strings.TrimSuffix(filepath.Base(first), ".md")
	bundle.Proposals = append(bundle.Proposals, quiz.Proposal{
		ID:    "design-" + slug,
		Title: slug,
		URL:   "https://github.com/golang/proposal/blob/master/design/" + filepath.Base(first),
		Quizzes: []quiz.Quiz{
			{
				ID:            "design-" + slug + "-q01",
				Kind:          quiz.KindProse,
				Index:         0,
				ContextBefore: "(Phase 1 placeholder — masking is implemented in Phase 2.) The proposal at ",
				MaskedText:    "____",
				ContextAfter:  " describes a Go language change.",
				Answer:        slug,
				Choices:       []string{slug, "placeholder-a", "placeholder-b", "placeholder-c"},
			},
		},
	})

	if err := writeJSON(*out, &bundle); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %d proposal(s) / %d quiz(zes) to %s\n",
		len(bundle.Proposals), countQuizzes(&bundle), *out)
	return nil
}

func listMarkdown(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	sort.Strings(out)
	return out, nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir output dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	return nil
}

func countQuizzes(b *quiz.Bundle) int {
	n := 0
	for _, p := range b.Proposals {
		n += len(p.Quizzes)
	}
	return n
}
