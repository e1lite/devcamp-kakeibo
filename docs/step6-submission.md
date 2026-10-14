# Step 6 提出物

種別：**スナップショット**（Step 6 提出時点で固定し、以降は更新しない）。その後の変更は、次の Step 以降の提出物の 0 章に記録する

サービス名：決済メール解析による自動家計簿サービス
リポジトリ：https://github.com/e1lite/devcamp-kakeibo

| 提出項目 | 場所 |
|---|---|
| 概要 | 本ファイルの ① |
| ソースコード | https://github.com/e1lite/devcamp-kakeibo/tree/main/frontend（構成は [frontend/README.md](../frontend/README.md)） |
| 画面の動作確認 | 本ファイルの ②（スクリーンショットは [docs/step6-screenshots/](./step6-screenshots/)） |
| 静的解析・自動テスト | 本ファイルの ③（出力の全文は [docs/step6-check-output.txt](./step6-check-output.txt)） |
| まとめ | 本ファイルの ④ |
| （範囲外）メール取込の最小版 | 本ファイルの 0 章 D（計画と記録は [docs/mail-import-mvp.md](./mail-import-mvp.md)） |

---

## 0. 前の Step からの変更

### A. イベント設計の前提にしていたバックエンドの変更（6 件、実装済み）

Step 4・5 で API 仕様書には反映していたが、バックエンドが未実装だったもの（[イベント設計 4 章](./step5-events.md)）。
Step 5 の提出物では「前提 4 件」と書いていたが、Step 5 の途中で 2 件（5・6）増えていた。

| # | 変更 | 前提にしていた画面 |
|---|---|---|
| 1 | API-003：ブラウザ遷移時のエラーを JSON で返さず、フロントへ `#error=<コード>` でリダイレクトする | SCR-003 → SCR-002 のエラー表示 |
| 2 | API-010：`category_id=none` で未分類（`category_id IS NULL`）を絞り込む | SCR-010 のカテゴリ「未分類」 |
| 3 | API-031 決済手段一覧（`GET /api/v1/payment-methods`）を追加 | SCR-010・011・013 の決済手段の選択肢 |
| 4 | 「未分類」を `category_id = NULL` に一本化。初期カテゴリの `未分類` を削除するマイグレーション（000007） | 画面全体の「未分類」 |
| 5 | 他ユーザのリソースには `403` ではなく、存在しない場合と同じ `404` を返す | SCR-012・013・099 |
| 6 | 初期カテゴリ（`is_system`）も削除できるようにし、`400 SYSTEM_CATEGORY_NOT_DELETABLE` を廃止 | SCR-020 |

1 はハンドラの単体テスト（7 ケース）、2・3 はリポジトリの統合テスト、すべてを `scripts/verify-all.sh` の動作確認（100 件）で確かめた。

### B. 実装で設計から変えたこと（4 件）

| # | 変更 | 理由 |
|---|---|---|
| 1 | **Next.js 16 の `cacheComponents` を使わない**（create-next-app の既定では有効） | 有効にすると、画面を移っても前の画面が消えずに隠れるだけになり（React の Activity）、入力途中のフォームや開いたダイアログが残る。イベント設計は「開くたびに取り直す」「登録後は空のフォーム」が前提で、食い違う。また全画面が `sessionStorage` のトークンで API を呼ぶ Client Component のため、サーバ側の事前描画・キャッシュの利点も無い |
| 2 | **取引が見つからない（`404`）ときの SCR-099 は、共通ヘッダーの中に出す** | SCR-099 にヘッダーを出さないのは「ログインしていなくても表示する画面のため」（Step 4）だった。取引の `404` はログインした状態で取引詳細・編集を開いたときにだけ起き、ヘッダーから一覧やカテゴリへ移れる方が使いやすい。URL は変えず、文言（「取引が見つかりません」）も設計のまま。存在しない URL と想定外の例外は設計どおりヘッダーなし |
| 3 | **フォントは OS の和文フォント（ヒラギノ角ゴ・游ゴシックなど）を使う**（画面レイアウトでは Zen Kaku Gothic New などの Web フォントを先頭に指定） | Web フォントはビルド時に外部の配信元から取得する。Step 7 以降のコンテナのビルドを、外部の配信元に依存させないため。金額の「等幅数字」は `font-variant-numeric: tabular-nums` で満たした |
| 4 | **カテゴリ管理のスマホの操作は、アイコンではなく文字のボタン（「編集」「削除」）** | アイコンだけでは「削除」の意味が伝わりにくく、押せる範囲（44px 四方）も文字のボタンで確保できたため |

