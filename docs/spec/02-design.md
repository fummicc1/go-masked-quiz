# 設計仕様: go-masked-quiz MVP

ステージ2 / 4 — 仕様駆動開発。前提は `docs/spec/01-requirements.md`。

---

## 1. システム全体図

```
┌────────────────────────────┐   git push    ┌──────────────────────┐
│ 開発者ローカル              │ ─────────────→ │ GitHub               │
│  - golang-proposal clone   │                │  fummicc1/           │
│    (iCloud 外、任意パス)    │                │   go-masked-quiz     │
│  - quizgen CLI              │                │  (main / spec/*)     │
│  - output/quizzes.json 生成 │                └──────┬───────────────┘
└────────────────────────────┘                       │ webhook
                                                     │ または手動 deploy
                                                     ▼
                                            ┌────────────────────┐
                                            │ Cloudflare Pages   │
                                            │  - cdn/v2/         │
                                            │      quizzes.json  │
                                            │  - _headers        │
                                            └────────┬───────────┘
                                                     │ HTTPS GET
                                                     │ ETag / 304
                                                     ▼
                                            ┌────────────────────┐
                                            │ iOS App (SwiftUI)  │
                                            │  - URLSession      │
                                            │  - Library/Caches  │
                                            │  - Block Renderer  │
                                            └────────────────────┘
```

3 層構成:

| 層 | 責務 | 主要技術 |
|---|---|---|
| **Generation** | proposal Markdown → Block 列 JSON v2 | Go 1.26、goldmark、go/parser、math/rand/v2 |
| **Distribution** | JSON を CDN 配信 | Cloudflare Pages、`_headers` (Cache-Control, ETag) |
| **Presentation** | フェッチ・キャッシュ・描画・インタラクション | SwiftUI、URLSession、AttributedString、UserDefaults |

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

### 2.3 Swift Codable (`ios/.../Models/`)

```swift
struct QuizBundle: Decodable, Sendable {
    let version: Int
    let generatedAt: Date
    let sourceRepo: URL
    let sourceFork: URL
    let sourceCommit: String?
    let sourceLicense: String
    let sourceLicenseUrl: URL  // JSON: source_license_url
    let proposals: [Proposal]
}

struct Proposal: Decodable, Identifiable, Sendable {
    let id: String
    let title: String
    let url: URL
    let quizzes: [Quiz]
}

struct Quiz: Decodable, Identifiable, Sendable, Equatable {
    let id: String
    let kind: Kind
    let index: Int
    let blocks: [Block]
    let answer: String
    let choices: [String]
    enum Kind: String, Decodable, Sendable { case prose, code }
}

/// 判別共用体としての Block。`@frozen` で将来追加時にコンパイラが網羅性を要求。
enum Block: Decodable, Sendable, Equatable, Hashable {
    case text(String)
    case inlineCode(String)
    case codeBlock(String)
    case mask

    private enum CodingKeys: String, CodingKey { case type, value }
    private enum Kind: String, Decodable {
        case text, inline_code, code_block, mask
    }
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        switch try c.decode(Kind.self, forKey: .type) {
        case .text:        self = .text(try c.decode(String.self, forKey: .value))
        case .inline_code: self = .inlineCode(try c.decode(String.self, forKey: .value))
        case .code_block:  self = .codeBlock(try c.decode(String.self, forKey: .value))
        case .mask:        self = .mask
        }
    }
}
```

`JSONDecoder` の設定:
- `keyDecodingStrategy = .convertFromSnakeCase`
- `dateDecodingStrategy = .iso8601`

---

## 3. Go CLI 設計 (`tools/quizgen`)

### 3.1 パッケージ構成

