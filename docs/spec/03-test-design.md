# テスト設計仕様: go-masked-quiz MVP

ステージ3 / 4 — 仕様駆動開発。前提: `01-requirements.md`, `02-design.md`。

---

## 1. テスト戦略

### 1.1 ピラミッド

```
            ┌────────────┐
            │  E2E (手動)│        少: 4-6 ケース
            └────────────┘
         ┌────────────────┐
         │  Integration   │     中: 10-20 ケース
         └────────────────┘
      ┌──────────────────────┐
      │  UI Snapshot         │   中: 6-10 ケース
      └──────────────────────┘
   ┌────────────────────────────┐
   │  Unit / Property / Golden  │ 多: 60+ ケース
   └────────────────────────────┘
```

ピラミッド構成、機械テスト優先 (NFR4.2)、手動 E2E は受け入れ基準のみ。

### 1.2 テスト ID 体系

```
TC-{layer}-{category}-{nn}

layer:    G = Go CLI / S = Swift / D = CDN / E = E2E / Z = Security / P = Performance
category: P = parser, M = masker, B = blocks, E = CLI E2E, S = schema,
          M = Models, L = Loader, Q = QuizSession state, V = ViewModel,
          U = UI Snapshot
```

例: `TC-G-B-04` = Go の blocks パッケージのテスト 4 件目。

### 1.3 カバレッジ目標 (NFR4.2)

| 層 | 目標 |
|---|---|
| `internal/parser` | 行カバレッジ ≥ 80% |
| `internal/masker` | 行カバレッジ ≥ 85% |
| `internal/blocks` | 行カバレッジ ≥ 90% (新規・複雑) |
| `cmd/quizgen` | ゴールデンテストで E2E カバレッジ |
| iOS Models | 全 decode パスを網羅 |
| iOS Services | 主要分岐 (200/304/失敗/サイズ超/version 外) を網羅 |
| iOS Features ViewModel | 状態遷移を網羅 |

---

## 2. Go テストケース

### 2.1 `internal/parser`

ファイル: `internal/parser/proposal_test.go`, `codeblock_test.go`

| ID | 内容 | 期待値 |
|---|---|---|
| TC-G-P-01 | 既存 `99999-sample.md` を `LoadProposal` | `Slug`, `Title`, `InlineCodes` 1 件以上, `CodeBlocks` 1 件以上 |
| TC-G-P-02 | inline code 複数を含む Markdown | `InlineCodes` 全件抽出、`ByteOffset` 昇順 |
| TC-G-P-03 | 複数 ` ```go ` ブロック | `CodeBlocks` 全件抽出、各 `Source` が valid bytes |
| TC-G-P-04 | ` ```python ` ブロック (Go 以外) | `CodeBlocks` に含まれない |
| TC-G-P-05 | 空ファイル | error なし、`InlineCodes` / `CodeBlocks` 空 |
| TC-G-P-06 | タイトル (H1) なし | `Title == Slug` |
| TC-G-P-07 | 改行を含む大きな Markdown (10 KB+) | parse 完了、メモリ問題なし |
| TC-G-P-08 | `` `nested \`code\` ` `` | inline code として 1 件抽出 (goldmark 仕様準拠) |
| TC-G-P-09 | 存在しないファイルパス | error 返却 |

### 2.2 `internal/masker`

ファイル: `internal/masker/{rand,candidates,prose,code}_test.go`

| ID | 内容 | 期待値 |
|---|---|---|
| TC-G-M-01 | `NewRNG(42, "x")` を 2 回呼び 100 回 `Int64()` 比較 | 全一致 |
| TC-G-M-02 | `NewRNG(42, "x")` vs `NewRNG(42, "y")` を 64 回比較 | 同値は ≤ 2 回 |
| TC-G-M-03 | `GoKeywords()` | len = 25 |
| TC-G-M-04 | `GenerateChoices` 契約: 件数, answer 含む, 重複なし | 4 件 / answer ∈ choices / lowercase 重複なし |
| TC-G-M-05 | `GenerateChoices` 同 seed で 2 回 | 完全一致 |
| TC-G-M-06 | `CollectProseSeeds` で stopword (`"the"`, `"is"`) | 除外される |
| TC-G-M-07 | `CollectProseSeeds` で 2 文字 (`"go"`) | 除外される |
| TC-G-M-08 | `CollectProseSeeds` で同一 inline code を重複 | 1 件のみ採用 |
| TC-G-M-09 | `CollectCodeSeeds` で `func Hello()` | `Hello` 抽出 |
| TC-G-M-10 | `CollectCodeSeeds` で snippet (package 句なし) | `package _x` ラップでフォールバック成功 |
| TC-G-M-11 | `CollectCodeSeeds` で破損 Go コード | seeds = empty、panic なし |
| TC-G-M-12 | `levenshtein("foo", "for")` | 1 |
| TC-G-M-13 | `rankByEdit(["bar", "baz", "qux"], "foo")` | 距離昇順 |

