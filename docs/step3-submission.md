# Step 3 提出物

サービス名: 決済メール解析による自動家計簿サービス
提出日: 2026-09-16
リポジトリ: https://github.com/e1lite/devcamp-kakeibo

---

## 0. Step 2 の仕様書からの変更点

実装に先立ち、および実装を通じて Step 2 の仕様書を更新した。
**仕様書と実装が食い違ったまま残らないよう、変更はすべて仕様書側に反映済み**である。

対象ドキュメント:
[step2-api-spec.md](./step2-api-spec.md) / [step2-db-spec.md](./step2-db-spec.md)

### A. Step 2 のフィードバックを受けた変更（3 件）

| # | 変更 | 理由 | 反映箇所 |
|---|---|---|---|
| A-1 | **Web フレームワークを Gin から `net/http` に変更** | Step 3 の実施要件が「Step 1・2 で学んだ Web フレームワーク（FastAPI もしくは `net/http`）」を指定しているため。Go 1.22 以降の `http.ServeMux` はメソッド + パスパターンをサポートしており、本仕様のルーティングは標準ライブラリで充足できる | [API 仕様書 冒頭](./step2-api-spec.md) |
| A-2 | **API-015 のパスを `POST /transactions/merge` に変更** | マージは特定リソース 1 件への操作ではなくコレクション全体への操作であり、URL にどちらか一方の ID を含める必然性が薄いため（フィードバックの指摘に従った） | [API 仕様書 6 章 API-015](./step2-api-spec.md) |
| A-3 | **一覧表に `実装フェーズ` 列を追加し、範囲を明示** | 「全エンドポイントを Step 3 の期間内に実装するのは分量的に厳しく、最低限ラインを先に切る」というフィードバックでの合意を仕様書に落とすため。これにより「仕様書で定義したすべてのエンドポイントを実装する」という判定基準と、範囲を絞る方針が両立する | [API 仕様書 5.1](./step2-api-spec.md) / [DB 仕様書 2.1](./step2-db-spec.md) |

あわせて、フィードバックで結論の出た相談事項（月次サマリの集計方式・エラーコードの粒度・
ページネーション方式・本文の保持期間）を仕様書に反映した。
未確認のまま残っている 3 点は本書の末尾に記載する。

### B. 実装を通じて仕様書を更新した点（6 件）

実装中に「仕様書に書いていないが実装には必要」「仕様書の記述が実態と合わない」箇所が
見つかったため、仕様書を更新した。

| # | 変更 | 理由 |
|---|---|---|
| B-1 | **DB: `CHECK` 制約 8 件を明記** | `kind` / `status` / `source` / `category_source` / `geocode_status` / `provider` の取り得る値と `amount_minor > 0` を、アプリだけでなくデータベース側でも守る。仕様書には値の一覧が書かれていたが制約としては記載していなかった |
| B-2 | **DB: `mail_accounts.credential_encrypted` を `NOT NULL DEFAULT ''` と明記** | 連携解除時に「行は残して認証情報だけ破棄する」ため、空文字を入れる運用になる |
| B-3 | **API-011 / API-013 のレスポンスを API-012 の詳細と同形式に変更** | 作成・修正の直後にクライアントがそのまま表示へ使えるよう、取得系と同じ表現を返す。当初は項目を絞った定義だったが、同じ取引に 3 種類の表現があるとフロント側の型定義が増える |
| B-4 | **API-013 の `category_source` を「常に `user`」から「カテゴリを変更した場合は `user`」に修正** | `category_source` はカテゴリがどう決まったかを表す列であり、金額やメモだけを修正したときに `user` へ書き換えると誤った記録になる。カテゴリ自動分類（P6）はこの列を見てユーザ修正を上書きしないよう判断するため、意味がずれると分類側の挙動に影響する |
| B-5 | **API-011 に `400 UNSUPPORTED_CURRENCY` を追加** | `amount_jpy_minor` はサーバが算出するが、為替レートの取得元がまだ無いため `JPY` 以外は算出できない。多通貨対応は Step 8 以降 |
| B-6 | **エラーコードの追記**（API-003 の `VALIDATION_ERROR`、API-004 の `USER_NOT_FOUND`、API-010 のクエリ形式不正、API-013 のマスタ参照エラー） | 実装で返しているが仕様書の表に無かったもの |