```
tools/quizgen/
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

## 4. CDN 配信層 (Cloudflare Pages)

### 4.1 リポジトリ構成

```
/cdn/                               # Cloudflare Pages の公開ルート
├── _headers                        # ヘッダ設定
├── v2/
│   └── quizzes.json                # 公開する v2 JSON
├── robots.txt                      # 検索エンジン除外
└── (将来 v3/ など)
```

### 4.2 `_headers` 例

```
/v2/*
  Content-Type: application/json; charset=utf-8
  Cache-Control: public, max-age=300, stale-while-revalidate=86400
  Access-Control-Allow-Origin: *
  X-Content-Type-Options: nosniff
```

ETag は Cloudflare が自動付与。

### 4.3 デプロイフロー

MVP は **手動 deploy** で開始 (FR5.5):

```sh
# 開発者ローカルで
go run ./tools/quizgen/cmd/quizgen generate \
    --proposals ~/Work/LocalApps/golang-proposal/design \
    --out       ./output/quizzes.json \
    --commit    "$(git -C ~/Work/LocalApps/golang-proposal rev-parse HEAD)" \
    --seed      42

cp ./output/quizzes.json ./cdn/v2/quizzes.json
git add cdn/v2/quizzes.json output/quizzes.json
git commit -m "data: refresh quizzes.json"

npx wrangler pages deploy ./cdn --project-name=go-masked-quiz-data
```

将来 (FR5.5 Optional) は `.github/workflows/deploy-cdn.yml` で `cdn/**` 変更時に自動デプロイ。

### 4.4 配信 URL とアプリ側設定

```
https://go-masked-quiz-data.pages.dev/v2/quizzes.json
```

(カスタムドメインは将来検討。`go-quiz.fummicc1.dev` 案)

iOS アプリは `Configuration.swift` に以下をハードコード:

```swift
enum Configuration {
    static let quizDataURL = URL(string: "https://go-masked-quiz-data.pages.dev/v2/quizzes.json")!
    static let acceptedVersions: ClosedRange<Int> = 2...2
    static let fetchTimeoutSeconds: TimeInterval = 10
    static let resourceTimeoutSeconds: TimeInterval = 30
}
```

ビルド構成で dev / prod を切り替える必要は MVP 段階ではなし。

### 4.5 CDN 障害時の挙動 (再掲)

| ケース | 挙動 |
|---|---|
| 初回起動、CDN ダウン | エラー画面 + 再試行 (FR2.6) |
| 2 回目起動、CDN ダウン | キャッシュで継続 (FR2.7)、UI に小さく "オフライン中" バナー |
| version 範囲外の JSON 取得 | キャッシュ継続 + アプリ更新を促すモーダル (FR2.19) |

---

## 5. iOS アプリ設計

### 5.1 ファイル構成

```
ios/GoMaskedQuiz/
├── GoMaskedQuiz.xcodeproj
└── GoMaskedQuiz/
    ├── GoMaskedQuizApp.swift
    ├── Configuration.swift                  # CDN URL 等
    ├── Models/
    │   ├── QuizBundle.swift
    │   ├── Proposal.swift
    │   ├── Quiz.swift
    │   └── Block.swift
    ├── Services/
    │   ├── QuizLoader.swift                 # フェッチ + キャッシュ
    │   ├── QuizCache.swift                  # Library/Caches I/O
    │   └── ProgressStore.swift              # UserDefaults
    ├── Features/
    │   ├── Root/
    │   │   ├── RootView.swift               # LaunchPhase で分岐
    │   │   └── LaunchPhase.swift
    │   ├── ProposalList/
    │   │   ├── ProposalListView.swift
    │   │   └── ProposalListViewModel.swift
    │   ├── QuizSession/
    │   │   ├── QuizSessionView.swift
    │   │   ├── QuizSessionViewModel.swift
    │   │   ├── QuizContentView.swift        # blocks の Renderer 切替
    │   │   ├── ProseRenderer.swift
    │   │   ├── CodeRenderer.swift
    │   │   ├── MaskState.swift              # mask の state machine
    │   │   ├── ChoiceButtonsView.swift
    │   │   └── QuizFeedbackView.swift
    │   ├── Result/
    │   │   ├── ResultSummaryView.swift
    │   │   └── ResultSummaryViewModel.swift
    │   └── About/
    │       └── AcknowledgmentsView.swift
    └── Resources/
        └── Assets.xcassets
```

### 5.2 状態遷移

#### 5.2.1 LaunchPhase (アプリ全体)

```swift
enum LaunchPhase {
    case loading
    case ready(QuizBundle, source: BundleSource)   // ready で表示開始
    case error(LoadError, retryable: Bool)
}

enum BundleSource {
    case fresh      // この起動で取得した最新
    case cached     // キャッシュから読込中、背景で fresh フェッチ中
}
```

遷移:

```
[loading]
    │ fetch()
    ├─→ initial run, success      → [ready(fresh)]
    ├─→ initial run, failure      → [error(.network, retryable: true)]
    ├─→ subsequent, cache loaded  → [ready(cached)]
    │       │ (背景フェッチ)
    │       └─→ success           → [ready(fresh)]    // 再表示は次回起動
    │       └─→ failure           → そのまま (silent)
    ├─→ cache corrupt, fresh ok   → [ready(fresh)]
    └─→ version out of range      → [error(.unsupported, retryable: false)]
                                    + キャッシュあれば [ready(cached)] にフォールバック
```

#### 5.2.2 MaskState (mask 1 つ分)

```swift
enum MaskState: Sendable, Equatable {
    case empty
    case preview(String)
    case correct(String)
    case incorrect(submitted: String, correct: String)

    var displayText: String {
        switch self {
        case .empty: return "____"
        case .preview(let s), .correct(let s): return s
        case .incorrect(let s, _): return s
        }
    }
}
```

```
empty ──(tap choice c)──→ preview(c)
preview(c) ──(tap c')──→ preview(c')           // 上書き可
preview(c) ──(Submit, c == ans)──→ correct(c)
preview(c) ──(Submit, c != ans)──→ incorrect(c, ans)
correct/incorrect ──(Next)──→ 次クイズの empty
```

#### 5.2.3 QuizSessionPhase

```swift
@Observable
final class QuizSessionViewModel {
    enum Phase { case playing, reviewing }
    private(set) var phase: Phase = .playing
    private(set) var maskState: MaskState = .empty
    private(set) var quizIndex: Int = 0
    let proposal: Proposal
    let progress: ProgressStore

    var currentQuiz: Quiz { proposal.quizzes[quizIndex] }
    var hasNext: Bool { quizIndex + 1 < proposal.quizzes.count }

    func selectChoice(_ choice: String) {
        guard phase == .playing else { return }
        maskState = .preview(choice)
    }

    func submit() {
        guard phase == .playing, case .preview(let c) = maskState else { return }
        if c == currentQuiz.answer {
            maskState = .correct(c)
            progress.recordResult(quizID: currentQuiz.id, correct: true)
        } else {
            maskState = .incorrect(submitted: c, correct: currentQuiz.answer)
            progress.recordResult(quizID: currentQuiz.id, correct: false)
        }
        phase = .reviewing
    }

    func next() {
        guard phase == .reviewing, hasNext else { return }
        quizIndex += 1
        maskState = .empty
        phase = .playing
    }
}
```

### 5.3 データレイヤ

#### 5.3.1 QuizLoader

```swift
actor QuizLoader {
    enum LoadResult {
        case fresh(QuizBundle)
        case cached(QuizBundle, refreshing: Task<QuizBundle?, Never>?)
    }

    private let url: URL
    private let cache: QuizCache
    private let session: URLSession
    private let acceptedVersions: ClosedRange<Int>

    func load() async throws -> LoadResult {
        // 1) キャッシュをまず試す
        let cached = try? await cache.read()

        // 2) ETag 付きで fresh フェッチ
        var req = URLRequest(url: url, timeoutInterval: Configuration.fetchTimeoutSeconds)
        if let etag = await cache.lastETag() {
            req.setValue(etag, forHTTPHeaderField: "If-None-Match")
        }
        let (data, response): (Data, URLResponse)
        do {
            (data, response) = try await session.data(for: req)
        } catch {
            if let cached { return .cached(cached, refreshing: nil) }
            throw LoadError.network(error)
        }

        guard let http = response as? HTTPURLResponse else {
            if let cached { return .cached(cached, refreshing: nil) }
            throw LoadError.invalidResponse
        }

        if http.statusCode == 304, let cached {
            return .cached(cached, refreshing: nil)
        }
        guard http.statusCode == 200 else {
            if let cached { return .cached(cached, refreshing: nil) }
            throw LoadError.http(http.statusCode)
        }

        // 3) decode + version check
        let bundle = try JSONDecoder.quiz.decode(QuizBundle.self, from: data)
        guard acceptedVersions.contains(bundle.version) else {
            if let cached { return .cached(cached, refreshing: nil) }
            throw LoadError.unsupportedVersion(bundle.version)
        }

        // 4) save
        let etag = http.value(forHTTPHeaderField: "ETag")
        try await cache.write(data: data, etag: etag)
        return .fresh(bundle)
    }
}
```

#### 5.3.2 QuizCache

```swift
actor QuizCache {
    private let fileURL: URL          // Library/Caches/quizzes.json
    private let etagDefaultsKey = "QuizCache.etag"

    func read() async throws -> QuizBundle {
        let data = try Data(contentsOf: fileURL)
        return try JSONDecoder.quiz.decode(QuizBundle.self, from: data)
    }

    func write(data: Data, etag: String?) async throws {
        try data.write(to: fileURL, options: .atomic)
        UserDefaults.standard.set(etag, forKey: etagDefaultsKey)
    }

    func lastETag() async -> String? {
        UserDefaults.standard.string(forKey: etagDefaultsKey)
    }
}
```

`Library/Caches/` は OS によって自動削除される可能性があるが、その場合は次回起動時に初回扱いで再取得 (FR2.7 / 5.2.1 の遷移参照)。

#### 5.3.3 ProgressStore

```swift
struct ProgressStore: Sendable {
    private let defaults: UserDefaults

    func recordResult(quizID: String, correct: Bool) {
        defaults.set(correct, forKey: "progress.\(quizID)")
    }
    func result(for quizID: String) -> Bool? {
        defaults.object(forKey: "progress.\(quizID)") as? Bool
    }
    func correctCount(in proposal: Proposal) -> Int {
        proposal.quizzes.filter { result(for: $0.id) == true }.count
    }
}
```

将来 (Phase 5) は SwiftData に移行可能なように、`protocol ProgressStoring` で抽象化しておく。

### 5.4 レンダリングレイヤ

#### 5.4.1 ProseRenderer (AttributedString 結合方式)

```swift
struct ProseRenderer: View {
    let blocks: [Block]
    let maskState: MaskState

    var body: some View {
        Text(attributed)
            .textSelection(.disabled)
            .multilineTextAlignment(.leading)
            .accessibilityElement(children: .combine)
    }

    private var attributed: AttributedString {
        var out = AttributedString()
        for block in blocks {
            switch block {
            case .text(let s):
                out.append(AttributedString(s))
            case .inlineCode(let s):
                var a = AttributedString(s)
                a.font = .system(.body, design: .monospaced)
                a.backgroundColor = .secondary.opacity(0.15)
                out.append(a)
            case .codeBlock:
                continue   // 不変則違反 (prose に code_block) なら無視
            case .mask:
                out.append(maskStyledFragment())
            }
        }
        return out
    }

    private func maskStyledFragment() -> AttributedString {
        var a = AttributedString(maskState.displayText)
        a.font = .system(.body, design: .monospaced).bold()
        switch maskState {
        case .empty:
            a.backgroundColor = .yellow.opacity(0.4)
        case .preview:
            a.backgroundColor = .blue.opacity(0.4)
        case .correct:
            a.backgroundColor = .green.opacity(0.4)
        case .incorrect:
            a.backgroundColor = .red.opacity(0.4)
        }
        // 枠線は AttributedString では難しいので背景色 + bold で代替
        return a
    }
}
```

#### 5.4.2 CodeRenderer

```swift
struct CodeRenderer: View {
    let blocks: [Block]
    let maskState: MaskState

    var body: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            Text(attributed)
                .font(.system(.body, design: .monospaced))
                .padding(12)
        }
        .background(.secondary.opacity(0.1))
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }

    private var attributed: AttributedString {
        var out = AttributedString()
        for block in blocks {
            switch block {
            case .codeBlock(let s):
                out.append(AttributedString(s))
            case .mask:
                var a = AttributedString(maskState.displayText)
                a.font = .system(.body, design: .monospaced).bold()
                switch maskState {
                case .empty:     a.backgroundColor = .yellow.opacity(0.4)
                case .preview:   a.backgroundColor = .blue.opacity(0.4)
                case .correct:   a.backgroundColor = .green.opacity(0.4)
                case .incorrect: a.backgroundColor = .red.opacity(0.4)
                }
                out.append(a)
            case .text, .inlineCode:
                continue   // 不変則違反は無視
            }
        }
        return out
    }
}
```

#### 5.4.3 QuizContentView (kind で振り分け)

```swift
struct QuizContentView: View {
    let quiz: Quiz
    let maskState: MaskState

    var body: some View {
        switch quiz.kind {
        case .prose: ProseRenderer(blocks: quiz.blocks, maskState: maskState)
        case .code:  CodeRenderer(blocks: quiz.blocks, maskState: maskState)
        }
    }
}
```

### 5.5 インタラクションレイヤ

#### 5.5.1 ChoiceButtonsView

```swift
struct ChoiceButtonsView: View {
    let choices: [String]
    let maskState: MaskState
    let phase: QuizSessionViewModel.Phase
    let correctAnswer: String
    let onSelect: (String) -> Void

    var body: some View {
        VStack(spacing: 8) {
            ForEach(choices, id: \.self) { c in
                Button {
                    onSelect(c)
                } label: {
                    HStack {
                        Text(c).font(.system(.body, design: .monospaced))
                        Spacer()
                        statusIcon(for: c)
                    }
                    .padding()
                    .frame(maxWidth: .infinity)
                    .background(background(for: c))
                    .clipShape(RoundedRectangle(cornerRadius: 8))
                }
                .buttonStyle(.plain)
                .disabled(phase == .reviewing)
                .accessibilityHint(accessibilityHint(for: c))
            }
        }
    }

    @ViewBuilder
    private func statusIcon(for c: String) -> some View {
        if phase == .reviewing {
            if c == correctAnswer {
                Image(systemName: "checkmark.circle.fill").foregroundStyle(.green)
            } else if case .preview(let s) = maskState, s == c {
                Image(systemName: "xmark.circle.fill").foregroundStyle(.red)
            }
        } else if case .preview(let s) = maskState, s == c {
            Image(systemName: "circle.fill").foregroundStyle(.blue)
        }
    }
    // background, accessibilityHint も同様に状態で分岐
}
```

#### 5.5.2 QuizSessionView (全体組み立て)

```swift
struct QuizSessionView: View {
    @State private var vm: QuizSessionViewModel

    var body: some View {
        VStack(spacing: 16) {
            QuizContentView(quiz: vm.currentQuiz, maskState: vm.maskState)
                .padding(.horizontal)
            Spacer()
            ChoiceButtonsView(
                choices: vm.currentQuiz.choices,
                maskState: vm.maskState,
                phase: vm.phase,
                correctAnswer: vm.currentQuiz.answer,
                onSelect: vm.selectChoice
            )
            .padding()

            HStack {
                Button("Submit", action: vm.submit)
                    .disabled(vm.phase == .reviewing || vm.maskState == .empty)
                Spacer()
                Button(vm.hasNext ? "Next" : "Finish", action: handleNext)
                    .disabled(vm.phase == .playing)
            }.padding()
        }
        .navigationTitle(vm.proposal.title)
    }

    private func handleNext() {
        if vm.hasNext { vm.next() }
        else { /* 結果画面へ navigate */ }
    }
}
```

### 5.6 エラー UI

```swift
struct ErrorView: View {
    let error: LoadError
    let onRetry: (() -> Void)?

