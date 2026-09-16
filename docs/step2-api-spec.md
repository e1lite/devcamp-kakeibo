# API 仕様書

サービス名: 決済メール解析による自動家計簿サービス
言語 / フレームワーク: Go / net/http（標準ライブラリ）+ GORM
対応する仕様書: [step2-db-spec.md](./step2-db-spec.md) / [step2-er.md](./step2-er.md)

> メンタリングのフィードバックを反映済み。全 28 エンドポイントの詳細仕様を記載している。
> 挙動が同形のエンドポイントは、レスポンス構造などを
> **「API-0XX と同形式」の参照表記で簡略化**している（フィードバックで許容された運用）。

> **フレームワーク変更（Step 3 反映）**
> 当初は Gin を想定していたが、Step 3 の実施要件が「Step 1・2 で学んだ Web フレームワーク
> （FastAPI もしくは net/http）」を指定しているため、**標準ライブラリの `net/http` に変更**した。
> Go 1.22 以降の `http.ServeMux` は `"GET /api/v1/transactions/{id}"` 形式の
> メソッド + パスパターンをサポートしており、本仕様のルーティングは標準ライブラリで充足できる。

---

## 1. API の概要

決済通知メールから自動抽出した支出データを提供する REST API。

扱う主なデータは以下の 4 つ。

| データ | 説明 |
|---|---|
| 取引（transaction） | 名寄せ後の実際の支出 1 件 |
| 決済イベント（payment_event） | メールから抽出した生の決済 1 件。取引の構成要素 |
| 店舗（merchant） | 正規化済みの店舗。住所・座標を持つ |
| 通知（notification） | 即時通知・予算超過通知・月次サマリの送信履歴 |

メールの取得・解析・名寄せ・分類はバックグラウンドのバッチが行い、
本 API は **その結果の参照と訂正**、および設定の管理を担当する。

---

## 2. 共通仕様

### ベース URL

```
http://<your-host>/api/v1
```

### 日時フォーマット

ISO 8601（例: `2026-09-09T12:34:56+09:00`）。**タイムゾーンオフセットを必ず含む。**

### 金額

すべて **最小単位の整数**（円の場合 1 = 1 円）で表現する。小数は用いない。

```json
{ "amount_minor": 580, "currency": "JPY" }
```

### ページネーション

一覧系のエンドポイントは以下のクエリパラメータを受け付ける。

| パラメータ | 型 | 既定値 | 説明 |
|---|---|---|---|
| limit | integer | 20 | 取得件数（最大 100） |
| offset | integer | 0 | 取得開始位置 |

レスポンスの `meta` に総件数を含める。

### レスポンス形式

成功時:

```json
{ "data": { }, "meta": { } }
```

エラー時:

```json
{ "error": { "code": "ERROR_CODE", "message": "エラーメッセージ" } }
```

---

## 3. 認証方式

本サービスの認証は **2 層に分かれる**。混同しないよう明確に分離する。

| 層 | 対象 | 方式 |
|---|---|---|
| **アプリ認証** | この API を呼ぶ権利 | Google OAuth でログイン後に発行する **Bearer トークン（JWT / HS256、有効期限 24 時間）** |
| **メール連携認証** | ユーザのメールを読む権利 | Google OAuth（readonly スコープ）等。取得した認証情報は暗号化して `mail_accounts` に保存し、API のリクエストには使わない |

ヘルスチェックと OAuth の開始・コールバックを除く、すべてのリクエストに Bearer トークンが必要。

```
Authorization: Bearer <access_token>
```

---

## 4. ステータスコード一覧

| ステータスコード | 意味 |
|---|---|
| 200 | リクエスト成功 |
| 201 | リソース作成成功 |
| 202 | 非同期処理を受け付けた（同期の手動実行など） |
| 204 | 成功、返却するコンテンツなし |
| 400 | バリデーションエラー / ビジネスロジックエラー |
| 401 | 認証エラー（トークン不正・期限切れ） |
| 403 | 権限エラー（他ユーザのリソースへのアクセス） |
| 404 | リソースが見つからない |
| 409 | 競合（重複登録、状態不整合） |
| 500 | サーバ内部エラー |
| 503 | サービス利用不可（ヘルスチェックで DB へ疎通できない場合） |

---

## 5. エンドポイント一覧

| No. | API ID | API 名 | メソッド | エンドポイント | 認証 | 実装フェーズ |
|---|---|---|---|---|---|---|
| 1 | API-001 | ヘルスチェック | GET | `/health` | 不要 | **Step 3（コア）** |
| 2 | API-002 | Google OAuth 開始 | GET | `/api/v1/auth/google` | 不要 | **Step 3（コア）** |
| 3 | API-003 | Google OAuth コールバック | GET | `/api/v1/auth/google/callback` | 不要 | **Step 3（コア）** |
| 4 | API-004 | 現在のユーザ取得 | GET | `/api/v1/auth/me` | 必要 | **Step 3（コア）** |
| 5 | API-005 | ログアウト | POST | `/api/v1/auth/logout` | 必要 | **Step 3（コア）** |
| 6 | API-006 | 連携メールアカウント一覧 | GET | `/api/v1/mail-accounts` | 必要 | Step 8 以降（拡張） |
| 7 | API-007 | 連携解除 | DELETE | `/api/v1/mail-accounts/{id}` | 必要 | Step 8 以降（拡張） |
| 8 | API-008 | 手動同期の実行 | POST | `/api/v1/mail-accounts/{id}/sync` | 必要 | Step 8 以降（拡張） |
| 9 | API-009 | 同期履歴取得 | GET | `/api/v1/sync-jobs` | 必要 | Step 8 以降（拡張） |
| 10 | API-010 | 取引一覧取得 | GET | `/api/v1/transactions` | 必要 | **Step 3（コア）** |
| 11 | API-011 | 取引の手動登録 | POST | `/api/v1/transactions` | 必要 | **Step 3（コア）** |
| 12 | API-012 | 取引詳細取得 | GET | `/api/v1/transactions/{id}` | 必要 | **Step 3（コア）** |
| 13 | API-013 | 取引の修正 | PUT | `/api/v1/transactions/{id}` | 必要 | **Step 3（コア）**※1 |
| 14 | API-014 | 取引の削除 | DELETE | `/api/v1/transactions/{id}` | 必要 | **Step 3（コア）** |
| 15 | API-015 | 取引のマージ（名寄せ） | POST | `/api/v1/transactions/merge` | 必要 | Step 8 以降（拡張） |
| 16 | API-016 | マージ解除 | POST | `/api/v1/transactions/{id}/unmerge` | 必要 | Step 8 以降（拡張） |
| 17 | API-017 | 未解析メール一覧 | GET | `/api/v1/inbox/unparsed` | 必要 | Step 8 以降（拡張） |
| 18 | API-018 | 決済メール判定の教示 | POST | `/api/v1/inbox/{id}/classify` | 必要 | Step 8 以降（拡張） |
| 19 | API-019 | カテゴリ一覧 | GET | `/api/v1/categories` | 必要 | **Step 3（コア）** |
| 20 | API-020 | カテゴリ作成 | POST | `/api/v1/categories` | 必要 | **Step 3（コア）** |
| 21 | API-021 | カテゴリ更新 / 削除 | PUT / DELETE | `/api/v1/categories/{id}` | 必要 | **Step 3（コア）** |
| 22 | API-022 | 店舗一覧 / 修正 | GET / PUT | `/api/v1/merchants[/{id}]` | 必要 | Step 8 以降（拡張） |
| 23 | API-023 | 月次サマリ取得 | GET | `/api/v1/summaries/monthly` | 必要 | Step 8 以降（拡張） |
| 24 | API-024 | サブスク一覧 | GET | `/api/v1/subscriptions` | 必要 | Step 8 以降（拡張） |
| 25 | API-025 | サブスク状態更新 | PUT | `/api/v1/subscriptions/{id}` | 必要 | Step 8 以降（拡張） |
| 26 | API-026 | 予算の取得 / 設定 | GET / PUT | `/api/v1/budgets` | 必要 | Step 8 以降（拡張） |
| 27 | API-027 | 通知設定の取得 / 更新 | GET / PUT | `/api/v1/notification-settings` | 必要 | Step 8 以降（拡張） |
| 28 | API-028 | 通知履歴一覧 | GET | `/api/v1/notifications` | 必要 | Step 8 以降（拡張） |

※1 API-013 のうち、**「副作用としてカテゴリ規則を学習する」処理は Step 8 以降に回す**。
学習には `category_rules` テーブルが必要だが、同テーブルは Step 3 のコア 6 テーブルに含めないため。
Step 3 では取引の更新のみを行い、`category_rules` への書き込みは実装しない。

### 5.1 実装フェーズの考え方

Step 2 のフィードバックで、全エンドポイントを Step 3 の期間内に実装するのは分量的に厳しく、
後続の Step 4〜6（フロント）・Step 7 以降（インフラ）を止めないための
**最低限ラインを先に切る**方針で合意した。本仕様書ではその線を上表の `実装フェーズ` 列で明示する。

