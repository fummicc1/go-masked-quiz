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
	"github.com/fummicc1/go-masked-quiz/quizgen/internal/llm"
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
	case "llm-generate":
		if err := runLLMGenerate(os.Args[2:]); err != nil {
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
  generate      Generate quizzes.json from design docs (--source design-docs)
                and/or golang/go proposal issues (--source github-issues).
                Pass --llm-cache <dir> to merge cached LLM quizzes (schema v4).
  llm-generate  Generate LLM quizzes and cache them on disk (run before
                "generate --llm-cache"). Provider --provider ollama|workers-ai,
                requires --model. Use --max-generations to batch within a free tier.

Run "quizgen generate -h" or "quizgen llm-generate -h" for flag details.`)
}

func runGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	var (
		sourceKind     = fs.String("source", "design-docs", "comma-separated data sources: design-docs, github-issues")
		proposals      = fs.String("proposals", "", "path to the design/ directory of golang/proposal (required for design-docs)")
		out            = fs.String("out", "output/quizzes.json", "output JSON path")
		seed           = fs.Int64("seed", 42, "deterministic RNG seed")
		commit         = fs.String("commit", "", "source repo commit SHA (recorded in metadata)")
		maxPerProposal = fs.Int("max-per-proposal", 5, "maximum quizzes of each kind per proposal")
		maxBlanks      = fs.Int("max-blanks-per-quiz", 3, "maximum blanks (distinct tokens masked) per quiz")
		choices        = fs.Int("choices", 4, "number of choices per blank")
		maxProposals   = fs.Int("max-proposals", 0, "max proposals to fetch (github-issues; 0 = no limit)")
		query          = fs.String("query", "", "issue-search query (github-issues; default: accepted proposals)")
		llmCache       = fs.String("llm-cache", "", "merge committed LLM quizzes from this dir and emit schema v4 (default: v3, mechanical only)")
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

	kinds, err := splitSources(*sourceKind)
	if err != nil {
		return err
	}

	var (
		allItems []genItem
		srcs     []quiz.Source
	)
	for _, k := range kinds {
		var items []genItem
		var src quiz.Source
		switch k {
		case "design-docs":
			items, err = collectDesignDocs(*proposals)
			src = quiz.Source{
				Kind:       "design-docs",
				Repo:       "https://github.com/golang/proposal",
				Commit:     *commit,
				License:    "BSD-3-Clause",
				LicenseURL: "https://go.googlesource.com/proposal/+/refs/heads/master/LICENSE",
			}
		case "github-issues":
			items, err = collectIssues(*query, *maxProposals)
			src = quiz.Source{
				Kind:       "github-issues",
				Repo:       "https://github.com/golang/go",
				License:    "BSD-3-Clause",
				LicenseURL: "https://go.dev/LICENSE",
			}
		default:
			return fmt.Errorf("--source %q: want design-docs and/or github-issues", k)
		}
		if err != nil {
			return err
		}
		allItems = append(allItems, items...)
		srcs = append(srcs, src)
	}
	if len(allItems) == 0 {
		return fmt.Errorf("no proposals collected from source(s) %q", *sourceKind)
	}

	version := quiz.SchemaVersion
	if *llmCache != "" {
		version = quiz.SchemaVersionV4
	}
	bundle, notes := buildBundle(allItems, srcs, generatedAt, *seed, *maxPerProposal, *maxBlanks, *choices, version, *llmCache)

	if err := writeJSON(*out, &bundle); err != nil {
		return err
	}
	for _, n := range notes {
		fmt.Fprintln(os.Stderr, "note:", n)
	}
	fmt.Fprintf(os.Stderr, "wrote %d proposal(s) / %d quiz(zes) (v%d) from %d source(s) to %s\n",
		len(bundle.Proposals), countQuizzes(&bundle), bundle.Version, len(srcs), *out)
	return nil
}

// splitSources parses the comma-separated --source value into a deduped,
// order-preserving list of source kinds.
func splitSources(s string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(s, ",") {
		k := strings.TrimSpace(part)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--source is empty")
	}
	return out, nil
}

// buildBundle assembles a bundle from already-collected items, pooling
// distractors across all sources. With a single source the legacy source_*
// fields are populated and Sources is omitted (byte-identical to the
// pre-multi-source output); with several sources, per-source attribution is
// added in Sources while the legacy fields keep describing the first source.
//
// When version >= 4, each proposal also carries source metadata, mechanical
// quizzes are tagged gen_method=mechanical, and — if llmCacheDir is set — each
// issue's cached LLM summary and quizzes are merged in. It returns notes for
// missing/stale LLM caches so the caller can surface them.
func buildBundle(items []genItem, srcs []quiz.Source, generatedAt time.Time, seed int64, maxPerProposal, maxBlanks, nChoices, version int, llmCacheDir string) (quiz.Bundle, []string) {
	bundle := quiz.Bundle{
		Version:     version,
		GeneratedAt: generatedAt,
		Proposals:   []quiz.Proposal{},
	}
	if len(srcs) > 0 {
		first := srcs[0]
		bundle.SourceRepo = first.Repo
		bundle.SourceCommit = first.Commit
		bundle.SourceLicense = first.License
		bundle.SourceLicenseURL = first.LicenseURL
		if first.Kind == "design-docs" {
			bundle.SourceFork = "https://github.com/fummicc1/golang-proposal"
		}
		if len(srcs) > 1 {
			bundle.Sources = srcs
		}
	}

	parsed := make([]*parser.Proposal, len(items))
	for i := range items {
		parsed[i] = items[i].p
	}
	crossProse, crossCode := crossPools(parsed)

	v4 := version >= quiz.SchemaVersionV4
	var notes []string

	for _, it := range items {
		quizzes := buildQuizzes(it.p, it.id, seed, maxPerProposal, maxBlanks, nChoices, crossProse, crossCode)
		p := quiz.Proposal{ID: it.id, Title: it.p.Title, URL: it.url}

		if v4 {
			p.SourceKind = it.sourceKind
			p.Status = it.status
			p.IssueNumber = it.issueNumber
			for i := range quizzes {
				quizzes[i].GenMethod = "mechanical"
			}
			if llmCacheDir != "" && it.issueNumber > 0 {
				if note := mergeLLM(&p, &quizzes, it, llmCacheDir); note != "" {
					notes = append(notes, note)
				}
			}
		}

		if len(quizzes) == 0 {
			continue
		}
		p.Quizzes = quizzes
		bundle.Proposals = append(bundle.Proposals, p)
	}
	return bundle, notes
}

// mergeLLM merges an issue's cached LLM summary and quizzes into p/quizzes,
// continuing the quiz numbering after the mechanical quizzes. It returns a note
// when the cache errors or is stale (so the caller can report it); a missing
// cache is silent (mechanical-only is a valid state).
func mergeLLM(p *quiz.Proposal, quizzes *[]quiz.Quiz, it genItem, cacheDir string) string {
	entry, err := llm.Load(cacheDir, it.issueNumber)
	if err != nil {
		return fmt.Sprintf("%s: LLM cache load error: %v", it.id, err)
	}
	if entry == nil {
		return ""
	}
	if !entry.MatchesBody(string(it.p.Source)) {
		return fmt.Sprintf("%s: LLM cache stale (issue body changed) — rerun `quizgen llm-generate`", it.id)
	}
	p.Summary = entry.Summary
	mech := len(*quizzes)
	for i, lq := range entry.Quizzes {
		lq.Index = mech + i
		lq.ID = fmt.Sprintf("%s-q%02d", it.id, mech+i+1)
		*quizzes = append(*quizzes, lq)
	}
	return ""
}

// genItem is one proposal queued for quiz generation, with its output ID, URL,
// and (for v4) per-proposal source metadata.
type genItem struct {
	p           *parser.Proposal
	id          string
	url         string
	sourceKind  string // "design-docs" | "github-issues"
	status      string // issues only: "accepted" | "active"
	issueNumber int    // issues only
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
			p:          p,
			id:         "design-" + p.Slug,
			url:        "https://github.com/golang/proposal/blob/master/design/" + filepath.Base(f),
			sourceKind: "design-docs",
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
		items = append(items, genItem{
			p:           pr.Parsed,
			id:          pr.Slug,
			url:         pr.URL,
			sourceKind:  "github-issues",
			status:      pr.Status,
			issueNumber: pr.Number,
		})
	}
	return items, nil
}

// runLLMGenerate generates LLM quizzes for golang/go issues via a local ollama
// server and caches the validated results on disk (to be committed). It is run
// locally and on demand — never in CI. "generate --llm-cache" later merges the
// cache; CI never calls a model.
func runLLMGenerate(args []string) error {
	fs := flag.NewFlagSet("llm-generate", flag.ContinueOnError)
	var (
		provider     = fs.String("provider", "ollama", "model provider: ollama | workers-ai")
		model        = fs.String("model", "", "model name (required); e.g. qwen2.5-coder:7b (ollama) or @cf/meta/llama-3.3-70b-instruct-fp8-fast (workers-ai)")
		ollamaURL    = fs.String("ollama-url", llm.DefaultOllamaURL, "ollama server URL (provider=ollama)")
		cfAccountID  = fs.String("cf-account-id", "", "Cloudflare account ID (provider=workers-ai; or env CLOUDFLARE_ACCOUNT_ID)")
		query        = fs.String("query", "", "issue-search query (default: accepted proposals)")
		maxProposals = fs.Int("max-proposals", 0, "max issues to consider (0 = no limit)")
		maxGen       = fs.Int("max-generations", 0, "stop after this many new generations (0 = no cap; use to stay within a daily free tier)")
		cacheDir     = fs.String("cache-dir", "cache/llm", "directory for committed LLM cache JSON")
		maxQuizzes   = fs.Int("max-quizzes", 5, "max LLM quizzes to request per proposal")
		maxBlanks    = fs.Int("max-blanks-per-quiz", 1, "max blanks per LLM quiz")
		choices      = fs.Int("choices", 4, "number of choices per blank")
		seed         = fs.Int64("seed", 42, "deterministic RNG seed for choices")
		force        = fs.Bool("force", false, "regenerate even if a fresh cache entry exists")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *model == "" {
		return fmt.Errorf("--model is required")
	}

	gen, err := newGenerator(*provider, *model, *ollamaURL, *cfAccountID)
	if err != nil {
		return err
	}

	items, err := collectIssues(*query, *maxProposals)
	if err != nil {
		return err
	}
	ctx := context.Background()

	var generated, upToDate, failed int
	for _, it := range items {
		if it.issueNumber == 0 {
			continue
		}
		body := string(it.p.Source)
		if !*force {
			if e, err := llm.Load(*cacheDir, it.issueNumber); err == nil && e.Fresh(body, *model) {
				upToDate++
				continue
			}
		}
		if *maxGen > 0 && generated >= *maxGen {
			fmt.Fprintf(os.Stderr, "reached --max-generations=%d; rerun to continue\n", *maxGen)
			break
		}
		content, err := gen.Generate(ctx, llm.SystemPrompt(*maxBlanks), llm.UserPrompt(it.p.Title, body, *maxQuizzes), *seed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: generate failed: %v\n", it.id, err)
			failed++
			continue
		}
		res, dropped, err := llm.Build(content, body, it.id, *seed, *choices, *maxBlanks)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: invalid model output: %v\n", it.id, err)
			failed++
			continue
		}
		for _, d := range dropped {
			fmt.Fprintf(os.Stderr, "%s: dropped %s\n", it.id, d)
		}
		if err := llm.Save(*cacheDir, &llm.Entry{
			IssueNumber:   it.issueNumber,
			BodyHash:      llm.BodyHash(body),
			Model:         *model,
			PromptVersion: llm.PromptVersion,
			Summary:       res.Summary,
			Quizzes:       res.Quizzes,
		}); err != nil {
			return fmt.Errorf("%s: save cache: %w", it.id, err)
		}
		generated++
		fmt.Fprintf(os.Stderr, "%s: %d quiz(zes) cached\n", it.id, len(res.Quizzes))
	}
	fmt.Fprintf(os.Stderr, "llm-generate: %d generated, %d up-to-date, %d failed (cache: %s)\n",
		generated, upToDate, failed, *cacheDir)
	if failed > 0 {
		return fmt.Errorf("%d proposal(s) failed", failed)
	}
	return nil
}

// newGenerator builds the LLM generator for the chosen provider. workers-ai
// reads its token from the CLOUDFLARE_API_TOKEN environment variable (never a
// flag) and its account ID from --cf-account-id or CLOUDFLARE_ACCOUNT_ID.
func newGenerator(provider, model, ollamaURL, cfAccountID string) (llm.Generator, error) {
	switch provider {
	case "ollama":
		return llm.NewOllamaClient(ollamaURL, model), nil
	case "workers-ai":
		acct := cfAccountID
		if acct == "" {
			acct = os.Getenv("CLOUDFLARE_ACCOUNT_ID")
		}
		if acct == "" {
			return nil, fmt.Errorf("provider workers-ai requires --cf-account-id or CLOUDFLARE_ACCOUNT_ID")
		}
		token := os.Getenv("CLOUDFLARE_API_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("provider workers-ai requires the CLOUDFLARE_API_TOKEN environment variable")
		}
		return llm.NewOpenAIClient(llm.WorkersAIBaseURL(acct), token, model), nil
	default:
		return nil, fmt.Errorf("--provider %q: want ollama or workers-ai", provider)
	}
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
