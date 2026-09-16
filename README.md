# 決済メール解析による自動家計簿サービス

DevOps Camp の最終課題。決済通知メールを解析して家計簿を自動生成するサービス。

| 項目 | 内容 |
|---|---|
| 言語 / フレームワーク | Go 1.26 / `net/http`（標準ライブラリ）+ GORM |
| データベース | PostgreSQL 17（Docker） |
| 設計ドキュメント | [docs/](./docs/)（日本語・正本） / [docs/zh/](./docs/zh/)（中文参考版） |

## ディレクトリ構成

```
.
├── docs/                    設計ドキュメント（Step 0〜2 の成果物）
├── backend/                 バックエンド（Step 3）
│   ├── cmd/api/             API サーバのエントリポイント
│   ├── cmd/migrate/         マイグレーション実行
│   ├── internal/
│   │   ├── config/          環境変数の読み込み
│   │   ├── database/        PostgreSQL への接続
│   │   ├── handler/         HTTP ハンドラ
│   │   ├── httpx/           共通レスポンス形式・エラーコード
│   │   └── server/          ルーティング・ミドルウェア
│   └── migrations/          スキーマ定義（SQL）
├── docker-compose.yml       PostgreSQL
└── Makefile                 開発コマンド
```

## セットアップ

前提: Go 1.26 以上、Docker、[golangci-lint](https://golangci-lint.run/) v2

```bash
make tools    # golangci-lint を入れる（未インストールの場合）
make setup    # PostgreSQL を起動し、マイグレーションを適用する
make run      # API サーバを起動する（http://localhost:8080）
```

動作確認:

```bash
curl -s http://localhost:8080/health | jq
```

```json
{
  "data": {
    "status": "ok",
    "version": "dev",
    "database": "ok",
    "checked_at": "2026-09-16T10:00:00+09:00"
  }
}
```

## よく使うコマンド

`make` だけ打つと一覧が出る。

| コマンド | 内容 |
|---|---|
| `make check` | 静的解析 + 自動テスト（**コミット前にこれを通す**） |
| `make lint` | 静的解析のみ（`go vet` + `golangci-lint`） |
| `make test` | 自動テストのみ |
| `make db-up` / `db-down` | PostgreSQL の起動 / 停止 |
| `make db-reset` | PostgreSQL を停止し、データも消す |
| `make db-shell` | `psql` に入る |
| `make migrate-up` | マイグレーションを最新まで適用 |
| `make migrate-down` | マイグレーションを 1 つ戻す |

## 設定

環境変数で設定する。`make` 経由なら既定値が入るため、通常は指定不要。

| 変数 | 既定値 | 説明 |
|---|---|---|
| `APP_ENV` | `local` | 実行環境 |
| `PORT` | `8080` | API サーバの待ち受けポート |
| `DATABASE_URL` | `postgres://kakeibo:kakeibo@localhost:5433/kakeibo?sslmode=disable` | PostgreSQL への接続文字列 |
| `JWT_SECRET` | （開発用の固定値） | Bearer トークンの署名鍵 |

> ホスト側のポートを **5433** にしているのは、ローカルに PostgreSQL が入っていても
> 衝突しないようにするため。

## 実装範囲

Step 2 のフィードバックを受け、Step 3 の実装対象は
**API 仕様書 5.1 の「Step 3（コア）」**に絞っている。

- **テーブル 6 件**: `users` / `mail_accounts` / `categories` / `payment_methods` / `merchants` / `transactions`
- **エンドポイント 13 件**: API-001〜005（ヘルスチェック・認証）、
  API-010〜014（取引 CRUD）、API-019〜021（カテゴリ CRUD）

メール連携・パースバッチ・名寄せ（P1〜P5）、通知・サブスク・予算・月次サマリ
（P4.5 / P7 / P8）は Step 8 以降に回す。詳細は
[docs/step2-api-spec.md](./docs/step2-api-spec.md) の 5.1 を参照。

## 設計上の約束

| 項目 | 方針 |
|---|---|
| 金額 | 最小単位の整数（`amount_minor`）。浮動小数点は使わない |
| 日時 | `timestamptz` で保持。API は ISO 8601 でタイムゾーンオフセットを必ず含める |
| 削除 | `transactions` のみ論理削除（`deleted_at`）。参照時は常に `deleted_at IS NULL` で絞る |
| マルチテナント | 全業務テーブルが `user_id` を持つ。他ユーザのリソースは `403` |
| マイグレーション | GORM の `AutoMigrate` は使わない。`migrations/` の SQL を正とする |
