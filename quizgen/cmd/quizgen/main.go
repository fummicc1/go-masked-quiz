// Command quizgen generates a JSON file of fill-in-the-blank quizzes from a
// local clone of the Go proposals repository.
//
// Pipeline: walk the proposals directory, parse each design/*.md with
// internal/parser, then build one quiz per unit (a prose paragraph or a code
// block). internal/masker picks up to --max-blanks-per-quiz identifiers per
// unit (code identifiers via go/scanner) and masks every occurrence of each;
// internal/blocks renders the unit into pre-parsed blocks. All randomness
// derives from --seed, so identical inputs yield byte-identical output (modulo
// generated_at).
package main

import (
	"context"
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
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/source"
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
  generate    Generate quizzes.json from design docs (--source design-docs)
              or from golang/go proposal issues (--source github-issues)

Run "quizgen generate -h" for flag details.`)
}

func runGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	var (
		sourceKind     = fs.String("source", "design-docs", "data source: design-docs | github-issues")
		proposals      = fs.String("proposals", "", "path to the design/ directory of golang/proposal (required for design-docs)")
		out            = fs.String("out", "output/quizzes.json", "output JSON path")
		seed           = fs.Int64("seed", 42, "deterministic RNG seed")
		commit         = fs.String("commit", "", "source repo commit SHA (recorded in metadata)")
		maxPerProposal = fs.Int("max-per-proposal", 5, "maximum quizzes of each kind per proposal")
		maxBlanks      = fs.Int("max-blanks-per-quiz", 3, "maximum blanks (distinct tokens masked) per quiz")
		choices        = fs.Int("choices", 4, "number of choices per blank")
		maxProposals   = fs.Int("max-proposals", 0, "max proposals to fetch (github-issues; 0 = no limit)")
		query          = fs.String("query", "", "issue-search query (github-issues; default: accepted proposals)")
		now            = fs.String("now", "", "fix generated_at to this RFC3339 time (for reproducible/golden output)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	generatedAt := time.Now().UTC()
	if *now != "" {
		t, err := time.Parse(time.RFC3339, *now)
		if err != nil {
			return fmt.Errorf("parse --now: %w", err)
		}
		generatedAt = t.UTC()
	}

	bundle := quiz.Bundle{
		Version:     quiz.SchemaVersion,
		GeneratedAt: generatedAt,
		Proposals:   []quiz.Proposal{},
	}

	var (
		items []genItem
		err   error
	)
	switch *sourceKind {
	case "design-docs":
		bundle.SourceRepo = "https://github.com/golang/proposal"
		bundle.SourceFork = "https://github.com/fummicc1/golang-proposal"
		bundle.SourceCommit = *commit
		bundle.SourceLicense = "BSD-3-Clause"
		bundle.SourceLicenseURL = "https://go.googlesource.com/proposal/+/refs/heads/master/LICENSE"
		items, err = collectDesignDocs(*proposals)
	case "github-issues":
		bundle.SourceRepo = "https://github.com/golang/go"
		bundle.SourceCommit = *commit
		bundle.SourceLicense = "BSD-3-Clause"
		bundle.SourceLicenseURL = "https://go.dev/LICENSE"
		items, err = collectIssues(*query, *maxProposals)
	default:
		return fmt.Errorf("--source %q: want design-docs or github-issues", *sourceKind)
	}
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no proposals collected from source %q", *sourceKind)
	}

	parsed := make([]*parser.Proposal, len(items))
	for i := range items {
		parsed[i] = items[i].p
	}
	crossProse, crossCode := crossPools(parsed)

	for _, it := range items {
		quizzes := buildQuizzes(it.p, it.id, *seed, *maxPerProposal, *maxBlanks, *choices, crossProse, crossCode)
		if len(quizzes) == 0 {
			continue
		}
		bundle.Proposals = append(bundle.Proposals, quiz.Proposal{
			ID:      it.id,
			Title:   it.p.Title,
			URL:     it.url,
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

// genItem is one proposal queued for quiz generation, with its output ID and URL.
type genItem struct {
	p   *parser.Proposal
	id  string
	url string
}

// collectDesignDocs loads every design/*.md under dir (the original source).
func collectDesignDocs(dir string) ([]genItem, error) {
	if dir == "" {
		return nil, fmt.Errorf("--proposals is required for --source design-docs")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("stat --proposals: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("--proposals %q is not a directory", dir)
	}
	mdFiles, err := listMarkdown(dir)
	if err != nil {
		return nil, err
	}
	if len(mdFiles) == 0 {
		return nil, fmt.Errorf("no *.md files found under %q", dir)
	}
	items := make([]genItem, 0, len(mdFiles))
	for _, f := range mdFiles {
		p, err := parser.LoadProposal(f)
		if err != nil {
			return nil, err
		}
		items = append(items, genItem{
			p:   p,
			id:  "design-" + p.Slug,
			url: "https://github.com/golang/proposal/blob/master/design/" + filepath.Base(f),
		})
	}
	return items, nil
}

// collectIssues fetches accepted proposal issues from golang/go. The slug
// "issue-<number>" doubles as the output ID and (number being stable) the RNG
// tag, so output stays reproducible.
func collectIssues(query string, max int) ([]genItem, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("--source github-issues requires the GITHUB_TOKEN environment variable")
	}
	client := source.NewClient(token)
	proposals, err := source.FetchProposals(context.Background(), client, source.FetchOptions{
		Query: query,
		Max:   max,
	})
	if err != nil {
		return nil, err
	}
	items := make([]genItem, 0, len(proposals))
	for _, pr := range proposals {
		items = append(items, genItem{p: pr.Parsed, id: pr.Slug, url: pr.URL})
	}
	return items, nil
}

// buildQuizzes produces one quiz per unit (prose paragraphs, then code blocks)
// for a proposal. id prefixes every quiz ID; RNG tags depend only on
// (seed, proposal slug, unit, purpose), so output never depends on the id or on
// iteration order.
func buildQuizzes(p *parser.Proposal, id string, seed int64, maxPerProposal, maxBlanks, nChoices int, crossProse, crossCode []string) []quiz.Quiz {
	var quizzes []quiz.Quiz

	proseTokens := masker.ProposalTokens(p)
	for _, ui := range selectUnits(seed, "prose-units:"+p.Slug, len(p.ProseUnits), maxPerProposal) {
		unit := p.ProseUnits[ui]
		rng := masker.NewRNG(seed, "prose-blanks:"+p.Slug+":"+strconv.Itoa(unit.Start))
		blanks := masker.SelectProseBlanks(rng, unit, maxBlanks)
		if len(blanks) == 0 {
			continue
		}
		quizzes = append(quizzes, quiz.Quiz{
			ID:     fmt.Sprintf("%s-q%02d", id, len(quizzes)+1),
			Kind:   quiz.KindProse,
			Index:  len(quizzes),
			Blocks: blocks.BuildProseBlocks(p.Source, unit, blanks),
			Blanks: makeBlanks(seed, p.Slug, quiz.KindProse, unit.Start, blanks, proseTokens, crossProse, nChoices),
		})
	}

	codeTokens := masker.CodeTokens(p)
	for _, ci := range selectUnits(seed, "code-units:"+p.Slug, len(p.CodeBlocks), maxPerProposal) {
		block := p.CodeBlocks[ci]
		rng := masker.NewRNG(seed, "code-blanks:"+p.Slug+":"+strconv.Itoa(ci))
		blanks := masker.SelectCodeBlanks(rng, block, maxBlanks)
		if len(blanks) == 0 {
			continue
		}
		quizzes = append(quizzes, quiz.Quiz{
			ID:     fmt.Sprintf("%s-q%02d", id, len(quizzes)+1),
			Kind:   quiz.KindCode,
			Index:  len(quizzes),
			Blocks: blocks.BuildCodeBlocks(block, blanks),
			Blanks: makeBlanks(seed, p.Slug, quiz.KindCode, ci, blanks, codeTokens, crossCode, nChoices),
		})
	}

	return quizzes
}

// makeBlanks attaches choices to each blank, excluding the other blanks'
// answers so one blank's options never reveal another's answer.
func makeBlanks(seed int64, slug string, kind quiz.Kind, unitKey int, blanks []masker.Blank, pool, cross []string, nChoices int) []quiz.Blank {
	answers := make([]string, len(blanks))
	for i, b := range blanks {
		answers[i] = b.Answer
	}
	out := make([]quiz.Blank, 0, len(blanks))
	for i, b := range blanks {
		exclude := make([]string, 0, len(answers))
		for j, a := range answers {
			if j != i {
				exclude = append(exclude, a)
			}
		}
		tag := "choice:" + string(kind) + ":" + slug + ":" + strconv.Itoa(unitKey) + ":" + b.Answer
		rng := masker.NewRNG(seed, tag)
		out = append(out, quiz.Blank{
			Answer:  b.Answer,
			Choices: masker.GenerateChoices(rng, b.Answer, pool, cross, exclude, nChoices),
		})
	}
	return out
}

// selectUnits returns the indices [0,n) to turn into quizzes, capped at max
// (deterministically sampled when n exceeds max), in ascending order.
func selectUnits(seed int64, tag string, n, max int) []int {
	all := make([]int, n)
	for i := range all {
		all[i] = i
	}
	if max <= 0 || n <= max {
		return all
	}
	idx := masker.NewRNG(seed, tag).Sample(n, max)
	sort.Ints(idx)
	return idx
}

// crossPools gathers distinct prose tokens and code identifiers across all
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
		if name := e.Name(); strings.HasSuffix(name, ".md") {
			out = append(out, filepath.Join(dir, name))
		}
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