### C. Step 5 のフィードバックへの対応

| フィードバック | 対応 |
|---|---|
| 相談事項 1：拡張 10 画面は Step 8 で | コアの 9 画面を実装した。拡張のうち、メール取込の最小版（SCR-050）だけは先に作った（0 章 D）。講座の Step 8 は「インフラ 2」のため、残りの拡張の進め方を相談事項 1 で相談したい |
| 相談事項 2：各文書に「スナップショット / 生きた文書」の種別を明示する | Step 4・5 の文書は提出時のまま残し、変更は本ファイルの 0 章に記録した。種別のラベルは未対応（相談事項 2） |
| Tips：「API を呼ぶイベントを 1 つに集める」を実装とドキュメントに | `useTransactionList`（[src/features/transactions/hooks/](../frontend/src/features/transactions/hooks/useTransactionList.ts)）だけが API-010 を呼ぶ形にし、理由を [src/features/transactions/README.md](../frontend/src/features/transactions/README.md) に書いた |
| Tips：画面レイアウトの HTML に固定の目次 | 未対応 |

### D. メール取込の最小版（拡張の一部を先に作った）

Step 6 の範囲（コアの 9 画面）とは別に、テーマの中心である「通知メールから家計簿が自動でできる」を先に動かすため、メール取込の最小版を作った（相談事項 1）。
計画・設計判断・計画から変えたことは [docs/mail-import-mvp.md](./mail-import-mvp.md) にまとめている。

| 項目 | 内容 |
|---|---|
| できること | 「設定 → メール連携」で Gmail を連携し、「今すぐ同期」を押すと、過去 90 日の三井住友カードの利用通知を取り込んで取引にする |
| 画面 | SCR-050 メール連携（最小版）を追加。取込んだ取引は、既存の取引一覧・詳細に「速報」の印（確定前の金額）と取込元「メール」で出る |
| API | API-006〜009（連携一覧・解除・手動同期・同期履歴）、API-029・030（連携の開始・コールバック） |
| テーブル | `sync_jobs`・`parser_templates`・`email_messages`・`payment_events`（マイグレーション 000008〜000011） |
| 結果 | 利用者の Gmail から 142 通を取得し、**139 件を取引にした**（抽出の失敗 0）。2 回目の同期では同じメールを二重に取り込まない |
| 作っていないもの | 名寄せ（SCR-040）・未判定メールの教示（SCR-041）・定期実行・通知など。SCR-050 の画面レイアウトとイベント設計（Step 5 の形式）もまだ書いていない |

---

## ① 概要

### 構成

| 項目 | 内容 |
|---|---|
| フレームワーク | **Next.js 16.4（App Router）** / React 19 / TypeScript 5 |
| スタイリング | Tailwind CSS 4。色は画面レイアウトの変数に合わせ、ダークモード（OS の設定）にも対応 |
| API の呼び出し | Client Component の `fetch`。`/api/*` を Next.js の rewrites で API サーバ（`localhost:8080`）へ転送し、**同一オリジン**として呼ぶ（API は CORS ヘッダを返さないため） |
| 認証 | Google OAuth（API-002 / 003）で受け取った Bearer トークンを `sessionStorage` に置く（Step 4 判断 9） |
| 状態管理 | ライブラリは使わない。絞り込み条件は URL、ログイン中のユーザと完了メッセージは共通レイアウト、データは画面ごとに取得（Step 5 の想定どおり） |
| 静的解析 | `tsc --noEmit` / ESLint 9（`eslint-config-next`。React Hooks と React Compiler の規則を含む） |
| 自動テスト | Vitest（画面に依存しない純粋な関数の単体テスト 67 件） |

