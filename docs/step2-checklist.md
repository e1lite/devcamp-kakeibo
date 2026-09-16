# Step2: バックエンド2 — やることリスト & 成果物の目次

出典: [Step2: バックエンド2](https://devopscamp.reheartcloud.com/final-challenge/app/step2) / [設計サンプル](https://devopscamp.reheartcloud.com/final-challenge/app/design-samples)
テーマ: [決済メール解析による自動家計簿サービス](./theme-kakeibo.md)（Go）
設計の根拠: [step0-pipeline.md](./step0-pipeline.md)
中文参考版: [zh/step2-checklist.md](./zh/step2-checklist.md)

> Step2 のゴール: **コードは書かない**。Step3（バックエンド実装）の土台として
> **API 仕様書 / DB 仕様書 / ER 図** の 3 点を設計ドラフトとして固める。
> ※ 課題提出・メンタリングはプレミアムプラン限定

---

## 全体像（3ブロック）

| # | ブロック | 内容 | 成果物 |
|---|---------|------|--------|
| A | 講座の履修 | Go 講座 後半 7 章 | （提出物なし・スキップ可） |
| B | 課題の実施 | 設計ドキュメント作成 | API 仕様書 / DB 仕様書 / ER 図 / 設計判断 / まとめ |
| C | メンタリング報告 | 口頭報告 | 講座の振り返り / 提出課題の説明 |

---

## A. 講座の履修（Go 講座 後半）

同等の知識があればスキップ可。

- [ ] [Goでデータベースを操作しよう](https://devopscamp.reheartcloud.com/go/database-operation) — Go から DB 接続、CRUD 実装
- [ ] [GORM入門](https://devopscamp.reheartcloud.com/go/gorm-intro) — テーブルと構造体のマッピング
- [ ] [net／http入門](https://devopscamp.reheartcloud.com/go/net-http-intro) — 基本エンドポイント
- [ ] [Gin入門](https://devopscamp.reheartcloud.com/go/gin-intro) — ルーティング・リクエスト処理
- [ ] [GoでREST APIを作ろう](https://devopscamp.reheartcloud.com/go/rest-api) — Gin + GORM で CRUD 型 REST API
- [ ] [Goで静的解析をしよう](https://devopscamp.reheartcloud.com/go/static-analysis) — golangci-lint 導入
- [ ] [Goで自動テストをしよう](https://devopscamp.reheartcloud.com/go/automated-testing) — testing パッケージ

---

## B. 課題の実施（提出物）

### 提出フォームの 5 項目

- [x] **① API 仕様書** → [step2-api-spec.md](./step2-api-spec.md)（全体仕様 + 全 28 エンドポイントの一覧と詳細）
- [x] **② データベース仕様書** → [step2-db-spec.md](./step2-db-spec.md)（全 17 テーブルのカラム定義・制約・インデックス）
- [x] **③ ER 図** → [step2-er.md](./step2-er.md)（Mermaid。draw.io での清書はスキーマ確定後）
- [x] **④ 設計判断** → 各仕様書の「主な設計判断」節に記載
- [x] **⑤ まとめ** → 各仕様書の「判断に迷った点」節に記載

> **残タスク（2026-09-16 時点）**: なし。
> フィードバックで指摘された残り 22 エンドポイントの詳細を展開し、
> API-015 のパスを `POST /transactions/merge` に変更済み。
> 挙動が同形のエンドポイントは「API-0XX と同形式」の参照表記で簡略化している。
>
> 相談事項も両仕様書に確認結果を反映済み（API 仕様書 8 章 / DB 仕様書 6 章）。
> **次回のメンタリングに持ち越す未確認の 3 点**:
> `GET /inbox/unparsed` の位置づけ / `transactions.status` の要否 / `merchants` を支店単位にした判断。

---

### ① API 仕様書 — 目次

形式自由（Markdown / スプレッドシート / Excel）。

```
1. APIの概要
   - どんな機能を持つ API か
   - 対象となるデータは何か
2. 共通仕様
   - ベース URL
   - 日時フォーマット
   - ページネーション仕様
   - レスポンス形式（成功時 / エラー時の JSON エンベロープ）
3. 認証方式
4. ステータスコード一覧
5. エンドポイント一覧（No. / API ID / API名 / メソッド / パス / 認証）
6. 各 API の詳細仕様  ← 全エンドポイント必須
   6.x API-00X <API名>
       - リクエストパラメータ（パラメータ名 / 型 / 必須 / 説明）
       - リクエストボディの JSON 例
       - レスポンス（フィールド名 / 型 / 説明）
       - レスポンスの JSON 例
       - エラーレスポンス（ステータスコード / エラーコード / エラー内容）
```

#### 本テーマ向け エンドポイント一覧（ドラフト）

| No. | API ID | API名 | メソッド | エンドポイント | 認証 |
|---|---|---|---|---|---|
| 1 | API-001 | ヘルスチェック | GET | `/health` | 不要 |
| 2 | API-002 | Google OAuth 開始 | GET | `/api/v1/auth/google` | 不要 |
| 3 | API-003 | Google OAuth コールバック | GET | `/api/v1/auth/google/callback` | 不要 |
| 4 | API-004 | 現在のユーザ取得 | GET | `/api/v1/auth/me` | 必要 |
| 5 | API-005 | ログアウト | POST | `/api/v1/auth/logout` | 必要 |
| 6 | API-006 | 連携メールアカウント一覧 | GET | `/api/v1/mail-accounts` | 必要 |
| 7 | API-007 | 連携解除 | DELETE | `/api/v1/mail-accounts/{id}` | 必要 |
| 8 | API-008 | 手動同期の実行 | POST | `/api/v1/mail-accounts/{id}/sync` | 必要 |
| 9 | API-009 | 同期履歴取得 | GET | `/api/v1/sync-jobs` | 必要 |
| 10 | API-010 | 取引一覧取得 | GET | `/api/v1/transactions` | 必要 |
| 11 | API-011 | 取引の手動登録（現金など） | POST | `/api/v1/transactions` | 必要 |
| 12 | API-012 | 取引詳細取得 | GET | `/api/v1/transactions/{id}` | 必要 |
| 13 | API-013 | 取引の修正 | PUT | `/api/v1/transactions/{id}` | 必要 |
| 14 | API-014 | 取引の削除 | DELETE | `/api/v1/transactions/{id}` | 必要 |
| 15 | API-015 | 取引の手動マージ（名寄せ） | POST | `/api/v1/transactions/{id}/merge` | 必要 |
| 16 | API-016 | マージ解除 | POST | `/api/v1/transactions/{id}/unmerge` | 必要 |
| 17 | API-017 | 未解析メール一覧 | GET | `/api/v1/inbox/unparsed` | 必要 |
| 18 | API-018 | 決済メール判定の教示 | POST | `/api/v1/inbox/{id}/classify` | 必要 |
| 19 | API-019 | カテゴリ一覧 | GET | `/api/v1/categories` | 必要 |
| 20 | API-020 | カテゴリ作成 | POST | `/api/v1/categories` | 必要 |
| 21 | API-021 | カテゴリ更新／削除 | PUT / DELETE | `/api/v1/categories/{id}` | 必要 |
| 22 | API-022 | 店舗（正規化名）一覧・修正 | GET / PUT | `/api/v1/merchants[/{id}]` | 必要 |
| 23 | API-023 | 月次サマリ取得 | GET | `/api/v1/summaries/monthly?month=YYYY-MM` | 必要 |
| 24 | API-024 | サブスク一覧 | GET | `/api/v1/subscriptions` | 必要 |
| 25 | API-025 | サブスク状態更新（解約済み等） | PUT | `/api/v1/subscriptions/{id}` | 必要 |
| 26 | API-026 | 予算の取得・設定 | GET / PUT | `/api/v1/budgets` | 必要 |
| 27 | API-027 | 通知設定の取得・更新 | GET / PUT | `/api/v1/notification-settings` | 必要 |
| 28 | API-028 | 通知履歴一覧 | GET | `/api/v1/notifications` | 必要 |

---

### ② データベース仕様書 — 目次

```
1. データベース概要
2. テーブル一覧（No. / テーブル名 / 概要）
3. 各テーブルの詳細仕様  ← 全テーブル必須
   3.x <テーブル名>
       - テーブル概要
       - カラム定義（カラム名 / 型 / 必須 / 制約(PK/FK/UNIQUE) / 説明）
       - 制約（複合 UNIQUE など）
       - インデックス（インデックス名 / 対象カラム / 用途）
```

#### 本テーマ向け テーブル一覧（ドラフト）

**コア（Step3 で必ず実装）**

| No. | テーブル名 | 概要 |
|---|---|---|
| 1 | `users` | アプリ利用者・認証情報 |
| 2 | `mail_accounts` | 連携した Gmail アカウントとトークン |
| 3 | `sync_jobs` | メール取得ジョブの実行履歴 |
| 4 | `email_messages` | 取得したメールの生データ（Gmail message ID で冪等） |
| 5 | `parser_templates` | 送信元＋件名パターン → 解析ルール |
| 6 | `payment_events` | メール 1 通から抽出した決済イベント（名寄せ前） |
| 7 | `transactions` | 名寄せ後の実取引（1 回の買い物 = 1 行） |
| 8 | `transaction_links` | `payment_events` ↔ `transactions` の対応 |
| 9 | `merchants` | 正規化済み店舗マスタ（支店単位 + `brand_name` + 位置情報） |
| 10 | `merchant_aliases` | 表記ゆれ → `merchants` のマッピング |
| 11 | `payment_methods` | 決済手段（カード / QR / 銀行 / 現金） |
| 12 | `categories` | カテゴリマスタ |
| 13 | `category_rules` | ブランド・店舗種別・キーワード → カテゴリの分類ルール |
| 14 | `notification_settings` | 通知チャネル・最低金額・静音時間帯 |
| 15 | `notifications` | 通知の送信履歴（即時 / 予算超過 / 月次サマリ共用） |

**拡張（余力があれば）**

| No. | テーブル名 | 概要 |
|---|---|---|
| 16 | `subscriptions` | 検出したサブスク |
| 17 | `budgets` | 予算設定（予算超過通知用） |
| 18 | `monthly_summaries` | 月次サマリの生成・配信記録 |

---

### ③ ER 図

- [ ] draw.io（[app.diagrams.net](https://app.diagrams.net/)）で作成
- [ ] 1対1 / 1対多 / 多対多 のリレーションを記号で表現
- [ ] 画像としてエクスポートして提出
- 特に明示したいリレーション:
  - `email_messages` 1 — N `payment_events`
  - `payment_events` N — N `transactions`（`transaction_links` 経由）← **本テーマの肝**
  - `merchants` 1 — N `merchant_aliases`
  - `merchants` 1 — N `transactions`
  - `categories` 1 — N `transactions`
  - `payment_events` 1 — N `notifications`（即時通知）
  - `users` 1 — N すべてのユーザ所有データ

---

### ④ 設計判断（提出項目・メンタリングでも説明する）

書くべき論点:

- [ ] **`email_messages` / `payment_events` / `transactions` の 3 層分割** — 生データを保持し、名寄せロジックを変更しても全件再計算できるようにするため
- [ ] **`email_messages` 1 — N `payment_events`** — 銀行のまとめ通知など、1 通から複数イベントが出るため
- [ ] **名寄せを中間テーブル（`transaction_links`）で表現した理由** — EC 側とカード側の N:N を扱い、マージ解除を可能にするため
- [ ] **`payment_events.event_type` を中心列にした理由** — 金額の確からしさ判定（P5）と即時通知の対象絞り込み（P4.5）の両方で使う
- [ ] **`merchants` を支店単位にし、`brand_name` を列で持った理由** — 地図は支店単位、集計とカテゴリ規則はブランド単位。`brands` テーブルを分けなかったのはスコープ判断
- [ ] **金額を `bigint`（最小単位）で保持** — 浮動小数点の誤差を避ける
- [ ] **Gmail message ID に UNIQUE 制約** — 再取得時の冪等性を担保
- [ ] **通知の二重送信防止** — `payment_events.notified_at` と `notifications` による冪等性
- [ ] **エンドポイント粒度** — `merge` / `unmerge` / `classify` を汎用 PUT ではなく専用エンドポイントにした理由
- [ ] **インデックス選定** — `(user_id, occurred_at)` 複合、`(user_id, category_id, occurred_at)`、`merchant_aliases.alias` など
- [ ] **マルチテナント設計** — 運用はシングルユーザでも全業務テーブルに `user_id` を持たせた理由
- [ ] **サブスクをテーブルで持つか、都度検出か**
- [ ] **認証が 2 種類ある点** — アプリ自身の認証（Bearer/JWT）と Gmail 連携の Google OAuth を分離

### ⑤ まとめ（提出項目）

- [ ] 設計中に苦労したこと
- [ ] 判断に迷ったこと
- [ ] 学んだこと

---

## C. メンタリング報告

### C-1. 講座の振り返り

下記 5 要素 × 3 観点（**概要 / 何ができるのか / 使うときに気をつけるべきこと**）で報告。

- [ ] データベース操作
- [ ] REST API のフレームワーク（net/http・Gin）
- [ ] ORM（GORM）
- [ ] 静的解析（golangci-lint）
- [ ] 自動テスト（testing）

### C-2. 提出課題の説明

- [ ] API とデータベースの設計判断（エンドポイント設計・テーブル設計・リレーション・インデックス）
- [ ] 設計中に迷った点・判断に悩んだ箇所