### 2.3 `internal/blocks` (新規)

ファイル: `internal/blocks/{prose,code,window}_test.go`

| ID | 内容 | 期待 blocks 列 |
|---|---|---|
| TC-G-B-01 | prose: `"hello `world`!"`, seed=`world` | `[text "hello ", mask, text "!"]` |
| TC-G-B-02 | prose: 周辺 inline code 残る | `[text, inline_code, text, mask, text]` |
| TC-G-B-03 | prose: 周辺ウィンドウが改行で切れる | `text` は改行を超えない |
| TC-G-B-04 | code: `func F() { fmt.Println(...) }`, seed=`F` | `[code_block "func ", mask, code_block "() { fmt.Println(...) }"]` |
| TC-G-B-05 | code: 周辺ウィンドウ N=120 を超えるコード | window 内のみ含む |
| TC-G-B-06 | prose: mask が 1 つだけ | `count(blocks where type=mask) == 1` |
| TC-G-B-07 | 空 value のブロックは出力しない | `text "" は含まれない` |
| TC-G-B-08 | 連続する `text` セグメントは結合 | 隣接する `text` は連結された 1 つの value |

### 2.4 `cmd/quizgen` (E2E ゴールデン)

ファイル: `cmd/quizgen/main_test.go`、`testdata/golden/*.json`

| ID | 内容 | 検証手段 |
|---|---|---|
| TC-G-E-01 | `testdata/proposals/` → JSON 生成 (seed=42) | `testdata/golden/quizzes-seed42.json` とバイト一致 (`generated_at` を fixed time に置換) |
| TC-G-E-02 | 同コマンド 2 回実行 | `generated_at` を除く JSON が完全一致 |
| TC-G-E-03 | `--commit abcdef` | 出力 JSON の `source_commit == "abcdef"` |
| TC-G-E-04 | `--max-per-proposal 3` | 全 proposal で `len(quizzes) <= 3` |
| TC-G-E-05 | `--choices 2` | 全 quiz で `len(choices) == 2` |
| TC-G-E-06 | 存在しないディレクトリ | exit code != 0 + stderr エラー |
| TC-G-E-07 | 空ディレクトリ | exit code != 0 (no *.md) |
| TC-G-E-08 | proposals に *.md と *.txt 混在 | *.md のみ処理、*.txt は無視 |
| TC-G-E-09 | proposal 内に code_block 0 / inline_code 0 | quiz 0 件で panic せず (FR1.12) |

実装ヒント: テスト時間を固定するため、`time.Now` を `--now` フラグで上書き or `internal/timeutil` で抽象化。

### 2.5 スキーマ不変則 (Property-based)

ファイル: `cmd/quizgen/schema_test.go`

| ID | 内容 | 検証 |
|---|---|---|
| TC-G-S-01 | 任意 seed (1..100) で生成された JSON: `version == 2` | テーブル駆動 |
| TC-G-S-02 | 全 quiz で `count(mask) == 1` | for-each |
| TC-G-S-03 | 全 quiz で `answer ∈ choices` (lowercase 比較) | for-each |
| TC-G-S-04 | 全 quiz の `choices` lowercase 重複なし | set サイズ比較 |
| TC-G-S-05 | prose クイズに `code_block` 含まれない | for-each |
| TC-G-S-06 | code クイズに `text` / `inline_code` 含まれない | for-each |
| TC-G-S-07 | 全ブロックの `value` が空ではない (mask 除く) | for-each |

---

## 3. iOS テストケース

### 3.1 Models / Codable

ファイル: `ios/GoMaskedQuizTests/ModelsTests.swift`