### 実装した画面

| 画面 | URL | 呼ぶ API |
|---|---|---|
| SCR-001 ルート | `/` | なし（トークンの有無で振り分け） |
| SCR-002 ログイン | `/login` | API-002（ブラウザごと遷移） |
| SCR-003 ログイン処理中 | `/auth/callback` | なし |
| 共通レイアウト | （SCR-010〜020） | API-004（読み込み時に 1 回）、API-005 |
| SCR-010 取引一覧 | `/transactions` | API-010、API-019、API-031 |
| SCR-011 取引登録 | `/transactions/new` | API-011、API-019、API-031 |
| SCR-012 取引詳細 | `/transactions/{id}` | API-012、API-014 |
| SCR-013 取引編集 | `/transactions/{id}/edit` | API-012、API-013、API-019、API-031 |
| SCR-020 カテゴリ管理 | `/settings/categories` | API-019、API-020、API-021 |
| SCR-099 エラー | 該当なしの URL ほか | なし |
| （拡張）SCR-050 メール連携 | `/settings/mail-accounts` | API-006〜009、API-029（同期中は数秒ごとに API-006・009 を取り直す） |

### 工夫した点

1. **API の呼び出しを 1 か所にまとめた**（[src/lib/api/client.ts](../frontend/src/lib/api/client.ts)）。トークンの付与、エラーの形の変換、`401` のときにログイン画面へ送る処理を各画面に書かない。画面を開くと API を並行して呼ぶため、`401` が同時に返っても遷移は 1 回だけにした
2. **取引一覧の絞り込みは URL を書き換えるだけにし、API-010 を呼ぶのは `useTransactionList` の 1 か所**（イベント設計の方針 #1）。条件が変わったら前の取得を `AbortController` で中断し、遅れて返った古い結果で一覧が上書きされないようにした
3. **送る値をイベント設計の表どおりに組み立てる関数を、画面から切り出した**（[form.ts](../frontend/src/features/transactions/form.ts)・[listQuery.ts](../frontend/src/features/transactions/listQuery.ts)）。編集は変えた項目だけを送る、未分類に戻すと `null`、カテゴリを変えたら `learn_category: false`、店舗で絞り込むときは期間を渡さない、など。表の行がそのまま単体テストのケースになる
4. **完了メッセージは遷移先で 1 回だけ出す**。共通レイアウトの state に持ち、遷移先の画面に着いたら 4 秒出して消す。リロードでは再び出ない
5. **「戻る」は履歴を戻り、アプリの中に戻れる履歴が無ければ一覧へ**。URL を直接開いたときに「戻る」でアプリの外へ出てしまわないよう、アプリの中での画面の移動を数えている
6. **アクセシビリティ**。入力欄とラベル・エラーを `label` / `aria-describedby` で結ぶ、送信時は最初のエラー欄へフォーカスを移す、ダイアログの初期フォーカスは「キャンセル」、閉じたら開いたボタンへフォーカスを戻す、押せる範囲は 44px 四方以上
7. **日時は JST 固定で変換する**（[date.ts](../frontend/src/lib/date.ts)）。ブラウザのタイムゾーンに関係なく、「当月」や登録した取引の月を JST で決める

---

## ② 画面の動作確認

API サーバ（`make run`）とフロント（`npm run dev`）を起動し、ブラウザで Google ログインから操作した。
スクリーンショットは、Google ログインで得たトークンを使い、ヘッドレスの Chrome（puppeteer）で同じ操作をして撮った。

> メールから取込んだ取引はすべて未分類のため、そのままでは絞り込み・カテゴリの件数・親子の字下げが画面に出ない。
> 撮る前に手入力の取引 3 件（近所のスーパー・ドラッグストア・駅前のラーメン屋）と子カテゴリ「外食」を足し、撮ったあと削除した。
> 16 は、連携した Gmail のアドレスを画面上で伏せ字にしてから撮った。