    var body: some View {
        VStack(spacing: 16) {
            Image(systemName: "wifi.exclamationmark")
                .font(.system(size: 48))
            Text(title).font(.title2).bold()
            Text(description).foregroundStyle(.secondary)
            if let onRetry {
                Button("再試行", action: onRetry).buttonStyle(.borderedProminent)
            }
        }.padding()
    }
}
```

---

## 6. アクセシビリティ設計

| 対象 | 対応 |
|---|---|
| VoiceOver | `MaskState` の状態を `accessibilityLabel` で読み上げ ("穴埋め欄、未回答" / "選択中: golang/proposal" / "正解" / "不正解、正解は X") |
| Dynamic Type | システムフォント (.body) のみ使用、Auto-scale |
| Color Contrast | 緑/赤は SF Symbols のチェック/バツアイコンを併用 |
| Reduce Motion | アニメーションは `accessibilityReduceMotion` で無効化 |
| Switch Control | 各 Button が独立した focusable element |

---

## 7. テスト戦略の方針 (Stage 3 で詳細化)

| 層 | 種別 | 主要対象 |
|---|---|---|
| Go | Unit | `parser`, `masker.candidates`, `masker.rand`, `blocks.{prose,code,window}` |
| Go | Golden | testdata/proposals/*.md → output JSON の snapshot |
| Go | Property-based | 4 択生成の不変則 (重複なし、answer 含む、サイズ) |
| Swift | Unit | `MaskState` 遷移、`QuizSessionViewModel`、`Block` decoder |
| Swift | Integration | `QuizLoader` (`URLProtocol` 差し替えで HTTP モック) |
| Swift | UI Snapshot | `ProseRenderer` / `CodeRenderer` の 4 状態 |
| Manual | E2E | 機内モード、初回 / 2 回目、CDN 差し替えで更新検知 |

詳細は Stage 3 で。

---

## 8. 段階的実装計画

| Phase | スコープ | 状態 |
|---|---|---|
| 1 | リポジトリ骨子・LICENSE/NOTICE・CLI スケルトン | **完了** (`95b6590`) |
| 2 | 機械的マスキング v1 (`context_*`) | **完了 (未コミット)** |
| 2.5 | v1 → v2 移行 (`Block[]` スキーマ) | 未着手 |
| 3 | CDN: `cdn/v2/quizzes.json`, `_headers`, 手動デプロイ手順、AC9 検証 | 未着手 |
| 4 | iOS MVP: フェッチ / キャッシュ / Block 描画 / Tap-to-fill / Result / Acknowledgments | 未着手 |
| 5 | 進捗保存・復習機能・進捗バッジ | 未着手 |
| 6 | (Optional) GitHub Actions による CDN 自動デプロイ | 未着手 |
| 7 | (Optional) LLM 拡張 | 未着手 |

---

## 9. セキュリティ設計

要件 NFR8.1-8.6 / AC18-21 / R11-R13 を満たすための具体的な設計指針。
脅威モデル: 機密性ゼロ・改ざん耐性中・可用性中、を前提とする。

### 9.1 通信層 (TLS / ATS)

- iOS `Info.plist` の `NSAppTransportSecurity` は **追加せず、ATS 既定のまま** にする (= HTTPS 強制 + TLS 1.2+ 強制)
- `URLSession` の構成:

  ```swift
  let config = URLSessionConfiguration.ephemeral
  config.tlsMinimumSupportedProtocolVersion = .TLSv12
  config.timeoutIntervalForRequest = Configuration.fetchTimeoutSeconds   // 10s
  config.timeoutIntervalForResource = Configuration.resourceTimeoutSeconds // 30s
  config.httpAdditionalHeaders = [:]  // 独自 UA は付けない (NFR8.6)
  let session = URLSession(configuration: config)
  ```

- `Configuration.quizDataURL` は `https://` をリテラルに含める。`http://` URL の混入をビルド時に防ぐため、`URL(string:)` の `https` プレフィックス検証を Configuration 初期化時にアサート

