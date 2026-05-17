# 要件仕様: go-masked-quiz MVP

ステージ1 / 4 — 仕様駆動開発

---

## 1. 背景と目的

Go 言語の design proposals (`golang/proposal` の `design/*.md`) は、言語が
なぜ今の姿になったかを語る一次資料だが、量が多く読解の入り口が高い。

本プロジェクトは、proposal 内のキーワードや Go コード片を 4 択穴埋めクイズに
変換することで、能動的に proposal を読む動機づけと、Go の言語仕様への理解の
深まりを提供する。参考にした `fummicc1/se-masked-quiz` (Swift Evolution 版)
を Go 向けに移植したコンセプトプロダクト。

## 2. スコープ

### 2.1 In Scope (MVP)

| 領域 | 含まれるもの |
|---|---|
| クイズ生成 CLI | `tools/quizgen` (Go 製)。proposal Markdown を読み JSON を出力 |
| iOS アプリ | SwiftUI、iOS 17+。バンドル済み JSON を完全オフライン再生 |
| ライセンス遵守 | BSD 3-Clause 表示の 3 層化 (NOTICE / JSON メタ / Acknowledgments) |
| マスキング | 機械的 (goldmark + go/parser)。決定論的 RNG |

### 2.2 Out of Scope (将来)

- LLM 動的クイズ生成 (参考リポジトリの `LLMQuiz` 相当)
- バックエンドサーバー / クイズデータの API 配信
- Android、Web 等 iOS 以外のクライアント
- マルチ言語 UI (MVP は英語のみ)
- ユーザー間スコア共有 / ランキング
- App Store / TestFlight 配布手続き

## 3. ステークホルダー

| 役割 | 関心 |
|---|---|
| エンドユーザー (Go プログラマ・学習者) | クイズを通じて proposal を効率よく学びたい |
| プロジェクトオーナー (Fumiya Tanaka) | MVP を最小コストで完成させ、後続改善の土台にする |
| 上流著作権者 (The Go Authors) | BSD 3-Clause 表示義務の遵守 |

## 4. ユーザーストーリー

### US1: プロポーザル選択

> Go プログラマとして、proposal の一覧を眺めて気になるテーマを選びたい。
> その proposal のクイズの問題数と進捗を一目で把握したい。

### US2: クイズ実行

> 選んだ proposal について、文脈付きの 4 択穴埋めクイズを順番に解きたい。
> 解いた直後に正解と簡単な解説（最低でも正答）を見たい。

### US3: 結果確認

> セッション終了時に、自分の正答率と間違えた問題を確認したい。

### US4: 出典の確認

> アプリ内で出典 (golang/proposal、BSD 3-Clause) を確認したい。

### US5: 開発者として再現可能な生成

> 開発者として、ある時点の golang/proposal から同じ seed で生成した JSON が
> 完全に同一のバイト列であることを期待する (CI 差分レビューが容易)。

## 5. 機能要件 (FR)

### FR1. クイズ生成 CLI (`quizgen`)

| ID | 要件 |
|---|---|
| FR1.1 | サブコマンド `generate` を持つ |
| FR1.2 | `--proposals <dir>` で `design/*.md` のディレクトリを指定 (必須) |
| FR1.3 | `--out <path>` で出力 JSON パス (既定 `output/quizzes.json`) |
| FR1.4 | `--seed <int64>` で決定論的 RNG シード (既定 42) |
| FR1.5 | `--commit <sha>` で上流 commit SHA を JSON メタに記録 (任意) |
| FR1.6 | `--max-per-proposal <n>` で proposal あたり最大件数 (既定 5) |
| FR1.7 | `--choices <n>` で 1 問あたりの選択肢数 (既定 4) |
| FR1.8 | `*.md` を goldmark で AST 化し、`` ` ` `` inline code と ` ```go ` 両方のスパンを位置情報付きで抽出 |
| FR1.9 | ` ```go ` ブロックは `go/parser` の寛容モードで AST 化し、`FuncDecl` / `TypeSpec` / `CallExpr` から識別子を抽出 |
| FR1.10 | 構文不正な Go スニペットは `package _x` で包むフォールバック後もダメなら**スキップ**し、CLI 全体は失敗させない |
| FR1.11 | 4 択は (正答, 同 proposal 内類似トークン, 横断プール, Go 予約語) のミックスで生成。ケース非感応で重複排除 |
| FR1.12 | proposal 0 件 / コードブロック 0 件 / inline code 0 件 のいずれでも panic せずに空配列を返す |
| FR1.13 | 出力 JSON は `version`, `generated_at`, `source_repo`, `source_fork`, `source_commit`, `source_license`, `source_license_url`, `proposals[]` をトップレベルに含む |
| FR1.14 | 各 quiz は `id`, `kind ("prose"\|"code")`, `index`, `context_before`, `masked_text ("____")`, `context_after`, `answer`, `choices[]` を持つ |