| # | 画面・状態 | 確認したイベント（イベント設計の #） | スクリーンショット |
|---|---|---|---|
| 1 | SCR-002 ログイン | 通常の表示。受け取る情報とタブについての案内 | [01-login.png](./step6-screenshots/01-login.png) |
| 2 | SCR-002 ログイン（`?error=TOKEN_EXPIRED`） | #1 エラーの文言を表から選ぶ（クエリの文字列は出さない） | [02-login-error.png](./step6-screenshots/02-login-error.png) |
| 3 | SCR-010 取引一覧（PC） | #1 当月の件数・合計・一覧。メールから取込んだ取引には「速報」の印 | [03-list.png](./step6-screenshots/03-list.png) |
| 4 | SCR-010 未分類で絞り込み | #3 `category_id=none`、適用中の条件 | [04-list-uncategorized.png](./step6-screenshots/04-list-uncategorized.png) |
| 5 | SCR-010 取引一覧（スマホ） | カード表示、「絞り込み（1）」（未分類で絞り込み中）、下部に固定した「登録」 | [05-list-mobile.png](./step6-screenshots/05-list-mobile.png) |
| 6 | SCR-011 取引登録の入力エラー | #2 欄から離れたときの検証（金額 12.5） | [06-new-error.png](./step6-screenshots/06-new-error.png) |
| 7 | SCR-010 登録のあと | #3 登録した取引の月の一覧と「✓ 登録しました」 | [07-list-created.png](./step6-screenshots/07-list-created.png) |
| 8 | SCR-012 取引詳細 | #1 金額・曜日つきの日時・未分類・手入力 | [08-detail.png](./step6-screenshots/08-detail.png) |
| 9 | SCR-013 取引編集 | #1 初期値、店舗名は表示のみ、変更が無い間は「保存」を押せない | [09-edit.png](./step6-screenshots/09-edit.png) |
| 10 | SCR-012 保存のあと | #8 「✓ 保存しました」、カテゴリに「手動」の印 | [10-detail-saved.png](./step6-screenshots/10-detail-saved.png) |
| 11 | SCR-012 削除の確認 | #3 日時・店舗・金額を示し、初期フォーカスは「キャンセル」 | [11-detail-delete.png](./step6-screenshots/11-detail-delete.png) |
| 12 | SCR-020 カテゴリ管理 | #1 親子の字下げ（食費の下の外食）と取引件数 | [12-categories.png](./step6-screenshots/12-categories.png) |
| 13 | SCR-020 名前の重複 | #4 `409 DUPLICATE_CATEGORY_NAME` を名前欄の下に | [13-categories-duplicate.png](./step6-screenshots/13-categories-duplicate.png) |
| 14 | SCR-020 削除の確認 | #6 未分類になる件数と、最上位に移る子カテゴリ | [14-categories-delete.png](./step6-screenshots/14-categories-delete.png) |
| 15 | SCR-099 エラー | 存在しない URL（ヘッダーなし） | [15-not-found.png](./step6-screenshots/15-not-found.png) |
| 16 | （拡張）SCR-050 メール連携 | 連携中の Gmail、最終同期と最新の同期結果（2 回目の同期は取得 0 件＝同じメールを二重に取り込まない） | [16-mail-accounts.png](./step6-screenshots/16-mail-accounts.png) |
| 17 | （拡張）SCR-012 メールから取込んだ取引 | 「速報（確定前の金額）」、テンプレートから作った決済手段「三井住友カード（クレジット）」、取込元「メール」 | [17-detail-email.png](./step6-screenshots/17-detail-email.png) |

スクリーンショットのほかに、以下も操作で確かめた。

| 確認したこと | 結果 |
|---|---|
| ログインせずに `/transactions?category_id=none` を開く → ログイン → 元の URL へ戻る（SCR-003 #1） | 戻り先が使われ、URL のフラグメント（トークン）は消えている |
| 壊れたトークンで一覧を開く（1.2 の `401`） | トークンを破棄し、`/login?error=INVALID_TOKEN` へ。戻り先が保存されている |
| 開始日を終了日より後にする（SCR-010 #2） | 欄の下に「開始日は終了日以前にしてください」。URL は変わらない |
| 削除の確認を Esc で閉じる（SCR-012 #5） | ダイアログが閉じ、「削除」ボタンへフォーカスが戻る |
| `/transactions/abc`・存在しない取引の詳細と編集（SCR-012 / 013 の `404`） | 「取引が見つかりません」 |