| フェーズ | 内容 | 件数 |
|---|---|---|
| **Step 3（コア）** | 手入力の家計簿として成立し、Step 7 以降のインフラ構築に必要な「動くバックエンド」が成立する最小範囲 | 13 行 / 14 操作 |
| Step 8 以降（拡張） | メール連携・パースバッチ・名寄せ（P1〜P5）、通知・サブスク・予算・月次サマリ（P4.5 / P7 / P8） | 15 行 |

コア範囲の選定根拠は以下のとおり。

| API ID | コアに含める理由 |
|---|---|
| API-001 | Step 7 以降のインフラでヘルスチェックに使う |
| API-002 / API-003 | ログインできないと Bearer トークンが発行されず、他のすべてのエンドポイントを検証できない |
| API-004 / API-005 | 認証まわりの最小セット |
| API-010〜014 | 家計簿の中核。取引の CRUD が揃えばフロントが動作検証できる |
| API-019〜021 | 取引のカテゴリ分類に必要なマスタ。CRUD の形を最初に定型化する対象でもある |

> **Step 3 の実装対象は「`実装フェーズ` 列が `Step 3（コア）` の全エンドポイント」とする。**
> Step 8 以降（拡張）のエンドポイントも本仕様書では設計を確定させておき、コアが動いた後に段階的に追加する。

> **削除系エンドポイントの挙動**
>
> `DELETE` は「クライアントから見てそのリソースを消す」という意味であり、
> **サーバ側の物理削除とは一致しない**。本サービスでは以下のとおり扱う。
>
> | エンドポイント | サーバ側の処理 | ステータス |
> |---|---|---|
> | API-007 連携解除 | `mail_accounts.status = 'disabled'` に更新し `credential_encrypted` を破棄。**行は削除しない**（メールの生データを CASCADE で失わないため） | 204 |
> | API-014 取引の削除 | `transactions.deleted_at = now()` の**論理削除**。物理削除すると決済イベントが未名寄せに戻り、次のバッチで同じ取引が再生成されるため | 204 |
> | API-021 カテゴリ削除 | 物理削除。参照している取引の `category_id` は FK により `NULL` になる | 204 |
>
> 詳細は [step2-db-spec.md](./step2-db-spec.md) の 3.2 / 3.7 の削除方針を参照。

**API の類型**（詳細仕様は 6 章に全 28 件を記載。以下は形ごとの代表例）

| 類型 | 代表 | 同形のエンドポイント |
|---|---|---|
| 認証が必要な最小の取得系 | API-004 | API-027（設定 1 件の取得） |
| 一覧系（ページネーション + 絞り込み） | API-010 | API-009 / API-017 / API-028 |
| 登録系（バリデーション + 201） | API-011 | API-020 |
| 更新系（**副作用としてカテゴリ規則を学習する**） | API-013 | API-021 PUT / API-022 PUT / API-025 |
| CRUD に収まらないアクション型 | API-015 | API-008 / API-016 / API-018 |
| 集計系（DB のレコードをそのまま返さない） | API-023 | — |
| 削除系（サーバ側の処理がエンドポイントごとに異なる） | API-014 | API-007 / API-021 DELETE |

---

## 6. 各 API の詳細仕様

> 特に記載のない限り、すべてのエンドポイントは `401 INVALID_TOKEN`（トークン不正）と
> `401 TOKEN_EXPIRED`（有効期限切れ）を返しうる。以降のエラー表では**この 2 つを省略**する。
> 他ユーザのリソースを指定した場合は一律 `403 FORBIDDEN` を返す（`404` にしない。
> 存在の有無を推測されないようにするより、**自分のデータでないことを明示する**方が UI 上扱いやすいため）。

### API-001 ヘルスチェック

死活監視用。**認証不要**。Step 7 以降のインフラ（ロードバランサ・コンテナのヘルスチェック）から呼ばれる。

```
GET /health
```

**リクエストパラメータ**: なし

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data.status | string | 常に `ok` |
| data.version | string | アプリのバージョン（ビルド時に埋め込む） |
| data.database | string | DB への疎通結果（`ok`） |
| data.checked_at | string | 確認日時（ISO 8601） |

**レスポンス例**

```json
{
  "data": {
    "status": "ok",
    "version": "1.0.0",
    "database": "ok",
    "checked_at": "2026-09-16T10:00:00+09:00"
  }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 503 | DATABASE_UNAVAILABLE | DB へ疎通できない |

> **`200` を返す条件に DB 疎通を含める理由**: プロセスが生きていても DB に繋がらなければ
> リクエストは処理できない。ヘルスチェックが `200` を返し続けると、
> 壊れたインスタンスにトラフィックが流れ続けてしまう。

---

### API-002 Google OAuth 開始

Google の認可画面へリダイレクトする。**認証不要**。

```
GET /api/v1/auth/google
```

**リクエストパラメータ（クエリ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| redirect_uri | string | - | 認証完了後に戻すフロントの URL（未指定時は既定値） |

**レスポンス（302 Found）**

JSON は返さず、`Location` ヘッダで Google の認可エンドポイントへリダイレクトする。
CSRF 対策の `state` を発行し、`redirect_uri` とともにサーバ側に一時保存する（有効期限 10 分）。

| ヘッダ名 | 説明 |
|---|---|
| Location | `https://accounts.google.com/o/oauth2/v2/auth?...&state=<発行した state>` |

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | INVALID_REDIRECT_URI | `redirect_uri` が許可リストにない |

> **要求するスコープ**: ログインには `openid` / `email` / `profile` のみを要求する。
> メール読み取りの `gmail.readonly` は**この時点では要求しない**。
> 3 章のとおり認証を 2 層に分けており、メール連携は別途ユーザが明示的に許可する導線とする。

---

### API-003 Google OAuth コールバック

Google からのリダイレクトを受け、ユーザを特定して Bearer トークンを発行する。**認証不要**。
`users` に該当レコードがなければ作成する（`google_sub` で突合）。

```
GET /api/v1/auth/google/callback
```

**リクエストパラメータ（クエリ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| code | string | ○ | Google が発行した認可コード |
| state | string | ○ | API-002 で発行した `state` |

**レスポンス（302 Found / 200 OK）**

| 条件 | 挙動 |
|---|---|
| 既定（ブラウザからの遷移） | API-002 で保存した `redirect_uri` へ 302。トークンは URL フラグメント（`#access_token=...`）に載せる |
| `Accept: application/json` | 200 で下記の JSON を返す。**curl での動作確認に使う** |

| フィールド名 | 型 | 説明 |
|---|---|---|
| data.access_token | string | Bearer トークン（JWT / HS256） |
| data.token_type | string | 常に `Bearer` |
| data.expires_in | integer | 有効期限（秒。86400） |
| data.user.id | integer | ユーザ ID |
| data.user.email | string | メールアドレス |
| meta.is_new_user | boolean | 今回のログインで `users` を新規作成したか |

**レスポンス例**