| ID | 内容 | 期待 |
|---|---|---|
| TC-S-M-01 | v2 サンプル JSON を `QuizBundle` に decode | 成功、全フィールド一致 |
| TC-S-M-02 | `{"type":"text", "value":"x"}` → `Block.text("x")` | OK |
| TC-S-M-03 | `{"type":"inline_code", "value":"x"}` → `Block.inlineCode("x")` | OK |
| TC-S-M-04 | `{"type":"code_block", "value":"x"}` → `Block.codeBlock("x")` | OK |
| TC-S-M-05 | `{"type":"mask"}` (value なし) → `Block.mask` | OK |
| TC-S-M-06 | `{"type":"unknown"}` | `DecodingError` を throw |
| TC-S-M-07 | `Quiz` で `answer` フィールド欠落 | `DecodingError` を throw |
| TC-S-M-08 | `QuizBundle` で `version` フィールド欠落 | `DecodingError` を throw |
| TC-S-M-09 | `generated_at` を ISO8601 文字列で decode | `Date` 取得 |
| TC-S-M-10 | `Quiz.Kind = "unknown"` | `DecodingError` を throw |

### 3.2 Services / QuizLoader (URLProtocol 差し替え)

ファイル: `ios/GoMaskedQuizTests/QuizLoaderTests.swift`

`MockURLProtocol` で `URLSession` を差し替え、HTTP 応答を制御。

| ID | シナリオ | 入力 | 期待 |
|---|---|---|---|
| TC-S-L-01 | 初回 200 OK + JSON | キャッシュなし | `.fresh(bundle)`、キャッシュ書き込み、ETag 保存 |
| TC-S-L-02 | 304 Not Modified | キャッシュあり + ETag | `.cached(bundle)`、キャッシュ維持 |
| TC-S-L-03 | 500 Server Error | キャッシュあり | `.cached(bundle)` (silent fail) |
| TC-S-L-04 | 500 Server Error | キャッシュなし | `LoadError.http(500)` を throw |
| TC-S-L-05 | Timeout | キャッシュあり | `.cached(bundle)` |
| TC-S-L-06 | Timeout | キャッシュなし | `LoadError.network(...)` |
| TC-S-L-07 | 200 + body 3 MB | キャッシュなし | `LoadError.payloadTooLarge` |
| TC-S-L-08 | Content-Length 1 MB / body 3 MB (詐称) | キャッシュなし | `LoadError.payloadTooLarge` (二重防御) |
| TC-S-L-09 | 200 + `version: 99` | キャッシュなし | `LoadError.unsupportedVersion(99)` |
| TC-S-L-10 | 200 + 不正 JSON (フィールド欠落) | キャッシュなし | `LoadError.schemaInvalid` |
| TC-S-L-11 | 200 + 不正 JSON | キャッシュあり | キャッシュ継続 (silent) |
| TC-S-L-12 | If-None-Match 送信確認 | キャッシュに ETag あり | リクエストヘッダに ETag が含まれる |

### 3.3 Services / QuizCache

| ID | 内容 | 期待 |
|---|---|---|
| TC-S-C-01 | `write` 後 `read` で同一 bundle 取得 | OK |
| TC-S-C-02 | ファイル削除後 `read` | error throw |
| TC-S-C-03 | 破損 JSON を書いた後 `read` | `DecodingError` throw |
| TC-S-C-04 | `lastETag()` で `write` 時の etag 取得 | 一致 |

### 3.4 Features / MaskState (状態機械)

ファイル: `MaskStateTests.swift`

| ID | 初期 | 操作 | 期待 |
|---|---|---|---|
| TC-S-Q-01 | `.empty` | `vm.selectChoice("a")` | `.preview("a")` |
| TC-S-Q-02 | `.preview("a")` | `vm.selectChoice("b")` | `.preview("b")` (上書き) |
| TC-S-Q-03 | `.preview("ans")` | `vm.submit()` (`answer == "ans"`) | `.correct("ans")` |
| TC-S-Q-04 | `.preview("wrong")` | `vm.submit()` (`answer == "ans"`) | `.incorrect(submitted:"wrong", correct:"ans")` |
| TC-S-Q-05 | `.empty` | `vm.submit()` | 状態変化なし、`phase` も `.playing` |
| TC-S-Q-06 | `.correct(...)` | `vm.selectChoice("x")` | 状態変化なし (reviewing 中は disabled) |
| TC-S-Q-07 | `.correct(...)` | `vm.next()` | 次クイズ・`.empty` / `.playing` |
| TC-S-Q-08 | 最終クイズ `.correct(...)` | `vm.next()` | no-op (hasNext == false) |