### スクリーンショット

#### 1. SCR-002 ログイン

![SCR-002 ログイン](./step6-screenshots/01-login.png)

#### 2. SCR-002 ログイン（`?error=TOKEN_EXPIRED`）

![SCR-002 ログイン（?error=TOKEN_EXPIRED）](./step6-screenshots/02-login-error.png)

#### 3. SCR-010 取引一覧（PC）

![SCR-010 取引一覧（PC）](./step6-screenshots/03-list.png)

#### 4. SCR-010 未分類で絞り込み

![SCR-010 未分類で絞り込み](./step6-screenshots/04-list-uncategorized.png)

#### 5. SCR-010 取引一覧（スマホ）

![SCR-010 取引一覧（スマホ）](./step6-screenshots/05-list-mobile.png)

#### 6. SCR-011 取引登録の入力エラー

![SCR-011 取引登録の入力エラー](./step6-screenshots/06-new-error.png)

#### 7. SCR-010 登録のあと

![SCR-010 登録のあと](./step6-screenshots/07-list-created.png)

#### 8. SCR-012 取引詳細

![SCR-012 取引詳細](./step6-screenshots/08-detail.png)

#### 9. SCR-013 取引編集

![SCR-013 取引編集](./step6-screenshots/09-edit.png)

#### 10. SCR-012 保存のあと

![SCR-012 保存のあと](./step6-screenshots/10-detail-saved.png)

#### 11. SCR-012 削除の確認

![SCR-012 削除の確認](./step6-screenshots/11-detail-delete.png)

#### 12. SCR-020 カテゴリ管理

![SCR-020 カテゴリ管理](./step6-screenshots/12-categories.png)

#### 13. SCR-020 名前の重複

![SCR-020 名前の重複](./step6-screenshots/13-categories-duplicate.png)

#### 14. SCR-020 削除の確認

![SCR-020 削除の確認](./step6-screenshots/14-categories-delete.png)

#### 15. SCR-099 エラー

![SCR-099 エラー](./step6-screenshots/15-not-found.png)

#### 16. （拡張）SCR-050 メール連携

![（拡張）SCR-050 メール連携](./step6-screenshots/16-mail-accounts.png)

#### 17. （拡張）SCR-012 メールから取込んだ取引

![（拡張）SCR-012 メールから取込んだ取引](./step6-screenshots/17-detail-email.png)

---

## ③ 静的解析・自動テスト

バックエンドとフロントエンドをまとめて実行するスクリプト `scripts/check-all.sh` を追加した。出力の全文は [docs/step6-check-output.txt](./step6-check-output.txt)。

```
$ ./scripts/check-all.sh

### 1. バックエンド（go vet + golangci-lint + go test）
$ make check
cd backend && go vet ./...
cd backend && golangci-lint run ./...
0 issues.
cd backend && go test ./... -count=1 -cover
ok  	.../internal/auth	        coverage: 91.5% of statements
ok  	.../internal/config	        coverage: 100.0% of statements
ok  	.../internal/extract	    coverage: 86.8% of statements
ok  	.../internal/gmail	        coverage: 88.5% of statements
ok  	.../internal/handler	    coverage: 19.7% of statements
ok  	.../internal/httpx	        coverage: 26.8% of statements
ok  	.../internal/mailparse	    coverage: 91.5% of statements
ok  	.../internal/repository	    coverage: 70.3% of statements
ok  	.../internal/server	        coverage: 97.0% of statements

### 2. フロントエンド（tsc + ESLint + Vitest）
$ cd frontend && npm run check
> tsc --noEmit
> eslint
> vitest run
 ✓ src/features/categories/form.test.ts (7 tests)
 ✓ src/features/transactions/listQuery.test.ts (20 tests)
 ✓ src/features/transactions/form.test.ts (22 tests)
 ✓ src/lib/lib.test.ts (18 tests)
 Test Files  4 passed (4)
      Tests  67 passed (67)

 すべて成功しました
```

