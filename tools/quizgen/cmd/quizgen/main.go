// Command quizgen generates a JSON file of fill-in-the-blank quizzes from a
// local clone of the Go proposals repository.
//
// Pipeline: walk the proposals directory, parse each design/*.md with
// internal/parser, choose identifiers to blank out with internal/masker
// (prose inline-code spans via text rules, code-block identifiers via
// go/scanner), expand each into pre-parsed blocks with internal/blocks, and
// emit a v2 quizzes.json. All randomness derives from --seed, so the same
// inputs always yield byte-identical output (modulo generated_at).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fummicc1/go-masked-quiz/quizgen/internal/blocks"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/masker"
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/parser"
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
		seed           = fs.Int64("seed", 42, "deterministic RNG seed")
		commit         = fs.String("commit", "", "source repo commit SHA (recorded in metadata)")
		maxPerProposal = fs.Int("max-per-proposal", 5, "maximum quizzes of each kind per proposal")
		choices        = fs.Int("choices", 4, "number of choices per quiz")
		ctxProse       = fs.Int("context-prose", 80, "prose context window (bytes per side)")
		ctxCode        = fs.Int("context-code", 120, "code context window (bytes per side)")
		now            = fs.String("now", "", "fix generated_at to this RFC3339 time (for reproducible/golden output)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
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

	generatedAt := time.Now().UTC()
	if *now != "" {
		t, err := time.Parse(time.RFC3339, *now)
		if err != nil {
			return fmt.Errorf("parse --now: %w", err)
		}
		generatedAt = t.UTC()
	}

	mdFiles, err := listMarkdown(*proposals)
	if err != nil {
		return err
	}
	if len(mdFiles) == 0 {
		return fmt.Errorf("no *.md files found under %q", *proposals)
	}

	// Parse all proposals first so we can build cross-proposal distractor pools.
	parsed := make([]*parser.Proposal, 0, len(mdFiles))
	for _, f := range mdFiles {
		p, err := parser.LoadProposal(f)
		if err != nil {
			return err
		}
		parsed = append(parsed, p)
	}
	crossProse, crossCode := crossPools(parsed)

	bundle := quiz.Bundle{
		Version:          quiz.SchemaVersion,
		GeneratedAt:      generatedAt,
		SourceRepo:       "https://github.com/golang/proposal",
		SourceFork:       "https://github.com/fummicc1/golang-proposal",
		SourceCommit:     *commit,
		SourceLicense:    "BSD-3-Clause",
		SourceLicenseURL: "https://go.googlesource.com/proposal/+/refs/heads/master/LICENSE",
		Proposals:        []quiz.Proposal{},
	}

	for i, p := range parsed {
		quizzes := buildQuizzes(p, *seed, *maxPerProposal, *choices, *ctxProse, *ctxCode, crossProse, crossCode)
		if len(quizzes) == 0 {
			continue // proposals with no maskable content are skipped
		}
		bundle.Proposals = append(bundle.Proposals, quiz.Proposal{
			ID:      "design-" + p.Slug,
			Title:   p.Title,
			URL:     "https://github.com/golang/proposal/blob/master/design/" + filepath.Base(mdFiles[i]),
			Quizzes: quizzes,
		})
	}

	if err := writeJSON(*out, &bundle); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %d proposal(s) / %d quiz(zes) to %s\n",
		len(bundle.Proposals), countQuizzes(&bundle), *out)
	return nil
}

// buildQuizzes produces the prose quizzes followed by the code quizzes for one
// proposal. Each RNG stream is tagged so it depends only on (seed, proposal,
// purpose) — never on iteration order — keeping output deterministic.
func buildQuizzes(p *parser.Proposal, seed int64, maxPer, nChoices, ctxProse, ctxCode int, crossProse, crossCode []string) []quiz.Quiz {
	id := "design-" + p.Slug
	var quizzes []quiz.Quiz

	proseSeeds := masker.CollectProseSeeds(masker.NewRNG(seed, "prose:"+p.Slug), p, maxPer)
	proseTokens := masker.ProposalTokens(p)
	for _, s := range proseSeeds {
		tag := "choice:prose:" + p.Slug + ":" + strconv.Itoa(s.Start)
		quizzes = append(quizzes, quiz.Quiz{
			ID:      fmt.Sprintf("%s-q%02d", id, len(quizzes)+1),
			Kind:    quiz.KindProse,
			Index:   len(quizzes),
			Blocks:  blocks.BuildProseBlocks(p, s, ctxProse),
			Answer:  s.Answer,
			Choices: masker.GenerateChoices(masker.NewRNG(seed, tag), s.Answer, proseTokens, crossProse, nChoices),
		})
	}

	codeSeeds := masker.CollectCodeSeeds(masker.NewRNG(seed, "code:"+p.Slug), p, maxPer)
	codeTokens := masker.CodeTokens(p)
	for _, s := range codeSeeds {
		tag := "choice:code:" + p.Slug + ":" + strconv.Itoa(s.BlockIndex) + ":" + strconv.Itoa(s.Start)
		quizzes = append(quizzes, quiz.Quiz{
			ID:      fmt.Sprintf("%s-q%02d", id, len(quizzes)+1),
			Kind:    quiz.KindCode,
			Index:   len(quizzes),
			Blocks:  blocks.BuildCodeBlocks(p, s, ctxCode),
			Answer:  s.Answer,
			Choices: masker.GenerateChoices(masker.NewRNG(seed, tag), s.Answer, codeTokens, crossCode, nChoices),
		})
	}

	return quizzes
}

// crossPools gathers the distinct prose tokens and code identifiers across all
// proposals, used as cross-proposal distractor pools.
func crossPools(parsed []*parser.Proposal) (prose, code []string) {
	seenP, seenC := map[string]bool{}, map[string]bool{}
	for _, p := range parsed {
		for _, t := range masker.ProposalTokens(p) {
			if !seenP[strings.ToLower(t)] {
				seenP[strings.ToLower(t)] = true
				prose = append(prose, t)
			}
		}
		for _, t := range masker.CodeTokens(p) {
			if !seenC[t] {
				seenC[t] = true
				code = append(code, t)
			}
		}
	}
	return prose, code
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
