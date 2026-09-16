# ER 図

サービス名: 決済メール解析による自動家計簿サービス
対応する仕様書: [step2-db-spec.md](./step2-db-spec.md)

> ドラフト版。Mermaid で作成。draw.io での清書はスキーマ確定後に行う。

---

## 1. 全体 ER 図

```mermaid
erDiagram
    users ||--o{ mail_accounts : "連携する"
    users ||--o{ email_messages : "所有する"
    users ||--o{ payment_events : "所有する"
    users ||--o{ transactions : "所有する"
    users ||--o{ merchants : "所有する"
    users ||--o{ payment_methods : "所有する"
    users ||--o{ categories : "所有する"
    users ||--o{ category_rules : "所有する"
    users ||--|| notification_settings : "設定を持つ"
    users ||--o{ notifications : "受け取る"
    users ||--o{ subscriptions : "所有する"
    users ||--o{ budgets : "設定する"
    users ||--o{ monthly_summaries : "受け取る"

    mail_accounts ||--o{ sync_jobs : "実行される"
    mail_accounts ||--o{ email_messages : "取得する"

    parser_templates ||--o{ email_messages : "判定する"
    parser_templates ||--o{ payment_events : "抽出する"

    email_messages ||--o{ payment_events : "1通からN件"
    transactions ||--o{ payment_events : "名寄せで束ねる"

    merchants ||--o{ merchant_aliases : "表記ゆれを持つ"
    merchants ||--o{ payment_events : "紐づく"
    merchants ||--o{ transactions : "紐づく"
    merchants ||--o{ subscriptions : "課金元"

    payment_methods ||--o{ payment_events : "決済手段"
    payment_methods ||--o{ transactions : "決済手段"
    payment_methods ||--o{ parser_templates : "既定の決済手段"

    categories ||--o{ transactions : "分類する"
    categories ||--o{ category_rules : "適用先"
    categories ||--o{ budgets : "予算対象"
    categories ||--o{ categories : "親子"

    transactions ||--o{ transactions : "マージ統合先"

    payment_events ||--o{ notifications : "即時通知の対象"
    notifications ||--o| monthly_summaries : "配信結果"

    users {
        bigint id PK
        varchar google_sub UK
        varchar email UK
        varchar display_name
        varchar timezone
    }

    mail_accounts {
        bigint id PK
        bigint user_id FK
        varchar email_address
        varchar provider
        text credential_encrypted
        varchar sync_cursor
        varchar status
    }

    sync_jobs {
        bigint id PK
        bigint mail_account_id FK
        varchar trigger
        varchar status
        integer fetched_count
        integer failed_count
    }

    email_messages {
        bigint id PK
        bigint user_id FK
        bigint mail_account_id FK
        bigint matched_template_id FK
        varchar provider_message_id UK
        varchar from_address
        text subject
        timestamptz received_at
        varchar classification
        varchar parse_status
    }

    parser_templates {
        bigint id PK
        bigint user_id FK
        varchar name
        varchar from_pattern
        varchar subject_pattern
        varchar event_type
        jsonb extraction_rules
        boolean is_multi_event
        integer priority
    }

    payment_events {
        bigint id PK
        bigint user_id FK
        bigint email_message_id FK
        bigint transaction_id FK
        bigint merchant_id FK
        bigint payment_method_id FK
        varchar event_type
        timestamptz occurred_at
        bigint amount_jpy_minor
        varchar raw_merchant_text
        timestamptz notified_at
    }

    transactions {
        bigint id PK
        bigint user_id FK
        bigint merchant_id FK
        bigint category_id FK
        bigint payment_method_id FK
        bigint merged_into_id FK
        timestamptz occurred_at
        bigint amount_jpy_minor
        varchar category_source
        varchar source
        varchar status
        boolean is_user_edited
        boolean is_possible_duplicate
        timestamptz deleted_at
    }

    merchants {
        bigint id PK
        bigint user_id FK
        varchar name UK
        varchar brand_name
        boolean is_online
        varchar address
        numeric latitude
        numeric longitude
        varchar place_types
        varchar geocode_status
    }

    merchant_aliases {
        bigint id PK
        bigint user_id FK
        bigint merchant_id FK
        varchar alias UK
        varchar source
    }

    payment_methods {
        bigint id PK
        bigint user_id FK
        varchar name
        varchar kind
        varchar card_last4
    }

    categories {
        bigint id PK
        bigint user_id FK
        bigint parent_id FK
        varchar name UK
        integer sort_order
    }

    category_rules {
        bigint id PK
        bigint user_id FK
        bigint category_id FK
        varchar match_type
        varchar match_value
        integer priority
    }

    notification_settings {
        bigint id PK
        bigint user_id FK "UNIQUE"
        varchar channel
        boolean instant_enabled
        bigint instant_min_amount_minor
        time quiet_hours_start
        time quiet_hours_end
    }

    notifications {
        bigint id PK
        bigint user_id FK
        bigint payment_event_id FK
        varchar kind
        varchar channel
        varchar dedupe_key UK
        varchar status
        timestamptz sent_at
    }

    subscriptions {
        bigint id PK
        bigint user_id FK
        bigint merchant_id FK
        bigint amount_minor
        varchar cycle
        timestamptz next_expected_at
        varchar status
    }

    budgets {
        bigint id PK
        bigint user_id FK
        bigint category_id FK
        bigint amount_minor
        varchar alert_thresholds
    }

    monthly_summaries {
        bigint id PK
        bigint user_id FK
        bigint notification_id FK
        char year_month UK
        bigint total_amount_minor
        jsonb breakdown
    }
```