型チェック・ESLint は違反ゼロ（出力なし）、テストはすべて成功。

| 対象 | テストの内容 |
|---|---|
| フロント：取引一覧の条件（20 件） | URL ↔ API-010 の値の変換。当月の既定期間（JST で月をまたぐ時刻を含む）、店舗で絞り込むときは期間を渡さない、条件を変えたらページを 1 に戻す、など |
| フロント：取引のフォーム（22 件） | 金額の検証（全角・桁区切り・小数）、メモの文字数（サロゲートペア）、登録で送らない項目、編集で変えた項目だけを送る・`null`・`learn_category: false` |
| フロント：カテゴリ・共通（25 件） | 戻り先として使ってよいパス（オープンリダイレクト対策）、日時の変換、カテゴリの親子の並べ替えと子孫 |
| バックエンド：追加分 | API-003 のリダイレクト（7 ケース。Google のトークン交換の失敗も、通信を差し替えて確かめる）、未分類の絞り込み、決済手段一覧 |
| バックエンド：メール取込（0 章 D） | Gmail API の呼び出し（403 を「再認証が必要」と「利用上限」に分ける、利用上限はやり直す）、メールの解読（ISO-2022-JP・multipart）、テンプレートの判定と抽出（クレジット・デビット・返品は取り込まない）、認証情報の暗号化、同じメールを二重に保存しない・取引と `payment_events` を作る（統合テスト） |

**気をつけたこと**

- ESLint の React Compiler の規則（`set-state-in-effect`・`refs`）で 2 件の指摘を受けた。`eslint-disable` で消さず、effect の中で state を同期的に書き換えない書き方（描画中に前回の値と比べる）に直した
- 単体テストは、画面の描画ではなく**送る値・URL の変換**を対象にした。イベント設計の表の行（「未分類に戻した → `null`」など）がそのままテストの 1 ケースになり、設計と実装のずれを機械的に見つけられる
- `next build` も通すことを確認した。`useSearchParams` を Suspense で囲まないとビルドで失敗するなど、開発サーバでは出ないエラーがあるため
- メール取込の抽出ルールは、**手元に保存した実際の通知メールにも当てるテスト**を用意した（[samples_test.go](../backend/internal/extract/samples_test.go)）。個人のメールはリポジトリに入れないため、ファイルが無い環境ではこのテストは飛ばす。同期の流れ（`service` パッケージ）は単体テストが無く、実際の Gmail での取込で確かめた

---

## ④ まとめ

### 苦労したこと

**Next.js 16 の既定値が、設計の前提と合わなかったこと。** create-next-app の既定では `cacheComponents` が有効で、画面を移っても前の画面が隠れるだけで残ります。最初はそのまま進めようとしましたが、登録したあとに「登録」を開き直すと前の入力が残る、といった挙動になり、イベント設計の「開くたびに取り直す」と食い違うと分かりました。講座で学んだ Next.js と手元のバージョンの違いを、文書を読んで確かめるところから始める必要がありました。

**動作確認で初めて見つかった不具合。** 型チェックと ESLint が通っても、操作すると次のような問題がありました。

- スマホで「登録」ボタンが 2 つ出た。`hidden` と、ボタンの共通の `inline-flex` が衝突し、後者が勝っていた
- ダイアログを Esc で閉じると、フォーカスが「削除」ボタンに戻らなかった。閉じると同時に中身が消え、フォーカスしていた要素も消えるため、ブラウザが戻さない
- 期間の欄に入力した直後に欄を離れると、描画前の古い値で判定していた

### 工夫したこと

① の「工夫した点」のとおり。特に、**画面から純粋な関数を切り出してイベント設計の表をテストにした**ことで、「何を送るか」「何を送らないか」を実装のあとも確かめ続けられるようにしました。

### 学んだこと