### 9.2 サイズ上限 (DoS / メモリ枯渇対策)

```swift
enum LoadError: Error {
    case payloadTooLarge(Int)
    case schemaInvalid
    case unsupportedVersion(Int)
    case network(Error)
    case http(Int)
    case invalidResponse
}

private let maxBytes = 2 * 1024 * 1024  // 2 MB (NFR8.2)

// QuizLoader.load() 内
if let contentLength = http.expectedContentLength, contentLength > Int64(maxBytes) {
    throw LoadError.payloadTooLarge(Int(contentLength))
}
if data.count > maxBytes {            // ボディサイズの再チェック (Content-Length 詐称対策)
    throw LoadError.payloadTooLarge(data.count)
}
```

`URLSession` は `data(for:)` でメモリに一括展開するが、2 MB 上限なら問題なし。ストリーミング decode は採用しない (実装複雑化に見合わない)。

### 9.3 スキーマ検証 (堅牢な decode)

`Block` enum / `Quiz.Kind` enum で網羅的に decode することにより、JSON
側に未知の文字列が来た場合に `JSONDecoder.decode` が throw する。これを
catch して以下のフローへ:

```swift
do {
    let bundle = try JSONDecoder.quiz.decode(QuizBundle.self, from: data)
    guard acceptedVersions.contains(bundle.version) else {
        throw LoadError.unsupportedVersion(bundle.version)
    }
    // additional invariant checks (1 mask per quiz, answer ∈ choices)
    try validateInvariants(bundle)
    return bundle
} catch is DecodingError {
    throw LoadError.schemaInvalid
}
```