### 3.5 Features / QuizSessionViewModel

| ID | 内容 | 期待 |
|---|---|---|
| TC-S-V-01 | `selectChoice` 中 `progress.recordResult` 未呼出 | 呼ばれない |
| TC-S-V-02 | `submit` (correct) | `progress.recordResult(quizID, true)` 1 回 |
| TC-S-V-03 | `submit` (incorrect) | `progress.recordResult(quizID, false)` 1 回 |
| TC-S-V-04 | `next` 後の `currentQuiz` | `quizzes[oldIndex + 1]` |
| TC-S-V-05 | 全問終了後の `hasNext` | `false` |

### 3.6 UI Snapshot (Xcode Preview ベース)

`@Preview` を利用、SnapshotTesting ライブラリ or Xcode 標準 Preview スクショ。

| ID | 対象 | 状態 |
|---|---|---|
| TC-S-U-01 | `ProseRenderer` | `.empty` |
| TC-S-U-02 | `ProseRenderer` | `.preview("answer")` |
| TC-S-U-03 | `ProseRenderer` | `.correct("answer")` |
| TC-S-U-04 | `ProseRenderer` | `.incorrect("wrong", "answer")` |
| TC-S-U-05 | `CodeRenderer` | `.empty` |
| TC-S-U-06 | `CodeRenderer` | `.correct("Println")` |
| TC-S-U-07 | `ChoiceButtonsView` | `phase == .playing`, `maskState == .empty` |
| TC-S-U-08 | `ChoiceButtonsView` | `phase == .playing`, `maskState == .preview("a")` |
| TC-S-U-09 | `ChoiceButtonsView` | `phase == .reviewing` (正解 / 不正解アイコン表示) |
| TC-S-U-10 | `ErrorView` | retryable / non-retryable 双方 |

MVP では UI snapshot を **必須にはしない** (Preview 目視 + 手動操作で代替可)。CI で機械検査するか否かは Stage 4 で判断。

---

## 4. CDN テスト

ファイル: `tools/scripts/test-cdn.sh` (手動 / cron)

| ID | コマンド | 期待 |
|---|---|---|
| TC-D-01 | `curl -I https://<cdn>/v2/quizzes.json` | 200 + `Content-Type: application/json` + `ETag` + `Cache-Control: public, max-age=300, stale-while-revalidate=86400` |
| TC-D-02 | `curl -H "If-None-Match: <etag>" -I ...` | 304 Not Modified |
| TC-D-03 | `curl -I http://<cdn>/v2/quizzes.json` | 301/308 → https:// にリダイレクト or 拒否 |
| TC-D-04 | `curl -H "Origin: https://example.com" -I ...` | `Access-Control-Allow-Origin: *` |
| TC-D-05 | `curl https://<cdn>/v2/quizzes.json \| jq '.version'` | `2` |
| TC-D-06 | レスポンスサイズ確認 | `< 2 MB` (NFR8.2 上限内) |

---

## 5. E2E (手動)

| ID | 手順 | 検証 |
|---|---|---|
| TC-E-01 | CLI で JSON 生成 → wrangler deploy → iOS シミュレータで起動 | 提案一覧 → クイズ → 結果まで完走 |
| TC-E-02 | 1 回目起動成功後、機内モードで再起動 | キャッシュで全機能動作 (FR2.18) |
| TC-E-03 | CDN の JSON を差し替え (新クイズ追加) → シミュレータ 2 回目起動 | 新クイズが反映される (US6 / AC13) |
| TC-E-04 | 初回起動でネット切断 | エラー画面 + 再試行ボタン (FR2.6 / AC10) |
| TC-E-05 | 初回起動成功 → アプリ削除 → ネット切断 → 再インストール起動 | エラー画面 (= 初回扱い) |
| TC-E-06 | クイズ最後まで完走 → 結果画面で正答率確認 | 進捗が UserDefaults に保存される |

---

## 6. セキュリティテスト