**フレームワークの既定値も、設計の前提になること。** Step 5 では「API の既定値も画面の設計の一部」だと学びましたが、今回はフレームワーク側の既定値（`cacheComponents`）が画面の振る舞いを変えました。使う側が既定値を知らないと、設計どおりに書いたつもりでも違う動きになります。

**設計どおりかは、イベント設計の表の行ごとに操作して確かめる必要があること。** 「完了メッセージは遷移先で 1 回だけ」「閉じたら開いたボタンへフォーカスを戻す」のような振る舞いは、型や静的解析では確かめられず、表の 1 行ずつを実際に操作して初めて確認できました。

### メール取込の最小版（範囲外）で苦労したこと・学んだこと

**外部 API のエラーは、利用者に求める行動ごとに分ける必要があること。** 初回の取込の途中で Gmail API が `403` を返し、それを一律に「再認証が必要」と扱ったため、105 通を取得したところで連携が止まりました。`403` には「権限が無い（再認証が必要）」と「利用上限（待てば回復する）」の両方があり、後者は待ってやり直すよう分けました。原因を後から追えるよう、Google が返した理由をエラーの文言に残しています。

**抽出のルールは、実物のメールを見てから作ること。** 最初は件名だけで探して実際の利用通知を確かめてから、送信元と項目の書き方に合わせたテンプレートを作りました。クレジットとデビットで項目名の後の空白や時刻の秒の有無が違う、返品も同じ件名で届く、といった違いは、実物を見るまで分かりませんでした。

---

## メンタリング報告（2.1 提出課題の説明）

| 報告内容 | 対応箇所 |
|---|---|
| 実装した画面の概要と、Step 4・5 の設計からの変更点とその理由 | ①「実装した画面」、0 章 B |
| バックエンド連携・認証・セッション管理で気をつけたポイント | 下表 |
| 静的解析・自動テストで気をつけたポイント | ③「気をつけたこと」 |
| 実装中に工夫したこと・苦労したこと | ①「工夫した点」、④ |
| （範囲外）メール取込の最小版のデモ | 0 章 D。Gmail を連携 → 「今すぐ同期」→ 取引一覧に「速報」の取引が並ぶ |

| 論点 | 気をつけたこと |
|---|---|
| バックエンドとの接続 | `/api/*` を Next.js の rewrites で転送し、同一オリジンとして呼ぶ。API サーバに CORS の設定を足さずに済む |
| トークンの受け取り | Google からの戻りではトークンが URL のフラグメントに載るため、**取り出したらすぐに URL から消す**（履歴・画面共有に残さない） |
| ログイン後の戻り先 | `/` で始まり `//` で始まらないパスだけを使う（オープンリダイレクト対策） |
| ログインの確認 | 共通レイアウトで API-004 を 1 回だけ呼び、成功するまで画面の中身を出さない |
| トークンの期限切れ | どの API の `401` でも共通の処理でログイン画面へ。同時に返っても遷移は 1 回だけ。ログアウトだけはエラーを出さない |
| エラーの文言 | ログイン画面の文言は表から選び、クエリの文字列をそのまま出さない（偽の案内を出させない） |

---

## 相談させていただきたいこと

1. **メールの取込（拡張）の進め方**
   - Step 5 のフィードバックで「拡張は Step 8 で」と回答をいただいたが、講座の Step 8 は「インフラ 2」で、Step 7〜12 に機能を作る Step が無い
   - テーマの中心である「メールから家計簿が自動でできる」を発表で見せるため、**最小版を Step 6 と並行して先に作った**（0 章 D。Gmail API で連携し、1 社のカード利用通知だけを手動の同期で取り込む。名寄せ・定期実行・通知は作らない）。この進め方でよいか
   - SCR-050 は、Step 5 の形式の設計（画面レイアウト・イベント設計）より先に実装した。設計は後から書き足す予定だが、**拡張の設計（Step 5 要件相当）と残りの拡張の実装を、どの Step で扱えばよいか**
2. **文書の種別のラベル（Step 5 の相談事項 2 の発展的な提案）**
   - 未対応のまま提出した。Step 2 の仕様書は「生きた文書」、Step 4・5 の文書は「スナップショット」として、冒頭にラベルを付ける形でよいか
