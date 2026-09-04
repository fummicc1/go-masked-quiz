# テスト設計仕様: go-masked-quiz MVP

ステージ3 / 4 — 仕様駆動開発。前提: `01-requirements.md`, `02-design.md`。

> **注記 (実装後の追記)**: 本ドキュメントは策定当時「iOS 単体アプリ (SwiftUI) + Cloudflare Pages」を前提に書かれた。実装はクロスプラットフォームの Go + Gio アプリ (`mobile/`) と jsDelivr 配信に置き換わっており、3 章 (旧「iOS テストケース」) と 4 章 (CDN テスト) をこの実態に合わせて改訂した。テスト ID の接頭辞 (`TC-S-*`, `TC-D-*` 等) は既存参照との互換のため変更していない。2 章 (Go CLI 側) は当初の想定にほぼ沿って実装されているため変更していない。

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

layer:    G = Go CLI / S = mobile app (Go + Gio。策定当時は Swift/iOS を想定していたため
              接頭辞に S が残っている) / D = CDN / E = E2E / Z = Security / P = Performance
category: P = parser, M = masker, B = blocks, E = CLI E2E, S = schema,
          M = Models (quiz.Bundle 等), L = Loader (フェッチ/キャッシュ), Q = blank の回答状態,
          V = 進捗保存, U = UI Snapshot
