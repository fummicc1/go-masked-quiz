# go-masked-quiz

Fill-in-the-blank quiz over the Go language design proposals.
Inspired by [fummicc1/se-masked-quiz](https://github.com/fummicc1/se-masked-quiz)
(the Swift Evolution version).

The project consists of:

- **`quizgen/`** — a Go CLI that reads Go proposals (Markdown), masks
  identifiers in both prose and ` ```go ` code blocks, and emits a single
  `output/quizzes.json` consumed by the iOS app.
- **`ios/GoMaskedQuiz/`** — a SwiftUI iOS app that ships `quizzes.json` in its
  resource bundle and lets the user play the quizzes fully offline.

The proposals themselves come from
[fummicc1/golang-proposal](https://github.com/fummicc1/golang-proposal),
which is a fork of [golang/proposal](https://github.com/golang/proposal).

## Getting the sources

```sh
git clone git@github.com:fummicc1/go-masked-quiz.git
cd go-masked-quiz
```

### Cloning the Go proposals locally

The proposals fork is **not bundled as a Git submodule** (this repository
lives in iCloud Drive and the rapid file writes during a submodule checkout
fail under iCloud File Provider). Clone it to any local path **outside
iCloud Drive**, then point `quizgen` at it:

```sh
# Example: alongside the project
git clone git@github.com:fummicc1/golang-proposal.git ~/Work/LocalApps/golang-proposal
```

You may also clone it into `./third_party/golang-proposal` of this repo;
that path is `.gitignore`d and will not be committed.

## Building the quiz data (Go CLI)

```sh
cd quizgen
go run ./cmd/quizgen generate \
  --proposals ~/Work/LocalApps/golang-proposal/design \
  --out       ../../output/quizzes.json \
  --seed 42
```

Use `--commit <sha>` to record the upstream commit of the proposals fork in
the JSON metadata for reproducibility:

```sh
go run ./cmd/quizgen generate \
  --proposals ~/Work/LocalApps/golang-proposal/design \
  --out       ../../output/quizzes.json \
  --commit    "$(git -C ~/Work/LocalApps/golang-proposal rev-parse HEAD)" \
  --seed 42
```

### Building from golang/go proposal issues

Most Go proposals live as issues on the `golang/go` tracker (the `Proposal` /
`Proposal-Accepted` labels), not as design docs. Generate quizzes directly from
those issues with `--source github-issues`. This needs a GitHub token (a
classic or fine-grained PAT with public read is enough) in `GITHUB_TOKEN`:

```sh
export GITHUB_TOKEN=ghp_...
go run ./cmd/quizgen generate \
  --source        github-issues \
  --out           ../../output/quizzes.json \
  --max-proposals 200 \
  --seed 42
```

By default it pulls accepted proposals, freshest first
(`repo:golang/go label:Proposal-Accepted sort:updated-desc`). Override the
selection with `--query`, e.g. to include open proposals:

```sh
go run ./cmd/quizgen generate --source github-issues \
  --query 'repo:golang/go label:Proposal sort:updated-desc' \
  --max-proposals 100 --seed 42
```

### Combining both sources

`--source` accepts a comma-separated list, so one run can merge design docs and
issues into a single bundle (distractors are pooled across both):

```sh
export GITHUB_TOKEN=ghp_...
go run ./cmd/quizgen generate \
  --source        design-docs,github-issues \
  --proposals     ~/Work/LocalApps/golang-proposal/design \
  --max-proposals 200 \
  --out           ../../cdn/v3/quizzes.json \
  --seed 42
```

A multi-source bundle adds a `sources[]` array describing each upstream for
attribution. Single-source bundles omit it and stay byte-identical to before
(the schema version is still `3`; v3 clients ignore the extra field).

## Automated refresh (CDN)

`cdn/v3/quizzes.json` is refreshed by the
[`generate-quizzes`](.github/workflows/generate.yml) GitHub Actions workflow in
this repo: daily (and on demand) it clones `golang/proposal` upstream, fetches
golang/go proposal issues, generates the **merged** bundle, and commits only
when the content (ignoring `generated_at`) actually changes.

This workflow is the **single writer** of `cdn/v3/quizzes.json`. Do not add
another workflow — in this repo or any fork — that also writes that file;
concurrent writers would race. (The legacy `generate.yml` in the
`fummicc1/golang-proposal` fork is superseded by this one and should be
disabled.)

## Running the iOS app

Open `ios/GoMaskedQuiz/GoMaskedQuiz.xcodeproj` in Xcode and run the
`GoMaskedQuiz` scheme on an iOS 17+ simulator. The build phase will pick up
the latest `output/quizzes.json`.

## License

- `go-masked-quiz` itself: BSD 3-Clause, Copyright (c) 2026 Fumiya Tanaka.
  See [`LICENSE`](./LICENSE).
- Generated `output/quizzes.json` and the iOS bundle contain short fragments
  derived from [`golang/proposal`](https://github.com/golang/proposal),
  Copyright (c) The Go Authors, licensed under BSD 3-Clause.
  See [`NOTICE`](./NOTICE) for the required attribution that downstream
  redistributors must preserve.

## Acknowledgments

This project would not exist without the work of:

- The Go Authors — the design proposals at
  [golang/proposal](https://github.com/golang/proposal).
- [fummicc1/se-masked-quiz](https://github.com/fummicc1/se-masked-quiz) —
  the original Swift Evolution quiz project that inspired the design.
