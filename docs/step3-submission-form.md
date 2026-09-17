# Step 3 提出フォーム 貼り付け用

> このファイルは**提出フォームにそのまま貼るためのもの**。
> 下の区切り線の間をすべてコピーして、提出フォームのテキストエリアに貼る。
> フォームは Markdown 対応のため、記法はそのままで表示される。
>
> 詳細版は [step3-submission.md](./step3-submission.md)（こちらは提出せず、手元の記録用）。

---
---
---

## 0. Step 2 の仕様書からの変更点

実装に先立ち、および実装を通じて Step 2 の仕様書を更新しました。
**仕様書と実装が食い違ったまま残らないよう、変更はすべて仕様書側に反映済み**です。

### A. フィードバックを受けた変更

| # | 変更 | 理由 |
|---|---|---|
| A-1 | Web フレームワークを Gin から `net/http` に変更 | Step 3 の実施要件が「Step 1・2 で学んだ Web フレームワーク（FastAPI もしくは `net/http`）」を指定しているため。Go 1.22 以降の `http.ServeMux` はメソッド + パスパターンをサポートしており、標準ライブラリで充足できました |
| A-2 | API-015 のパスを `POST /transactions/merge` に変更 | マージは特定リソース 1 件への操作ではなくコレクション全体への操作であるため（ご指摘に従いました） |
| A-3 | 仕様書の一覧表に `実装フェーズ` 列を追加 | 「最低限ラインを先に切る」というご助言を仕様書に反映するため。これにより「仕様書で定義したすべてのエンドポイントを実装する」という判定基準と、範囲を絞る方針が両立します |

相談事項のうち結論の出た 4 点（月次サマリの集計方式・エラーコードの粒度・ページネーション方式・メール本文の保持期間）も仕様書に反映しました。

### B. 実装を通じて仕様書を更新した点

実装中に「仕様書に書いていないが必要」「記述が実態と合わない」箇所が見つかったため、仕様書を更新しました。

| # | 変更 | 理由 |
|---|---|---|
| B-1 | DB: `CHECK` 制約 8 件を明記 | `kind` / `status` / `source` / `category_source` / `geocode_status` / `provider` の取り得る値と `amount_minor > 0` を、アプリだけでなく DB 側でも守るため |
| B-2 | DB: `mail_accounts.credential_encrypted` を `NOT NULL DEFAULT ''` と明記 | 連携解除時に「行は残して認証情報だけ破棄する」運用のため |
| B-3 | API-011 / API-013 のレスポンスを API-012 の詳細と同形式に変更 | 作成・修正の直後にクライアントがそのまま表示へ使えるようにするため。同じ取引に 3 種類の表現があるとフロント側の型定義が増えます |
| B-4 | API-013 の `category_source` を「常に `user`」から「カテゴリを変更した場合は `user`」に修正 | `category_source` はカテゴリがどう決まったかを表す列であり、金額やメモだけを修正したときに `user` へ書き換えると誤った記録になるため |
| B-5 | API-011 に `400 UNSUPPORTED_CURRENCY` を追加 | `amount_jpy_minor` はサーバが算出しますが、為替レートの取得元がまだ無く `JPY` 以外は算出できないため |
| B-6 | エラーコードの追記（API-003 / 004 / 010 / 013） | 実装で返しているが仕様書の表に無かったもの |

**B-4 と B-5 はフィードバックを受けた変更ではなく、こちらの判断による仕様変更です。メンタリングで妥当性を確認させてください。**

### C. Step 8 以降に回した機能（レスポンスの形は変えていません）

| 項目 | Step 3 での扱い | 理由 |
|---|---|---|
| `event_count` | 常に `0` | `payment_events` テーブルがコア 6 テーブルに含まれないため |
| `payment_events[]` | 常に空配列 | 同上 |
| `learn_category`（API-013） | 受け付けるが無視し、`meta.learned_rule` は常に `null` | `category_rules` テーブルがコア 6 テーブルに含まれないため |

いずれもリクエスト・レスポンスの形は仕様書どおりのため、将来実装してもフロントエンドの改修は不要です。

---

## ① 概要