NFR8 / AC18-21 の検証。

| ID | 対象 | 内容 | 検証 |
|---|---|---|---|
| TC-Z-01 | NFR8.1 / AC18 | `Info.plist` 検査: `NSAppTransportSecurity` 削除/既定のまま | `plutil -p Info.plist \| grep -v NSAllowsArbitraryLoads` または同等 |
| TC-Z-02 | NFR8.1 | `Configuration.quizDataURL` が `https://` で始まる | `Configuration` の init 内 `precondition` を追加し、ユニットテストで `https` URL を確認 |
| TC-Z-03 | NFR8.2 / AC19 | 3 MB JSON モックで `LoadError.payloadTooLarge` | `MockURLProtocol` で `Content-Length: 3000000` を返す |
| TC-Z-04 | NFR8.2 | Content-Length 詐称 (1MB と申告して 3MB 返す) | 二重防御で payloadTooLarge throw |
| TC-Z-05 | NFR8.3 / AC20 | `{"type":"weird_kind"}` のブロック | `LoadError.schemaInvalid` |
| TC-Z-06 | NFR8.3 | `{"version": 1}` (旧スキーマ) | `LoadError.unsupportedVersion` |
| TC-Z-07 | NFR8.3 | mask が 2 つ含まれる quiz | `validateInvariants` で `LoadError.schemaInvalid` |
| TC-Z-08 | NFR8.3 | `answer` が `choices` に含まれない | 同上 |
| TC-Z-09 | NFR8.4 / AC21 | `cd quizgen && go mod verify` | exit 0 |
| TC-Z-10 | NFR8.4 | `go.sum` がリポジトリにコミットされている | `git ls-files \| grep go.sum` |
| TC-Z-11 | NFR8.5 | `.gitignore` に `.wrangler/` `.dev.vars` が含まれる | grep |
| TC-Z-12 | NFR8.5 | リポジトリに API token らしき文字列が存在しない | `git log -p \| rg -i "cloudflare.*token"` の手動チェック / `gitleaks` 自動化は将来 |
| TC-Z-13 | NFR8.6 | `URLSession` リクエストヘッダに独自 UA がない | `MockURLProtocol` の `URLRequest.allHTTPHeaderFields` を確認 |

---

## 7. 性能・負荷テスト

NFR1 系の検証。

| ID | 対象 | 内容 | 期待 |
|---|---|---|---|
| TC-P-01 | NFR1.1 | 100 個の proposal Markdown (合計 5 MB) を quizgen 処理 | `time` で ≤ 10 秒 (M1 Mac, MVP) |
| TC-P-02 | NFR1.1 | 同上 + `--seed` 変えても処理時間がブレない | ±20% 以内 |
| TC-P-03 | NFR1.2 | iOS シミュレータでコールドスタート → ProposalList 表示 | ≤ 2 秒 (Caches に既存 JSON あり) |
| TC-P-04 | NFR1.2 | ProposalList → QuizSession 遷移時間 | ≤ 200 ms |
| TC-P-05 | NFR1.1 (CLI) | メモリ使用量 (`/usr/bin/time -l`) | ピーク ≤ 200 MB |

MVP では TC-P-01,02 のみ自動化 (`go test -bench`)、TC-P-03,04 はシミュレータ手動。

---

## 8. テストデータ管理

### 8.1 `testdata/proposals/`

