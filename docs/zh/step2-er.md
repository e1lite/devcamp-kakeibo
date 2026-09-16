# ER 图

> 中文参考版。正本：[../step2-er.md](../step2-er.md)（日文，提交用）
> 表名、列名保持原样不翻译。

服务名：決済メール解析による自動家計簿サービス
对应规格书：[step2-db-spec.md](./step2-db-spec.md)

> 草案版。用 Mermaid 制作。draw.io 誊清等 schema 定稿后再做。

---

## 1. 全体 ER 图

```mermaid
erDiagram
    users ||--o{ mail_accounts : "连携"
    users ||--o{ email_messages : "拥有"
    users ||--o{ payment_events : "拥有"
    users ||--o{ transactions : "拥有"
    users ||--o{ merchants : "拥有"
    users ||--o{ payment_methods : "拥有"
    users ||--o{ categories : "拥有"
    users ||--o{ category_rules : "拥有"
    users ||--|| notification_settings : "持有设置"
    users ||--o{ notifications : "接收"
    users ||--o{ subscriptions : "拥有"
    users ||--o{ budgets : "设置"
    users ||--o{ monthly_summaries : "接收"

    mail_accounts ||--o{ sync_jobs : "被执行"
    mail_accounts ||--o{ email_messages : "取得"

    parser_templates ||--o{ email_messages : "判定"
    parser_templates ||--o{ payment_events : "抽出"

    email_messages ||--o{ payment_events : "一封产生N条"
    transactions ||--o{ payment_events : "名寄せ束成一笔"

    merchants ||--o{ merchant_aliases : "持有写法差异"
    merchants ||--o{ payment_events : "关联"
    merchants ||--o{ transactions : "关联"
    merchants ||--o{ subscriptions : "扣款来源"

    payment_methods ||--o{ payment_events : "支付手段"
    payment_methods ||--o{ transactions : "支付手段"
    payment_methods ||--o{ parser_templates : "默认支付手段"

    categories ||--o{ transactions : "分类"
    categories ||--o{ category_rules : "适用对象"
    categories ||--o{ budgets : "预算对象"
    categories ||--o{ categories : "父子"

    transactions ||--o{ transactions : "合并目标"

    payment_events ||--o{ notifications : "即时通知的对象"
    notifications ||--o| monthly_summaries : "配信结果"

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

## 2. 核心部分 ER 图（讲解用）

省掉连向 `users` 的线，只留下**数据流动的三层结构**。
**メンタリング 时用这张讲。**

```mermaid
erDiagram
    mail_accounts ||--o{ email_messages : "P1 收集"
    parser_templates ||--o{ email_messages : "P2 判定"
    email_messages ||--o{ payment_events : "P3 抽出（一封→N条）"
    transactions ||--o{ payment_events : "P5 名寄せ（N条→一笔）"
    merchants ||--o{ merchant_aliases : "P4 吸收写法差异"
    merchants ||--o{ transactions : ""
    categories ||--o{ transactions : "P6 分类"
    payment_events ||--o{ notifications : "P4.5 即时通知"

    email_messages {
        varchar provider_message_id UK "幂等性的核心"
        varchar classification "payment/not_payment/unknown"
        text body_text "为重算而保留"
    }
    payment_events {
        varchar event_type "金额可信度 + 是否通知"
        bigint transaction_id FK "名寄せ结果。NULL表示未处理"
        timestamptz notified_at "通知的幂等性"
    }
    transactions {
        boolean is_user_edited "保护，不参与重算"
        boolean is_possible_duplicate "待确认队列"
        varchar source "email/manual（现金挂0条也成立）"
        bigint merged_into_id FK "合并目标。解除时置回 NULL"
        timestamptz deleted_at "逻辑删除。留住行以攥着事件"
    }
```

---

## 3. 关系的要点

| 关系 | 多重度 | 理由 |
|---|---|---|
| `email_messages` → `payment_events` | **1 — N** | 银行的汇总通知、卡的月度确定明细，一封里含多笔支付 |
| `transactions` → `payment_events` | **1 — N** | 同一笔购买会从电商侧和卡侧各发一封邮件，要把多条事件束成一笔 |
| `payment_events` → `transactions` | **N — 1** | 一条支付事件所属的交易必定只有一个。**不需要中间表** |
| `merchants` → `merchant_aliases` | 1 — N | 把「AMAZON.CO.JP」「AMZN Mktp JP」等写法差异集约到一家店铺 |
| `users` → `notification_settings` | **1 — 1** | 通知设置一个用户一行（`UNIQUE(user_id)`） |
| `categories` → `categories` | 自引用 | 分类的父子关系 |
| **`transactions` → `transactions`** | **自引用** | 用 `merged_into_id` 指向合并目标。合并来源用逻辑删除保留，让 `unmerge` 能复原 |
| `transactions` → `payment_events` | **允许 0 条** | 现金等手动录入的交易，不挂任何事件 |

---

## 4. 补充

- **`payment_events.transaction_id` 为 NULL** 的行表示「还没被 名寄せ 的事件」。
  名寄せ 批处理就是按这个条件捡对象的。
- **`transactions` 用逻辑删除（`deleted_at`）**。物理删除的话 `transaction_id` 会变 SET NULL、
  事件回到「未名寄せ」，下次批处理**又生成一模一样的交易**。
  留住行让它继续攥着事件，就挡住了这个问题。
- **解除合并**是把合并来源的 `deleted_at` 和 `merged_into_id` 置回 NULL，
  再把支付事件挪回去。不物理删除合并来源就是为了这个。
- **`transactions` 持有自己的字段**，所以改了 名寄せ 逻辑做全量重算时，
  `is_user_edited = true` 的行不受影响。
- 地理编码的结果（地址・经纬度・店铺种别）放在 `merchants` 而不是 `transactions`。
  同一家店铺不管有多少笔交易，外部 API 只调用一次。
