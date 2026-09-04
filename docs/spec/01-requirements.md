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
| クイズ生成 CLI | `quizgen` (Go 製)。proposal Markdown を読み JSON を出力 |
| **CDN 配信** | **jsDelivr の GitHub CDN 機能**で `cdn/v2/quizzes.json` を静的配信。アプリリリースなしでクイズ内容を更新可能にする |
| モバイルアプリ | Go + [Gio](https://gioui.org) (`mobile/`)。**起動ごとに CDN から再取得**し、失敗時はローカルキャッシュで動作 |
| ライセンス遵守 | BSD 3-Clause 表示の 3 層化 (NOTICE / JSON メタ / Acknowledgments) |
| マスキング | 機械的 (goldmark + go/parser)。決定論的 RNG |

> **注記 (実装後の追記)**: 本ドキュメントは策定当時「iOS 単体アプリ (SwiftUI) + Cloudflare Pages」を前提に書かれたが、実装は Gio によるクロスプラットフォームアプリ (`mobile/`、デスクトップ / Android / iOS を同一コードでビルド) と jsDelivr 配信に置き換わっている。以下 FR2・FR5 および関連する NFR・AC・用語集はこの実態に合わせて改訂した。マスキングロジック (FR1) や CLI 設計は当初の想定のまま実装されており変更していない。

### 2.2 Out of Scope (将来)

- LLM 動的クイズ生成 (参考リポジトリの `LLMQuiz` 相当) — ただし `quiz.KindLLM` 等スキーマ上の受け口は既に用意されている
- 動的バックエンド API (ユーザーごとのカスタムクイズ生成等)。CDN による**静的 JSON 配信**は In Scope
- Web クライアント (デスクトップ / Android / iOS は Gio の同一コードでカバー済み)
- マルチ言語 UI (MVP は英語のみ)
- ユーザー間スコア共有 / ランキング
- App Store / TestFlight 配布手続き
- バンドル同梱のシード JSON (初回オフライン対応) — 埋め込みスナップショットは一度実装され、CDN パスが壊れた際にサイレントな旧データ表示を招いたため撤去された (`mobile/data.go` のコメント参照)

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

### US6: アプリリリースなしのクイズ更新

> プロジェクトオーナーとして、新しい proposal が追加されたりマスキングロジック
> を改善した際に、**App Store の審査・リリースを経ずに**ユーザーへ更新クイズ
> を届けたい。CLI で再生成 → CDN に push → 次回起動時に各ユーザーへ反映。

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
| FR1.13 | 出力 JSON のトップレベル `version` は **`2`**。`generated_at`, `source_repo`, `source_fork`, `source_commit`, `source_license`, `source_license_url`, `proposals[]` を含む |
| FR1.14 | 各 quiz は `id`, `kind ("prose"\|"code")`, `index`, **`blocks[]`**, `answer`, `choices[]` を持つ。旧 v1 の `context_before` / `masked_text` / `context_after` は廃止 (理由: クライアント側で再パースが不要になり、レンダリングが決定論的になる) |
| FR1.15 | `blocks[]` の各要素は `{type: "text"\|"inline_code"\|"code_block"\|"mask", value?: string}` の形式 |
| FR1.16 | `type: "mask"` の要素は `value` を持たない (空欄を表す)。それ以外の type は `value` を必須とする |
| FR1.17 | prose クイズの blocks は `text` / `inline_code` / `mask` の組み合わせ。`code_block` を含まない |
| FR1.18 | code クイズの blocks は `code_block` / `mask` の組み合わせ。`text` / `inline_code` を含まない |
| FR1.19 | どの quiz も `blocks[]` 内に **ちょうど 1 つ** `type: "mask"` を含む |

### FR2. モバイルアプリ (`mobile/`, Go + Gio)

| ID | 要件 | 実装状況 |
|---|---|---|
| FR2.1 | 起動時に CDN URL (`https://cdn.jsdelivr.net/gh/fummicc1/go-masked-quiz@main/cdn/v2/quizzes.json`) から `quizzes.json` を取得し、`quiz.Bundle` 型にデコード | 実装済み (`mobile/data.go` `fetchRemote`) |
| FR2.2 | 取得した JSON を Gio の `app.DataDir()` 配下 (`quizzes.json`) に永続化 | 実装済み (`mobile/data.go` `cacheFilePath`) |
| FR2.3 | 起動ごとに CDN へ再取得を試み、失敗した場合のみローカルキャッシュへフォールバックする | 実装済み。**stale-while-revalidate ではない**: キャッシュを即表示してから背景更新する2段構えではなく、まず remote を試し、失敗時に限りキャッシュを読む単純なフォールバックのみ (`mobile/data.go` `loadBundle`) |
| FR2.4 | HTTP リクエストは `ETag` / `If-None-Match` を尊重し、変更がない場合は 304 で帯域節約 | **未実装**。毎回フルボディを取得する (条件付きリクエストなし) |
| FR2.5 | タイムアウト: 10 秒 | 実装済み (`mobile/data.go` の `context.WithTimeout`)。リソースタイムアウトの区別はなし |
| FR2.6 | ネット失敗時、キャッシュも読めない場合: エラー画面を表示し「Retry」ボタン提供 | 実装済み (`mobile/ui_list.go` `layoutLoadFailed`) |
| FR2.7 | ネット失敗時、キャッシュが読める場合: キャッシュをそのまま使用し、取得元を画面下部に表示 (`source · remote` / `source · cache`) | 実装済み (`mobile/ui_list.go` の `source` キャプション) |
| FR2.8 | Proposal 一覧画面: 番号バッジ + タイトル + 進捗バー (未回答数 / 全 blank 数) を表示。検索ボックスでタイトル・番号を絞り込み可能 | 実装済み (`mobile/ui_list.go`) |
| FR2.9 | Proposal を選択すると、その proposal のドキュメント全体を 1 画面でスクロール表示する読者ビューに遷移する | 実装済み (`mobile/ui_doc.go` `screenQuiz`)。**「クイズ実行画面」という独立したモード遷移ではない** — 文書中の空欄 (mask) をその場でタップして埋めていく形 |
| FR2.10 | ドキュメントは見出し・本文・コードブロック・引用など Markdown の構造をブロック単位で描画し、`mask` は色付きチップで目立たせる | 実装済み (`mobile/ui_doc.go` の `docBlock`、`mobile/widgets.go`... 相当 の `buildBlockView`) |
| FR2.11 | コードブロックのシンタックスハイライト: 等幅フォントのみ (色付けなし) | 実装済み |
| FR2.12 | 回答インタラクション: mask をタップすると選択肢シートが下からせり上がり、選択肢をタップした時点で即座に確定する | 実装済み (`mobile/ui_doc.go` `choiceSheet` / `sheetChoice`)。**Submit ボタンによる確定・プレビューの上書きはない** — タップ = 即確定で、同じ blank は再タップ不可 (`maskTargets` が未回答の mask にのみタップ領域を張る) |
| FR2.13 | 確定後は mask のチップを緑 (正解) / 赤 (不正解) に変え、ラベルを正解の文字列に置き換える | 実装済み (`mobile/widgets.go` `buildBlockView` の `SpanMask` 分岐) |
| FR2.14 | 「Next」ボタンや問題単位の遷移はなく、ユーザーは文書をスクロールして次の空欄に進む。最後まで解いても専用の結果画面には遷移しない | 実装済み (仕様どおり単一画面) |
| FR2.15 | 正答率と進捗はドキュメント画面のヘッダーに常時表示する (`N/M blanks · X% correct`)。独立した「結果サマリー画面」はない | 実装済み (`mobile/ui_doc.go` `docHeader`) |
| FR2.16 | Acknowledgments 画面: NOTICE 相当のテキストと上流リポジトリへのリンク | 実装済み (`mobile/ui_about.go`)。一覧画面右上の「About」から遷移する「About & Licenses」画面で、`Bundle.SourceRepo` / `SourceLicense` / `SourceLicenseURL` (または `Bundle.Sources`) を読み、自アプリと上流双方の著作権表示 + BSD 3-Clause 全文を表示する |
| FR2.17 | 進捗保存: proposal ID + blank index → 回答・正誤を JSON ファイル (`scores.json`、`app.DataDir()` 配下) に永続化 | 実装済み (`mobile/score.go` `scoreStore`)。UserDefaults 相当の仕組みではなく、自前の JSON ファイルへの都度全体書き込み (atomic rename) |
| FR2.18 | 機内モード (キャッシュ取得後): 一覧・ドキュメント表示・回答記録・Acknowledgments の全機能が動作する | 実装済み。About 画面はロード済みの `Bundle` から読むだけでネットワーク不要 |
| FR2.19 | スキーマ非互換検出: 取得した JSON の `version` が現在の `quiz.SchemaVersion` (= 2) と**完全一致**しない場合は decode を拒否する | 実装済みだが**許容範囲ではなく厳密一致** (`mobile/data.go` `decodeBundle`)。バージョン不一致時はキャッシュへフォールバックし、キャッシュもなければエラー画面 |

### FR3. ライセンスコンプライアンス

| ID | 要件 | 実装状況 |
|---|---|---|
| FR3.1 | リポジトリルートに `NOTICE` を置き、上流 (The Go Authors, golang/proposal) と fork (fummicc1/golang-proposal) の著作権・BSD 3-Clause を明記 | 実装済み (`/NOTICE`) |
| FR3.2 | `output/quizzes.json` のメタに `source_repo`, `source_fork`, `source_commit`, `source_license`, `source_license_url` の 5 種を必ず含む。CDN 配信版もこのメタを保持 | 実装済み (`quizgen/quiz/model.go` `Bundle`) |
| FR3.3 | アプリ内 (旧称: iOS Acknowledgments 画面) に上流の著作権表示 + BSD 3-Clause 全文 URL を含む | 実装済み (`mobile/ui_about.go`)。「About & Licenses」画面に (1) 自アプリの Copyright (c) 2026 Fumiya Tanaka、(2) `Bundle` から読んだ上流の著作権表示・ライセンス種別・`SourceLicenseURL`、(3) BSD 3-Clause の条件・免責事項全文、を表示する。URL へのリンク遷移 (ブラウザを開く) はせず、テキストとして表示するのみ |
| FR3.4 | アプリ説明文・宣伝文で "The Go Authors" の名称を販促目的で使用しない (BSD 第 3 条) | 該当なし (MVP はストア配布前) |

### FR4. 開発者向けワークフロー

| ID | 要件 |
|---|---|
| FR4.1 | `git clone` → `go run ./cmd/quizgen generate --proposals <local>/design` で JSON が生成できる |
| FR4.2 | `golang-proposal` の clone はリポジトリに含めない (`.gitignore` で `third_party/` 除外)。理由: iCloud Drive 配下のため submodule の checkout が File Provider 干渉で決定論的に失敗するため |
| FR4.3 | `go test ./...` が緑になる |

### FR5. クイズデータ CDN 配信 (jsDelivr)

| ID | 要件 | 実装状況 |
|---|---|---|
| FR5.1 | `cdn/v2/quizzes.json` を **jsDelivr の GitHub CDN 機能**で配信する。専用のホスティング基盤は持たず、パブリック GitHub リポジトリのファイルをそのまま配信元にする | 実装済み |
| FR5.2 | 配信 URL はバージョンパス付き (`https://cdn.jsdelivr.net/gh/fummicc1/go-masked-quiz@main/cdn/v2/quizzes.json`)。`v2/` は JSON top-level `version` と整合 | 実装済み (`mobile/data.go` `quizDataURL`) |
| FR5.3 | レスポンスヘッダ (`Content-Type`, `Cache-Control`, 圧縮) は jsDelivr が既定で付与する。Cloudflare Pages の `_headers` に相当するカスタマイズ手段はなく、リポジトリ側で制御できない | jsDelivr 既定に依存 |
| FR5.4 | ETag は jsDelivr が既定で付与するが、クライアント側 (FR2.4) はまだこれを利用していない | ヘッダは付与されるがクライアント未対応 |
| FR5.5 | デプロイ操作は不要。`git push` で `main` ブランチの `cdn/v2/quizzes.json` を更新すれば、jsDelivr のキャッシュ更新後に配信内容が変わる (`@main` のような可変refはjsDelivr側で比較的短いキャッシュ寿命になる点に留意) | 実装済み。`wrangler` 等のデプロイツールは不要 |
| FR5.6 | `cdn/v2/quizzes.json` の更新は `.github/workflows/generate.yml` が単一の書き手として担う (README「Automated refresh (CDN)」参照) | 実装済み |
| FR5.7 | 公開後、`curl -I https://cdn.jsdelivr.net/gh/.../cdn/v2/quizzes.json` で 200 / Content-Type / キャッシュ関連ヘッダを確認できる | 手動検証可能 |
| FR5.8 | CDN 障害時の挙動: アプリはキャッシュがあればキャッシュで継続動作 (FR2.7)。キャッシュもなければリトライ画面 (FR2.6) | 実装済み |

## 6. 非機能要件 (NFR)

| ID | カテゴリ | 要件 |
|---|---|---|
| NFR1.1 | 性能 (CLI) | 100 proposals (約 5MB の Markdown) の処理を 10 秒以内に完了 |
| NFR1.2 | 性能 (モバイルアプリ) | アプリ起動から一覧表示まで 2 秒以内、画面遷移 200ms 以内 |
| NFR2.1 | 再現性 | 同一 `(seed, source files, max-per-proposal, choices)` の生成結果は `generated_at` を除き完全に同一バイト列 |
| NFR3.1 | オフライン | 起動ごとに CDN 再取得を試みるため厳密には「初回のみネット必須」ではないが、取得失敗時はローカルキャッシュで全機能オフライン動作 (FR2.3) |
| NFR3.2 | 通信量 | 受信ボディは 32 MB で読み込み上限 (`mobile/data.go` `maxBundleBytes`)。ETag による差分取得は未実装 (FR2.4) のため、起動ごとに全量を再取得する |
| NFR3.3 | CDN 可用性 | jsDelivr の可用性に依存。SLA は公開されていないが実質的に高可用。MVP では独自 SLA を持たない |
| NFR4.1 | 保守性 | Go モジュールはパッケージ分離 (`parser`, `masker`, `quiz`, `cmd/quizgen`) |
| NFR4.2 | 保守性 | 主要パッケージ (`parser`, `masker`) にユニットテストがある |
| NFR4.3 | 保守性 | モバイルアプリは 1 パッケージ (`mobile/`, `package main`) 内でファイル分割 (`data.go` 取得/キャッシュ、`score.go` 進捗、`ui_list.go`/`ui_doc.go` 画面、`widgets.go`/`theme.go` 見た目)。SwiftUI 版で想定していた feature 単位ディレクトリ構成は取っていない |
| NFR5.1 | ポータビリティ (CLI) | macOS / Linux / Windows で動作 (標準ライブラリ + goldmark のみ) |
| NFR5.2 | ポータビリティ (モバイルアプリ) | Gio によりデスクトップ / Android / iOS を同一コードでビルド (README 参照)。iOS は Xcode 署名を伴う `mobile/build-ios.sh` でビルドし、Swift 6 strict concurrency は非該当 (Swift を使わない) |
| NFR6.1 | プライバシー | 個人情報・トラッキング・テレメトリは収集しない (MVP) |
| NFR7.1 | ローカル開発環境 | iCloud Drive 配下のリポジトリで `go build` / `go test` が動作する (Go ビルドキャッシュは iCloud 外を使う) |
| NFR8.1 | セキュリティ (通信) | 通信は `net/http` (`http.DefaultClient`) 経由で **HTTPS のみ** (`quizDataURL` はハードコードされた `https://` URL)。iOS の ATS / `Info.plist` に相当する OS 標準の TLS 検証に依存し、アプリ側で追加設定は行っていない |
| NFR8.2 | セキュリティ (サイズ上限) | 受信ボディは `io.LimitReader` で 32 MB に制限 (`maxBundleBytes`)。`Content-Length` の事前チェックはなく、読み込み時の上限のみの一段防御 |
| NFR8.3 | セキュリティ (スキーマ検証) | `encoding/json` の decode 失敗、または `version` の不一致 (FR2.19) で安全に読み込み拒否。失敗時はキャッシュ継続 (あれば) / エラー画面 (なければ)。`block.type` 等の網羅的な不変則検証は行っていない |
| NFR8.4 | セキュリティ (依存管理) | Go モジュール依存は `go.sum` でハッシュ固定。CI で `go vet ./... && go mod verify` を実行 (将来 CI 採用時)。Dependabot を有効化 |
| NFR8.5 | セキュリティ (token 管理) | 該当なし。jsDelivr はパブリックリポジトリのファイルを配信するだけで、デプロイや認証を伴わないため Cloudflare API token の管理は不要になった |
| NFR8.6 | セキュリティ (プライバシー) | リクエストの User-Agent は Go の `net/http` デフォルト値。アプリ独自の識別子・テレメトリ・クラッシュレポート (Sentry 等) は MVP では導入しない |

## 7. ライセンス・法的制約

- 上流 `golang/proposal`: BSD 3-Clause、Copyright (c) The Go Authors
- 自リポジトリ `LICENSE`: BSD 3-Clause、Copyright (c) 2026 Fumiya Tanaka
- 派生物 (`output/quizzes.json`、CDN 配信版、モバイルアプリのローカルキャッシュ) は上流 BSD 3-Clause を継承し、3 層 (NOTICE / JSON メタ / アプリ内 Acknowledgments) で再配布時の表示義務を満たす。アプリ内表示層は `mobile/ui_about.go` の About 画面として実装済み (FR3.3)
- CDN 公開 (jsDelivr) は派生物の**再配布**にあたるため、配信されるすべての JSON にメタ情報を含めて出典を保持する

## 8. 受け入れ基準 (Acceptance Criteria)

| AC | 内容 | 検証手段 |
|---|---|---|
| AC1 | CLI が testdata から決定論的に JSON を生成 | `go run ... --seed 42` を 2 回実行、`generated_at` を除き完全一致 |
| AC2 | 実 `golang-proposal/design/*.md` から MVP として 100 問以上のクイズを生成 | 数百 KB の `quizzes.json` を目視確認 + 件数チェック |
| AC3 | モバイルアプリで proposal 一覧 → ドキュメント表示 → 空欄への回答が完走 | `cd mobile && go run .` でゴールデンパス手動確認 |
| AC4 | 機内モードでモバイルアプリが (キャッシュ取得後) 全機能動作 | ネットワーク遮断状態での手動確認 |
| AC5 | NOTICE / JSON メタ / アプリ内 Acknowledgments (About 画面) の 3 層すべてに表示が存在 | grep / 目視。About 画面は `go test ./mobile/... -run TestRenderAbout` が書き出す `mobile/testdata/screen-about.png` でも確認可能 |
| AC6 | `go vet ./...` / `go test ./...` 緑 | CI もしくはローカル実行 |
| AC7 | クイズ JSON が規定スキーマを満たす | JSON Schema 検証または Go decode 成功 |
| AC8 | ~~iOS ビルドが Swift 6 strict concurrency で警告なし~~ → `go vet ./mobile/...` が警告なし | Go ビルドログ (Swift を使わないため元の基準は不成立) |
| AC9 | CDN URL (`cdn.jsdelivr.net/gh/...`) に対する `curl -I` が 200 + `Content-Type: application/json` を返す | コマンドライン検証。ETag / Cache-Control の値は jsDelivr 側の既定に依存し保証はしない |
| AC10 | ネット不可かつキャッシュもない場合、エラー画面 + 再試行ボタンが表示される | ネットワーク遮断テスト |
| AC11 | ネット不可でもキャッシュがあれば、全機能が動作する | ネットワーク遮断後の再起動で確認 |
| AC12 | CLI 出力 → `cdn/v2/quizzes.json` コミット → jsDelivr 配信 → モバイルアプリ取得 → 表示 のパイプラインが手動で完走 | エンドツーエンド手動検証 |
| AC13 | モバイルアプリ側でアプリリリースなしに JSON 更新が反映される | CDN の JSON を差し替え、次回起動で内容変更を確認 |
| AC14 | `blocks[]` / `Document` スキーマで生成された JSON がモバイルアプリで `encoding/json` デコード成功 | ユニットテスト |
| AC15 | モバイルアプリで blocks が描画され、`mask` が他の text/inline_code/code_block と視覚的に区別できる | 実機/デスクトップ目視 |
| AC16 | Tap-to-fill 挙動: mask タップで選択肢シートが開き、選択肢タップで即時確定する (Submit ボタン・プレビュー上書きはない) | 実機/デスクトップ手動操作 |
| AC17 | 確定後のフィードバック: mask のチップが緑/赤に染まり、ラベルが正解の文字列に置き換わる | 実機/デスクトップ目視 |
| AC18 | ~~iOS `Info.plist` の `NSAppTransportSecurity` が ATS 既定~~ → `quizDataURL` が `https://` で始まることをコードレビューで確認 | ソースコード検査 (plist は存在しない) |
| AC19 | 32 MB 超のレスポンスを送っても `io.LimitReader` で読み込みが打ち切られ、アプリがクラッシュせずエラー扱いになる | モックサーバーで負荷データ注入 |
| AC20 | `version` 不一致 / 不正 JSON で decode がエラーを返し、キャッシュ継続 or エラー画面に遷移する | デコーダ単体テスト + 統合テスト |
| AC21 | `go mod verify` が緑、`go.sum` がコミットされている | CLI 実行 |

## 9. 仮定 (Assumptions)

- A1. 開発者は手元の iCloud 外パスに `fummicc1/golang-proposal` を `git clone` できる
- A2. 開発者は Go 1.26 以上の環境を持つ。iOS ビルドを行う場合は追加で Xcode (署名用) が必要 (`mobile/build-ios.sh`)
- A3. proposal Markdown の構造 (見出し、` ```go ` フェンス、inline code の規則) は短期的に大きく変わらない
- A4. デスクトップ (`go run .`) での動作を最低限の動作確認とする。Android / iOS の実機検証は都度手動
- A5. `fummicc1/go-masked-quiz` が GitHub 上でパブリックであり続ける (jsDelivr の GitHub CDN 機能はパブリックリポジトリのみを配信対象とする)
- A6. ユーザーは初回起動時にネットワーク接続を持っている (モバイル / Wi-Fi)

## 10. リスクと緩和策

| ID | リスク | 影響 | 緩和策 |
|---|---|---|---|
| R1 | golang-proposal の Markdown 構造変化で parser が壊れる | クイズ品質低下 | goldmark の標準 AST を使い、構造依存を最小化。テストデータをスナップショット化 |
| R2 | Go コードスニペットの構文が parser でパース不能 | code クイズの欠落 | `parser.SkipObjectResolution` + `package _x` ラップでフォールバック。失敗ブロックはスキップ (FR1.10) |
| R3 | iCloud Drive のファイル同期遅延でビルド/git 操作が不安定 | 開発体験悪化 | `third_party/` を `.gitignore`、submodule を使わない (FR4.2)。`GOCACHE` を iCloud 外に設定 |
| R4 | BSD 3-Clause 表示の漏れによる法的リスク | ライセンス違反 | 3 層表示を AC5 で検証し、リリース前チェックリストに組み込む |
| R5 | クイズの難易度バランスが偏る | 学習効果低下 | `--max-per-proposal` と prose:code = 3:2 の目標比で平準化。Phase 4 以降で難易度ラベルを検討 |
| R6 | LLM 拡張 (Phase 5) で JSON スキーマが破壊的変更になる | 旧バージョンのクライアントとの非互換 | スキーマに `version` を含め、`kind` を enum で拡張可能に。`kind: "llm"` は既に追加済み (`quiz.KindLLM`)。FR2.19 でクライアント側の非互換検出も実装 (厳密一致のみ) |
| R7 | CDN 障害 (jsDelivr 全断) でユーザーの起動が失敗 | ユーザーの離脱 | キャッシュがあればキャッシュで継続動作 (FR2.7)。実害はキャッシュを持たない初回ユーザーに限定。リトライ画面 (FR2.6) を提供 |
| R8 | jsDelivr は無料の公共サービスであり、SLA や料金体系の保証がない (Cloudflare Pages のような有償プランへの切替手段がない) | 運用コスト以前に可用性そのものが外部サービス次第 | 低トラフィックの個人開発リポジトリとして許容範囲と判断。悪化時はセルフホスト CDN (Cloudflare Pages 含む) への移行を検討 |
| R9 | 旧バージョンのアプリが新スキーマ JSON を取得して破綻 | クラッシュ・無反応 | 実装は `version` の**厳密一致**チェックのみ (FR2.19)。レンジ許容や `/v3/` 分岐などの後方互換戦略は未整備で、破壊的変更時は全クライアントが同時に追従する必要がある |
| R10 | CDN 配信される派生物が `golang/proposal` の更新タイミングと乖離 | 古いクイズが配信され続ける | `.github/workflows/generate.yml` が日次で自動更新 (README「Automated refresh (CDN)」参照)。単一の書き手であることをワークフローのコメントで明示 |
| R11 | CDN 配信 JSON が改ざんされる (CDN 側侵害、設定ミス、MITM) | 不正クイズ・任意 URL リダイレクト | TLS で MITM 防御 (NFR8.1)。CDN 内部改ざんは MVP では受容。Phase 5+ で `quizzes.json.sha256` の併置 → クライアント側ハッシュ検証、さらに Ed25519 署名検証を検討 |
| R12 | 巨大 JSON / 不正 JSON でアプリが OOM・クラッシュ | 起動失敗・利用不能 | NFR8.2 (32 MB 上限)、NFR8.3 (スキーマ検証で safely throw)。decode 失敗時はキャッシュ継続 |
| R13 | 依存パッケージのサプライチェーン攻撃 (goldmark 等の悪意版差し込み) | 任意コード実行 | NFR8.4: `go.sum` ハッシュ固定 + `go mod verify` + Dependabot。最小限の依存 (goldmark のみ) を維持 |

## 11. 用語集

| 用語 | 定義 |
|---|---|
| proposal | `golang/proposal/design/NNNN-<slug>.md` の Markdown 文書 |
| prose クイズ | proposal の本文中 inline code (`` ` `` で囲まれたトークン) をマスクしたクイズ |
| code クイズ | proposal の `` ```go `` ブロック内の関数名/型名/呼び出し先をマスクしたクイズ |
| 派生物 (derived work) | proposal 本文断片を含む `quizzes.json` およびモバイルアプリのローカルキャッシュ内クイズデータ |
| seed | `quizgen` の `--seed` フラグ。決定論的 RNG の初期値 |
| tag | `masker.NewRNG(seed, tag)` の文字列引数。proposal slug + 処理段階で分岐させる |
| CDN | Content Delivery Network。本プロジェクトでは jsDelivr の GitHub CDN 機能 (`cdn.jsdelivr.net/gh/...`) を指す |
| stale-while-revalidate | キャッシュを返しつつバックグラウンドで再取得し、次回からは新版を返す HTTP キャッシング戦略。**本アプリは採用していない** — 起動ごとに remote を試し、失敗時のみキャッシュへフォールバックする単純な方式 (FR2.3) |
| ETag | レスポンスの一意な識別子。次回 `If-None-Match` で送ると未変更なら 304 が返り帯域節約できる。jsDelivr は付与するが、クライアントはまだ利用していない (FR2.4) |
| ローカルキャッシュ | Gio の `app.DataDir()` 配下の `quizzes.json` (`mobile/data.go`)。OS の容量逼迫時に削除される可能性がある領域 |
| block | quiz JSON 内の表示単位。`type` で `text` / `inline_code` / `code_block` / `mask` を区別 |
| mask | quiz 内の穴。`{type: "mask"}` ブロックとして表現され、UI 上は枠線・背景色で強調表示 |
| ~~preview state~~ | 策定時に想定していた「選択肢タップ後、Submit 前の未確定表示」の状態。実装では選択肢タップ = 即時確定のためこの中間状態は存在しない (FR2.12) |