```

例: `TC-G-B-04` = Go の blocks パッケージのテスト 4 件目。

### 1.3 カバレッジ目標 (NFR4.2)

| 層 | 目標 |
|---|---|
| `internal/parser` | 行カバレッジ ≥ 80% |
| `internal/masker` | 行カバレッジ ≥ 85% |
| `internal/blocks` | 行カバレッジ ≥ 90% (新規・複雑) |
| `cmd/quizgen` | ゴールデンテストで E2E カバレッジ |
| mobile (デコード) | `quiz.Bundle` の decode パスを網羅 |
| mobile (フェッチ/キャッシュ) | 主要分岐 (200/失敗/サイズ超/version 不一致) を網羅。304 分岐は該当なし (ETag 未実装) |
| mobile (回答・進捗) | blank への回答 → `scoreStore` への反映を網羅 |

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

## 3. モバイルアプリ テストケース (`mobile/`, Go + Gio)

### 3.1 デコード (`quiz.Bundle`)

ファイル: `mobile/data_test.go`

`ModelsTests.swift` に相当する Codable の網羅的な decode テスト（型ごとの
`Block.text`/`inlineCode`/`codeBlock`/`mask` 分岐、未知の `kind` での throw 等）は
存在しない — `quiz.Bundle` は `encoding/json` の構造体タグでそのまま decode され、
`quizgen` 側の型定義とクライアントが型を共有しているため、型分岐の網羅テストは
主に `quizgen/quiz` 側の責務になる。実際にある decode 系テストは:

| ID | 内容 | 期待 | 状態 |
|---|---|---|---|
| TC-S-M-01 | 現行スキーマの固定バージョン (`quiz.SchemaVersion`) と、クライアントがフェッチする URL のバージョンパス (`.../cdn/v2/...`) が一致する | 一致 | 実装済み (`TestQuizDataURLMatchesSchemaVersion`) |
| TC-S-M-02 | `mobile/testdata/bundle.json` (テスト用フィクスチャ) が現行スキーマで decode できる | 成功 | 実装済み (`TestFixtureIsCurrentSchema`) |
| TC-S-M-03 | `version` フィールドが `quiz.SchemaVersion` と不一致 | `decodeBundle` がエラーを返す | 未カバー (`decodeBundle` のユニットテストは未整備。`fetchRemote` 経由の統合テストのみ) |
| TC-S-M-04 | 不正な JSON (フィールド欠落等) | `json.Unmarshal` がエラーを返す | 未カバー |

### 3.2 フェッチ + キャッシュ (`loadBundle` / `fetchRemote`)

ファイル: `mobile/data_test.go`

`QuizLoader`/`QuizCache` のような独立したコンポーネントはなく、`loadBundle` 1 関数が
両方を担う。ETag・`If-None-Match`・304 分岐は実装がないため対応するテストケースも
ない。

| ID | シナリオ | 入力 | 期待 | 状態 |
|---|---|---|---|---|
| TC-S-L-01 | remote 取得失敗 | キャッシュなし | エラーを返す | 実装済み (`TestLoadBundleReportsFailure`) |
| TC-S-L-02 | remote 取得失敗 | キャッシュに有効な JSON あり | キャッシュ内容を `SourceCache` として返す | 実装済み (`TestLoadBundleUsesCacheWhenFetchFails`) |
| ~~TC-S-L-02'~~ (304 Not Modified) | — | — | 該当なし (ETag 未実装のため 304 という状態自体が発生しない) | — |
| TC-S-L-07 | 200 + body 32 MB 超 | キャッシュ有無問わず | `io.LimitReader` で読み込みが 32 MB (`maxBundleBytes`) で打ち切られ、不正な JSON としてデコード失敗する | 未カバー (モックサーバーでの検証が必要) |
| TC-S-L-09 | 200 + `version` が `quiz.SchemaVersion` と不一致 | キャッシュなし | `decodeBundle` がエラー → `loadBundle` もエラー | 未カバー |
| TC-S-L-12 | If-None-Match 送信確認 | — | 該当なし。リクエストヘッダに ETag 相当を付与する実装がない | — |

### 3.3 blank の回答状態 (状態機械なし)

`MaskState` のような enum ベースの状態機械は存在しない。blank は「未回答」か
「回答済み (`answer{Choice, Correct}`)」の 2 値のみを持ち、選択肢シート
(`ui_quiz.go` `choiceSheet`/`sheetChoice`) をタップした瞬間に確定する。プレビュー・
上書き・Submit の分岐は実装がないため、対応するテストケースもない。

| ID | 初期 | 操作 | 期待 | 状態 |
|---|---|---|---|---|
| TC-S-Q-01 | 未回答 | 選択肢シートで選択肢をタップ | `scoreStore.record` が呼ばれ、シートが閉じる | 実装済み (`TestRenderChoiceSheet` で描画経路を確認) |
| TC-S-Q-02 | シート表示中 | シート外 (scrim) をタップ | シートが閉じ、回答は記録されない | 実装済み (`TestChoiceSheetDismissWithoutSelection`) |
| TC-S-Q-03 | 回答済み | 同じ blank を再タップ | タップ領域がない (`maskTargets` が回答済みの blank にターゲットを張らない) ため無反応 | 未カバー (ユニットテストなし) |
| ~~TC-S-Q-04~~ (Submit / プレビュー上書き) | — | — | 該当なし。そのような中間状態を持たない | — |

### 3.4 進捗保存 (`scoreStore`)

ファイル: なし（`mobile/score.go` に対応する `*_test.go` は未整備）

| ID | 内容 | 期待 | 状態 |
|---|---|---|---|
| TC-S-V-01 | `record` 後の `progress` | answered/correct が反映される | 未カバー |
| TC-S-V-02 | `record` 後、プロセス再起動を模した `newScoreStore(dir)` | ディスク上の `scores.json` から復元される | 未カバー |
| TC-S-V-03 | `reset` 後の `progress` | `(0, 0)` に戻る | 未カバー |

`internal/masker` や `internal/blocks` (2 章) と異なり、`score.go` には現時点で
専用のユニットテストがない。これは既存のギャップとして記録する。

### 3.5 UI Snapshot (headless GPU)

ファイル: `mobile/render_test.go`。`@Preview` + SnapshotTesting ライブラリの代わりに、
`gioui.org/gpu/headless` で実際にレイアウト・ペイントを実行し `image.RGBA` を得る。

| ID | 対象 | 状態 | 状態 (実装) |
|---|---|---|---|
| TC-S-U-01 | 一覧画面、ロード失敗時 | エラー文言 + Retry ボタン | 実装済み (`TestRenderLoadFailed`) |
| TC-S-U-02 | 一覧画面、通常表示 | proposal のカード一覧 | 実装済み (`TestRenderList`) |
| TC-S-U-03 | 一覧画面、検索フィルタ後 | 絞り込まれたカードのみ | 実装済み (`TestRenderListFiltered`) |
| TC-S-U-04 | ドキュメント画面 | 見出し・本文・mask チップを含むブロック列 | 実装済み (`TestRenderDocument`) |
| TC-S-U-05 | 選択肢シート表示中 | シートのオーバーレイ | 実装済み (`TestRenderChoiceSheet`) |
| TC-S-U-06 | 選択肢シート、選択せず閉じる | シートが閉じた状態 | 実装済み (`TestChoiceSheetDismissWithoutSelection`) |
| TC-S-U-07 | About 画面 (FR3.3) | 自アプリ・上流双方の著作権表示と BSD 3-Clause 全文を含む 3 カード | 実装済み (`TestRenderAbout`)。一覧⇄About の画面遷移自体は `TestAboutNavigation` (非スナップショット) でカバー |

`ProseRenderer`/`CodeRenderer`/`ChoiceButtonsView`/`ErrorView` という個別コンポーネント
単位のスナップショットではなく、画面単位 (`u.layout` 呼び出し) でのスナップショットに
なっている。

---

## 4. CDN テスト

ファイル: 専用スクリプトはない。手動 `curl` 検証。`<cdn>` =
`cdn.jsdelivr.net/gh/fummicc1/go-masked-quiz@main`。

`_headers` によるヘッダのカスタマイズ (`Cache-Control` の具体値強制、CORS の
`Access-Control-Allow-Origin: *` 明示) は jsDelivr では行えないため、それを
前提にした検証はできない。jsDelivr の既定挙動を観測するテストに置き換える。

| ID | コマンド | 期待 | 備考 |
|---|---|---|---|
| TC-D-01 | `curl -I https://<cdn>/cdn/v2/quizzes.json` | 200 + `Content-Type: application/json` | `Cache-Control` / `ETag` の具体値は jsDelivr 既定に依存し固定値を期待しない |
| ~~TC-D-02~~ (If-None-Match → 304) | — | クライアントが ETag を送らないため未検証 (3.2 参照) | クライアント未対応 |
| TC-D-03 | `curl -I http://<cdn>/cdn/v2/quizzes.json` | https:// へリダイレクトされる (jsDelivr 側の既定) | |
| ~~TC-D-04~~ (CORS ヘッダ明示) | — | jsDelivr は CORS を許可する既定ヘッダを返すが、`_headers` で明示制御しているわけではない | 参考情報として観測のみ |
| TC-D-05 | `curl https://<cdn>/cdn/v2/quizzes.json \| jq '.version'` | `2` | |
| TC-D-06 | レスポンスサイズ確認 | クライアントの読み込み上限 32 MB (`maxBundleBytes`) 未満であることを確認 | NFR8.2 の上限値を 2 MB → 32 MB に更新済み (01-requirements.md 参照) |