### FR2. iOS アプリ (`GoMaskedQuiz`)

| ID | 要件 |
|---|---|
| FR2.1 | バンドル内 `quizzes.json` を起動時に読み込み、`QuizBundle` 型にデコード |
| FR2.2 | Proposal 一覧画面: タイトル + 問題数を表示し、進捗バッジ (正解数 / 全問数) を表示 |
| FR2.3 | Proposal を選択するとクイズ実行画面に遷移 |
| FR2.4 | クイズ実行画面: `context_before` + `____` + `context_after` を表示。コード問題は等幅フォント |
| FR2.5 | 4 つの選択肢ボタンが presented される。タップで即時に正誤判定し、正解を視覚的に示す |
| FR2.6 | 全問終了後に結果サマリー画面へ遷移。正答率 + 各問の正解/不正解一覧 |
| FR2.7 | Acknowledgments 画面: NOTICE 相当のテキストと上流リポジトリへのリンク |
| FR2.8 | 進捗保存: クイズ ID → 正解実績の boolean を UserDefaults に永続化 |
| FR2.9 | 機内モード (完全オフライン) で全機能が動作する |

### FR3. ライセンスコンプライアンス

| ID | 要件 |
|---|---|
| FR3.1 | リポジトリルートに `NOTICE` を置き、上流 (The Go Authors, golang/proposal) と fork (fummicc1/golang-proposal) の著作権・BSD 3-Clause を明記 |
| FR3.2 | `output/quizzes.json` のメタに `source_repo`, `source_fork`, `source_commit`, `source_license`, `source_license_url` の 5 種を必ず含む |
| FR3.3 | iOS Acknowledgments 画面に上流の著作権表示 + BSD 3-Clause 全文 URL を含む |
| FR3.4 | アプリ説明文・宣伝文で "The Go Authors" の名称を販促目的で使用しない (BSD 第 3 条) |

### FR4. 開発者向けワークフロー

| ID | 要件 |
|---|---|
| FR4.1 | `git clone` → `go run ./cmd/quizgen generate --proposals <local>/design` で JSON が生成できる |
| FR4.2 | `golang-proposal` の clone はリポジトリに含めない (`.gitignore` で `third_party/` 除外)。理由: iCloud Drive 配下のため submodule の checkout が File Provider 干渉で決定論的に失敗するため |
| FR4.3 | `go test ./...` が緑になる |

## 6. 非機能要件 (NFR)

| ID | カテゴリ | 要件 |
|---|---|---|
| NFR1.1 | 性能 (CLI) | 100 proposals (約 5MB の Markdown) の処理を 10 秒以内に完了 |
| NFR1.2 | 性能 (iOS) | アプリ起動から一覧表示まで 2 秒以内、画面遷移 200ms 以内 |
| NFR2.1 | 再現性 | 同一 `(seed, source files, max-per-proposal, choices)` の生成結果は `generated_at` を除き完全に同一バイト列 |
| NFR3.1 | オフライン | iOS アプリは初回起動以降ネットワーク不要 |
| NFR4.1 | 保守性 | Go モジュールはパッケージ分離 (`parser`, `masker`, `quiz`, `cmd/quizgen`) |
| NFR4.2 | 保守性 | 主要パッケージ (`parser`, `masker`) にユニットテストがある |
| NFR4.3 | 保守性 | iOS アプリは feature 単位ディレクトリで構造化 (`Features/ProposalList`, `Features/QuizSession`, ...) |
| NFR5.1 | ポータビリティ (CLI) | macOS / Linux / Windows で動作 (標準ライブラリ + goldmark のみ) |
| NFR5.2 | ポータビリティ (iOS) | iOS 17+。Swift 6 strict concurrency 警告なし |
| NFR6.1 | プライバシー | 個人情報・トラッキング・テレメトリは収集しない (MVP) |
| NFR7.1 | ローカル開発環境 | iCloud Drive 配下のリポジトリで `go build` / `go test` が動作する (Go ビルドキャッシュは iCloud 外を使う) |

