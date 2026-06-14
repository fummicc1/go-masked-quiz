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
| **CDN 配信** | **Cloudflare Pages** で `quizzes.json` を静的配信。アプリリリースなしでクイズ内容を更新可能にする |
| iOS アプリ | SwiftUI、iOS 17+。**初回起動時に CDN から取得**し、以降はローカルキャッシュで動作 |
| ライセンス遵守 | BSD 3-Clause 表示の 3 層化 (NOTICE / JSON メタ / Acknowledgments) |
| マスキング | 機械的 (goldmark + go/parser)。決定論的 RNG |

### 2.2 Out of Scope (将来)

- LLM 動的クイズ生成 (参考リポジトリの `LLMQuiz` 相当)
- 動的バックエンド API (ユーザーごとのカスタムクイズ生成等)。CDN による**静的 JSON 配信**は In Scope
- Android、Web 等 iOS 以外のクライアント
- マルチ言語 UI (MVP は英語のみ)
- ユーザー間スコア共有 / ランキング
- App Store / TestFlight 配布手続き
- バンドル同梱のシード JSON (初回オフライン対応)

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
| FR1.14 | 各 quiz は `id`, `kind ("prose"\|"code")`, `index`, **`blocks[]`**, `answer`, `choices[]` を持つ。旧 v1 の `context_before` / `masked_text` / `context_after` は廃止 (理由: iOS 側で再パースが不要になり、レンダリングが決定論的になる) |
| FR1.15 | `blocks[]` の各要素は `{type: "text"\|"inline_code"\|"code_block"\|"mask", value?: string}` の形式 |
| FR1.16 | `type: "mask"` の要素は `value` を持たない (空欄を表す)。それ以外の type は `value` を必須とする |
| FR1.17 | prose クイズの blocks は `text` / `inline_code` / `mask` の組み合わせ。`code_block` を含まない |
| FR1.18 | code クイズの blocks は `code_block` / `mask` の組み合わせ。`text` / `inline_code` を含まない |
| FR1.19 | どの quiz も `blocks[]` 内に **ちょうど 1 つ** `type: "mask"` を含む |

### FR2. iOS アプリ (`GoMaskedQuiz`)

| ID | 要件 |
|---|---|
| FR2.1 | 初回起動時に CDN URL (`https://<cloudflare-pages-domain>/v1/quizzes.json`) から `quizzes.json` を取得し、`QuizBundle` 型にデコード |
| FR2.2 | 取得した JSON を `FileManager.SearchPathDirectory.cachesDirectory` (`Library/Caches/quizzes.json`) に永続化 |
| FR2.3 | 2 回目以降の起動: ローカルキャッシュを即座に表示しつつ、バックグラウンドで CDN を再取得 (stale-while-revalidate)。差分があれば次回起動から反映 |
| FR2.4 | HTTP リクエストは `ETag` / `If-None-Match` を尊重し、変更がない場合は 304 で帯域節約 |
| FR2.5 | タイムアウト: 接続 10 秒、リソース 30 秒 |
| FR2.6 | 初回起動でネット失敗: エラー画面を表示し「再試行」ボタン提供 (キャッシュなしのため続行不可) |
| FR2.7 | 2 回目以降でネット失敗: キャッシュをそのまま使用し、エラーは無視 (ログのみ) |
| FR2.8 | Proposal 一覧画面: タイトル + 問題数を表示し、進捗バッジ (正解数 / 全問数) を表示 |
| FR2.9 | Proposal を選択するとクイズ実行画面に遷移 |
| FR2.10 | クイズ実行画面: `blocks[]` を順に描画する。`text` は通常スタイル、`inline_code` は等幅 + 控えめな背景、`code_block` は等幅のコード領域、`mask` は強調スタイル (枠線・背景色) で目立たせる |
| FR2.11 | コードクイズのシンタックスハイライト: **MVP では等幅フォントのみ** (色付けなし)。Phase 4 以降で拡張可 |
| FR2.12 | 回答インタラクション: **Tap-to-fill + Submit** モデル。選択肢ボタンタップで `mask` に preview 表示 (未確定)、別の選択肢タップで preview を上書き可能 |
| FR2.13 | `Submit` ボタンで回答を確定。確定後は `mask` の色を緑 (正解) / 赤 (不正解) に変え、正解の選択肢を別途ハイライト表示 |
| FR2.14 | `Next` ボタンで次の問題へ。最後の問題後は結果サマリー画面へ自動遷移 |
| FR2.15 | 結果サマリー画面: 正答率 + 各問の正解 / 不正解一覧 |
| FR2.16 | Acknowledgments 画面: NOTICE 相当のテキストと上流リポジトリへのリンク |
| FR2.17 | 進捗保存: クイズ ID → 正解実績の boolean を UserDefaults に永続化 |
| FR2.18 | 機内モード (キャッシュ取得後): 出題・結果・Acknowledgments の全機能が動作する |
| FR2.19 | スキーマ非互換検出: 取得した JSON の `version` が許容範囲外なら、キャッシュを使い続けつつアプリ更新を促すメッセージを表示 (キャッシュもない場合はエラー画面) |