**B-4 と B-5 はフィードバックを受けた変更ではなく、こちらの判断による仕様変更**のため、
メンタリングで妥当性を確認したい。

### C. Step 8 以降に回した機能（レスポンスの形は変えていない）

| 項目 | Step 3 での扱い | 理由 |
|---|---|---|
| `event_count` | 常に `0` | `payment_events` テーブルはコア 6 テーブルに含まれない |
| `payment_events[]` | 常に空配列 | 同上 |
| `learn_category`（API-013） | 受け付けるが無視し、`meta.learned_rule` は常に `null` | `category_rules` テーブルがコア 6 テーブルに含まれない（API 仕様書 5.1 の ※1） |

いずれも**リクエスト・レスポンスの形は仕様書どおりに保っている**ため、
将来これらを実装してもフロントエンドの改修は不要になる。

---

## ① 概要

Step 2 で作成した API 仕様書・データベース仕様書に沿ってバックエンドを実装した。
ローカル環境（Docker の PostgreSQL）に接続し、Google OAuth でのログインから
取引の CRUD までが動作する。

### 構成

| 項目 | 内容 |
|---|---|
| 言語 | Go 1.26 |
| Web フレームワーク | **`net/http`（標準ライブラリ）** |
| ORM | GORM |
| データベース | PostgreSQL 17（Docker Compose） |
| マイグレーション | golang-migrate（SQL ファイル） |
| 静的解析 | `go vet` + golangci-lint v2 |

```
backend/
├── cmd/api/             API サーバのエントリポイント
├── cmd/migrate/         マイグレーション実行
├── internal/
│   ├── auth/            JWT の発行・検証、OAuth の state 管理
│   ├── config/          環境変数の読み込み
│   ├── database/        PostgreSQL への接続
│   ├── handler/         HTTP ハンドラ（リクエスト解釈・エラーの対応づけ）
│   ├── httpx/           共通レスポンス形式・エラーコード・JSON デコード
│   ├── model/           テーブルに対応する構造体
│   ├── repository/      データベースアクセス
│   ├── server/          ルーティング・ミドルウェア
│   └── service/         業務ロジック
└── migrations/          スキーマ定義（SQL）
```

### 実装範囲

API 仕様書 5.1 の「Step 3（コア）」に対応する **6 テーブル / 13 行・14 操作**を実装した。

| API ID | メソッド | エンドポイント |
|---|---|---|
| API-001 | GET | `/health` |
| API-002 | GET | `/api/v1/auth/google` |
| API-003 | GET | `/api/v1/auth/google/callback` |
| API-004 | GET | `/api/v1/auth/me` |
| API-005 | POST | `/api/v1/auth/logout` |
| API-010 | GET | `/api/v1/transactions` |
| API-011 | POST | `/api/v1/transactions` |
| API-012 | GET | `/api/v1/transactions/{id}` |
| API-013 | PUT | `/api/v1/transactions/{id}` |
| API-014 | DELETE | `/api/v1/transactions/{id}` |
| API-019 | GET | `/api/v1/categories` |
| API-020 | POST | `/api/v1/categories` |
| API-021 | PUT / DELETE | `/api/v1/categories/{id}` |

テーブルは `users` / `mail_accounts` / `categories` / `payment_methods` /
`merchants` / `transactions` の 6 件。

### 工夫した点

**1. 認証を 2 層に分離し、ログインに必要な最小のスコープだけを要求した**

Google OAuth で要求するスコープは `openid` / `email` / `profile` のみとし、
メール読み取りの `gmail.readonly` は要求しない。
ユーザは**ログインだけして、メール連携は許可しない**という選択ができ、
手入力の家計簿としては即座に使い始められる。

**2. トークンを URL フラグメントで渡し、リダイレクト先を出口でも検証した**

トークンはクエリ文字列ではなくフラグメント（`#access_token=...`）に載せる。
フラグメントはサーバへ送信されず `Referer` ヘッダにも乗らないため、
経路上のログにトークンが残らない。

`redirect_uri` は認可開始時に許可リストと照合しているが、
**実際にリダイレクトする直前にもう一度確認**している。
state ストアの内容を信頼せず、「トークンを渡す先は必ず許可リストのいずれか」を
出口側で保証するため。