Step 2 で作成した API 仕様書・データベース仕様書に沿ってバックエンドを実装しました。ローカル環境（Docker の PostgreSQL）に接続し、Google OAuth でのログインから取引の CRUD までが動作します。

### 構成

| 項目 | 内容 |
|---|---|
| 言語 | Go 1.26 |
| Web フレームワーク | `net/http`（標準ライブラリ） |
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
│   ├── handler/         HTTP ハンドラ
│   ├── httpx/           共通レスポンス形式・エラーコード・JSON デコード
│   ├── model/           テーブルに対応する構造体
│   ├── repository/      データベースアクセス
│   ├── server/          ルーティング・ミドルウェア
│   └── service/         業務ロジック
└── migrations/          スキーマ定義（SQL）
```

### 実装範囲

API 仕様書 5.1 の「Step 3（コア）」に対応する **6 テーブル / 13 行・14 操作**を実装しました。

- **テーブル**: `users` / `mail_accounts` / `categories` / `payment_methods` / `merchants` / `transactions`
- **エンドポイント**: API-001（ヘルスチェック）、API-002〜005（認証）、API-010〜014（取引 CRUD）、API-019〜021（カテゴリ CRUD）

### 工夫した点

**1. 認証を 2 層に分離し、ログインに必要な最小のスコープだけを要求した**

Google OAuth で要求するスコープは `openid` / `email` / `profile` のみとし、メール読み取りの `gmail.readonly` は要求していません。ユーザは「ログインだけして、メール連携は許可しない」という選択ができ、手入力の家計簿としては即座に使い始められます。

**2. トークンを URL フラグメントで渡し、リダイレクト先を出口でも検証した**

トークンはクエリ文字列ではなくフラグメント（`#access_token=...`）に載せています。フラグメントはサーバへ送信されず `Referer` ヘッダにも乗らないため、経路上のログにトークンが残りません。

また `redirect_uri` は認可開始時に許可リストと照合していますが、実際にリダイレクトする直前にもう一度確認しています。state ストアの内容を信頼せず、「トークンを渡す先は必ず許可リストのいずれか」を出口側で保証するためです。

**3. 重複名の検出を UNIQUE 制約違反の捕捉で行った**

事前に `SELECT` で存在確認をすると、確認から `INSERT` までの間に他のリクエストが同じ名前を作る余地が残ります。PostgreSQL のエラーコード `23505` を捕まえて `409` に変換することで、競合しても正しく動きます。

**4. 親カテゴリの循環を再帰 CTE で検出した**

新しい親から祖先を辿って自分に到達するかを 1 クエリで判定しています。

```sql
WITH RECURSIVE ancestors AS (
    SELECT id, parent_id FROM categories WHERE id = ? AND user_id = ?
    UNION ALL
    SELECT c.id, c.parent_id FROM categories c JOIN ancestors a ON c.id = a.parent_id
)
SELECT EXISTS (SELECT 1 FROM ancestors WHERE id = ?)
```

**5. マスタの参照は `user_id` まで確認した**

外部キー制約は「その ID が存在すること」しか保証しません。`user_id` を含めて確認しないと、他ユーザのカテゴリを自分の取引に紐づけられてしまいます。

**6. 未知のフィールドをエラーにした**

`DisallowUnknownFields()` により、`sort_oder` のような打ち間違いは `400` になります。黙って無視すると「なぜか反映されない」という分かりにくい不具合になるためです。

**7. 論理削除の絞り込みを 1 箇所に集約した**

`transactions` の参照系は必ず `deleted_at IS NULL` で絞る必要があります。各クエリに書くと付け忘れが起きるため、共通の入口を通すようにしました。合計金額の集計にも同じ条件がかかることを統合テストで保証しています。

---

## ② ソースコード

https://github.com/e1lite/devcamp-kakeibo

---

## ③ API の動作確認

`scripts/verify-all.sh` で正常系・異常系・永続化の確認を一括実行できます。

```bash
make run                    # 別のターミナルで API サーバを起動
./scripts/verify-all.sh
```

### 結果