`validateInvariants` で:

- 各 `quiz.blocks` 内に `.mask` がちょうど 1 つ存在
- `quiz.choices` が `quiz.answer` を含む
- `quiz.choices` の要素数 ≥ 2
- `quiz.kind == .prose` ⇒ blocks に `.codeBlock` を含まない (逆も同様)

LoadError 発生時の挙動: キャッシュがあればキャッシュ継続 (`LoadResult.cached`)、なければエラー画面。

### 9.4 依存パッケージ管理 (Supply chain)

- `tools/quizgen/go.mod` は最小依存 (`goldmark` のみ) を維持
- `tools/quizgen/go.sum` はコミット必須。Phase 2.5 の移行で追加された依存も含めて固定
- CI (将来) で `go mod verify` を実行し、`go.sum` 不一致を検出
- GitHub Dependabot を `.github/dependabot.yml` で有効化:

  ```yaml
  version: 2
  updates:
    - package-ecosystem: "gomod"
      directory: "/tools/quizgen"
      schedule: { interval: "weekly" }
    - package-ecosystem: "github-actions"
      directory: "/"
      schedule: { interval: "weekly" }
  ```

iOS は `Package.swift` / SPM 依存を当面持たない予定 (Phase 1-4 では標準ライブラリのみ)。将来追加時は `Package.resolved` をコミットしハッシュ固定。