```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "token_type": "Bearer",
    "expires_in": 86400,
    "user": { "id": 1, "email": "user@example.com" }
  },
  "meta": { "is_new_user": true }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | INVALID_STATE | `state` が未発行・期限切れ・不一致（CSRF の疑い） |
| 400 | OAUTH_EXCHANGE_FAILED | 認可コードとトークンの交換に失敗 |
| 403 | ACCESS_DENIED | ユーザが Google の同意画面で拒否した |

> **新規ユーザ作成時の初期データ**: `categories` に `is_system = true` の初期カテゴリ
> （`未分類` を含む）と、`payment_methods` に `現金` を作成する。
> カテゴリが 1 件もないと取引を登録できないため。

---

### API-004 現在のユーザ取得

```
GET /api/v1/auth/me
```

**リクエストパラメータ**: なし

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data.id | integer | ユーザ ID |
| data.email | string | メールアドレス |
| data.display_name | string / null | 表示名 |
| data.timezone | string | 集計の基準タイムゾーン |
| data.mail_accounts_count | integer | 連携済みメールアカウント数 |
| data.needs_reauth | boolean | 再認証が必要な連携があるか |

**レスポンス例**

```json
{
  "data": {
    "id": 1,
    "email": "user@example.com",
    "display_name": "楊金澤",
    "timezone": "Asia/Tokyo",
    "mail_accounts_count": 1,
    "needs_reauth": false
  }
}
```

**エラーレスポンス**

共通の `401` のみ（冒頭の注記を参照）。

---

### API-005 ログアウト

```
POST /api/v1/auth/logout
```

**リクエストパラメータ**: なし

**レスポンス（204 No Content）**

ボディなし。

**エラーレスポンス**

共通の `401` のみ。

> **サーバ側で何をするか**
> 本 API は**ステートレスな JWT** を採用しているため、サーバに失効リストを持たない。
> ログアウトの実体は「クライアントがトークンを破棄すること」であり、
> 本エンドポイントは**その完了を受け付けて `204` を返すだけ**である。
>
> 代償として、漏洩したトークンは最大 24 時間（`expires_in`）有効なままになる。
> 即時失効が必要になった場合は、JWT に `jti` を持たせて失効リストのテーブルを
> 追加する形で対応する（有効期限が 24 時間のため、リストの保持も 24 時間で足りる）。
> 現時点では、そのテーブルと全リクエストでの参照コストに見合わないと判断した。

---

### API-006 連携メールアカウント一覧

```
GET /api/v1/mail-accounts
```

**リクエストパラメータ**: なし

> 1 ユーザあたり数件を想定するため、**ページネーションは設けない**。

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data[].id | integer | 連携 ID |
| data[].email_address | string | 連携したメールアドレス |
| data[].provider | string | `gmail_api` / `gas` / `imap` |
| data[].status | string | `active` / `reauth_required` / `disabled` |
| data[].last_synced_at | string / null | 最終同期日時。**`status = active` なのに古い場合はバッチ停止を示す** |
| data[].credential_expires_at | string / null | 認証情報の有効期限 |
| data[].backfilled_until | string / null | 過去メールの取込が完了している起点 |
| meta.total | integer | 件数 |

**レスポンス例**

```json
{
  "data": [
    {
      "id": 1,
      "email_address": "user@gmail.com",
      "provider": "gmail_api",
      "status": "active",
      "last_synced_at": "2026-09-16T09:59:30+09:00",
      "credential_expires_at": null,
      "backfilled_until": "2026-06-01T00:00:00+09:00"
    }
  ],
  "meta": { "total": 1 }
}
```

**エラーレスポンス**

共通の `401` のみ。

> `credential_encrypted` は**いかなる場合もレスポンスに含めない**。

---

### API-007 連携解除

**行は削除しない。** `status = 'disabled'` に更新し、`credential_encrypted` を空にする。
`email_messages` 以降のデータはすべて残る（DB 仕様書 3.2 の削除方針を参照）。

```
DELETE /api/v1/mail-accounts/{id}
```

**リクエストパラメータ（パス）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| id | integer | ○ | 連携 ID |

**レスポンス（204 No Content）**

ボディなし。

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 403 | FORBIDDEN | 他ユーザの連携 |
| 404 | MAIL_ACCOUNT_NOT_FOUND | 連携が存在しない |
| 409 | ALREADY_DISABLED | すでに解除済み |

---

### API-008 手動同期の実行

メール取得バッチを即時実行する。**非同期**のため `202` で受け付けのみを返す。
進捗は API-009 で確認する。

```
POST /api/v1/mail-accounts/{id}/sync
```

**リクエストパラメータ（パス）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| id | integer | ○ | 連携 ID |

**リクエストパラメータ（ボディ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| mode | string | - | `incremental`（既定、差分取得） / `backfill`（過去メールの遡り取得） |

**リクエスト例**

```json
{ "mode": "incremental" }
```

**レスポンス（202 Accepted）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data.sync_job_id | integer | 作成された同期ジョブ ID（API-009 で照会する） |
| data.status | string | 常に `running` |
| data.trigger | string | 常に `manual` |
| data.started_at | string | 開始日時 |

**レスポンス例**

```json
{
  "data": {
    "sync_job_id": 892,
    "status": "running",
    "trigger": "manual",
    "started_at": "2026-09-16T10:00:00+09:00"
  }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 403 | FORBIDDEN | 他ユーザの連携 |
| 404 | MAIL_ACCOUNT_NOT_FOUND | 連携が存在しない |
| 409 | SYNC_ALREADY_RUNNING | 同じ連携の同期が実行中 |
| 409 | ACCOUNT_DISABLED | 連携が解除済み |
| 409 | REAUTH_REQUIRED | 認証情報が失効しており再認証が必要 |

> **`202` を返す理由**: 取得件数によっては数十秒かかるため、同期の完了を待たない。
> `sync_jobs` に行を作ってからジョブを起動し、その ID を返すことで、
> クライアントは進捗を追跡できる。

---

### API-009 同期履歴取得

一覧系のため、パラメータとページネーションの扱いは **API-010 と同形式**。

```
GET /api/v1/sync-jobs
```

**リクエストパラメータ（クエリ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| mail_account_id | integer | - | 連携で絞り込む |
| status | string | - | `running` / `success` / `partial` / `failed` |
| limit | integer | - | 取得件数（既定 20、最大 100） |
| offset | integer | - | 取得開始位置（既定 0） |

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data[].id | integer | ジョブ ID |
| data[].mail_account_id | integer | 対象の連携 |
| data[].trigger | string | `scheduled` / `manual` / `backfill` / `push` |
| data[].started_at | string | 開始日時 |
| data[].finished_at | string / null | 終了日時（実行中は `null`） |
| data[].status | string | `running` / `success` / `partial` / `failed` |
| data[].fetched_count | integer | 取得したメール数 |
| data[].parsed_count | integer | 抽出に成功した数 |
| data[].failed_count | integer | 抽出に失敗した数 |
| data[].error_message | string / null | エラー内容 |
| meta.total | integer | 条件に合致する総件数 |

**レスポンス例**

```json
{
  "data": [
    {
      "id": 892,
      "mail_account_id": 1,
      "trigger": "manual",
      "started_at": "2026-09-16T10:00:00+09:00",
      "finished_at": "2026-09-16T10:00:12+09:00",
      "status": "success",
      "fetched_count": 3,
      "parsed_count": 3,
      "failed_count": 0,
      "error_message": null
    }
  ],
  "meta": { "total": 41 }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | LIMIT_TOO_LARGE | `limit` が 100 を超えている |

> **この一覧に「新着なし」の実行は現れない。** DB 仕様書 3.3 の方針どおり、
> `fetched_count = 0` かつ `status = success` の実行は `sync_jobs` に記録しないため。
> バッチが動いているかは API-006 の `last_synced_at` で確認する。

---

### API-010 取引一覧取得

```
GET /api/v1/transactions
```

**リクエストパラメータ（クエリ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| from | string | - | 対象期間の開始日（`YYYY-MM-DD`、JST 基準） |
| to | string | - | 対象期間の終了日（`YYYY-MM-DD`、JST 基準） |
| category_id | integer | - | カテゴリで絞り込む |
| payment_method_id | integer | - | 決済手段で絞り込む |
| merchant_id | integer | - | 店舗で絞り込む |
| needs_review | boolean | - | `true` の場合、疑似重複フラグが立っている取引のみ |
| q | string | - | 店舗名・メモの部分一致検索 |
| limit | integer | - | 取得件数（既定 20、最大 100） |
| offset | integer | - | 取得開始位置（既定 0） |

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data[].id | integer | 取引 ID |
| data[].occurred_at | string | 決済日時（ISO 8601） |
| data[].amount_minor | integer | 金額（最小単位） |
| data[].currency | string | 通貨コード |
| data[].merchant | object / null | 店舗情報 |
| data[].merchant.id | integer | 店舗 ID |
| data[].merchant.name | string | 店舗名 |
| data[].merchant.address | string / null | 住所（ジオコーディング済みの場合） |
| data[].category | object / null | カテゴリ情報 |
| data[].payment_method | object / null | 決済手段 |
| data[].source | string | `email` / `manual` |
| data[].status | string | `pending` / `confirmed` |
| data[].is_possible_duplicate | boolean | 疑似重複の確認待ちか |
| data[].event_count | integer | 紐づく決済イベント数 |
| meta.total | integer | 条件に合致する総件数 |
| meta.total_amount_minor | integer | 条件に合致する取引の合計金額 |

**レスポンス例**

```json
{
  "data": [
    {
      "id": 1024,
      "occurred_at": "2026-09-09T12:31:00+09:00",
      "amount_minor": 580,
      "currency": "JPY",
      "merchant": {
        "id": 42,
        "name": "セブン-イレブン渋谷道玄坂店",
        "address": "東京都渋谷区道玄坂1-1-1"
      },
      "category": { "id": 3, "name": "食費" },
      "payment_method": { "id": 2, "name": "楽天カード", "kind": "credit_card" },
      "source": "email",
      "status": "confirmed",
      "is_possible_duplicate": false,
      "event_count": 1
    }
  ],
  "meta": { "total": 187, "total_amount_minor": 143250 }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | INVALID_DATE_RANGE | `from` が `to` より後 |
| 400 | LIMIT_TOO_LARGE | `limit` が 100 を超えている |

---

### API-011 取引の手動登録

現金など、通知メールが発生しない決済を登録する。
決済イベントを経由せず `transactions` に直接作成される（`source = manual`）。

```
POST /api/v1/transactions
```

**リクエストパラメータ（ボディ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| occurred_at | string | ○ | 決済日時（ISO 8601） |
| amount_minor | integer | ○ | 金額（最小単位、1 以上） |
| currency | string | - | 通貨コード（既定 `JPY`） |
| merchant_name | string | - | 店舗名（既存になければ新規作成する） |
| category_id | integer | - | カテゴリ ID |
| payment_method_id | integer | - | 決済手段 ID |
| note | string | - | メモ（500 文字以内） |

**リクエスト例**

```json
{
  "occurred_at": "2026-09-09T19:20:00+09:00",
  "amount_minor": 1200,
  "merchant_name": "近所の定食屋",
  "category_id": 3,
  "payment_method_id": 5,
  "note": "現金"
}
```

**レスポンス（201 Created）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data.id | integer | 作成された取引 ID |
| data.occurred_at | string | 決済日時 |
| data.amount_minor | integer | 金額 |
| data.merchant | object / null | 店舗（新規作成された場合を含む） |
| data.category | object / null | カテゴリ |
| data.source | string | 常に `manual` |
| data.is_user_edited | boolean | 常に `true`（再計算の対象外） |

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | 必須項目の欠落、形式不正 |
| 400 | INVALID_AMOUNT | 金額が 0 以下 |
| 404 | CATEGORY_NOT_FOUND | 指定したカテゴリが存在しない |
| 404 | PAYMENT_METHOD_NOT_FOUND | 指定した決済手段が存在しない |

---

### API-012 取引詳細取得

一覧（API-010）では返さない**メモ・円換算額・紐づく決済イベント**を含めて返す。

```
GET /api/v1/transactions/{id}
```

**リクエストパラメータ（パス）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| id | integer | ○ | 取引 ID |

**レスポンス（200 OK）**

`data` の基本構造は **API-010 の `data[]` と同形式**。加えて以下を含む。

| フィールド名 | 型 | 説明 |
|---|---|---|
| data.amount_jpy_minor | integer | 円換算額（集計に使う値） |
| data.category_source | string | `rule` / `place_type` / `user` / `default` |
| data.is_user_edited | boolean | ユーザが手修正したか（`true` は再計算の対象外） |
| data.note | string / null | メモ |
| data.payment_events[] | array | 紐づく決済イベント。**手動登録（`source = manual`）では空配列** |
| data.payment_events[].id | integer | 決済イベント ID |
| data.payment_events[].event_type | string | `order_confirm` / `usage_notice` / `settlement` / `receipt` / `bank_debit` |
| data.payment_events[].occurred_at | string | 決済日時 |
| data.payment_events[].amount_minor | integer | 金額 |
| data.payment_events[].raw_merchant_text | string / null | 正規化前の生の店舗名 |
| data.payment_events[].link_type | string / null | `auto` / `manual`（名寄せの経路） |
| data.payment_events[].link_score | number / null | 自動名寄せ時のスコア |
| data.payment_events[].email_message_id | integer | 抽出元メールの ID |

**レスポンス例**

```json
{
  "data": {
    "id": 1024,
    "occurred_at": "2026-09-09T12:31:00+09:00",
    "amount_minor": 580,
    "currency": "JPY",
    "amount_jpy_minor": 580,
    "merchant": { "id": 42, "name": "セブン-イレブン渋谷道玄坂店", "address": "東京都渋谷区道玄坂1-1-1" },
    "category": { "id": 3, "name": "食費" },
    "payment_method": { "id": 2, "name": "楽天カード", "kind": "credit_card" },
    "category_source": "rule",
    "source": "email",
    "status": "confirmed",
    "is_user_edited": false,
    "is_possible_duplicate": false,
    "note": null,
    "payment_events": [
      {
        "id": 5501,
        "event_type": "usage_notice",
        "occurred_at": "2026-09-09T12:31:00+09:00",
        "amount_minor": 580,
        "raw_merchant_text": "ｾﾌﾞﾝ-ｲﾚﾌﾞﾝ ｼﾌﾞﾔﾄﾞｳｹﾞﾝｻﾞｶ",
        "link_type": "auto",
        "link_score": 0.95,
        "email_message_id": 8801
      }
    ]
  }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 403 | FORBIDDEN | 他ユーザの取引 |
| 404 | TRANSACTION_NOT_FOUND | 取引が存在しない、または論理削除済み |

> 論理削除済み（`deleted_at IS NOT NULL`）の取引は `404` を返す。
> クライアントから見れば削除されているため、存在しないものとして扱う。

---

### API-013 取引の修正

**副作用を持つ点が重要。** カテゴリを変更した場合、その店舗のブランドに対する
`category_rules` を upsert し、以降の同ブランドの取引が自動で正しく分類されるようにする。
また、修正した取引は `is_user_edited = true` となり、名寄せの再計算対象から外れる。

> **Step 3 での実装範囲**（5.1 の ※1）
> **カテゴリ規則の学習は Step 8 以降に回す。** 学習には `category_rules` テーブルが必要だが、
> 同テーブルは Step 3 のコア 6 テーブルに含まれないため。
> Step 3 では取引の更新のみを行い、`learn_category` は受け付けたうえで無視し、
> `meta.learned_rule` は常に `null`、`meta.affected_future` は常に `false` を返す。
> **リクエスト・レスポンスの形は変えない**ので、Step 8 で学習を足してもフロントの改修は不要。

```
PUT /api/v1/transactions/{id}
```

**リクエストパラメータ（パス）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| id | integer | ○ | 取引 ID |

**リクエストパラメータ（ボディ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| occurred_at | string | - | 決済日時 |
| amount_minor | integer | - | 金額（最小単位） |
| merchant_id | integer | - | 店舗 ID |
| category_id | integer | - | カテゴリ ID |
| payment_method_id | integer | - | 決済手段 ID |
| note | string | - | メモ |
| learn_category | boolean | - | `true`（既定）でカテゴリ規則を学習する。`false` でこの取引のみ変更 |

**リクエスト例**

```json
{
  "category_id": 7,
  "learn_category": true
}
```

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data.id | integer | 取引 ID |
| data.category | object | 更新後のカテゴリ |
| data.category_source | string | 常に `user` |
| data.is_user_edited | boolean | 常に `true` |
| meta.learned_rule | object / null | 学習したカテゴリ規則 |
| meta.learned_rule.match_type | string | `brand` / `merchant` |
| meta.learned_rule.match_value | string | 照合値 |
| meta.affected_future | boolean | 今後の取引に適用されるか |

**レスポンス例**

```json
{
  "data": {
    "id": 1024,
    "category": { "id": 7, "name": "日用品" },
    "category_source": "user",
    "is_user_edited": true
  },
  "meta": {
    "learned_rule": { "match_type": "brand", "match_value": "セブン-イレブン" },
    "affected_future": true
  }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | 形式不正 |
| 403 | FORBIDDEN | 他ユーザの取引 |
| 404 | TRANSACTION_NOT_FOUND | 取引が存在しない |

---

### API-014 取引の削除

**論理削除**（`deleted_at = now()`）。物理削除しないのは、決済イベントが「未名寄せ」に戻り、
次の名寄せバッチで**同じ取引が再生成されてしまう**ため（DB 仕様書 3.7 の削除方針を参照）。

```
DELETE /api/v1/transactions/{id}
```

**リクエストパラメータ（パス）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| id | integer | ○ | 取引 ID |

**レスポンス（204 No Content）**

ボディなし。

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 403 | FORBIDDEN | 他ユーザの取引 |
| 404 | TRANSACTION_NOT_FOUND | 取引が存在しない |
| 409 | ALREADY_MERGED | マージの統合元であり、単独では削除できない（先に API-016 で解除する） |

> **削除後の挙動**: 以降 API-010 / API-012 からは見えなくなる（参照系は常に `deleted_at IS NULL` で絞る）。
> 紐づく `payment_events.transaction_id` は**そのまま残す**。
> これにより名寄せバッチは「処理済み」と判断でき、削除した取引が復活しない。

---

### API-015 取引のマージ（名寄せ）

疑似重複として提示された 2 件の取引を、同一の買い物として 1 件に統合する。
統合先に決済イベントを付け替え、**統合元は論理削除する**
（`deleted_at = now()`、`merged_into_id = 統合先の ID`）。
どちらの金額を採用するかは `event_type` の確からしさに従う。

**統合元を物理削除しないのは、API-016 `unmerge` で復元できるようにするため。**
物理削除すると、統合元に入っていたメモやカテゴリの手修正が失われ、
「マージを元に戻す」が成立しない。

```
POST /api/v1/transactions/merge
```

> **パス設計（メンタリングでの確認結果）**
> 当初は `POST /transactions/{id}/merge` としていたが、**両方の ID をボディで指定する形に変更**した。
>
> | 理由 | 内容 |
> |---|---|
> | 操作の対象 | マージは特定リソース 1 件への操作ではなく、**複数の取引を 1 件に束ねるコレクション全体への操作**であり、URL にどちらか一方の ID を含める必然性が薄い |
> | 可読性 | `{id}/merge` は「対象を merge する」のか「対象と何かを merge する」のか URL から読み取れず、統合元と統合先のどちらを `{id}` に置くかで実装ごとに揺れる |
> | 拡張性 | 将来 3 件以上のマージへ拡張する場合、ボディ指定の方が余地を残しやすい |
>
> 「どちらが残るか」は `data.id` と `meta.merged_transaction_id` で明示されるため、
> URL に持たなくても情報は損なわれない。

**リクエストパラメータ（ボディ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| target_transaction_id | integer | ○ | 統合先の取引 ID（**この取引が残る**） |
| source_transaction_id | integer | ○ | 統合元の取引 ID（**この取引は論理削除される**） |

**リクエスト例**

```json
{
  "target_transaction_id": 1024,
  "source_transaction_id": 1025
}
```

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data.id | integer | 統合後の取引 ID |
| data.amount_minor | integer | 採用された金額 |
| data.event_count | integer | 統合後に紐づく決済イベント数 |
| data.is_possible_duplicate | boolean | 常に `false` |
| meta.merged_transaction_id | integer | 論理削除された統合元の取引 ID（`unmerge` に渡す） |
| meta.amount_source_event_type | string | 金額の採用元となったイベント種別 |

**レスポンス例**

```json
{
  "data": {
    "id": 1024,
    "amount_minor": 3000,
    "event_count": 2,
    "is_possible_duplicate": false
  },
  "meta": {
    "merged_transaction_id": 1025,
    "amount_source_event_type": "settlement"
  }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | `target_transaction_id` / `source_transaction_id` の欠落・形式不正 |
| 400 | SAME_TRANSACTION | 統合元と統合先が同一 |
| 403 | FORBIDDEN | 他ユーザの取引 |
| 404 | TRANSACTION_NOT_FOUND | いずれかの取引が存在しない |
| 409 | ALREADY_MERGED | 統合元がすでに他の取引に統合されている（`merged_into_id` が設定済み） |
| 409 | ALREADY_DELETED | いずれかの取引がユーザによって削除済み（`deleted_at` が設定済み、かつマージ由来でない） |

> 統合元を論理削除で残しているため、**`409` の 2 つを状態から判定できる**。
> 物理削除にすると行が消えて `404` としか答えられず、
> 「すでにマージ済み」なのか「そもそも存在しない」のかを区別できなくなる。

---

### API-016 マージ解除

API-015 で統合された取引を元に戻す。
統合元の `deleted_at` と `merged_into_id` を NULL に戻し、決済イベントを統合元へ付け替える。

**マージ済みの取引 1 件に対する操作**であるため、API-015 と違いパスに `{id}` を持つ。

```
POST /api/v1/transactions/{id}/unmerge
```

**リクエストパラメータ（パス）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| id | integer | ○ | **統合先**の取引 ID |

**リクエストパラメータ（ボディ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| merged_transaction_id | integer | ○ | 復元する統合元の取引 ID（API-015 の `meta.merged_transaction_id`） |

**リクエスト例**

```json
{ "merged_transaction_id": 1025 }
```

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data.id | integer | 統合先の取引 ID |
| data.amount_minor | integer | 復元後の金額 |
| data.event_count | integer | 統合先に残った決済イベント数 |
| meta.restored_transaction_id | integer | 復元された取引 ID |
| meta.restored_event_count | integer | 統合元へ戻した決済イベント数 |

**レスポンス例**

```json
{
  "data": { "id": 1024, "amount_minor": 1500, "event_count": 1 },
  "meta": { "restored_transaction_id": 1025, "restored_event_count": 1 }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | `merged_transaction_id` の欠落・形式不正 |
| 403 | FORBIDDEN | 他ユーザの取引 |
| 404 | TRANSACTION_NOT_FOUND | いずれかの取引が存在しない |
| 409 | NOT_MERGED | 指定した取引が `{id}` に統合されていない（`merged_into_id` が不一致） |
| 409 | USER_DELETED | 統合元がマージではなくユーザ操作で削除されている（復元対象ではない） |

> `409` の 2 つを区別できるのは、統合元を**論理削除で残している**ため。
> `merged_into_id` が設定されていればマージ由来、NULL ならユーザ削除と判定できる。

---

### API-017 未解析メール一覧

判定・抽出が完了していないメールを一覧する。
ユーザが「決済メールなのに取り込まれていない」ものを見つけ、API-018 で教示するための導線。

一覧系のため、ページネーションの扱いは **API-010 と同形式**。

```
GET /api/v1/inbox/unparsed
```

**リクエストパラメータ（クエリ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| classification | string | - | `unknown`（既定） / `payment` / `not_payment` |
| parse_status | string | - | `pending` / `failed` / `skipped` |
| from_address | string | - | 送信元で絞り込む |
| limit | integer | - | 取得件数（既定 20、最大 100） |
| offset | integer | - | 取得開始位置（既定 0） |

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data[].id | integer | メール ID |
| data[].from_address | string | 送信元アドレス |
| data[].from_name | string / null | 送信者名 |
| data[].subject | string / null | 件名 |
| data[].received_at | string | 受信日時 |
| data[].classification | string | `payment` / `not_payment` / `unknown` |
| data[].parse_status | string | `pending` / `success` / `failed` / `skipped` |
| data[].parse_error | string / null | 抽出失敗の内容 |
| data[].matched_template_id | integer / null | マッチしたテンプレート |
| meta.total | integer | 条件に合致する総件数 |

**レスポンス例**

```json
{
  "data": [
    {
      "id": 8830,
      "from_address": "info@example-card.co.jp",
      "from_name": "サンプルカード",
      "subject": "ご利用のお知らせ",
      "received_at": "2026-09-15T21:04:00+09:00",
      "classification": "unknown",
      "parse_status": "pending",
      "parse_error": null,
      "matched_template_id": null
    }
  ],
  "meta": { "total": 12 }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | LIMIT_TOO_LARGE | `limit` が 100 を超えている |
| 400 | INVALID_CLASSIFICATION | `classification` が定義外の値 |

> **本文（`body_text` / `body_html`）は返さない。** 一覧に必要なのは
> 「どの送信元の、いつの、どんな件名のメールが未処理か」までであり、
> 本文を含めると転送量が大きく、メール全文をブラウザに出す必要も薄いため。

---

### API-018 決済メール判定の教示

自動判定が誤った、または判定できなかったメールについて、ユーザが正解を教える。
`payment` と教示された場合は**再解析をキューに積む**（副作用を持つアクション型）。

```
POST /api/v1/inbox/{id}/classify
```

**リクエストパラメータ（パス）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| id | integer | ○ | メール ID |

**リクエストパラメータ（ボディ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| classification | string | ○ | `payment` / `not_payment` |
| reparse | boolean | - | `true`（既定）で再解析をキューに積む。`classification = not_payment` の場合は無視される |

**リクエスト例**

```json
{ "classification": "payment", "reparse": true }
```

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data.id | integer | メール ID |
| data.classification | string | 更新後の判定 |
| data.classified_at | string | 判定日時 |
| data.parse_status | string | 再解析をキューに積んだ場合は `pending` |
| meta.reparse_queued | boolean | 再解析をキューに積んだか |

**レスポンス例**

```json
{
  "data": {
    "id": 8830,
    "classification": "payment",
    "classified_at": "2026-09-16T10:05:00+09:00",
    "parse_status": "pending"
  },
  "meta": { "reparse_queued": true }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | INVALID_CLASSIFICATION | `classification` が `payment` / `not_payment` 以外 |
| 403 | FORBIDDEN | 他ユーザのメール |
| 404 | EMAIL_MESSAGE_NOT_FOUND | メールが存在しない |

> **`PUT /inbox/{id}` にしない理由**: この操作は `classification` を書き換えるだけでなく、
> **再解析をキューに積むという副作用**を持つ。汎用の `PUT` に含めると、
> どのフィールドを送ったかで挙動が変わり分かりにくくなるため、
> 意図が明示されるアクション型に分離した（7 章の設計判断 1 と同じ考え方）。

---

### API-019 カテゴリ一覧

```
GET /api/v1/categories
```

**リクエストパラメータ（クエリ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| include_counts | boolean | - | `true` で各カテゴリの取引件数を含める（既定 `false`） |

> マスタであり 1 ユーザあたり数十件を想定するため、**ページネーションは設けない**。
> 並び順は `sort_order` 昇順、同値の場合は `id` 昇順。

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data[].id | integer | カテゴリ ID |
| data[].name | string | カテゴリ名 |
| data[].parent_id | integer / null | 親カテゴリ ID |
| data[].sort_order | integer | 表示順 |
| data[].is_system | boolean | 初期作成カテゴリか（**`true` は削除不可**） |
| data[].transaction_count | integer | 取引件数（`include_counts = true` のときのみ） |
| meta.total | integer | 件数 |

**レスポンス例**

```json
{
  "data": [
    { "id": 3, "name": "食費", "parent_id": null, "sort_order": 1, "is_system": true },
    { "id": 7, "name": "日用品", "parent_id": null, "sort_order": 2, "is_system": false }
  ],
  "meta": { "total": 2 }
}
```

**エラーレスポンス**

共通の `401` のみ。

---

### API-020 カテゴリ作成

登録系のため、バリデーションと `201` の扱いは **API-011 と同形式**。

```
POST /api/v1/categories
```

**リクエストパラメータ（ボディ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| name | string | ○ | カテゴリ名（50 文字以内） |
| parent_id | integer | - | 親カテゴリ ID |
| sort_order | integer | - | 表示順（既定 0） |

**リクエスト例**

```json
{ "name": "交際費", "parent_id": null, "sort_order": 5 }
```

**レスポンス（201 Created）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data.id | integer | 作成されたカテゴリ ID |
| data.name | string | カテゴリ名 |
| data.parent_id | integer / null | 親カテゴリ ID |
| data.sort_order | integer | 表示順 |
| data.is_system | boolean | 常に `false` |

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | `name` の欠落、51 文字以上 |
| 404 | PARENT_NOT_FOUND | 指定した親カテゴリが存在しない |
| 409 | DUPLICATE_CATEGORY_NAME | 同名のカテゴリが存在する（`UNIQUE(user_id, name)`） |

---

### API-021 カテゴリ更新 / 削除

同一パスに `PUT` と `DELETE` を持つ。

```
PUT    /api/v1/categories/{id}
DELETE /api/v1/categories/{id}
```

**リクエストパラメータ（パス）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| id | integer | ○ | カテゴリ ID |

#### PUT（更新）

**リクエストパラメータ（ボディ）** — 指定した項目のみ更新する

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| name | string | - | カテゴリ名（50 文字以内） |
| parent_id | integer / null | - | 親カテゴリ ID（`null` で親を外す） |
| sort_order | integer | - | 表示順 |

**リクエスト例**

```json
{ "name": "食費・日用品", "sort_order": 1 }
```

**レスポンス（200 OK）**

`data` の構造は **API-020 の `data` と同形式**。

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | 形式不正 |
| 400 | INVALID_PARENT | 自分自身、または子孫を親に指定した（循環する） |
| 403 | FORBIDDEN | 他ユーザのカテゴリ |
| 404 | CATEGORY_NOT_FOUND | カテゴリが存在しない |
| 404 | PARENT_NOT_FOUND | 指定した親カテゴリが存在しない |
| 409 | DUPLICATE_CATEGORY_NAME | 同名のカテゴリが存在する |

#### DELETE（削除）

**物理削除**。参照している取引の `category_id` は FK の `SET NULL` により `NULL` になる。

**レスポンス（204 No Content）**

ボディなし。

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | SYSTEM_CATEGORY_NOT_DELETABLE | `is_system = true` のカテゴリ（`未分類` など）は削除できない |
| 403 | FORBIDDEN | 他ユーザのカテゴリ |
| 404 | CATEGORY_NOT_FOUND | カテゴリが存在しない |

> **物理削除にする理由**: 論理削除にすると、削除済みの行が `UNIQUE(user_id, name)` を
> 占有し、**同名のカテゴリを作り直せなくなる**（DB 仕様書 3.7 を参照）。
> FK が `SET NULL` のため、物理削除しても取引側に不整合は起きない。
>
> **子カテゴリの扱い**: `parent_id` も `SET NULL` のため、削除されたカテゴリの子は
> トップレベルに繰り上がる。ツリーが壊れて孤児が残ることはない。
>
> **紐づく `category_rules`** は FK が `CASCADE` のため連鎖削除される。
> 存在しないカテゴリを指すルールが残ると、分類バッチが毎回失敗するため。

---

### API-022 店舗一覧 / 修正

同一リソースに対する `GET`（コレクション）と `PUT`（個別）を持つ。

```
GET /api/v1/merchants
PUT /api/v1/merchants/{id}
```

#### GET（一覧）

一覧系のため、ページネーションの扱いは **API-010 と同形式**。

**リクエストパラメータ（クエリ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| q | string | - | 店舗名・ブランド名の部分一致検索 |
| brand_name | string | - | ブランドで絞り込む |
| geocode_status | string | - | `pending` / `success` / `not_found` / `skipped` |
| limit | integer | - | 取得件数（既定 20、最大 100） |
| offset | integer | - | 取得開始位置（既定 0） |

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data[].id | integer | 店舗 ID |
| data[].name | string | 正規化済みの表示名 |
| data[].brand_name | string / null | チェーン名 |
| data[].is_online | boolean | EC などオンライン決済か |
| data[].address | string / null | 住所 |
| data[].latitude | number / null | 緯度 |
| data[].longitude | number / null | 経度 |
| data[].place_types | array | 店舗種別（カテゴリ推定に使う） |
| data[].geocode_status | string | ジオコーディングの状態 |
| data[].transaction_count | integer | この店舗の取引件数 |
| meta.total | integer | 条件に合致する総件数 |

**レスポンス例**

```json
{
  "data": [
    {
      "id": 42,
      "name": "セブン-イレブン渋谷道玄坂店",
      "brand_name": "セブン-イレブン",
      "is_online": false,
      "address": "東京都渋谷区道玄坂1-1-1",
      "latitude": 35.658034,
      "longitude": 139.701636,
      "place_types": ["convenience_store", "food"],
      "geocode_status": "success",
      "transaction_count": 23
    }
  ],
  "meta": { "total": 118 }
}
```

#### PUT（修正）

正規化が誤った店舗名・ブランド名をユーザが修正する。

**リクエストパラメータ（ボディ）** — 指定した項目のみ更新する

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| name | string | - | 表示名（255 文字以内） |
| brand_name | string / null | - | チェーン名 |
| address | string / null | - | 住所 |
| is_online | boolean | - | オンライン決済か |

**リクエスト例**

```json
{ "brand_name": "セブン-イレブン" }
```

**レスポンス（200 OK）**

`data` の構造は **GET の `data[]` と同形式**。加えて以下を含む。

| フィールド名 | 型 | 説明 |
|---|---|---|
| meta.regeocode_queued | boolean | `address` を変更したためジオコーディングを再実行するか |

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | 形式不正 |
| 403 | FORBIDDEN | 他ユーザの店舗 |
| 404 | MERCHANT_NOT_FOUND | 店舗が存在しない |
| 409 | DUPLICATE_MERCHANT_NAME | 同名の店舗が存在する（`UNIQUE(user_id, name)`） |

> **表示名を変えても `merchant_aliases` は変更しない。**
> `merchants.name` は「画面に出す名前」、`merchant_aliases.alias` は
> 「メール中の生文字列と突き合わせる照合キー」であり、別の概念である。
> 表示名の修正で照合キーを書き換えると、**次のメールから名寄せが外れる**。
>
> **`brand_name` の変更は既存の `category_rules`（`match_type = brand`）に波及する。**
> ブランドを変えると、そのブランドに紐づく分類ルールが効かなくなるため、
> 画面側で「この変更は分類ルールに影響します」と提示する想定。

---

### API-023 月次サマリ取得

DB のレコードをそのまま返さず、集計結果を返す。
月末の配信メールと同じ内容を画面でも参照できるようにする。

```
GET /api/v1/summaries/monthly
```

**リクエストパラメータ（クエリ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| month | string | - | 対象月（`YYYY-MM`、既定は当月。JST 基準） |

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data.month | string | 対象月 |
| data.total_amount_minor | integer | 当月の合計支出 |
| data.prev_total_amount_minor | integer / null | 前月の合計支出 |
| data.diff_ratio | number / null | 前月比（`1.0` で同額） |
| data.transaction_count | integer | 取引件数 |
| data.breakdown[].category | object | カテゴリ |
| data.breakdown[].amount_minor | integer | カテゴリ別の合計 |
| data.breakdown[].ratio | number | 全体に占める割合 |
| data.active_subscriptions[] | array | 継続中のサブスク |
| data.budget.amount_minor | integer / null | 当月の予算 |
| data.budget.used_ratio | number / null | 予算の消化率 |
| meta.is_complete | boolean | 対象月が終了しているか（当月は `false`） |

**レスポンス例**

```json
{
  "data": {
    "month": "2026-08",
    "total_amount_minor": 143250,
    "prev_total_amount_minor": 128900,
    "diff_ratio": 1.11,
    "transaction_count": 187,
    "breakdown": [
      { "category": { "id": 3, "name": "食費" }, "amount_minor": 62400, "ratio": 0.44 },
      { "category": { "id": 7, "name": "日用品" }, "amount_minor": 31200, "ratio": 0.22 }
    ],
    "active_subscriptions": [
      { "id": 5, "merchant_name": "Netflix", "amount_minor": 1590, "cycle": "monthly" }
    ],
    "budget": { "amount_minor": 150000, "used_ratio": 0.96 }
  },
  "meta": { "is_complete": true }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | INVALID_MONTH_FORMAT | `month` の形式が `YYYY-MM` でない |
| 404 | NO_DATA | 対象月にデータが存在しない |

---

### API-024 サブスク一覧

一覧系のため、ページネーションの扱いは **API-010 と同形式**。

```
GET /api/v1/subscriptions
```

**リクエストパラメータ（クエリ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| status | string | - | `active`（既定） / `suspected_stopped` / `cancelled` / `all` |
| limit | integer | - | 取得件数（既定 20、最大 100） |
| offset | integer | - | 取得開始位置（既定 0） |

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data[].id | integer | サブスク ID |
| data[].merchant | object | 店舗（構造は API-010 の `merchant` と同形式） |
| data[].amount_minor | integer | 課金額 |
| data[].currency | string | 通貨コード |
| data[].cycle | string | `monthly` / `yearly` / `weekly` |
| data[].occurrence_count | integer | 検出に使った課金回数 |
| data[].first_charged_at | string | 初回課金日 |
| data[].last_charged_at | string | 最終課金日 |
| data[].next_expected_at | string / null | 次回課金の予測日 |
| data[].status | string | `active` / `suspected_stopped` / `cancelled` |
| meta.total | integer | 条件に合致する総件数 |
| meta.monthly_total_amount_minor | integer | **月額換算した合計**（`yearly` は 1/12、`weekly` は 52/12 で換算） |

**レスポンス例**

```json
{
  "data": [
    {
      "id": 5,
      "merchant": { "id": 77, "name": "Netflix" },
      "amount_minor": 1590,
      "currency": "JPY",
      "cycle": "monthly",
      "occurrence_count": 6,
      "first_charged_at": "2026-04-15T00:00:00+09:00",
      "last_charged_at": "2026-09-15T00:00:00+09:00",
      "next_expected_at": "2026-10-15T00:00:00+09:00",
      "status": "active"
    }
  ],
  "meta": { "total": 4, "monthly_total_amount_minor": 4280 }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | LIMIT_TOO_LARGE | `limit` が 100 を超えている |
| 400 | INVALID_STATUS | `status` が定義外の値 |

> **`meta.monthly_total_amount_minor` を返す理由**: サブスク一覧で知りたいのは
> 「毎月いくら固定費が出ているか」であり、`cycle` が混在した金額の単純合計には意味がない。
> 換算をサーバ側で行うことで、クライアントごとに換算式がぶれるのを防ぐ。

---

### API-025 サブスク状態更新

更新系のため、扱いは **API-013 と同形式**（ただし副作用としての学習は行わない）。

```
PUT /api/v1/subscriptions/{id}
```

**リクエストパラメータ（パス）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| id | integer | ○ | サブスク ID |

**リクエストパラメータ（ボディ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| status | string | ○ | `active` / `cancelled` のみ指定可 |

**リクエスト例**

```json
{ "status": "cancelled" }
```

**レスポンス（200 OK）**

`data` の構造は **API-024 の `data[]` と同形式**。

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | INVALID_STATUS | `status` が `active` / `cancelled` 以外 |
| 403 | FORBIDDEN | 他ユーザのサブスク |
| 404 | SUBSCRIPTION_NOT_FOUND | サブスクが存在しない |

> **`suspected_stopped` をユーザが指定できない理由**: この値は
> 「`next_expected_at` を過ぎても課金が観測されない」という**バッチの検出結果**であり、
> ユーザの意思表示ではない。ユーザが表明できるのは「まだ使っている（`active`）」か
> 「解約した（`cancelled`）」かの 2 つなので、指定可能な値をこの 2 つに限定する。

---

### API-026 予算の取得 / 設定

コレクション単位で取得・一括更新する。

```
GET /api/v1/budgets
PUT /api/v1/budgets
```

#### GET（取得）

**リクエストパラメータ（クエリ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| month | string | - | 消化率を算出する対象月（`YYYY-MM`、既定は当月） |

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data[].id | integer | 予算 ID |
| data[].category | object / null | カテゴリ。**`null` は全体予算** |
| data[].period | string | 予算の期間（`monthly`） |
| data[].amount_minor | integer | 予算額 |
| data[].alert_thresholds | array | 通知する到達率（%） |
| data[].is_active | boolean | 有効フラグ |
| data[].used_amount_minor | integer | 対象月の消化額 |
| data[].used_ratio | number | 消化率（`1.0` で予算ちょうど） |
| meta.month | string | 対象月 |

**レスポンス例**

```json
{
  "data": [
    {
      "id": 1,
      "category": null,
      "period": "monthly",
      "amount_minor": 150000,
      "alert_thresholds": [80, 100],
      "is_active": true,
      "used_amount_minor": 143250,
      "used_ratio": 0.96
    }
  ],
  "meta": { "month": "2026-09" }
}
```

#### PUT（一括設定）

**リクエストパラメータ（ボディ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| budgets[] | array | ○ | 設定する予算の配列 |
| budgets[].category_id | integer / null | ○ | カテゴリ ID。`null` で全体予算 |
| budgets[].amount_minor | integer | ○ | 予算額（0 以上） |
| budgets[].alert_thresholds | array | - | 通知する到達率（既定 `[80, 100]`） |
| budgets[].is_active | boolean | - | 有効フラグ（既定 `true`） |

**リクエスト例**

```json
{
  "budgets": [
    { "category_id": null, "amount_minor": 150000, "alert_thresholds": [80, 100] },
    { "category_id": 3, "amount_minor": 60000 }
  ]
}
```

**レスポンス（200 OK）**

`data` の構造は **GET の `data[]` と同形式**（`used_amount_minor` / `used_ratio` は当月の値）。

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | `budgets` の欠落、形式不正 |
| 400 | INVALID_AMOUNT | 予算額が負の値 |
| 400 | INVALID_THRESHOLD | 到達率が 1〜200 の範囲外、または重複 |
| 400 | DUPLICATE_CATEGORY | 同じ `category_id` が配列内に複数ある |
| 404 | CATEGORY_NOT_FOUND | 指定したカテゴリが存在しない |

> **個別の `POST` / `DELETE` を設けず、コレクションへの `PUT` にした理由**:
> 予算設定は「画面で数値を並べて編集し、まとめて保存する」操作であり、
> 1 件ずつのリクエストに分けるとフロント側で作成・更新・削除の差分計算が必要になる。
> `UNIQUE(user_id, category_id, period)` があるため、
> `category_id` をキーに upsert する形なら 1 リクエストで完結する。
> 配列に含まれない既存の予算は**変更しない**（削除したい場合は `is_active = false` を送る）。

---

### API-027 通知設定の取得 / 更新

ユーザ 1 人につき 1 行のため、パスに ID を持たない。

```
GET /api/v1/notification-settings
PUT /api/v1/notification-settings
```

#### GET（取得）

**リクエストパラメータ**: なし

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data.channel | string | `email` / `slack` |
| data.email_to | string / null | 配信先メールアドレス |
| data.slack_webhook_configured | boolean | **Slack Webhook が設定済みか**（URL 自体は返さない） |
| data.instant_enabled | boolean | 即時通知の ON / OFF |
| data.instant_min_amount_minor | integer | この金額未満は即時通知しない |
| data.quiet_hours_start | string / null | 静音時間帯の開始（`HH:MM`） |
| data.quiet_hours_end | string / null | 静音時間帯の終了（`HH:MM`） |
| data.budget_alert_enabled | boolean | 予算超過通知の ON / OFF |
| data.monthly_summary_enabled | boolean | 月次サマリの ON / OFF |
| data.monthly_summary_send_at | string | 月次サマリの配信時刻（`HH:MM`） |

**レスポンス例**

```json
{
  "data": {
    "channel": "email",
    "email_to": "user@example.com",
    "slack_webhook_configured": false,
    "instant_enabled": true,
    "instant_min_amount_minor": 1000,
    "quiet_hours_start": "23:00",
    "quiet_hours_end": "07:00",
    "budget_alert_enabled": true,
    "monthly_summary_enabled": true,
    "monthly_summary_send_at": "08:00"
  }
}
```

#### PUT（更新）

**リクエストパラメータ（ボディ）** — 指定した項目のみ更新する

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| channel | string | - | `email` / `slack` |
| email_to | string / null | - | 配信先メールアドレス |
| slack_webhook | string / null | - | **書き込み専用。** Slack Webhook URL（暗号化して保存。`null` で削除） |
| instant_enabled | boolean | - | 即時通知の ON / OFF |
| instant_min_amount_minor | integer | - | 即時通知の下限金額（0 以上） |
| quiet_hours_start | string / null | - | 静音時間帯の開始（`HH:MM`） |
| quiet_hours_end | string / null | - | 静音時間帯の終了（`HH:MM`） |
| budget_alert_enabled | boolean | - | 予算超過通知の ON / OFF |
| monthly_summary_enabled | boolean | - | 月次サマリの ON / OFF |
| monthly_summary_send_at | string | - | 月次サマリの配信時刻（`HH:MM`） |

**リクエスト例**

```json
{ "instant_min_amount_minor": 3000, "quiet_hours_start": "22:30" }
```

**レスポンス（200 OK）**

`data` の構造は **GET の `data` と同形式**。

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | 形式不正 |
| 400 | INVALID_TIME_FORMAT | `HH:MM` 形式でない |
| 400 | EMAIL_TO_REQUIRED | `channel = email` なのに `email_to` が未設定 |
| 400 | SLACK_WEBHOOK_REQUIRED | `channel = slack` なのに Webhook が未設定 |

> **`slack_webhook` を書き込み専用にした理由**: Webhook URL は
> **それ自体が投稿権限を持つ秘密情報**であり、一度設定したら読み出す必要がない。
> GET では `slack_webhook_configured` の真偽だけを返し、URL は返さない。
> DB 上も `slack_webhook_encrypted` として暗号化して保持する。
>
> **静音時間帯が日をまたぐ場合**（`start = 23:00`、`end = 07:00`）は、
> `start > end` を「日跨ぎ」として扱う。両方 `null` の場合は静音なし。
> 静音時間帯に発生した即時通知は**破棄せず、明けに送る**
> （`notifications.dedupe_key` があるため、再実行しても二重送信にならない）。

---

### API-028 通知履歴一覧

一覧系のため、ページネーションの扱いは **API-010 と同形式**。

```
GET /api/v1/notifications
```

**リクエストパラメータ（クエリ）**

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| kind | string | - | `instant` / `budget_alert` / `monthly_summary` |
| status | string | - | `pending` / `sent` / `failed` |
| limit | integer | - | 取得件数（既定 20、最大 100） |
| offset | integer | - | 取得開始位置（既定 0） |

**レスポンス（200 OK）**

| フィールド名 | 型 | 説明 |
|---|---|---|
| data[].id | integer | 通知 ID |
| data[].kind | string | `instant` / `budget_alert` / `monthly_summary` |
| data[].channel | string | `email` / `slack` |
| data[].subject | string / null | 件名 |
| data[].status | string | `pending` / `sent` / `failed` |
| data[].sent_at | string / null | 送信日時 |
| data[].error_message | string / null | 送信失敗の内容 |
| data[].created_at | string | 作成日時 |
| meta.total | integer | 条件に合致する総件数 |

**レスポンス例**

```json
{
  "data": [
    {
      "id": 3310,
      "kind": "instant",
      "channel": "email",
      "subject": "セブン-イレブン渋谷道玄坂店 580円",
      "status": "sent",
      "sent_at": "2026-09-09T12:31:40+09:00",
      "error_message": null,
      "created_at": "2026-09-09T12:31:35+09:00"
    }
  ],
  "meta": { "total": 264 }
}
```

**エラーレスポンス**

| ステータスコード | エラーコード | エラー内容 |
|---|---|---|
| 400 | LIMIT_TOO_LARGE | `limit` が 100 を超えている |
| 400 | INVALID_KIND | `kind` が定義外の値 |

> **本文（`body`）と `dedupe_key` は返さない。** 履歴一覧で必要なのは
> 「いつ・どの種別が・送れたか」までであり、本文は送信済みメールを見れば足りる。
> `dedupe_key` は冪等性を担保するための内部的な値で、クライアントには意味を持たない。

---

## 7. 主な設計判断

1. **アクション型エンドポイントを用意した**
   `merge` / `unmerge` / `classify` は、リソースの状態を書き換えるだけでなく
   **副作用（イベントの付け替え、ルールの学習）を伴う**。
   汎用の `PUT` に含めると、どのフィールドを変えたかで挙動が変わり分かりにくくなるため、
   意図が明示されるアクション型のエンドポイントに分離した。

2. **`PUT /transactions/{id}` の副作用を `meta` で返す**
   カテゴリ修正時に学習したルールを `meta.learned_rule` として返すことで、
   「この修正が今後にも効く」ことを画面上でユーザに伝えられる。
   学習させたくない場合のために `learn_category` フラグも用意した。

3. **金額を最小単位の整数で返す**
   クライアント側での丸め誤差を防ぐため。表示時の桁区切りはフロント側の責務とする。

4. **一覧の `meta` に合計金額を含めた**
   絞り込み条件を変えるたびに合計を別途取得する必要がなくなり、リクエスト数を削減できる。

5. **認証を 2 層に分離した**
   API を呼ぶための認証と、メールを読むための認証は目的も有効期限も異なる。
   後者のトークンは API リクエストには一切使わず、暗号化して DB に保持する。

6. **バッチ処理の API を最小限にした**
   取込・解析・名寄せはバッチが行うため、API 側は `POST /mail-accounts/{id}/sync`（202 で受付）と
   `GET /sync-jobs`（履歴参照）のみを提供する。

7. **`DELETE` の意味をエンドポイントごとに定義した**
   `DELETE` は「クライアントから見て消える」ことを表すだけで、
   サーバ側が物理削除するとは限らない。本サービスでは
   `transactions` は論理削除、`mail_accounts` は `status = 'disabled'`、
   `categories` は物理削除と使い分けており、その対応表を 5 章に明記した。
   物理削除にすると、**削除した取引が次の名寄せバッチで再生成される**（詳細は DB 仕様書 3.7）。

8. **`merge` の統合元を残すことで `unmerge` を成立させた**
   統合元を物理削除すると、そこに入っていたユーザの手修正が失われ、
   「マージを元に戻す」が実現できない。
   `merged_into_id` に統合先を記録して論理削除することで、
   両方の列を NULL に戻すだけで復元できるようにした。
   副次的に、`409 ALREADY_MERGED` を状態から判定できるようになっている。

9. **`merge` と `unmerge` でパスの形を変えた**
   `merge` は複数の取引を束ねるコレクションへの操作のため `POST /transactions/merge`（両方をボディで指定）、
   `unmerge` は**マージ済みの取引 1 件に対する操作**のため `POST /transactions/{id}/unmerge` とした。
   形が違うのは対象が違うためであり、揺れではない。

---

## 8. 判断に迷った点とメンタリングでの確認結果

Step 2 提出時に相談事項として挙げた 5 点のうち、**4 点はフィードバックで結論が出た**。
以下、各項目に確認結果と仕様書への反映箇所を記載する。

| # | 相談事項 | 状態 |
|---|---|---|
| 1 | `merge` のパス設計 | **確認済み**（変更あり） |
| 2 | 月次サマリの集計方式 | **確認済み**（現状維持） |
| 3 | `GET /inbox/unparsed` の位置づけ | **未確認** |
| 4 | エラーコードの粒度 | **確認済み**（現状維持） |
| 5 | ページネーション方式 | **確認済み**（現状維持） |

### 1. `merge` のパス設計 — 確認済み（変更あり）

> **相談内容**: `POST /transactions/{id}/merge` と
> `POST /transactions/merge`（両方をボディで指定）のどちらが適切か

**結論: `POST /transactions/merge` を採用する。**

マージは特定リソース 1 件への操作ではなく、複数の取引を 1 件に束ねる
**コレクション全体への操作**であるため、URL にどちらか一方の ID を含める必然性が薄い。
加えて `{id}/merge` は方向が URL から読み取れず、実装ごとに揺れる懸念がある。
将来 3 件以上のマージへ拡張する余地も残しやすい。

**反映箇所**: 5 章のエンドポイント一覧、6 章の API-015、7 章の設計判断 9。

### 2. 月次サマリの集計方式 — 確認済み（現状維持）

> **相談内容**: API で毎回集計するか、`monthly_summaries` に保存済みの結果を返すか

**結論: 当面は毎回集計する現状の設計を維持する。**

`monthly_summaries` は**配信履歴として持つ**に留め、参照系のクエリは
`idx_transactions_user_category_occurred` を使って `GROUP BY` する。
整合性（取引を修正したらサマリも即座に追随する）と単純さ（二重管理しない）の
バランスが取りやすいため。

**移行の判断基準**: パフォーマンスが問題になった時点でマテリアライズドビュー化を検討する。
先回りして保存済みの結果を返す設計にすると、
「取引を修正したのにサマリが古いまま」という不整合を自前で解消する必要が生じる。

**反映箇所**: 6 章の API-023（毎回集計する前提のレスポンス定義）。

### 3. `GET /inbox/unparsed` の位置づけ — 未確認

> **相談内容**: メールはリソースとして公開すべきか、
> 「要対応キュー」という別の概念にすべきか

**この点はフィードバックで言及がなかったため、次回のメンタリングで改めて相談する。**

現時点では**「`email_messages` のフィルタ済み一覧」として設計している**（6 章の API-017）。
`classification` / `parse_status` で絞り込む形にし、本文（`body_text` / `body_html`）は
レスポンスに含めない。メールをリソースとして公開しつつ、
「未処理のものだけを見る」導線をクエリパラメータで表現する折衷案にあたる。

**論点**: 「要対応キュー」を別概念にすると、
`GET /inbox/queue` のようなエンドポイントで**処理すべき順序**を
サーバ側が決められる一方、メール単体を参照する手段が別途必要になる。
現状は Step 8 以降（拡張）のエンドポイントであり、実装前に確定させれば間に合う。

### 4. エラーコードの粒度 — 確認済み（現状維持）

> **相談内容**: `VALIDATION_ERROR` にまとめるか、項目ごとに分けるか

**結論: 現状の粒度（フロントで分岐したい単位で分ける）を維持する。**

`VALIDATION_ERROR` にまとめる場合、どの項目が不正だったかを伝えるために
**レスポンスにフィールド単位の詳細を持たせる構造が必要**になる。
フロント側で分岐したい単位でコードを分ける現在のスタンスの方が、UI からは扱いやすい。

そのため本仕様書では、形式不正のような「どの項目でも起きうる」ものは `VALIDATION_ERROR` に
まとめつつ、`INVALID_AMOUNT` / `INVALID_DATE_RANGE` / `LIMIT_TOO_LARGE` /
`DUPLICATE_CATEGORY_NAME` のように**画面上で別のメッセージを出したいもの**は個別のコードにしている。

**反映箇所**: 6 章の各エンドポイントのエラーレスポンス表。

### 5. ページネーション方式 — 確認済み（現状維持）

> **相談内容**: offset 方式で始めたが、取引件数が増えた場合にカーソル方式へ移行すべきか

**結論: 当面は offset 方式を維持する。**

**移行の判断基準**: 取引件数が**数万件を超えた段階**でカーソル方式
（`occurred_at` + `id` の組み合わせ）への切り替えを検討する。
取引一覧の主軸インデックスが `(user_id, occurred_at DESC)` で揃っているため、
切り替え時の実装は最小で済む。

`meta.total`（総件数）を返す設計も offset 方式が前提になっている。
カーソル方式へ移行する際は、総件数を返し続けるか
（毎回 `COUNT` が走るため件数が増えるほど重い）も併せて判断する。

**反映箇所**: 2 章の共通仕様、6 章の一覧系エンドポイント。