| 対象 | 件数 |
|---|---|
| ヘルスチェックと認証（API-001〜005） | 10 件すべて成功 |
| カテゴリ CRUD（API-019 / 020 / 021） | 25 件すべて成功 |
| 取引 CRUD（API-010〜014） | 57 件すべて成功 |
| **合計** | **92 件すべて成功** |

件数には HTTP ステータスの確認に加え、レスポンスボディの検証（`meta.total` / `meta.total_amount_minor` / 更新後の値）も含みます。

実行ログの全文はリポジトリの `docs/step3-verify-output.txt` に含めています。

### 永続化の確認（作成 → 取得 → 更新 → 取得 → 削除 → 取得）

```
$ curl -X POST http://localhost:8080/api/v1/transactions \
    -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
    -d '{"occurred_at":"2026-09-10T19:20:00+09:00","amount_minor":1200,
         "merchant_name":"近所の定食屋","category_id":2,"note":"現金"}'
{"data":{"id":1,"occurred_at":"2026-09-10T19:20:00+09:00","amount_minor":1200,
  "currency":"JPY","merchant":{"id":1,"name":"近所の定食屋","address":null},
  "category":{"id":2,"name":"食費"},"payment_method":null,"source":"manual",
  "status":"confirmed","is_possible_duplicate":false,"event_count":0,
  "amount_jpy_minor":1200,"category_source":"user","is_user_edited":true,
  "note":"現金","payment_events":[]}}
=> 201 Created

$ curl -X GET http://localhost:8080/api/v1/transactions/1 -H "Authorization: Bearer $TOKEN"
{"data":{"id":1,...,"amount_minor":1200,"note":"現金",...}}
=> 200 OK   作成した内容が取得できる（永続化されている）

$ curl -X POST http://localhost:8080/api/v1/transactions \
    -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
    -d '{"occurred_at":"2026-09-01T12:00:00+09:00","amount_minor":580,
         "merchant_name":"セブン-イレブン渋谷店"}'
{"data":{"id":2,"occurred_at":"2026-09-01T12:00:00+09:00","amount_minor":580,
  "currency":"JPY","merchant":{"id":2,"name":"セブン-イレブン渋谷店","address":null},
  "category":null,"payment_method":null,"source":"manual",
  "status":"confirmed","is_possible_duplicate":false,"event_count":0,
  "amount_jpy_minor":580,"category_source":"default","is_user_edited":true,
  "note":null,"payment_events":[]}}
=> 201 Created   以降の合計の内訳になる 2 件目

$ curl -X GET http://localhost:8080/api/v1/transactions -H "Authorization: Bearer $TOKEN"
{"data":[...2 件...],"meta":{"total":2,"total_amount_minor":1780}}
=> 200 OK   修正前。1200 円 + 580 円 = 1780 円

$ curl -X PUT http://localhost:8080/api/v1/transactions/1 \
    -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
    -d '{"amount_minor":1500,"note":"修正後のメモ"}'
{"data":{"id":1,...,"amount_minor":1500,"note":"修正後のメモ","is_user_edited":true,...},
 "meta":{"affected_future":false,"learned_rule":null}}
=> 200 OK

$ curl -X GET http://localhost:8080/api/v1/transactions/1 -H "Authorization: Bearer $TOKEN"
{"data":{"id":1,...,"amount_minor":1500,"note":"修正後のメモ","is_user_edited":true,...}}
=> 200 OK   修正が反映されている

$ curl -X GET http://localhost:8080/api/v1/transactions -H "Authorization: Bearer $TOKEN"
{"data":[...2 件...],"meta":{"total":2,"total_amount_minor":2080}}
=> 200 OK   削除前。修正後の 1500 円 + 580 円 = 2080 円

$ curl -X DELETE http://localhost:8080/api/v1/transactions/1 -H "Authorization: Bearer $TOKEN"
=> 204 No Content

$ curl -X GET http://localhost:8080/api/v1/transactions/1 -H "Authorization: Bearer $TOKEN"
{"error":{"code":"TRANSACTION_NOT_FOUND","message":"取引が存在しません"}}
=> 404 Not Found   クライアントからは存在しない

$ curl -X GET http://localhost:8080/api/v1/transactions -H "Authorization: Bearer $TOKEN"
{"data":[...1 件...],"meta":{"total":1,"total_amount_minor":580}}
=> 200 OK   一覧から消え、合計金額も 2080 → 580 に変わっている
```