---

## 2. 中核部分の ER 図（説明用）

`users` への線を省き、**データが流れる 3 層構造**だけを取り出したもの。
メンタリングではこちらを使って説明する。

```mermaid
erDiagram
    mail_accounts ||--o{ email_messages : "P1 収集"
    parser_templates ||--o{ email_messages : "P2 判定"
    email_messages ||--o{ payment_events : "P3 抽出（1通からN件）"
    transactions ||--o{ payment_events : "P5 名寄せ（N件を1取引に）"
    merchants ||--o{ merchant_aliases : "P4 表記ゆれ吸収"
    merchants ||--o{ transactions : ""
    categories ||--o{ transactions : "P6 分類"
    payment_events ||--o{ notifications : "P4.5 即時通知"

    email_messages {
        varchar provider_message_id UK "冪等性の要"
        varchar classification "payment/not_payment/unknown"
        text body_text "再計算のため保持"
    }
    payment_events {
        varchar event_type "金額の確からしさ + 通知可否"
        bigint transaction_id FK "名寄せ結果。NULLなら未処理"
        timestamptz notified_at "通知の冪等性"
    }
    transactions {
        boolean is_user_edited "再計算から保護"
        boolean is_possible_duplicate "要確認キュー"
        varchar source "email/manual（現金は0件でも成立）"
        bigint merged_into_id FK "マージ統合先。解除で NULL に戻す"
        timestamptz deleted_at "論理削除。行を残しイベントを保持"
    }
```

---

## 3. リレーションの要点

| リレーション | 多重度 | 理由 |
|---|---|---|
| `email_messages` → `payment_events` | **1 — N** | 銀行のまとめ通知やカードの月次確定明細は、1 通に複数の決済が含まれる |
| `transactions` → `payment_events` | **1 — N** | 同じ買い物が EC 側とカード側から 2 通のメールを発生させるため、複数イベントを 1 取引に束ねる |
| `payment_events` → `transactions` | **N — 1** | 1 つの決済イベントが属する取引は必ず 1 つ。**中間テーブルは不要** |
| `merchants` → `merchant_aliases` | 1 — N | 「AMAZON.CO.JP」「AMZN Mktp JP」などの表記ゆれを 1 店舗に集約する |
| `users` → `notification_settings` | **1 — 1** | 通知設定はユーザにつき 1 行（`UNIQUE(user_id)`） |
| `categories` → `categories` | 自己参照 | カテゴリの親子関係 |
| **`transactions` → `transactions`** | **自己参照** | `merged_into_id` でマージの統合先を指す。統合元は論理削除で残し、`unmerge` で復元できるようにする |
| `transactions` → `payment_events` | **0 も許容** | 現金など手動入力の取引は、紐づくイベントを持たない |

---

## 4. 補足

- **`payment_events.transaction_id` が NULL** の行は「まだ名寄せされていないイベント」を表す。
  名寄せバッチはこの条件で対象を拾う。
- **`transactions` は論理削除（`deleted_at`）**。物理削除すると `transaction_id` が
  SET NULL になってイベントが「未名寄せ」に戻り、次のバッチで**同じ取引が再生成される**。
  行を残してイベントを保持させることでこれを防いでいる。
- **マージ解除**は、統合元の `deleted_at` と `merged_into_id` を NULL に戻し、
  決済イベントを付け替えることで実現する。統合元を物理削除しないのはこのため。
- **`transactions` は自前のフィールドを持つ**ため、名寄せロジックを変更して全件を再計算しても、
  `is_user_edited = true` の行は影響を受けない。
- ジオコーディングの結果（住所・緯度経度・店舗種別）は `transactions` ではなく `merchants` に持つ。
  同じ店舗の取引が何件あっても、外部 API の呼び出しは 1 回で済む。