### FR3. ライセンスコンプライアンス

| ID | 要件 |
|---|---|
| FR3.1 | リポジトリルートに `NOTICE` を置き、上流 (The Go Authors, golang/proposal) と fork (fummicc1/golang-proposal) の著作権・BSD 3-Clause を明記 |
| FR3.2 | `output/quizzes.json` のメタに `source_repo`, `source_fork`, `source_commit`, `source_license`, `source_license_url` の 5 種を必ず含む。CDN 配信版もこのメタを保持 |
| FR3.3 | iOS Acknowledgments 画面に上流の著作権表示 + BSD 3-Clause 全文 URL を含む。CDN 由来のキャッシュデータについても同等の表示義務を満たす |
| FR3.4 | アプリ説明文・宣伝文で "The Go Authors" の名称を販促目的で使用しない (BSD 第 3 条) |

### FR4. 開発者向けワークフロー

| ID | 要件 |
|---|---|
| FR4.1 | `git clone` → `go run ./cmd/quizgen generate --proposals <local>/design` で JSON が生成できる |
| FR4.2 | `golang-proposal` の clone はリポジトリに含めない (`.gitignore` で `third_party/` 除外)。理由: iCloud Drive 配下のため submodule の checkout が File Provider 干渉で決定論的に失敗するため |
| FR4.3 | `go test ./...` が緑になる |

### FR5. クイズデータ CDN 配信 (Cloudflare Pages)

| ID | 要件 |
|---|---|
| FR5.1 | `output/quizzes.json` を **Cloudflare Pages** プロジェクト (例: `go-masked-quiz-data`) で配信する |
| FR5.2 | 配信 URL はバージョンパス付き (例: `https://<project>.pages.dev/v1/quizzes.json` または カスタムドメイン)。`v1/` は JSON top-level `version` と整合 |
| FR5.3 | レスポンスヘッダ: `Content-Type: application/json; charset=utf-8`, `Cache-Control: public, max-age=300, stale-while-revalidate=86400`, 圧縮 (Cloudflare が gzip/brotli を自動付与) |
| FR5.4 | ETag を必ず付与 (Cloudflare 既定で OK) |
| FR5.5 | デプロイは GitHub Actions または `wrangler pages deploy` の手動実行のいずれか。CI 自動化は MVP では Optional (手動デプロイで開始) |
| FR5.6 | リポジトリの `cdn/` (or `dist/`) に Cloudflare Pages の公開対象を配置。`output/quizzes.json` → `cdn/v1/quizzes.json` をビルドステップでコピー |
| FR5.7 | 公開後、`curl -I` で 200 / ETag / Cache-Control / Content-Type を検証できる |
| FR5.8 | CDN 障害時の挙動: iOS アプリは 2 回目以降ならキャッシュで継続動作 (FR2.7)。初回起動はリトライ画面 (FR2.6) |

## 6. 非機能要件 (NFR)