**論理削除した取引が `meta.total_amount_minor` からも除かれる**ことを確認しています。参照系のクエリで `deleted_at IS NULL` の絞り込みが漏れると、ここが 2080 のままになります。

カテゴリ側も同様に、作成（201）→ 一覧に現れる → 更新（200）→ 削除（204）→ 取得が 404 → 一覧が初期 9 件に戻る、という流れを確認しました。

### 絞り込みとページネーション

```
$ curl -X GET 'http://localhost:8080/api/v1/transactions?from=2026-09-10&to=2026-09-10' ...
{"data":[...1 件...],"meta":{"total":1,"total_amount_minor":1200}}   => 200 OK

$ curl -X GET 'http://localhost:8080/api/v1/transactions?q=定食' ...
{"data":[...1 件...],"meta":{"total":1,"total_amount_minor":1200}}   => 200 OK  店舗名の部分一致

$ curl -X GET 'http://localhost:8080/api/v1/transactions?q=現金' ...
{"data":[...1 件...],"meta":{"total":1,"total_amount_minor":1200}}   => 200 OK  メモの部分一致

$ curl -X GET 'http://localhost:8080/api/v1/transactions?limit=1&offset=0' ...
{"data":[...1 件...],"meta":{"total":2,"total_amount_minor":1780}}   => 200 OK
```

`limit` で切り取っても `meta.total` は条件に合致する全体の件数（2 件）を返します。

### 異常系の確認

```
$ curl http://localhost:8080/api/v1/transactions                      # 認証ヘッダなし
{"error":{"code":"INVALID_TOKEN","message":"Authorization ヘッダが不正です"}}   => 401

$ curl -H 'Authorization: Bearer not-a-jwt' .../api/v1/auth/me
{"error":{"code":"INVALID_TOKEN","message":"トークンが不正です"}}              => 401

$ curl '.../api/v1/auth/google?redirect_uri=https://example.com/steal' ...
{"error":{"code":"INVALID_REDIRECT_URI","message":"redirect_uri が許可リストにありません"}}  => 400

$ curl '.../api/v1/auth/google/callback?code=dummy&state=never-issued' ...
{"error":{"code":"INVALID_STATE","message":"state が不正です。ログインをやり直してください"}} => 400

$ curl -X POST .../api/v1/transactions -d '{"occurred_at":"2026-09-10T19:20:00+09:00","amount_minor":0}'
{"error":{"code":"INVALID_AMOUNT","message":"金額は 1 以上で指定してください"}}  => 400

$ curl -X POST .../api/v1/transactions -d '{"occurred_at":"2026-09-10 19:20:00","amount_minor":100}'
{"error":{"code":"VALIDATION_ERROR",
  "message":"occurred_at は ISO 8601（タイムゾーンオフセットを含む）で指定してください"}}   => 400

$ curl -X POST .../api/v1/transactions -d '{...,"currency":"USD"}'
{"error":{"code":"UNSUPPORTED_CURRENCY","message":"現時点では JPY のみ対応しています"}}   => 400

$ curl -X POST .../api/v1/categories -d '{"name":"タイポ確認","sort_oder":1}'
{"error":{"code":"VALIDATION_ERROR",
  "message":"リクエストボディが不正です: json: unknown field \"sort_oder\""}}            => 400

$ curl -X POST .../api/v1/categories -d '{"name":"食費"}'
{"error":{"code":"DUPLICATE_CATEGORY_NAME","message":"同名のカテゴリが存在します"}}       => 409

$ curl -X PUT .../api/v1/categories/1017 -d '{"parent_id":1017}'
{"error":{"code":"INVALID_PARENT","message":"自分自身または子孫を親に指定できません"}}     => 400

$ curl -X DELETE .../api/v1/categories/1
{"error":{"code":"SYSTEM_CATEGORY_NOT_DELETABLE","message":"初期カテゴリは削除できません"}} => 400

$ curl '.../api/v1/transactions?from=2026-09-10&to=2026-09-01' ...
{"error":{"code":"INVALID_DATE_RANGE","message":"from が to より後です"}}                => 400

$ curl '.../api/v1/transactions?limit=101' ...
{"error":{"code":"LIMIT_TOO_LARGE","message":"limit は 100 以下で指定してください"}}      => 400

$ curl .../api/v1/transactions/999999 ...
{"error":{"code":"TRANSACTION_NOT_FOUND","message":"取引が存在しません"}}                 => 404

$ curl .../api/v1/no-such-endpoint
{"error":{"code":"NOT_FOUND","message":"エンドポイントが存在しません"}}                   => 404
```