**3. 重複名の検出を UNIQUE 制約違反の捕捉で行った**

事前に `SELECT` で存在確認をすると、確認から `INSERT` までの間に
他のリクエストが同じ名前を作る余地が残る。
PostgreSQL のエラーコード `23505` を捕まえて `409` に変換することで競合に強くした。

**4. 親カテゴリの循環を再帰 CTE で検出した**

新しい親から祖先を辿って自分に到達するかを 1 クエリで判定する。

```sql
WITH RECURSIVE ancestors AS (
    SELECT id, parent_id FROM categories WHERE id = ? AND user_id = ?
    UNION ALL
    SELECT c.id, c.parent_id FROM categories c JOIN ancestors a ON c.id = a.parent_id
)
SELECT EXISTS (SELECT 1 FROM ancestors WHERE id = ?)
```

**5. マスタの参照は `user_id` まで確認した**

外部キー制約は「その ID が存在すること」しか保証しない。
`user_id` を含めて確認しないと、**他ユーザのカテゴリを自分の取引に紐づけられる**。

**6. 未知のフィールドをエラーにした**

`DisallowUnknownFields()` により、`sort_oder` のような打ち間違いは `400` になる。
黙って無視すると「なぜか反映されない」という分かりにくい不具合になるため。

**7. 論理削除の絞り込みを 1 箇所に集約した**

`transactions` の参照系は必ず `deleted_at IS NULL` で絞る必要がある。
各クエリに書くと付け忘れが起きるため、`scoped()` という共通の入口を通す。
**合計金額の集計にも同じ条件がかかる**ことを統合テストで保証している。

---

## ② ソースコード

https://github.com/e1lite/devcamp-kakeibo

---

## ③ API の動作確認

`scripts/verify-all.sh` で正常系・異常系・永続化の確認を一括実行できる。
実行結果の全文は [step3-verify-output.txt](./step3-verify-output.txt) を参照。

```bash
make run                    # 別のターミナルで API サーバを起動
./scripts/verify-all.sh
```

### 結果

| 対象 | 件数 |
|---|---|
| ヘルスチェックと認証（API-001〜005） | 10 件すべて成功 |
| カテゴリ CRUD（API-019 / 020 / 021） | 18 件すべて成功 |
| 取引 CRUD（API-010〜014） | 28 件すべて成功 |
| **合計** | **56 件すべて成功** |

### 永続化の確認

判定基準の「作成・更新・削除系のエンドポイントは、実行後に取得系のエンドポイントも
呼び出して、データが永続化されていることを確認する」に対応する。

```
POST   /transactions      → 201 Created（id=31）
GET    /transactions/31   → 200 OK      作成した内容が取得できる
PUT    /transactions/31   → 200 OK      金額とメモを修正
GET    /transactions/31   → 200 OK      修正が反映され is_user_edited=true
DELETE /transactions/31   → 204 No Content
GET    /transactions/31   → 404 Not Found
GET    /transactions      → 200 OK      一覧から消え、合計金額からも除かれる
```

実際のレスポンス（抜粋）:

```json
// POST /api/v1/transactions
{"data":{"id":31,"occurred_at":"2026-09-10T19:20:00+09:00","amount_minor":1200,
  "merchant":{"id":4,"name":"近所の定食屋","address":null},
  "category":{"id":1,"name":"食費"},"source":"manual","is_user_edited":true,
  "note":"現金","payment_events":[]}}

// 削除前の一覧
{"data":[...2 件...],"meta":{"total":2,"total_amount_minor":1780}}

// 削除後の一覧 — 削除した 1500 円が合計からも除かれている
{"data":[...1 件...],"meta":{"total":1,"total_amount_minor":580}}
```

カテゴリ側も同様に、作成 → 一覧で確認 → 更新 → 削除 → 一覧が初期 9 件に戻ることを確認した。

### 異常系の確認（抜粋）

