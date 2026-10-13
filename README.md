# 決済メール解析による自動家計簿サービス

DevOps Camp の最終課題。決済通知メールを解析して家計簿を自動生成するサービス。

| 項目 | 内容 |
|---|---|
| 言語 / フレームワーク | Go 1.26 / `net/http`（標準ライブラリ）+ GORM |
| フロントエンド | Next.js 16（App Router）+ TypeScript + Tailwind CSS（[frontend/README.md](./frontend/README.md)） |
| データベース | PostgreSQL 17（Docker） |
| 設計ドキュメント | [docs/](./docs/) |

## ディレクトリ構成

```
.
├── docs/                    設計ドキュメント（Step 0〜4 の成果物）
├── backend/                 バックエンド（Step 3）
│   ├── cmd/api/             API サーバのエントリポイント
│   ├── cmd/migrate/         マイグレーション実行
│   ├── internal/
│   │   ├── auth/            JWT の発行・検証、OAuth の state 管理
│   │   ├── config/          環境変数の読み込み
│   │   ├── database/        PostgreSQL への接続
│   │   ├── handler/         HTTP ハンドラ
│   │   ├── httpx/           共通レスポンス形式・エラーコード
│   │   ├── model/           テーブルに対応する構造体
│   │   ├── repository/      データベースアクセス
│   │   ├── server/          ルーティング・ミドルウェア
│   │   └── service/         業務ロジック
│   └── migrations/          スキーマ定義（SQL）
├── frontend/                フロントエンド（Step 6）。構成は frontend/README.md
├── docker-compose.yml       PostgreSQL
└── Makefile                 開発コマンド
```

## セットアップ