| ID | カテゴリ | 要件 |
|---|---|---|
| NFR1.1 | 性能 (CLI) | 100 proposals (約 5MB の Markdown) の処理を 10 秒以内に完了 |
| NFR1.2 | 性能 (iOS) | アプリ起動から一覧表示まで 2 秒以内、画面遷移 200ms 以内 |
| NFR2.1 | 再現性 | 同一 `(seed, source files, max-per-proposal, choices)` の生成結果は `generated_at` を除き完全に同一バイト列 |
| NFR3.1 | オフライン | iOS アプリは**初回起動時のみネット必須**。CDN 取得成功後はローカルキャッシュで全機能オフライン動作 |
| NFR3.2 | 通信量 | 初回取得時のペイロード ≤ 1 MB (圧縮後)。差分取得は ETag による 304 で帯域節約 |
| NFR3.3 | CDN 可用性 | Cloudflare Pages の SLA に依存 (実質 99.9%+)。MVP では独自 SLA を持たない |
| NFR4.1 | 保守性 | Go モジュールはパッケージ分離 (`parser`, `masker`, `quiz`, `cmd/quizgen`) |
| NFR4.2 | 保守性 | 主要パッケージ (`parser`, `masker`) にユニットテストがある |
| NFR4.3 | 保守性 | iOS アプリは feature 単位ディレクトリで構造化 (`Features/ProposalList`, `Features/QuizSession`, ...) |
| NFR5.1 | ポータビリティ (CLI) | macOS / Linux / Windows で動作 (標準ライブラリ + goldmark のみ) |
| NFR5.2 | ポータビリティ (iOS) | iOS 17+。Swift 6 strict concurrency 警告なし |
| NFR6.1 | プライバシー | 個人情報・トラッキング・テレメトリは収集しない (MVP) |
| NFR7.1 | ローカル開発環境 | iCloud Drive 配下のリポジトリで `go build` / `go test` が動作する (Go ビルドキャッシュは iCloud 外を使う) |
| NFR8.1 | セキュリティ (通信) | iOS の通信は **HTTPS のみ**。`Info.plist` の ATS をデフォルトのまま (= 強制) とし、TLS 1.2+ を要求。`NSAllowsArbitraryLoads = false` |
| NFR8.2 | セキュリティ (サイズ上限) | 受信 JSON のサイズ上限は **2 MB**。`Content-Length` 事前チェック + ボディ読み込み上限の二重防御。超過時はエラー扱い (DoS / メモリ枯渇対策) |
| NFR8.3 | セキュリティ (スキーマ検証) | JSON スキーマ検証は必須フィールド欠落、未知の `kind` / `block.type` で安全に decode 失敗。失敗時はキャッシュ継続 (あれば) / エラー画面 (なければ) |
| NFR8.4 | セキュリティ (依存管理) | Go モジュール依存は `go.sum` でハッシュ固定。CI で `go vet ./... && go mod verify` を実行 (将来 CI 採用時)。Dependabot を有効化 |
| NFR8.5 | セキュリティ (token 管理) | Cloudflare API token は GitHub Repository Secrets で管理。ローカル開発の `wrangler` 認証ファイルは `.gitignore` 対象。token 漏洩時のリボーク手順を README に記載 |
| NFR8.6 | セキュリティ (プライバシー) | リクエストの User-Agent は iOS デフォルトの匿名 UA。アプリ独自の識別子・テレメトリ・クラッシュレポート (Sentry 等) は MVP では導入しない |

## 7. ライセンス・法的制約