### 9.5 Cloudflare API token 管理

- 開発者ローカル: `wrangler login` で OAuth フロー (token はキーチェーンに保存)
- CI (将来): `CLOUDFLARE_API_TOKEN` を GitHub Repository Secrets に登録
- token のスコープ: `Account.Cloudflare Pages: Edit` のみ (最小権限)
- `.gitignore` に以下を追加 (NFR8.5):

  ```
  # wrangler local auth
  .wrangler/
  .dev.vars
  ```

- README に **token リボーク手順** を明記: Cloudflare Dashboard → My Profile → API Tokens → 該当 token → "Roll" または "Delete"

### 9.6 プライバシー

- HTTP リクエストヘッダは最小限 (`If-None-Match` の ETag のみ追加)
- User-Agent はシステムデフォルト (`CFNetwork/...; Darwin/...`)。アプリ識別文字列を**追加しない** (NFR8.6)
- アクセス元 IP は Cloudflare のログに残るが、本アプリは個人情報と紐付けないため問題なし
- App Store 提出時は別途 App Privacy Manifest (`PrivacyInfo.xcprivacy`) を作成 (Out of Scope: MVP は内部配布)

### 9.7 ローカルストレージの保護

- `Library/Caches/quizzes.json`: iOS Sandbox で他アプリから不可視。暗号化不要
- `UserDefaults` の `progress.<quizID>` および `QuizCache.etag`: 同上
- OS 容量逼迫で `Caches/` が削除されても、次回起動の初回ロジックでフォールバック (5.3.2 参照)