---

## 5. E2E (手動)

| ID | 手順 | 検証 |
|---|---|---|
| TC-E-01 | CLI で JSON 生成 → `cdn/v2/quizzes.json` を commit/push → `cd mobile && go run .` で起動 | 提案一覧 → ドキュメント表示 → blank への回答まで完走 (専用の「結果画面」はない) |
| TC-E-02 | 1 回目起動成功後、ネットワークを切断して再起動 | キャッシュで全機能動作 (FR2.18) |
| TC-E-03 | CDN の JSON を差し替え (新クイズ追加) → 再起動 | 新クイズが反映される (US6 / AC13)。起動ごとに remote 再取得するため「2 回目起動」という条件は不要 |
| TC-E-04 | ネット切断 + キャッシュなし (例: `app.DataDir()` を空にした状態) で起動 | エラー画面 + Retry ボタン (FR2.6 / AC10) |
| TC-E-05 | 初回起動成功 → キャッシュファイルを手動削除 → ネット切断 → 再起動 | エラー画面 (キャッシュなし扱い) |
| TC-E-06 | blank に回答 → アプリ再起動 | 回答済みの blank が正誤色つきで復元される (`scores.json` からの復元) |

---

## 6. セキュリティテスト

NFR8 / AC18-21 の検証。

| ID | 対象 | 内容 | 検証 |
|---|---|---|---|
| TC-Z-01 | NFR8.1 / AC18 | ~~`Info.plist` の ATS 検査~~ | 該当なし (plist は存在しない)。代わりに `quizDataURL` が `https://` 定数であることをソースレビューで確認 |
| TC-Z-02 | NFR8.1 | `quizDataURL` が `https://` で始まる | 定数値のユニットテストで確認可能 (現状は `TestQuizDataURLMatchesSchemaVersion` がバージョン部分のみ検証。プレフィックス検証は未追加) |
| TC-Z-03 | NFR8.2 / AC19 | 32 MB 超のレスポンスで読み込みが打ち切られる | モックサーバー (`httptest.Server`) で 32 MB 超のボディを返し、`fetchRemote` がエラーになることを確認。未カバー |
| ~~TC-Z-04~~ (Content-Length 詐称の二重防御) | — | 該当なし。`Content-Length` の事前チェックを行っていないため、詐称対策という設計自体がない (`maxBundleBytes` による事後の読み込み制限のみ) | — |
| TC-Z-05 | NFR8.3 / AC20 | 未知の `block.type` を含む JSON | `json.Unmarshal` はフィールドの型が合えば成功しうる (Go の decode は Swift の網羅的 enum decode ほど厳格ではない)。未知の `type` 文字列自体は `quiz.BlockType`（単純な `string` 型）として素通りしうる点に注意 — 検証未整備 |
| TC-Z-06 | NFR8.3 | `{"version": 1}` (旧スキーマ) | `decodeBundle` がエラーを返す (現行 `quiz.SchemaVersion` との厳密不一致)。未カバー |
| ~~TC-Z-07~~ / ~~TC-Z-08~~ (mask 個数・answer∈choices の不変則検証) | — | 該当なし。クライアント側に `validateInvariants` 相当の処理がない (02-design.md 9.3 参照) | — |
| TC-Z-09 | NFR8.4 / AC21 | `cd quizgen && go mod verify` | exit 0 |
| TC-Z-10 | NFR8.4 | `go.sum` がリポジトリにコミットされている (`quizgen/go.sum`, `mobile/go.sum`) | `git ls-files \| grep go.sum` |
| ~~TC-Z-11~~ / ~~TC-Z-12~~ (Cloudflare token 管理) | — | 該当なし。jsDelivr への切替でデプロイ token・認証情報自体が存在しなくなった | — |
| TC-Z-13 | NFR8.6 | HTTP リクエストヘッダに独自 UA がない | `httptest.Server` でリクエストヘッダを記録し `User-Agent` が Go の既定値であることを確認。未カバー |