| 確認内容 | 結果 |
|---|---|
| 認証なし / 壊れたトークン | `401 INVALID_TOKEN` |
| 期限切れトークン | `401 TOKEN_EXPIRED`（単体テストで確認） |
| 許可リストにない `redirect_uri` | `400 INVALID_REDIRECT_URI` |
| 未発行の `state`（CSRF 対策） | `400 INVALID_STATE` |
| 金額が 0 | `400 INVALID_AMOUNT` |
| タイムゾーンの無い日時 | `400 VALIDATION_ERROR` |
| JPY 以外の通貨 | `400 UNSUPPORTED_CURRENCY` |
| 仕様にないフィールド | `400 VALIDATION_ERROR` |
| `from` が `to` より後 | `400 INVALID_DATE_RANGE` |
| `limit` が 100 超 | `400 LIMIT_TOO_LARGE` |
| 同名カテゴリ | `409 DUPLICATE_CATEGORY_NAME` |
| 自分自身を親に指定 | `400 INVALID_PARENT` |
| 初期カテゴリの削除 | `400 SYSTEM_CATEGORY_NOT_DELETABLE` |
| 存在しない ID | `404 *_NOT_FOUND` |

---

## ④ 静的解析・自動テスト

実行結果の全文は [step3-check-output.txt](./step3-check-output.txt) を参照。

```bash
make check    # go vet → golangci-lint → go test
```

### 静的解析

```
$ cd backend && go vet ./...
$ cd backend && golangci-lint run ./...
0 issues.
```

golangci-lint は標準セットに加え、`gosec`（セキュリティ）・`errorlint`・
`bodyclose`・`sqlclosecheck`・`noctx`・`nilerr` などを有効にしている。
`govet` の `shadow` も有効にし、`err` の握りつぶしを防いでいる。

### 自動テスト

```
$ cd backend && go test ./... -count=1 -cover
ok  .../internal/auth          coverage: 92.9% of statements
ok  .../internal/config        coverage: 100.0% of statements
ok  .../internal/handler       coverage: 3.9% of statements
ok  .../internal/httpx         coverage: 26.3% of statements
ok  .../internal/repository    coverage: 77.0% of statements
ok  .../internal/server        coverage: 95.3% of statements
    .../internal/service       coverage: 0.0% of statements
```

**リポジトリ層は実 PostgreSQL に対する統合テスト**とし、
各テストをトランザクションで包んでロールバックすることで、
開発用データベースを汚さずに実行できるようにしている。

```bash
make test        # 統合テストを含む（DB が起動していること）
make test-unit   # 単体テストのみ（DB 不要。go test -short）
```

**ハンドラ層とサービス層の単体テストは未整備**で、これらの動作は
③ の 56 件のエンドツーエンドの確認で担保している状態。今後の課題とする。

---

## ⑤ まとめ

### 苦労したこと

**GORM が構造体から SQL を推測することによる不具合を 2 回出した**

いずれも `go vet`・golangci-lint・単体テストのすべてを通過し、
**SQL を実行して初めて表面化した**。

| # | 症状 | 原因 |
|---|---|---|
| 1 | `/auth/me` が 500 | `Select` の引数をカンマで分けて渡したため、GORM がそれぞれを別の列名として扱い、プレースホルダが置換されないまま SQL が組み立てられた |
| 2 | `/categories` が 500 | 読み込み先の構造体に集計用のフィールドがあり、`Select` を省略したため存在しない列 `categories.transaction_count` を SELECT に含めた |

1 つ目は動作確認中に自分で踏み、2 つ目は**統合テストを追加した後だったため
テストが先に検出した**。この経験からリポジトリ層を統合テストにする方針を固めた。

**統合テストが開発用データベースの状態に依存していた**

「カテゴリが 9 件であること」という断言を書いたが、
既存のログインユーザのデータが入っていたため 18 件になり失敗した。
**件数の判定は絶対値ではなく処理の前後の差分で行う**ように修正した。

**コードを修正してもサーバに反映されず、原因の特定に時間を使った**

`go run` は起動時に一度だけコンパイルするため、
修正後に再起動しないと古いバイナリのまま動作する。
新しいエンドポイントが `404` を返す形で表面化した。

### 工夫したこと

① の「工夫した点」に記載したとおり。特に、
**セキュリティに関わる判断は必ずコードのコメントに理由を残す**ようにした。
後から読んだときに「なぜこうなっているか」が分かり、安易に変更されるのを防げるため。

### 学んだこと

**静的解析の警告への対応は「直す」「誤検知として理由を残す」「実際に強化する」の 3 通りがある**