## 7. ライセンス・法的制約

- 上流 `golang/proposal`: BSD 3-Clause、Copyright (c) The Go Authors
- 自リポジトリ `LICENSE`: BSD 3-Clause、Copyright (c) 2026 Fumiya Tanaka
- 派生物 (`output/quizzes.json`、iOS バンドル内のクイズデータ) は上流 BSD 3-Clause を継承し、3 層 (NOTICE / JSON メタ / iOS Acknowledgments) で再配布時の表示義務を満たす

## 8. 受け入れ基準 (Acceptance Criteria)

| AC | 内容 | 検証手段 |
|---|---|---|
| AC1 | CLI が testdata から決定論的に JSON を生成 | `go run ... --seed 42` を 2 回実行、`generated_at` を除き完全一致 |
| AC2 | 実 `golang-proposal/design/*.md` から MVP として 100 問以上のクイズを生成 | 数百 KB の `quizzes.json` を目視確認 + 件数チェック |
| AC3 | iOS アプリで proposal → クイズ → 結果の遷移が完走 | シミュレータでゴールデンパス手動確認 |
| AC4 | 機内モードで iOS が全機能動作 | シミュレータの Airplane Mode で全画面確認 |
| AC5 | NOTICE / JSON メタ / iOS Acknowledgments の 3 層に表示が存在 | grep / 目視 |
| AC6 | `go vet ./...` / `go test ./...` 緑 | CI もしくはローカル実行 |
| AC7 | クイズ JSON が規定スキーマを満たす | JSON Schema 検証または Go decode 成功 |
| AC8 | iOS ビルドが Swift 6 strict concurrency で警告なし | Xcode ビルドログ |

## 9. 仮定 (Assumptions)

- A1. 開発者は手元の iCloud 外パスに `fummicc1/golang-proposal` を `git clone` できる
- A2. 開発者は Go 1.26 以上、Xcode 26 以上の環境を持つ
- A3. proposal Markdown の構造 (見出し、` ```go ` フェンス、inline code の規則) は短期的に大きく変わらない
- A4. iOS シミュレータでの動作を最低限の動作確認とする (実機検証は MVP 後)

## 10. リスクと緩和策

| ID | リスク | 影響 | 緩和策 |
|---|---|---|---|
| R1 | golang-proposal の Markdown 構造変化で parser が壊れる | クイズ品質低下 | goldmark の標準 AST を使い、構造依存を最小化。テストデータをスナップショット化 |
| R2 | Go コードスニペットの構文が parser でパース不能 | code クイズの欠落 | `parser.SkipObjectResolution` + `package _x` ラップでフォールバック。失敗ブロックはスキップ (FR1.10) |
| R3 | iCloud Drive のファイル同期遅延でビルド/git 操作が不安定 | 開発体験悪化 | `third_party/` を `.gitignore`、submodule を使わない (FR4.2)。`GOCACHE` を iCloud 外に設定 |
| R4 | BSD 3-Clause 表示の漏れによる法的リスク | ライセンス違反 | 3 層表示を AC5 で検証し、リリース前チェックリストに組み込む |
| R5 | クイズの難易度バランスが偏る | 学習効果低下 | `--max-per-proposal` と prose:code = 3:2 の目標比で平準化。Phase 4 以降で難易度ラベルを検討 |
| R6 | LLM 拡張 (Phase 5) で JSON スキーマが破壊的変更になる | iOS 旧版との非互換 | スキーマに `version` を含め、`kind` を enum で拡張可能に。`kind: "llm"` を将来追加 |

## 11. 用語集

| 用語 | 定義 |
|---|---|
| proposal | `golang/proposal/design/NNNN-<slug>.md` の Markdown 文書 |
| prose クイズ | proposal の本文中 inline code (`` ` `` で囲まれたトークン) をマスクしたクイズ |
| code クイズ | proposal の `` ```go `` ブロック内の関数名/型名/呼び出し先をマスクしたクイズ |
| 派生物 (derived work) | proposal 本文断片を含む `quizzes.json` および iOS バンドル内クイズデータ |
| seed | `quizgen` の `--seed` フラグ。決定論的 RNG の初期値 |
| tag | `masker.NewRNG(seed, tag)` の文字列引数。proposal slug + 処理段階で分岐させる |