### 9.8 将来検討事項 (Phase 5+)

| ID | 内容 | トリガー |
|---|---|---|
| Sec-F1 | `quizzes.json.sha256` を併置し、iOS で SHA-256 検証 | 改ざん耐性を一段強化したくなったタイミング |
| Sec-F2 | Ed25519 署名検証 (`quizzes.json.sig` + 公開鍵をアプリにバンドル) | 上記でも足りない場合 |
| Sec-F3 | Certificate Pinning (`URLSessionTaskDelegate.urlSession(_:didReceive:completionHandler:)`) | 国家レベルの MITM を想定する場合 (MVP では過剰) |
| Sec-F4 | Crash reporting (Sentry, Crashlytics) 導入 | App Store 配布開始 + プライバシー方針策定 |

---

## 10. オープン質問 (Stage 2 で残る論点)

| ID | 質問 | デフォルト案 |
|---|---|---|
| Q1 | iOS の最低サポート OS は 17 か 18 か (Liquid Glass を採用するなら 26+) | iOS 17 |
| Q2 | iOS の配布手段は TestFlight / Ad-hoc / 内部ビルドのみ | MVP 内部ビルドのみ |
| Q3 | CDN のカスタムドメインを今 PR で決めるか | `*.pages.dev` で開始、後で変更 |
| Q4 | `version` 互換範囲 (`acceptedVersions`) のポリシー | MVP は完全一致 `2...2`、後で `1...2` 等に拡張 |
| Q5 | ProgressStore のクラウド同期 (iCloud KVS) を MVP に含めるか | 含めない |
| Q6 | Proposals 一覧の並び順 (ID 昇順 / カテゴリ別 / 進捗未完了優先) | ID 昇順 |
| Q7 | エラー UI の文言は日本語 / 英語 | 英語 (NFR: マルチ言語は Out of Scope) |
| Q8 | mask の視覚スタイルを背景色だけにするか、枠線も入れるか | 背景色 + bold (`AttributedString` で実現可能な範囲) |