gosec の `G710`（オープンリダイレクト）が 2 箇所で出たが、対応は別々にした。

- API-002 のリダイレクト先は oauth2 ライブラリが組み立てる固定のエンドポイントであり、
  クエリの `redirect_uri` は入り込まない。**誤検知としてコメントに理由を書いて抑制**した。
- API-003 のリダイレクト先は許可リストで検証済みだが、検証は state 発行時にしか
  行っていなかった。**リダイレクト直前にもう一度確認する処理を追加**し、実際に強化した。

抑制するにしても「なぜ問題ないか」を書き残さなければ、次に読む人が判断できない。

**テストが通ることと、動くことは別である**

カバレッジの高い層（config 100%・server 95%）には不具合が出ず、
カバレッジ 0% だったリポジトリ層に 2 件の不具合が出た。
**テストが無い場所にこそ不具合がある**という当たり前のことを、実際に踏んで理解した。

**ORM は「書く量が減る」代わりに「何が起きるか見えにくくなる」**

GORM の 2 件の不具合はいずれも「ライブラリが良かれと思って推測した結果」だった。
結合や集計を伴うクエリでは `Select` を明示し、推測させないようにしている。

**仕様書と実装は放っておくと乖離する**

実装中に「仕様書に書いていないが必要なもの」が 6 件見つかった（0 章の B）。
その場で仕様書に反映しないと、次に仕様書を読む人が実装と違うものを作ることになる。

---

## メンタリング報告

### 実装した機能の概要と、Step 2 の仕様書からの変更

**0 章に記載のとおり。** 特に報告したいのは以下の 2 点。

- **A-1〜A-3**: フィードバックを受けた変更。仕様書への反映も完了している
- **B-4 / B-5**: **こちらの判断による仕様変更**のため、妥当性を確認したい
  - API-013 の `category_source` を「常に `user`」から「カテゴリを変更した場合のみ」に修正した
  - `JPY` 以外の通貨を `400 UNSUPPORTED_CURRENCY` で弾くことにした

### 実装中に工夫したこと・苦労したこと

⑤ に記載のとおり。特に報告したいのは、
**GORM の推測による不具合が静的解析と単体テストをすり抜けた**こと。

### 静的解析・自動テストで気をつけたポイント

**1. リポジトリ層は実データベースに対して検証する**

モックでは GORM が組み立てる SQL の妥当性を検証できない。
トランザクションで包んでロールバックすることで、
開発用データベースを使いながら副作用を残さない形にした。

**2. 統合テストは冪等に書く**

データベースの現在の状態に依存する断言（「全体で 9 件」など）を書くと、
既存データがある環境で失敗する。
`user_id` で範囲を絞るか、処理の前後の差分で判定する。

**3. 静的解析の抑制には必ず理由を書く**

`//nolint` を付ける場合は、**なぜその警告が当てはまらないのか**をコメントに残す。
gosec の `G706`（ログインジェクション）は出力が `uint` であり
改行を注入できないため抑制したが、その判断の根拠をコードに残している。

---

## 相談したいこと

### 今回の実装で判断した点（0 章 B-4 / B-5）

- API-013 の `category_source` の扱い
- `JPY` 以外の通貨を弾くこと

### Step 2 から持ち越している未確認の 3 点

| # | 事項 | 現状の設計 |
|---|---|---|
| 1 | `GET /inbox/unparsed` の位置づけ | `email_messages` のフィルタ済み一覧として設計（API 仕様書 8 章） |
| 2 | `transactions.status`（`pending` / `confirmed`）の要否 | 保持する設計（DB 仕様書 6 章） |
| 3 | `merchants` を支店単位にした判断 | 1 行 = 1 支店、`brand_name` でチェーンを束ねる（DB 仕様書 6 章） |

### 今後の課題

- ハンドラ層・サービス層の単体テストが未整備（現状はエンドツーエンドの確認で担保）
- OAuth の `state` をプロセス内メモリに保持しており、複数インスタンスに対応できない
  （Step 7 以降で Redis 等の共有ストアに置き換える必要がある）
- ログアウトがステートレスなため、漏洩したトークンは最大 24 時間有効
  （即時失効が必要になった場合は `jti` の失効リストを導入する）