期限切れトークンに対する `401 TOKEN_EXPIRED` は、24 時間待てないため単体テストで確認しています。

---

## ④ 静的解析・自動テスト

`make check` で静的解析と自動テストをまとめて実行します。

```
$ make check
cd backend && go vet ./...
cd backend && golangci-lint run ./...
0 issues.
cd backend && go test ./... -count=1 -cover
        github.com/e1lite/devcamp-kakeibo/backend/cmd/api                coverage: 0.0% of statements
        github.com/e1lite/devcamp-kakeibo/backend/cmd/migrate            coverage: 0.0% of statements
ok      github.com/e1lite/devcamp-kakeibo/backend/internal/auth          coverage: 92.9% of statements
ok      github.com/e1lite/devcamp-kakeibo/backend/internal/config        coverage: 100.0% of statements
        github.com/e1lite/devcamp-kakeibo/backend/internal/database      coverage: 0.0% of statements
ok      github.com/e1lite/devcamp-kakeibo/backend/internal/handler       coverage: 3.9% of statements
ok      github.com/e1lite/devcamp-kakeibo/backend/internal/httpx         coverage: 26.3% of statements
?       github.com/e1lite/devcamp-kakeibo/backend/internal/model         [no test files]
ok      github.com/e1lite/devcamp-kakeibo/backend/internal/repository    coverage: 77.0% of statements
ok      github.com/e1lite/devcamp-kakeibo/backend/internal/server        coverage: 95.3% of statements
        github.com/e1lite/devcamp-kakeibo/backend/internal/service       coverage: 0.0% of statements
```

**静的解析は違反ゼロ、自動テストは全パスです。**

golangci-lint は標準セットに加え、`gosec`（セキュリティ）・`errorlint`・`bodyclose`・`sqlclosecheck`・`noctx`・`nilerr` などを有効にしています。`govet` の `shadow` も有効にし、`err` の握りつぶしを防いでいます。

リポジトリ層は**実 PostgreSQL に対する統合テスト**とし、各テストをトランザクションで包んでロールバックすることで、開発用データベースを汚さずに実行できるようにしています。DB を使わずに実行したい場合は `make test-unit`（`go test -short`）で単体テストのみを走らせられます。

**ハンドラ層とサービス層の単体テストは未整備**で、これらの動作は ③ の 92 件のエンドツーエンドの確認で担保している状態です。今後の課題とします。

---

## ⑤ まとめ

### 苦労したこと

**GORM が構造体から SQL を推測することによる不具合を 2 回出しました。**

いずれも `go vet`・golangci-lint・単体テストのすべてを通過し、SQL を実行して初めて表面化しました。

| # | 症状 | 原因 |
|---|---|---|
| 1 | `/auth/me` が 500 | `Select` の引数をカンマで分けて渡したため、GORM がそれぞれを別の列名として扱い、プレースホルダが置換されないまま SQL が組み立てられた |
| 2 | `/categories` が 500 | 読み込み先の構造体に集計用のフィールドがあり、`Select` を省略したため存在しない列 `categories.transaction_count` を SELECT に含めた |

1 つ目は動作確認中に自分で踏み、2 つ目は統合テストを追加した後だったため**テストが先に検出しました**。この経験からリポジトリ層を統合テストにする方針を固めました。

**統合テストが開発用データベースの状態に依存していました。** 「カテゴリが 9 件であること」という断言を書きましたが、既存のログインユーザのデータが入っていたため 18 件になり失敗しました。件数の判定は絶対値ではなく処理の前後の差分で行うように修正しました。