- 上流 `golang/proposal`: BSD 3-Clause、Copyright (c) The Go Authors
- 自リポジトリ `LICENSE`: BSD 3-Clause、Copyright (c) 2026 Fumiya Tanaka
- 派生物 (`output/quizzes.json`、CDN 配信版、iOS ローカルキャッシュ) は上流 BSD 3-Clause を継承し、3 層 (NOTICE / JSON メタ / iOS Acknowledgments) で再配布時の表示義務を満たす
- CDN 公開 (Cloudflare Pages) は派生物の**再配布**にあたるため、配信されるすべての JSON にメタ情報を含めて出典を保持する

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
| AC9 | CDN URL に対する `curl -I` が 200 + `Content-Type: application/json` + `ETag` + `Cache-Control` を返す | コマンドライン検証 |
| AC10 | 初回起動でネット不可の場合、エラー画面 + 再試行ボタンが表示される | シミュレータでネット遮断テスト |
| AC11 | 2 回目以降ネット不可の場合、キャッシュで全機能が動作する | シミュレータの Airplane Mode で 2 周目起動 |
| AC12 | CLI 出力 → CDN 公開 → iOS 取得 → クイズ表示 のパイプラインが手動で完走 | エンドツーエンド手動検証 |
| AC13 | iOS 側でアプリリリースなしに JSON 更新が反映される | CDN の JSON を差し替え、シミュレータの 2 回目起動で内容変更を確認 |
| AC14 | `blocks[]` スキーマで生成された JSON が iOS で `Codable` デコード成功 | ユニットテスト |
| AC15 | iOS で blocks が描画され、`mask` が他の text/inline_code/code_block と視覚的に区別できる | シミュレータ目視 |
| AC16 | Tap-to-fill 挙動: 選択肢タップで mask に preview、Submit 前なら別選択肢で上書き可能、Submit 後は固定 | シミュレータ手動操作 |
| AC17 | Submit 後のフィードバック: mask が緑/赤に染まり、正解の選択肢がハイライトされる | シミュレータ目視 |
| AC18 | iOS `Info.plist` の `NSAppTransportSecurity` が ATS 既定 (= TLS 1.2+ 強制、HTTP 不可) | plist 検査 |
| AC19 | 2 MB 超の JSON を意図的に投げてもアプリがクラッシュせずエラー扱いになる | URLProtocol モックで負荷データ注入 |
| AC20 | 必須フィールド欠落 / 未知 `kind` / 未知 `block.type` の JSON で decode が throw し、キャッシュ継続 or エラー画面に遷移する | デコーダ単体テスト + 統合テスト |
| AC21 | `go mod verify` が緑、`go.sum` がコミットされている | CLI 実行 |

## 9. 仮定 (Assumptions)

