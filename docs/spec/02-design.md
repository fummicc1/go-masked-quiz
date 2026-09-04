# 設計仕様: go-masked-quiz MVP

ステージ2 / 4 — 仕様駆動開発。前提は `docs/spec/01-requirements.md`。

> **注記 (実装後の追記)**: 本ドキュメントは策定当時「iOS 単体アプリ (SwiftUI) + Cloudflare Pages」を前提に書かれた。実装は Gio によるクロスプラットフォームアプリ (`mobile/`) と jsDelivr 配信に置き換わっており、以下の全体図・データモデルのクライアント側記述・4章・5章・9章の一部はこの実態に合わせて改訂している。CLI 側の設計 (3章) はほぼ当初の想定のまま実装された。

---

## 1. システム全体図

```
┌────────────────────────────┐   git push    ┌──────────────────────┐
│ 開発者ローカル              │ ─────────────→ │ GitHub               │
│  - golang-proposal clone   │                │  fummicc1/           │
│    (iCloud 外、任意パス)    │                │   go-masked-quiz     │
│  - quizgen CLI              │                │  (main / spec/*)     │
│  - output/quizzes.json 生成 │                │  - cdn/v2/           │
└────────────────────────────┘                 │      quizzes.json    │
                                                └──────┬───────────────┘
                                                       │ jsDelivr が
                                                       │ GitHub 上のファイルを
                                                       │ そのまま配信 (デプロイ不要)
                                                       ▼
                                            ┌────────────────────┐
                                            │ jsDelivr (GH CDN)  │
                                            │  cdn.jsdelivr.net/  │
                                            │  gh/.../cdn/v2/     │
                                            │      quizzes.json   │
                                            └────────┬───────────┘
                                                     │ HTTPS GET (毎起動)
                                                     ▼
                                            ┌────────────────────┐
                                            │ mobile/ (Go + Gio) │
                                            │  - net/http        │
                                            │  - app.DataDir()   │
                                            │  - styledtext      │
                                            │    (Block Renderer)│
                                            └────────────────────┘
```

3 層構成:

| 層 | 責務 | 主要技術 |
|---|---|---|
| **Generation** | proposal Markdown → Block 列 JSON v2 | Go 1.26、goldmark、go/parser、math/rand/v2 |
| **Distribution** | JSON を CDN 配信 | jsDelivr (GitHub CDN 機能)。ヘッダ制御・デプロイ手順は持たない |
| **Presentation** | フェッチ・キャッシュ・描画・インタラクション | Go + [Gio](https://gioui.org)、`net/http`、`gioui.org/x/styledtext`、自前 JSON ファイル (`scores.json`) |

---

## 2. データモデル

### 2.1 JSON Schema v2 (informal)

```jsonc
{
  "version": 2,                          // schema version (integer)
  "generated_at": "2026-05-18T00:00:00Z",
  "source_repo": "https://github.com/golang/proposal",
  "source_fork": "https://github.com/fummicc1/golang-proposal",
  "source_commit": "abc123...",          // 任意 (空文字なら omitempty)
  "source_license": "BSD-3-Clause",
  "source_license_url": "https://go.googlesource.com/proposal/+/refs/heads/master/LICENSE",
  "proposals": [
    {
      "id": "design-43651-type-parameters",
      "title": "Type Parameters Proposal",
      "url": "https://github.com/golang/proposal/blob/master/design/43651-type-parameters.md",
      "quizzes": [
        {
          "id": "design-43651-type-parameters-q01",
          "kind": "prose",               // "prose" | "code"
          "index": 0,
          "blocks": [
            { "type": "text",        "value": "Generics in Go use " },
            { "type": "inline_code", "value": "type" },
            { "type": "text",        "value": " parameters on functions and types like " },
            { "type": "mask" },
            { "type": "text",        "value": ", which is a new construct." }
          ],
          "answer": "comparable",
          "choices": ["comparable", "any", "ordered", "constraints"]
        }
      ]
    }
  ]
}
```

不変則 (Invariants):

- `version === 2`
- 各 `quiz.blocks` には **ちょうど 1 つ** の `{type: "mask"}` を含む
- `prose` クイズの blocks: `text` / `inline_code` / `mask` のみ
- `code` クイズの blocks: `code_block` / `mask` のみ
- `mask` ブロックは `value` を持たない (JSON 上は省略)
- `choices` は `answer` を必ず含む。要素数 = `--choices` (既定 4)
- `choices` 内に重複なし (lowercase 比較)

### 2.2 Go 型定義 (`internal/quiz/model.go`)

```go
package quiz

import "time"

type Bundle struct {
    Version          int        `json:"version"`
    GeneratedAt      time.Time  `json:"generated_at"`
    SourceRepo       string     `json:"source_repo"`
    SourceFork       string     `json:"source_fork"`
    SourceCommit     string     `json:"source_commit,omitempty"`
    SourceLicense    string     `json:"source_license"`
    SourceLicenseURL string     `json:"source_license_url"`
    Proposals        []Proposal `json:"proposals"`
}

type Proposal struct {
    ID      string `json:"id"`
    Title   string `json:"title"`
    URL     string `json:"url"`
    Quizzes []Quiz `json:"quizzes"`
}

type Quiz struct {
    ID      string   `json:"id"`
    Kind    Kind     `json:"kind"`
    Index   int      `json:"index"`
    Blocks  []Block  `json:"blocks"`
    Answer  string   `json:"answer"`
    Choices []string `json:"choices"`
}

type Kind string
const (
    KindProse Kind = "prose"
    KindCode  Kind = "code"
)

// Block は判別共用体的に使う。Value は Type が "mask" の場合空文字。
type Block struct {
    Type  BlockType `json:"type"`
    Value string    `json:"value,omitempty"`
}

type BlockType string
const (
    BlockText       BlockType = "text"
    BlockInlineCode BlockType = "inline_code"
    BlockCodeBlock  BlockType = "code_block"
    BlockMask       BlockType = "mask"
)
```

### 2.3 クライアント側の型 (`mobile/`)

クライアントは Swift Codable ではなく、CLI と**同じ Go の型定義を直接 import** する
(`github.com/fummicc1/go-masked-quiz/quizgen/quiz`)。generator と client で型を
手書き同期する必要がないのは、この 2.2 節の Go 型がそのまま両者の共有スキーマに
なっているため。デコードも `encoding/json` の `json.Unmarshal` を素朴に呼ぶだけで、
`keyDecodingStrategy` や `dateDecodingStrategy` に相当する設定は不要 (フィールドタグで
snake_case ↔ フィールド名を直接マッピングしている)。

```go
// mobile/data.go
func decodeBundle(raw []byte) (quiz.Bundle, error) {
    var b quiz.Bundle
    if err := json.Unmarshal(raw, &b); err != nil {
        return quiz.Bundle{}, fmt.Errorf("decode bundle: %w", err)
    }
    if b.Version != quiz.SchemaVersion {
        return quiz.Bundle{}, fmt.Errorf("decode bundle: unsupported schema version %d", b.Version)
    }
    return b, nil
}
```

版チェックは 2.1 節のような許容レンジではなく、`quiz.SchemaVersion` (現在は `2`) との
**厳密一致**のみ。なお `quizgen/quiz/model.go` の実際のフィールド構成は、LLM 由来の
質問 (`Quiz.GenMethod` / `Quiz.Difficulty`)・複数アップストリーム対応 (`Bundle.Sources`)
など、上の 2.1/2.2 節の記述時点から追加された項目を含む。スキーマそのものの節 (2.1/2.2)
の全面更新は本改訂の対象外としたので、最新のフィールドは `quizgen/quiz/model.go` を都度
確認すること。

---

## 3. Go CLI 設計 (`quizgen`)

### 3.1 パッケージ構成

```
quizgen/
├── cmd/quizgen/main.go           # CLI エントリ
├── internal/
│   ├── parser/                   # 既存。Markdown 解析 (goldmark)
│   │   ├── proposal.go
│   │   └── codeblock.go
│   ├── masker/                   # 既存。Seed 選定、4 択生成
│   │   ├── rand.go
│   │   ├── prose.go              # CollectProseSeeds (Seed の選定のみ)
│   │   ├── code.go               # CollectCodeSeeds
│   │   └── candidates.go
│   ├── blocks/                   # NEW: Block 列の生成
│   │   ├── prose.go              # ProseSeed + Proposal → []Block
│   │   ├── code.go               # CodeSeed + Proposal → []Block
│   │   └── window.go             # 周辺 N 文字の切り出しヘルパ
│   ├── quiz/
│   │   └── model.go              # v2 スキーマに改定
│   └── output/
│       └── writer.go
└── testdata/proposals/*.md
```

### 3.2 Block 分解アルゴリズム

#### 3.2.1 prose クイズ

入力:
- `proposal.Source` (全文 Markdown)
- `proposal.InlineCodes []InlineCode` (全 inline code のオフセット情報)
- `seed ProseSeed` (マスク対象 inline code の `Start`, `End`, `Answer`)

手順:
1. 周辺ウィンドウ `[wStart, wEnd]` を決定: `seed.Start - W`, `seed.End + W` (W = 80 既定)
2. 行境界に揃える: `wStart` は直前の `\n` 直後、`wEnd` は直後の `\n` 直前 (現状の `stripFromLastNL` / `stripAtFirstNL` 相当)
3. 窓内に位置する `InlineCode` 群を、`seed` を除き昇順に取得
4. 窓を `[wStart, ...inlineCode positions..., seed.Start, seed.End, ...inlineCode positions..., wEnd]` でセグメント分割
5. 各セグメントを `text` / `inline_code` ブロックに変換し、`seed` 範囲は `mask` に置換

```
Window: "real clone of `golang/proposal` is available"
                       ^^^^^^^^^^^^^^^^ seed (inline code 1 つ)

→ Blocks:
[
  { type: "text",        value: "real clone of " },
  { type: "mask" },
  { type: "text",        value: " is available" }
]
```

別 inline code が窓内にある場合:

```
Window: "the `quizgen` CLI parses `proposals` like this one"
              seed: `quizgen`

→ Blocks:
[
  { type: "text",        value: "the " },
  { type: "mask" },
  { type: "text",        value: " CLI parses " },
  { type: "inline_code", value: "proposals" },
  { type: "text",        value: " like this one" }
]
```

#### 3.2.2 code クイズ

入力:
- `proposal.CodeBlocks[bi].Source` (該当コードブロックの生ソース)
- `seed CodeSeed` (`BlockIndex`, `Start`, `End`, `Answer`)

手順:
1. 周辺ウィンドウ `[wStart, wEnd]` を決定 (`W` = 120 既定)
2. ウィンドウ内をそのまま 2 つの `code_block` セグメントに分割
   - `code_block(before)` + `mask` + `code_block(after)`

```
Block source: "package main\n\nfunc main() {\n    fmt.Println(\"hi\")\n}\n"
                                                   ^^^^^^^ seed

→ Blocks:
[
  { type: "code_block", value: "package main\n\nfunc main() {\n    fmt." },
  { type: "mask" },
  { type: "code_block", value: "(\"hi\")\n}\n" }
]
```

#### 3.2.3 セグメント結合と空文字最適化

- 連続する同型ブロックは結合 (例: `text` が 2 つ並ぶことはない)
- `value` が空文字のブロックは出力しない
- 先頭・末尾の空白は保持 (文脈として必要)

### 3.3 CLI フラグの追加・変更

| フラグ | 既存 | 変更 |
|---|---|---|
| `--proposals` | ✓ | 変更なし |
| `--out` | ✓ | 変更なし |
| `--seed` | ✓ | 変更なし |
| `--commit` | ✓ | 変更なし |
| `--max-per-proposal` | ✓ | 変更なし |
| `--choices` | ✓ | 変更なし |
| `--context-prose` | NEW | prose クイズの周辺窓サイズ (既定 80) |
| `--context-code` | NEW | code クイズの周辺窓サイズ (既定 120) |

### 3.4 Phase 2 (v1) → Phase 2.5 (v2) 移行

現状 (ローカル未コミット) は v1 スキーマで `BuildProseQuiz` / `BuildCodeQuiz` が `context_before/masked_text/context_after` を返す。これを以下の手順で移行する:

1. `internal/quiz/model.go` の `Quiz` から `ContextBefore` / `MaskedText` / `ContextAfter` を削除、`Blocks []Block` を追加
2. `internal/blocks/{prose,code,window}.go` を新規実装
3. `internal/masker/prose.go` の `BuildProseQuiz` は削除し、シードのみ返すように。代わりに `blocks.BuildProseQuiz(prop, seed, choices, id, idx)` を呼ぶ
4. 同様に `internal/masker/code.go` の `BuildCodeQuiz` を `blocks.BuildCodeQuiz` に移譲
5. `cmd/quizgen/main.go` の `buildQuizzes` を新 API に追従
6. `internal/quiz/model.go` のテストを v2 スキーマ前提に更新
7. `internal/parser/proposal_test.go` は変更不要
8. `internal/masker/*_test.go` も大幅変更不要 (シード生成のロジックは維持)
9. 新規 `internal/blocks/*_test.go` で text/inline_code/code_block/mask の組み合わせを検証

### 3.5 決定論性の保証

- 同一 `(seed, --max-per-proposal, --choices, source files)` → `generated_at` を除く JSON が完全一致
- Block 分解はオフセットベースで純粋関数 (RNG 不使用)
- `proposal` の処理順は ファイル名 ascending (`sort.Strings`)
- `proposals[]` 内の `quizzes[]` の順は prose 群 → code 群、各群内は `masker.Collect*Seeds` のシャッフル結果順 (同 seed なら同順)

---

## 4. CDN 配信層 (jsDelivr)

### 4.1 リポジトリ構成

```
/cdn/
├── v1/
│   └── quizzes.json                # frozen; schema v1, もう更新されない
└── v2/
    └── quizzes.json                # 公開する v2 JSON。generate ワークフローが書く
```

`_headers` や `robots.txt` は存在しない。jsDelivr はリポジトリのファイルをそのまま返す
だけで、Cloudflare Pages のようなヘッダのカスタマイズ層を持たないため、そうした設定
ファイル自体が不要になった。

### 4.2 配信ヘッダ

jsDelivr が既定で `Content-Type` / `Cache-Control` / ETag / 圧縮を付与する。
Cloudflare Pages 版で想定していた `_headers` によるカスタム値の強制は**できない** —
jsDelivr 側の既定に委ねる設計になっている。

### 4.3 デプロイフロー

デプロイという概念自体がない。`cdn/v2/quizzes.json` を `main` ブランチにコミットして
`git push` すれば、jsDelivr が (キャッシュ更新後に) 新しい内容を返す:

```sh
# 開発者ローカルで
go run ./quizgen/cmd/quizgen generate \
    --proposals ~/Work/LocalApps/golang-proposal/design \
    --out       ./output/quizzes.json \
    --commit    "$(git -C ~/Work/LocalApps/golang-proposal rev-parse HEAD)" \
    --seed      42

cp ./output/quizzes.json ./cdn/v2/quizzes.json
git add cdn/v2/quizzes.json output/quizzes.json
git commit -m "data: refresh quizzes.json"
git push
```

実際には手動実行ではなく `.github/workflows/generate.yml` が日次でこのフローを
自動実行し、内容 (`generated_at` を除く) に差分があるときだけコミットする。この
ワークフローが `cdn/v2/quizzes.json` の**単一の書き手**であることをワークフロー内の
コメントで明示しており、別のワークフローが同じファイルに書き込むと競合する。

### 4.4 配信 URL とアプリ側設定

```
https://cdn.jsdelivr.net/gh/fummicc1/go-masked-quiz@main/cdn/v2/quizzes.json
```

`mobile/data.go` にハードコードされている:

```go
// quizDataURL serves the published bundle straight from the repo's cdn/ via
// jsDelivr, so there is no server to run.
const quizDataURL = "https://cdn.jsdelivr.net/gh/fummicc1/go-masked-quiz@main/cdn/v2/quizzes.json"
```

`@main` のようなブランチ参照は jsDelivr 側で比較的短いキャッシュ寿命になる (タグ参照
より更新が反映されやすい代わりに、配信が完全に即時ではない)。dev / prod の切り替えや
カスタムドメインは持たない。

### 4.5 CDN 障害時の挙動 (再掲)

| ケース | 挙動 |
|---|---|
| キャッシュなしで CDN ダウン | エラー画面 + 再試行 (FR2.6) |
| キャッシュありで CDN ダウン | キャッシュで継続 (FR2.7)。"オフライン中" バナーのような専用UIはなく、画面下部の `source · cache` 表示のみ |
| version 不一致の JSON 取得 | キャッシュがあればキャッシュ継続、なければエラー画面 (FR2.19)。アプリ更新を促す専用モーダルはない |

---

## 5. モバイルアプリ設計 (`mobile/`, Go + Gio)

Gio は immediate-mode の UI ツールキットで、SwiftUI のようなビュー階層・
`@Observable`/actor は存在しない。レイアウト関数がフレームごとに丸ごと再実行され、
クリック状態などフレームを越えて残す必要のある状態だけが `widget.Clickable` 等の
struct として明示的に保持される。以下は当初想定した SwiftUI MVVM 構成に代わる、
実際の Go 実装の構造。

### 5.1 ファイル構成

```
mobile/
├── main.go       # エントリポイント、UI struct、フレームループ、loadState/screen
├── data.go       # フェッチ・デコード・キャッシュ (quizDataURL 等)
├── score.go      # 進捗の永続化 (scoreStore, scores.json)
├── ui_list.go     # Proposal 一覧画面
├── ui_doc.go      # ブロック描画の共通処理 (buildBlockView, layoutBlock)
├── ui_quiz.go     # ドキュメント読者画面と選択肢シート (docHeader, choiceSheet, sheetChoice)
├── ui_about.go    # Acknowledgments 相当の About 画面 (FR3.3)
├── widgets.go     # card / progressRail など見た目の共通部品
└── theme.go       # 配色・フォント設定
```

サブディレクトリはなく、すべて `package main` の 1 パッケージ。SwiftUI 版で想定していた
`Features/ProposalList`、`Services/QuizLoader`、`Features/About/AcknowledgmentsView.swift`
のような分割はない。Acknowledgments 画面は `ui_about.go` の `screenAbout` (`layoutAbout`)
として実装済み: 一覧画面右上の「About」ボタン (`ui_list.go`) から遷移し、`quiz.Bundle` の
`SourceRepo` / `SourceLicense` / `SourceLicenseURL` (または `Sources`) を読んで上流の著作権
表示を組み立て、BSD 3-Clause の条件・免責事項全文と合わせて表示する。ネットワークを使わず
ロード済みの `Bundle` から描画するため、機内モードでも動作する (FR2.18)。

### 5.2 状態遷移

#### 5.2.1 loadState と screen (アプリ全体)

```go
// main.go
type loadState int

const (
    loadLoading loadState = iota
    loadReady
    loadFailed
)

type screen int

const (
    screenList screen = iota
    screenQuiz
)
```

`LaunchPhase` に相当するのは `loadState`。`BundleSource`（fresh/cached の区別）に相当
するのは `Source`（`SourceRemote` / `SourceCache`、`data.go`）で、画面下部に
`source · remote` / `source · cache` として常時表示される。

遷移（想定していた `ready(cached)` → 背景フェッチ → `ready(fresh)` という
stale-while-revalidate 型の遷移は**採用していない**）:

```
[loadLoading]
    │ load() (毎起動 1 回)
    ├─→ remote 成功            → [loadReady, source=remote]
    ├─→ remote 失敗、cache 有  → [loadReady, source=cache]
    └─→ remote 失敗、cache 無  → [loadFailed]
                                  │ Retry タップ
                                  └─→ [loadLoading] (再試行)
```

version 不一致は「remote 失敗」と同じ扱いになる（`decodeBundle` がエラーを返すだけ
で、専用の `unsupportedVersion` 分岐や「アプリ更新を促すモーダル」はない）。

#### 5.2.2 blank の回答状態

`MaskState`（empty/preview/correct/incorrect の状態機械）に相当するものはない。
1 つの blank は「未回答」か「回答済み (`answer{Choice, Correct}`)」の 2 状態しか持たず、
中間状態 (preview) は存在しない — 選択肢をタップした時点で即座に確定する:

```go
// score.go
type answer struct {
    Choice  string `json:"choice"`
    Correct bool   `json:"correct"`
}
```

```
未回答 ──(選択肢シートで選択肢をタップ)──→ 回答済み(choice, correct)  // 即時確定、以後不可逆
```

`Submit` ボタン・プレビューの上書き・`Next` ボタンは存在しない。ユーザーは文書を
スクロールして次の blank に進むだけで、独立した「クイズ実行フェーズ」の管理も
（`QuizSessionViewModel.Phase` のような）状態機械も必要としない。

### 5.3 データレイヤ

#### 5.3.1 フェッチ + キャッシュ (`data.go`)

```go
func loadBundle(ctx context.Context, cachePath string) (quiz.Bundle, Source, error) {
    b, raw, err := fetchRemote(ctx)
    if err == nil {
        if cachePath != "" {
            _ = os.MkdirAll(filepath.Dir(cachePath), 0o755)
            _ = os.WriteFile(cachePath, raw, 0o644)
        }
        return b, SourceRemote, nil
    }
    if cachePath != "" {
        if cached, rerr := os.ReadFile(cachePath); rerr == nil {
            if b, derr := decodeBundle(cached); derr == nil {
                return b, SourceCache, nil
            }
        }
    }
    return quiz.Bundle{}, "", err
}
```

`QuizLoader`（ETag / `If-None-Match` / 304 対応）に相当する処理はない。`fetchRemote` は
10 秒タイムアウトで毎回フルボディを取得し、ボディは `io.LimitReader` で 32 MB
(`maxBundleBytes`) に制限する。キャッシュの読み書きに `actor` のような排他制御は
不要 — ロードはバックグラウンド goroutine 1 つだけが行い、結果は `pending`
（`main.go`）経由でUIゴルーチンに一度だけ受け渡される。

キャッシュの保存先は `QuizCache`（`Library/Caches/quizzes.json` + `UserDefaults` の
ETag）ではなく、Gio の `app.DataDir()` 配下の単一ファイル:

```go
func cacheFilePath() string {
    dir, err := app.DataDir()
    if err != nil {
        return ""
    }
    return filepath.Join(dir, "quizzes.json")
}
```

OS によってこのディレクトリが削除された場合、次回起動時は remote フェッチが
成功する限り問題にならず、失敗時のみ「キャッシュなし」として扱われる（5.2.1 参照）。

#### 5.3.2 進捗保存 (`score.go`)

`ProgressStore`（UserDefaults に `progress.<quizID>` を書く）に相当するのは
`scoreStore`。回答のたびに JSON ファイル (`scores.json`、`app.DataDir()` 配下) を
丸ごと書き直す（一時ファイル + `os.Rename` でアトミックに）:

```go
func (s *scoreStore) record(proposalID string, k blankKey, a answer) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.scores[proposalID] == nil {
        s.scores[proposalID] = map[blankKey]answer{}
    }
    s.scores[proposalID][k] = a
    s.flush()
}
```

`protocol ProgressStoring` のような抽象化はしていない — ファイル 1 個の
struct を直接使う。

### 5.4 レンダリングレイヤ (`ui_doc.go`)

`ProseRenderer` / `CodeRenderer` のような prose/code 別のビューはない。すべての
ブロックを `gioui.org/x/styledtext` で描画する 1 つの関数 (`buildBlockView`) が
担い、`mask` スパンだけ背景色つきの「チップ」として塗り分ける:

```go
case quiz.SpanMask:
    if s.BlankIndex == nil || *s.BlankIndex >= len(d.Blanks) {
        continue
    }
    bi := *s.BlankIndex
    label, fill := blankMarker(bi), colAccent
    if a, ok := answers[bi]; ok {
        label = d.Blanks[bi].Answer
        fill = colSuccess
        if !a.Correct {
            fill = colDanger
        }
    }
    add(styledtext.SpanStyle{
        Content: " " + label + " ", Size: unit.Sp(14), Color: colOnAccent,
        Font: font.Font{Typeface: "monospace", Weight: font.Bold},
    }, true, fill, bi)
```

未回答の blank は連番の丸数字 (`①②③...`) をラベルに表示し、回答済みになると
正解の文字列に置き換わる。AttributedString の背景色相当は Gio にはなく、
`layoutBlock`（`ui_doc.go`）がテキストを 2 パスで描画してチップの背景を先に塗る
（コメント参照: styledtext は各スパンの矩形をレイアウト後に返すため）。

Kind (`prose`/`code`) による分岐 (`QuizContentView`) の代わりに、ブロック単位の
`quiz.BlockCode` かどうかで見た目を分ける (`ui_quiz.go` `docBlock`)。

### 5.5 インタラクションレイヤ (`ui_quiz.go`)

`ChoiceButtonsView` + `Submit`/`Next` ボタンの代わりに、blank をタップすると
下からせり上がる選択肢シートが開き、選択肢をタップした時点で即時に回答が確定する:

```go
func (u *UI) sheetChoice(gtx layout.Context, th *material.Theme, v *docView, ci int, choice string, blank quiz.Blank) layout.Dimensions {
    c := v.choiceAt(choiceRef{Blank: v.open, Choice: ci})
    if c.Clicked(gtx) {
        a := answer{Choice: choice, Correct: choice == blank.Answer}
        v.answers[v.open] = a
        u.store.record(v.proposal.ID, blankKey{BlankIndex: v.open}, a)
        v.open = -1
        gtx.Execute(op.InvalidateCmd{})
        return layout.Dimensions{}
    }
    // ...
}
```

回答済みの blank にはタップ領域を張らない (`maskTargets`) ため、一度確定した回答は
UI 上やり直せない（`reset` ボタンで proposal 単位のリセットは可能）。

### 5.6 エラー UI (`ui_list.go`)

`ErrorView`（SF Symbols のアイコン付き）の代わりに、テキストと `Retry` ボタンだけの
簡素な画面 (`layoutLoadFailed`)。エラー本文はそのまま Go の `error.Error()` を表示する
（ユーザー向けの文言に変換していない）:

```go
func (u *UI) loadErrText() string {
    if u.loadErr == nil {
        return "No network connection."
    }
    return u.loadErr.Error()
}
```

---

## 6. アクセシビリティ設計

以下は SwiftUI 前提の想定であり、**Gio 実装 (`mobile/`) には対応する仕組みが一切ない**
(VoiceOver 相当の読み上げラベル、Dynamic Type 相当のフォントスケーリング、
Reduce Motion への追従、いずれも `mobile/*.go` に実装なし)。Gio 自体もアクセシビリティ
APIの提供が限定的で、この節は未着手の既知のギャップとして記録する。

| 対象 | 当初の想定 (SwiftUI) | 現状 (Gio) |
|---|---|---|
| VoiceOver | `accessibilityLabel` で blank の状態を読み上げ | 未対応 |
| Dynamic Type | システムフォントの Auto-scale | 未対応 (固定サイズの `unit.Sp`) |
| Color Contrast | 緑/赤 + SF Symbols アイコン併用 | 色のみ (アイコンなし) |
| Reduce Motion | `accessibilityReduceMotion` で無効化 | アニメーション自体を使っていない |
| Switch Control | 各 Button が独立した focusable element | 未対応 |

---

## 7. テスト戦略の方針 (Stage 3 で詳細化)

| 層 | 種別 | 主要対象 |
|---|---|---|
| Go | Unit | `parser`, `masker.candidates`, `masker.rand`, `blocks.{prose,code,window}` |
| Go | Golden | testdata/proposals/*.md → output JSON の snapshot |
| Go | Property-based | 4 択生成の不変則 (重複なし、answer 含む、サイズ) |
| Go (mobile) | Unit | `loadBundle` のフェッチ失敗時キャッシュ挙動、`filterProposals`、bundle デコード (`mobile/data_test.go`: `TestLoadBundleReportsFailure`, `TestLoadBundleUsesCacheWhenFetchFails`, `TestFilterProposals`, `TestFixtureIsCurrentSchema`) |
| Go (mobile) | UI Snapshot (headless GPU) | `gioui.org/gpu/headless` でウィンドウなしにレイアウト・ペイントを実行し、一覧・ドキュメント・選択肢シートのピクセル出力を得て検証 (`mobile/render_test.go`: `TestRenderList`, `TestRenderDocument`, `TestRenderChoiceSheet`, `TestChoiceSheetDismissWithoutSelection`)。2 パス描画のチップ背景バグはピクセル単位でしか見えないため、実レイアウト経路をそのまま通す |
| Manual | E2E | ネットワーク遮断、CDN (jsDelivr) 差し替えで更新検知 |

詳細は Stage 3 で。

---

## 8. 段階的実装計画

| Phase | スコープ | 状態 |
|---|---|---|
| 1 | リポジトリ骨子・LICENSE/NOTICE・CLI スケルトン | **完了** (`95b6590`) |
| 2 | 機械的マスキング v1 (`context_*`) | **完了** (その後 v2 の `Block[]`/`Document` スキーマへ移行) |
| 2.5 | v1 → v2 移行 (`Block[]` スキーマ) | **完了** |
| 3 | CDN 配信 (jsDelivr、`cdn/v2/quizzes.json`) | **完了**。当初想定の Cloudflare Pages / `_headers` / 手動デプロイからは方式転換 (4章参照) |
| 4 | モバイルアプリ MVP (Go + Gio): フェッチ / キャッシュ / Block 描画 / タップで即時回答 | **完了**。SwiftUI 前提だった Result 画面・Submit フローは当初想定と異なる形になった (5章参照)。Acknowledgments 画面は `ui_about.go` として実装済み (FR3.3) |
| 5 | 進捗保存 (`scores.json`) | **完了**。復習機能・進捗バッジは範囲外のまま |
| 6 | GitHub Actions による CDN 自動更新 (`generate.yml`) | **完了** |
| 7 | (Optional) LLM 拡張 | 部分的に着手 (`quiz.KindLLM` / `Quiz.GenMethod` 等スキーマの受け口はあるが、機能としての LLM クイズ生成は本書の対象外) |

---

## 9. セキュリティ設計

要件 NFR8.1-8.6 / AC18-21 / R11-R13 を満たすための具体的な設計指針。
脅威モデル: 機密性ゼロ・改ざん耐性中・可用性中、を前提とする。

### 9.1 通信層 (TLS)

- `quizDataURL` (`mobile/data.go`) は `https://` をハードコードしたリテラル定数。`http://` に切り替わる余地はコード上ない
- 通信は `net/http` の `http.DefaultClient` を使い、TLS 検証は Go 標準ライブラリの既定に委ねる。iOS の ATS / `Info.plist` に相当する追加設定 (最小 TLS バージョンの明示指定など) は行っていない
- タイムアウトは `context.WithTimeout(ctx, 10*time.Second)` の 1 種類のみ。接続タイムアウトとリソースタイムアウトを分けて設定してはいない

### 9.2 サイズ上限 (DoS / メモリ枯渇対策)

```go
// data.go
const maxBundleBytes = 32 << 20 // 32 MB

raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBundleBytes))
```

`Content-Length` の事前チェックはない。`io.LimitReader` で読み込み量そのものを
32 MB に制限する一段防御のみで、Swift 版で想定していた「`Content-Length` 詐称対策の
二重チェック」は行っていない。上限値も当初想定の 2 MB から、実際の公開バンドルの
サイズに合わせて 32 MB に変更されている。

### 9.3 スキーマ検証 (堅牢な decode)

```go
// data.go
func decodeBundle(raw []byte) (quiz.Bundle, error) {
    var b quiz.Bundle
    if err := json.Unmarshal(raw, &b); err != nil {
        return quiz.Bundle{}, fmt.Errorf("decode bundle: %w", err)
    }
    if b.Version != quiz.SchemaVersion {
        return quiz.Bundle{}, fmt.Errorf("decode bundle: unsupported schema version %d", b.Version)
    }
    return b, nil
}
```

検証はこの2点 (JSON デコード成功、`version` の厳密一致) のみ。Swift 版で想定していた
`validateInvariants`（1 quiz に mask が 1 つ、choices が answer を含む、prose/code の
blocks 構成が正しいか等の網羅的な不変則チェック）に相当する処理はクライアント側には
ない — 生成側 (`quizgen`) がこれらの不変則を守ることを前提にしている。

デコード失敗時の挙動: `loadBundle` がキャッシュへフォールバックし、キャッシュも
なければエラー画面 (5.2.1 参照)。

### 9.4 依存パッケージ管理 (Supply chain)

- `quizgen/go.mod` は最小依存 (`goldmark` のみ) を維持
- `quizgen/go.sum` はコミット必須。Phase 2.5 の移行で追加された依存も含めて固定
- CI (将来) で `go mod verify` を実行し、`go.sum` 不一致を検出
- GitHub Dependabot を `.github/dependabot.yml` で有効化:

  ```yaml
  version: 2
  updates:
    - package-ecosystem: "gomod"
      directory: "/quizgen"
      schedule: { interval: "weekly" }
    - package-ecosystem: "github-actions"
      directory: "/"
      schedule: { interval: "weekly" }
  ```

モバイルアプリ (`mobile/go.mod`) は `gioui.org` とその依存に加え、`quizgen/quiz`
を import する。`go.sum` はコミット必須で、CLI 側と同じ `go mod verify` の対象。
Swift/SPM は使っていないため `Package.resolved` に相当する管理は不要。

### 9.5 CDN 側の認証・デプロイ管理

該当なし。jsDelivr はパブリック GitHub リポジトリのファイルを配信するだけで、
デプロイ操作・API token・OAuth ログインのいずれも発生しない。Cloudflare Pages
時代に想定していた `wrangler` 認証情報の `.gitignore` 管理や token リボーク手順は、
その仕組み自体が存在しなくなったため不要になった。

### 9.6 プライバシー

- HTTP リクエストヘッダは Go の `net/http` が付ける既定のもののみ。ETag による
  条件付きリクエストは未実装 (9.3 参照) のため `If-None-Match` も送っていない
- User-Agent は Go の `net/http` デフォルト値。アプリ独自の識別文字列は追加していない (NFR8.6)
- アクセス元 IP は jsDelivr (バックエンドに複数の CDN プロバイダを使う) のログに残るが、本アプリは個人情報と紐付けないため問題なし
- App Store 提出時の Privacy Manifest 相当の作業は MVP の対象外 (内部配布のみ)

### 9.7 ローカルストレージの保護

- `app.DataDir()/quizzes.json` (取得済みバンドルのキャッシュ) と `app.DataDir()/scores.json` (回答履歴) は、いずれも Gio がOSごとに用意するアプリ専用データディレクトリに置かれる (iOS/Android ではアプリのサンドボックス内、デスクトップでは OS 標準のアプリデータディレクトリ)。暗号化はしていない
- OS 容量逼迫等でこのディレクトリが失われても、次回起動時に remote フェッチが成功する限り問題にならない。`scores.json` (進捗) が失われた場合の復旧手段はない — バックアップは取っていない

### 9.8 将来検討事項

| ID | 内容 | トリガー |
|---|---|---|
| Sec-F1 | `quizzes.json.sha256` を併置し、クライアント側で SHA-256 検証 | 改ざん耐性を一段強化したくなったタイミング |
| Sec-F2 | Ed25519 署名検証 (`quizzes.json.sig` + 公開鍵をアプリにバンドル) | 上記でも足りない場合 |
| Sec-F3 | Certificate Pinning (`http.Transport.TLSClientConfig` でカスタム検証) | 国家レベルの MITM を想定する場合 (MVP では過剰) |
| Sec-F4 | Crash reporting 導入 | ストア配布開始 + プライバシー方針策定 |

---

## 10. オープン質問 (Stage 2 で残る論点)

Gio 採用と jsDelivr 配信により、以下は解消または前提が変わった。表は当時の論点として残す。

| ID | 質問 | 当初のデフォルト案 | 現状 |
|---|---|---|---|
| Q1 | ~~iOS の最低サポート OS は 17 か 18 か~~ | iOS 17 | 不成立。SwiftUI/UIKit 固有の OS バージョン要件はなく、Gio が対応する範囲に従う |
| Q2 | 配布手段は TestFlight / Ad-hoc / 内部ビルドのみか | MVP 内部ビルドのみ | 実質そのまま: Android は APK を `adb install`、iOS は `mobile/build-ios.sh` で実機に直接インストール。ストア配布は未着手 |
| Q3 | CDN のカスタムドメインを今 PR で決めるか | `*.pages.dev` で開始、後で変更 | jsDelivr はカスタムドメインの仕組みを持たないため、この論点自体が解消 |
| Q4 | `version` 互換範囲 (`acceptedVersions`) のポリシー | MVP は完全一致 `2...2`、後で `1...2` 等に拡張 | 実装は今も完全一致のみ (`quiz.SchemaVersion` との等値比較)。範囲拡張は未着手 |
| Q5 | 進捗のクラウド同期 (iCloud KVS 等) を MVP に含めるか | 含めない | 含めていない。`scores.json` はデバイスローカルのみ |
| Q6 | Proposals 一覧の並び順 (ID 昇順 / カテゴリ別 / 進捗未完了優先) | ID 昇順 | 実装は `Bundle.Proposals` の生成順 (`quizgen` の処理順) をそのまま表示。検索フィルタ (`filterProposals`) はあるが並び替えはない |
| Q7 | エラー UI の文言は日本語 / 英語 | 英語 (NFR: マルチ言語は Out of Scope) | 実装は英語 (`Couldn't load quizzes` 等) |
| Q8 | mask の視覚スタイルを背景色だけにするか、枠線も入れるか | 背景色 + bold (`AttributedString` で実現可能な範囲) | 背景色つきチップ + bold monospace ラベル (`styledtext`)。枠線はなし |