**同じ誤りを動作確認スクリプトでも犯していました。** `check` は HTTP ステータスしか検証しておらず、「2 件・合計 1780 円」といった説明文はどこからも確認されないただのラベルでした。実際には前回の実行分がデータベースに残っていて 3 件・2360 円が返っていましたが、それでも全件 pass し、その誤った数値を提出物にも書き写していました。**確認したつもりの項目が実際には何も確認していなかった**ことになります。レスポンスボディを検証する `expect` / `expect_meta` を追加し、件数・合計は実行開始時点からの差分で判定するようにしたうえで、作成した取引を最後に必ず削除して何度実行しても同じ状態から始められるようにしました。後始末を入れたことで、異常系のリクエストが誤ってデータを作っていないことも最後の 1 件で確認できるようになりました。

**コードを修正してもサーバに反映されず、原因の特定に時間を使いました。** `go run` は起動時に一度だけコンパイルするため、修正後に再起動しないと古いバイナリのまま動作します。新しいエンドポイントが `404` を返す形で表面化しました。

### 工夫したこと

① に記載のとおりです。特に、**セキュリティに関わる判断は必ずコードのコメントに理由を残す**ようにしました。後から読んだときに「なぜこうなっているか」が分かり、安易に変更されるのを防げるためです。

### 学んだこと

**静的解析の警告への対応は「直す」「誤検知として理由を残す」「実際に強化する」の 3 通りがあること。**

gosec の `G710`（オープンリダイレクト）が 2 箇所で出ましたが、対応は別々にしました。

- API-002 のリダイレクト先は oauth2 ライブラリが組み立てる固定のエンドポイントであり、クエリの `redirect_uri` は入り込みません。**誤検知としてコメントに理由を書いて抑制**しました。
- API-003 のリダイレクト先は許可リストで検証済みですが、検証は state 発行時にしか行っていませんでした。**リダイレクト直前にもう一度確認する処理を追加**し、実際に強化しました。

抑制するにしても「なぜ問題ないか」を書き残さなければ、次に読む人が判断できません。

**テストが通ることと、動くことは別であること。** カバレッジの高い層（config 100%・server 95%）には不具合が出ず、カバレッジ 0% だったリポジトリ層に 2 件の不具合が出ました。テストが無い場所にこそ不具合があるという当たり前のことを、実際に踏んで理解しました。

**ORM は「書く量が減る」代わりに「何が起きるか見えにくくなる」こと。** GORM の 2 件の不具合はいずれも「ライブラリが良かれと思って推測した結果」でした。結合や集計を伴うクエリでは `Select` を明示し、推測させないようにしています。

**仕様書と実装は放っておくと乖離すること。** 実装中に「仕様書に書いていないが必要なもの」が 6 件見つかりました（0 章の B）。その場で仕様書に反映しないと、次に仕様書を読む人が実装と違うものを作ることになります。

---

## 相談させていただきたいこと

### 今回の実装で判断した点（0 章 B-4 / B-5）

- API-013 の `category_source` を「常に `user`」から「カテゴリを変更した場合のみ」に変更したこと
- `JPY` 以外の通貨を `400 UNSUPPORTED_CURRENCY` で弾くことにしたこと

### Step 2 から持ち越している未確認の 3 点

| # | 事項 | 現状の設計 |
|---|---|---|
| 1 | `GET /inbox/unparsed` の位置づけ | `email_messages` のフィルタ済み一覧として設計 |
| 2 | `transactions.status`（`pending` / `confirmed`）の要否 | 保持する設計 |
| 3 | `merchants` を支店単位にした判断 | 1 行 = 1 支店、`brand_name` でチェーンを束ねる |

### 今後の課題

- ハンドラ層・サービス層の単体テストが未整備（現状はエンドツーエンドの確認で担保）
- OAuth の `state` をプロセス内メモリに保持しており、複数インスタンスに対応できない（Step 7 以降で共有ストアに置き換える必要があります）
- ログアウトがステートレスなため、漏洩したトークンは最大 24 時間有効（即時失効が必要になった場合は `jti` の失効リストを導入します）