---

## 7. 性能・負荷テスト

NFR1 系の検証。

| ID | 対象 | 内容 | 期待 |
|---|---|---|---|
| TC-P-01 | NFR1.1 | 100 個の proposal Markdown (合計 5 MB) を quizgen 処理 | `time` で ≤ 10 秒 (M1 Mac, MVP) |
| TC-P-02 | NFR1.1 | 同上 + `--seed` 変えても処理時間がブレない | ±20% 以内 |
| TC-P-03 | NFR1.2 | `go run .` でコールドスタート → 一覧表示 | ≤ 2 秒 (キャッシュに既存 JSON あり) |
| TC-P-04 | NFR1.2 | 一覧 → ドキュメント画面の遷移時間 | ≤ 200 ms |
| TC-P-05 | NFR1.1 (CLI) | メモリ使用量 (`/usr/bin/time -l`) | ピーク ≤ 200 MB |
| TC-P-06 | (参考) | `mobile/memory_test.go` `TestBlockLayoutStaysWithinMemoryBudget` — 大きなコードブロック (~38k 文字) のレイアウトがメモリ上限内に収まる | 実装済み。`maxBlockChars` (4000) による切り詰めが効いていることを確認 |

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

### 8.3 mobile テストフィクスチャ

`mobile/testdata/`:

| ファイル | 役割 |
|---|---|
| `bundle.json` | 現行スキーマのサンプル bundle。`TestFixtureIsCurrentSchema` が decode を検証 |

当初想定していた `bundle-v1-legacy.json` (旧バージョン)・`bundle-malformed.json`
(フィールド欠落)・`bundle-unknown-kind.json` (未知の `block.type`) に相当する
異常系フィクスチャはまだ用意されていない (3.1/3.2 の「未カバー」項目に対応)。