前提: Go 1.26 以上、Docker、[golangci-lint](https://golangci-lint.run/) v2

```bash
make tools                            # golangci-lint を入れる（未インストールの場合）
cp .env.local.example .env.local      # Google OAuth の認証情報を設定する（下記）
make setup                            # PostgreSQL を起動し、マイグレーションを適用する
make run                              # API サーバを起動する（http://localhost:8080）
```

フロントエンドは別のターミナルで起動する（前提: Node.js 20.9 以上）。

```bash
cd frontend
npm ci
npm run dev                           # http://localhost:5173 を開き、Google でログインする
```

### Google OAuth のセットアップ

ログイン（API-002 / API-003）に Google OAuth を使うため、
自分の Google Cloud プロジェクトで OAuth クライアントを作る必要がある。

1. [Google Cloud Console](https://console.cloud.google.com/) でプロジェクトを作る
2. **API とサービス → OAuth 同意画面** で、User Type に「外部」を選び、
   アプリ名・サポートメール・デベロッパー連絡先を入力して保存する
3. スコープの追加で `openid` / `.../auth/userinfo.email` /
   `.../auth/userinfo.profile` の 3 つを選ぶ
   （メール読み取りの `gmail.readonly` は下の「メール連携の追加設定」で足す）
4. テストユーザーに自分の Google アカウントを追加する
5. **API とサービス → 認証情報 → 認証情報を作成 → OAuth クライアント ID**
   - アプリケーションの種類: **ウェブ アプリケーション**
   - 承認済みのリダイレクト URI: `http://localhost:8080/api/v1/auth/google/callback`
6. 表示されたクライアント ID とクライアントシークレットを `.env.local` に書く

```bash
cp .env.local.example .env.local
# エディタで GOOGLE_CLIENT_ID と GOOGLE_CLIENT_SECRET を埋める
```

> `.env.local` は `.gitignore` に入っているためコミットされない。

### メール連携（Gmail）の追加設定

決済の通知メールを取り込む（API-029 / API-030、[docs/mail-import-mvp.md](./docs/mail-import-mvp.md)）ため、
同じプロジェクトに次を足す。クライアント ID とシークレットはログインと同じものを使う。

1. **API とサービス → ライブラリ** で「Gmail API」を有効にする
2. **Google Auth Platform → データアクセス** で `.../auth/gmail.readonly` を追加する
   （「制限付きのスコープ」になるが、公開ステータスが「テスト」のままなら審査は不要）
3. **Google Auth Platform → 対象** のテストユーザーに、連携する Gmail のアドレスを追加する
   （ログインしたアカウントと別のアドレスでよい）
4. **クライアント** の承認済みのリダイレクト URI に
   `http://localhost:8080/api/v1/mail-accounts/google/callback` を追加する

> 公開ステータスが「テスト」の間は、Google がリフレッシュトークンを **7 日で失効**させる。
> 失効するとメール連携画面に「再認証」が出るため、押して連携し直す。

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
| `make test` | 自動テスト（統合テストを含む。DB が起動していること） |
| `make test-unit` | 単体テストのみ（DB 不要） |
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
| `GOOGLE_CLIENT_ID` | **必須**（`.env.local`） | Google OAuth のクライアント ID |
| `GOOGLE_CLIENT_SECRET` | **必須**（`.env.local`） | Google OAuth のクライアントシークレット |
| `GOOGLE_REDIRECT_URL` | `http://localhost:8080/api/v1/auth/google/callback` | Google に登録したコールバック URL |
| `DEFAULT_REDIRECT_URI` | `http://localhost:5173/auth/callback` | ログイン後の既定の戻り先 |
| `ALLOWED_REDIRECT_URIS` | ログインのコールバックとメール連携画面 | 戻り先の許可リスト（カンマ区切り） |
| `GOOGLE_MAIL_REDIRECT_URL` | `http://localhost:8080/api/v1/mail-accounts/google/callback` | Google に登録したメール連携のコールバック URL |
| `DEFAULT_MAIL_REDIRECT_URI` | `http://localhost:5173/settings/mail-accounts` | メール連携後の既定の戻り先 |
| `MAIL_CREDENTIAL_KEY` | （開発用の固定値） | メール連携の認証情報（リフレッシュトークン）を暗号化する鍵。32 バイトを base64 で。本番では `openssl rand -base64 32` で作った別の鍵を渡す |
| `MAIL_SEARCH_QUERY` | 件名で探す仮の条件 | テンプレートが 1 つも無いときに Gmail を探す条件 |
| `MAIL_BACKFILL_DAYS` | `90` | 初回の取込で遡る日数 |

> ホスト側のポートを **5433** にしているのは、ローカルに PostgreSQL が入っていても
> 衝突しないようにするため。

## 実装範囲

Step 2 のフィードバックを受け、Step 3 の実装対象は
**API 仕様書 5.1 の「Step 3（コア）」**に絞っている。

- **テーブル 6 件**: `users` / `mail_accounts` / `categories` / `payment_methods` / `merchants` / `transactions`
- **エンドポイント 13 件**: API-001〜005（ヘルスチェック・認証）、
  API-010〜014（取引 CRUD）、API-019〜021（カテゴリ CRUD）

Step 6 では、画面設計で足りないと分かった API-031（決済手段一覧）などを足した。

**メール取込は最小版を実装している**（[docs/mail-import-mvp.md](./docs/mail-import-mvp.md)）。
Gmail を連携し、手動の同期で三井住友カードの利用通知を取り込み、取引にする
（API-006〜009 / 029 / 030、テーブル `sync_jobs` / `parser_templates` / `email_messages` / `payment_events`）。
名寄せ（P5）・自動分類（P6）・定期実行、通知・サブスク・予算・月次サマリ（P4.5 / P7 / P8）は拡張に回す。
詳細は [docs/step2-api-spec.md](./docs/step2-api-spec.md) の 5.1 を参照。

## 設計上の約束

| 項目 | 方針 |
|---|---|
| 金額 | 最小単位の整数（`amount_minor`）。浮動小数点は使わない |
| 日時 | `timestamptz` で保持。API は ISO 8601 でタイムゾーンオフセットを必ず含める |
| 削除 | `transactions` のみ論理削除（`deleted_at`）。参照時は常に `deleted_at IS NULL` で絞る |
| マルチテナント | 全業務テーブルが `user_id` を持つ。他ユーザのリソースは、存在しない場合と同じ `404`（存在を伝えないため） |
| マイグレーション | GORM の `AutoMigrate` は使わない。`migrations/` の SQL を正とする |
| テスト | リポジトリ層は**実 PostgreSQL に対する統合テスト**。各テストをトランザクションで包んでロールバックするため、開発用データベースをそのまま使っても中身は汚れない |

> **なぜリポジトリ層を統合テストにするか**: GORM の API の使い方の誤りは
> SQL を実行して初めて表面化する。実際に `Select` の引数の渡し方を誤って
> 構文エラーになるバグが出たが、`go vet`・`golangci-lint`・モックを使った
> 単体テストのいずれも検出できなかった。