- A1. 開発者は手元の iCloud 外パスに `fummicc1/golang-proposal` を `git clone` できる
- A2. 開発者は Go 1.26 以上、Xcode 26 以上の環境を持つ
- A3. proposal Markdown の構造 (見出し、` ```go ` フェンス、inline code の規則) は短期的に大きく変わらない
- A4. iOS シミュレータでの動作を最低限の動作確認とする (実機検証は MVP 後)
- A5. プロジェクトオーナーは Cloudflare アカウントを保有し、Cloudflare Pages プロジェクトを作成できる (無料枠で十分)
- A6. ユーザーは初回起動時にネットワーク接続を持っている (モバイル / Wi-Fi)

## 10. リスクと緩和策

| ID | リスク | 影響 | 緩和策 |
|---|---|---|---|
| R1 | golang-proposal の Markdown 構造変化で parser が壊れる | クイズ品質低下 | goldmark の標準 AST を使い、構造依存を最小化。テストデータをスナップショット化 |
| R2 | Go コードスニペットの構文が parser でパース不能 | code クイズの欠落 | `parser.SkipObjectResolution` + `package _x` ラップでフォールバック。失敗ブロックはスキップ (FR1.10) |
| R3 | iCloud Drive のファイル同期遅延でビルド/git 操作が不安定 | 開発体験悪化 | `third_party/` を `.gitignore`、submodule を使わない (FR4.2)。`GOCACHE` を iCloud 外に設定 |
| R4 | BSD 3-Clause 表示の漏れによる法的リスク | ライセンス違反 | 3 層表示を AC5 で検証し、リリース前チェックリストに組み込む |
| R5 | クイズの難易度バランスが偏る | 学習効果低下 | `--max-per-proposal` と prose:code = 3:2 の目標比で平準化。Phase 4 以降で難易度ラベルを検討 |
| R6 | LLM 拡張 (Phase 5) で JSON スキーマが破壊的変更になる | iOS 旧版との非互換 | スキーマに `version` を含め、`kind` を enum で拡張可能に。`kind: "llm"` を将来追加。FR2.16 でクライアント側の非互換検出も実装 |
| R7 | CDN 障害 (Cloudflare 全断) でユーザーの初回起動が失敗 | 新規ユーザーの離脱 | 2 回目以降はキャッシュで継続動作 (FR2.7)。実害は初回起動中のユーザーに限定。リトライ画面 (FR2.6) を提供 |
| R8 | Cloudflare の料金体系変更で無料枠を逸脱 | 運用コスト発生 | MVP は無料枠 (1 ビルド/月以下、月 100k リクエスト未満を想定) で運用。超過時に jsDelivr 等への移行を検討 |
| R9 | 旧バージョンのアプリが新スキーマ JSON を取得して破綻 | クラッシュ・無反応 | JSON top-level `version` のレンジチェック (FR2.16)。スキーマ後方互換を維持しつつ、互換不可な変更は CDN パスを `/v2/` 等に分けて旧版を残す |
| R10 | CDN 配信される派生物が `golang/proposal` の更新タイミングと乖離 | 古いクイズが配信され続ける | デプロイの責任を明示する運用ドキュメントを README に追加。Phase 2 以降で CI 自動化を検討 |
| R11 | CDN 配信 JSON が改ざんされる (CDN 側侵害、設定ミス、MITM) | 不正クイズ・任意 URL リダイレクト | TLS で MITM 防御 (NFR8.1)。CDN 内部改ざんは MVP では受容。Phase 5+ で `quizzes.json.sha256` の併置 → iOS 側ハッシュ検証、さらに Ed25519 署名検証を検討 |
| R12 | 巨大 JSON / 不正 JSON でアプリが OOM・クラッシュ | 起動失敗・利用不能 | NFR8.2 (2 MB 上限)、NFR8.3 (スキーマ検証で safely throw)。decode 失敗時はキャッシュ継続 |
| R13 | 依存パッケージのサプライチェーン攻撃 (goldmark 等の悪意版差し込み) | 任意コード実行 | NFR8.4: `go.sum` ハッシュ固定 + `go mod verify` + Dependabot。最小限の依存 (goldmark のみ) を維持 |

## 11. 用語集

| 用語 | 定義 |
|---|---|
| proposal | `golang/proposal/design/NNNN-<slug>.md` の Markdown 文書 |
| prose クイズ | proposal の本文中 inline code (`` ` `` で囲まれたトークン) をマスクしたクイズ |
| code クイズ | proposal の `` ```go `` ブロック内の関数名/型名/呼び出し先をマスクしたクイズ |
| 派生物 (derived work) | proposal 本文断片を含む `quizzes.json` および iOS バンドル内クイズデータ |
| seed | `quizgen` の `--seed` フラグ。決定論的 RNG の初期値 |
| tag | `masker.NewRNG(seed, tag)` の文字列引数。proposal slug + 処理段階で分岐させる |
| CDN | Content Delivery Network。本プロジェクトでは Cloudflare Pages を指す |
| stale-while-revalidate | キャッシュを返しつつバックグラウンドで再取得し、次回からは新版を返す HTTP キャッシング戦略 |
| ETag | レスポンスの一意な識別子。次回 `If-None-Match` で送ると未変更なら 304 が返り帯域節約できる |
| ローカルキャッシュ | iOS の `Library/Caches/quizzes.json`。OS の容量逼迫時に削除される可能性がある領域 |
| block | quiz JSON 内の表示単位。`type` で `text` / `inline_code` / `code_block` / `mask` を区別 |
| mask | quiz 内の穴。`{type: "mask"}` ブロックとして表現され、UI 上は枠線・背景色で強調表示 |
| preview state | 選択肢を tap した後、Submit 前の状態。`mask` の表示は選択した文字列に置き換わるが、まだ正誤判定はされない |