---

## 9. CI 統合 (将来)

`.github/workflows/test.yml` (Phase 6 以降):

```yaml
name: test
on: [push, pull_request]
jobs:
  quizgen:
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

  mobile:
    runs-on: ubuntu-latest
    defaults: { run: { working-directory: mobile } }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.26' }
      - run: go mod verify
      - run: go vet ./...
      - run: go test ./... -count=1 -race
```

`xcodebuild` / macOS runner は不要になった。`mobile/render_test.go` は
`gioui.org/gpu/headless` を使うため、GPU の使える CI ランナーが必要な場合がある点に
注意 (ヘッドレス GL コンテキストが確保できない環境では `t.Skip` される)。

MVP では CI は Optional。ローカル `make test` 程度で開始。

---

## 10. 受け入れ基準とのマッピング

| AC | 関連テスト |
|---|---|
| AC1 (決定論) | TC-G-E-02, TC-G-S-* |
| AC2 (100+ クイズ生成) | E2E 手動 (実 golang-proposal で実測) |
| AC3 (モバイルアプリのゴールデンパス) | TC-E-01 |
| AC4 (機内モード) | TC-E-02, TC-S-L-02 |
| AC5 (3 層ライセンス表示) | grep + 目視。アプリ内表示層は `TestRenderAbout` / `TestAboutNavigation` (`mobile/render_test.go`) でカバー |
| AC6 (go vet/test 緑) | TC-G-* (全件)、mobile 側は `go vet ./mobile/... && go test ./mobile/...` |
| AC7 (JSON スキーマ) | TC-G-S-* |
| AC8 (Go ビルドが警告なし。元の Swift 6 concurrency 基準は不成立) | `go vet ./mobile/...` |
| AC9 (curl ヘッダ検証) | TC-D-01 |
| AC10 (キャッシュなしでの失敗 UI) | TC-E-04, TC-S-L-01 |
| AC11 (キャッシュありで継続) | TC-E-02, TC-S-L-02 |
| AC12 (E2E パイプライン) | TC-E-01 |
| AC13 (リリースなし更新) | TC-E-03 |
| AC14 (`encoding/json` デコード成功) | TC-S-M-01, TC-S-M-02 |
| AC15 (mask 視覚) | TC-S-U-04, TC-S-U-05 |
| AC16 (タップで即時確定) | TC-S-Q-01, TC-E-01 |
| AC17 (回答後のフィードバック) | TC-S-U-04, TC-E-01 |
| AC18 (`https://` 定数の確認。元の ATS 基準は不成立) | TC-Z-01, TC-Z-02 |
| AC19 (32 MB 超) | TC-Z-03 |
| AC20 (不正 JSON / version 不一致) | TC-Z-05, TC-Z-06, TC-S-M-03, TC-S-M-04 |
| AC21 (go mod verify) | TC-Z-09, TC-Z-10 |

---

## 11. オープン質問 (Stage 3 で残る論点)

| ID | 質問 | デフォルト案 |
|---|---|---|
| TQ1 | UI Snapshot (headless GPU の PNG) を CI で画像比較まで行うか? | 現状は毎回上書きするだけで比較していない。MVP では目視 (`git diff` での画像差分確認) で代替 |
| TQ2 | Property-based テスト (gopter 等) を導入するか? | テーブル駆動 + 複数 seed のループで代替 (MVP) |
| TQ3 | モバイルアプリの実機/実UI自動操作テストを導入するか? | MVP は手動 (TC-E-*)。Gio 向けの標準的な UI 自動操作フレームワークはまだ採用していない |
| TQ4 | カバレッジ目標を CI で gating するか? | Optional (達成しなくても fail にしない) |
| TQ5 | gitleaks 等の secret scan を CI に入れるか? | jsDelivr 切替により Cloudflare token 漏洩リスクは解消したが、他の secret 一般に対する導入自体は Phase 6 以降で検討 |
| TQ6 | `quizDataURL` をテスト用に切り替え可能にするか? | 現状はハードコードされた定数。DI で test 用 URL を注入できるようにするかは未決定 |