| ファイル | 役割 | 主な内容 |
|---|---|---|
| `99999-sample.md` (既存) | 基本動作の最小例 | H1 + 1 段落 + 1 ``` go ブロック + 1 inline code |
| `99998-prose-heavy.md` (新規) | prose seed の多様性 | inline code 8 個以上、stopword 混在、3 文字未満混在、重複混在 |
| `99997-code-heavy.md` (新規) | code seed の多様性 | ```go ブロック 3 個以上、関数/型/Call 識別子 |
| `99996-edge-empty.md` (新規) | エッジケース (0 件) | H1 のみ、本文・コードなし |
| `99995-edge-tricky.md` (新規) | パース難易度 | 入れ子バッククォート、長い行 (300+ 文字)、Go パース失敗するスニペット |
| `99994-edge-malformed.md` (新規) | パース耐性 | 閉じていない ```go フェンス、不完全 inline code |

### 8.2 `testdata/golden/`

| ファイル | 役割 |
|---|---|
| `quizzes-seed42.json` | 上記 testdata を seed=42 で生成した snapshot |
| `quizzes-seed7.json` | seed=7 (バリエーション) |

ゴールデン更新コマンド: `go test ./cmd/quizgen -update` フラグで再生成。差分は PR レビューで確認。

### 8.3 iOS テストフィクスチャ

`ios/GoMaskedQuizTests/Fixtures/`:

| ファイル | 役割 |
|---|---|
| `bundle-v2-minimal.json` | 1 proposal × 1 quiz の最小 v2 |
| `bundle-v2-prose-and-code.json` | prose / code 両方を含む |
| `bundle-v1-legacy.json` | `version: 1` (互換性テスト用 negative) |
| `bundle-malformed.json` | 必須フィールド欠落 |
| `bundle-unknown-kind.json` | 未知の `block.type` 含む |

---

## 9. CI 統合 (将来)

`.github/workflows/test.yml` (Phase 6 以降):

```yaml
name: test
on: [push, pull_request]
jobs:
  go:
    runs-on: ubuntu-latest
    defaults: { run: { working-directory: quizgen } }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.26' }
      - run: go mod verify
      - run: go vet ./...
      - run: go test ./... -count=1 -race -coverprofile=coverage.out
      - run: go tool cover -func=coverage.out

  ios:
    runs-on: macos-15
    steps:
      - uses: actions/checkout@v4
      - run: xcodebuild test -project ios/GoMaskedQuiz/GoMaskedQuiz.xcodeproj -scheme GoMaskedQuiz -destination 'platform=iOS Simulator,name=iPhone 15'
```

MVP では CI は Optional。ローカル `make test` 程度で開始。

---

## 10. 受け入れ基準とのマッピング

| AC | 関連テスト |
|---|---|
| AC1 (決定論) | TC-G-E-02, TC-G-S-* |
| AC2 (100+ クイズ生成) | E2E 手動 (実 golang-proposal で実測) |
| AC3 (iOS ゴールデンパス) | TC-E-01 |
| AC4 (機内モード) | TC-E-02, TC-S-L-05 |
| AC5 (3 層ライセンス表示) | grep + 目視 |
| AC6 (go vet/test 緑) | TC-G-* (全件) |
| AC7 (JSON スキーマ) | TC-G-S-* |
| AC8 (Swift 6 strict concurrency) | Xcode ビルドログ |
| AC9 (curl ヘッダ検証) | TC-D-01 |
| AC10 (初回失敗 UI) | TC-E-04, TC-S-L-06 |
| AC11 (2 回目失敗で継続) | TC-E-02 |
| AC12 (E2E パイプライン) | TC-E-01 |
| AC13 (リリースなし更新) | TC-E-03 |
| AC14 (Codable v2) | TC-S-M-01..05 |
| AC15 (mask 視覚) | TC-S-U-* |
| AC16 (Tap-to-fill) | TC-S-Q-01..06, TC-E-01 |
| AC17 (Submit フィードバック) | TC-S-U-03..04, TC-E-01 |
| AC18 (ATS) | TC-Z-01 |
| AC19 (2 MB 超) | TC-Z-03, TC-Z-04 |
| AC20 (不正 JSON) | TC-Z-05..08, TC-S-L-09..11 |
| AC21 (go mod verify) | TC-Z-09, TC-Z-10 |

---

## 11. オープン質問 (Stage 3 で残る論点)

| ID | 質問 | デフォルト案 |
|---|---|---|
| TQ1 | UI Snapshot を CI 必須にするか? | MVP では Optional (Preview 目視で代替) |
| TQ2 | Property-based テスト (gopter 等) を導入するか? | テーブル駆動 + 複数 seed のループで代替 (MVP) |
| TQ3 | iOS UI 自動操作テスト (XCUITest) を導入するか? | MVP は手動 (TC-E-*) |
| TQ4 | カバレッジ目標を CI で gating するか? | Optional (達成しなくても fail にしない) |
| TQ5 | gitleaks 等の secret scan を CI に入れるか? | Phase 6 以降で検討 |
| TQ6 | iOS のローカルテストのために `Configuration.quizDataURL` を環境別に切り替えるか? | DI で test 用 URL を注入できるようにする (本番ハードコードは維持) |
