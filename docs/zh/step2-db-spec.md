# 数据库规格书

> 中文参考版。正本：[../step2-db-spec.md](../step2-db-spec.md)（日文，提交用）
> 表名、列名、枚举值保持原样不翻译。

服务名：決済メール解析による自動家計簿サービス（解析支付邮件的自动记账服务）
语言 / 预定 DB：Go / PostgreSQL（GORM）
设计根据：[step0-pipeline.md](./step0-pipeline.md)

---

## 1. 数据库概要

管理从支付通知邮件自动抽出的支出数据。

结构是**三层**：原始邮件（`email_messages`）→ 从邮件抽出的支付事件（`payment_events`）→
名寄せ 后的实际交易（`transactions`）。
保留原始数据，是为了在抽出、名寄せ 逻辑变更时能全量重算。

另外还处理店铺主数据和写法差异吸收（`merchants` / `merchant_aliases`）、
分类自动判定（`categories` / `category_rules`）、
通知（`notification_settings` / `notifications`）、
以及订阅、预算、月末汇总。

**共通方针**

| 方针 | 内容 |
|---|---|
| 多租户 | 所有业务表带 `user_id`，用户间数据隔离 |
| 金额 | `bigint` 最小单位（日元 1 = 1円）。不用浮点 |
| 货币 | `currency`（ISO 4217）+ 日元换算额 `amount_jpy_minor` 并存 |
| 日时 | 用 `timestamptz` 保存。汇总按 JST |
| 主键 | `bigint` 自动采番 |
| 共通列 | 原则上所有表都有 `created_at` / `updated_at` |
| 删除 | 原则上物理删除。有两个例外：**`transactions` 用 `deleted_at` 做逻辑删除**（3.7）、**解除邮箱连携用 `mail_accounts.status = 'disabled'`**（3.2）。各自的理由写在对应小节 |

---

## 2. 表一览

| No. | 表名 | 概要 | 对应工序 | 实装阶段 |
|---|---|---|---|---|
| 1 | `users` | app 利用者 | P0 | **Step 3（核心）** |
| 2 | `mail_accounts` | 连携的邮箱账号和认证信息 | P0 | **Step 3（核心）** |
| 3 | `sync_jobs` | 取邮件 job 的执行历史 | P1 | Step 8 以后（扩展） |
| 4 | `email_messages` | 取到的邮件原始数据 | P1 / P2 | Step 8 以后（扩展） |
| 5 | `parser_templates` | 发件人・标题的判定条件和抽出规则 | P2 / P3 | Step 8 以后（扩展） |
| 6 | `payment_events` | 从邮件抽出的支付事件（名寄せ 前） | P3 / P4 / P4.5 | Step 8 以后（扩展） |
| 7 | `transactions` | 名寄せ 后的实际交易（一次支出 = 一行） | P5 / P6 | **Step 3（核心）** |
| 8 | `merchants` | 正规化店铺主数据（支店单位・带位置信息） | P4 / P4.6 | **Step 3（核心）** |
| 9 | `merchant_aliases` | 写法差异 → `merchants` 的映射 | P4 | Step 8 以后（扩展） |
| 10 | `payment_methods` | 支付手段主数据 | P4 | **Step 3（核心）** |
| 11 | `categories` | 分类主数据 | P6 | **Step 3（核心）** |
| 12 | `category_rules` | 分类自动判定规则 | P6 | Step 8 以后（扩展） |
| 13 | `notification_settings` | 通知设置（渠道・阈值・静音时段） | P4.5 / P8 | Step 8 以后（扩展） |
| 14 | `notifications` | 通知发送历史 | P4.5 / P8 | Step 8 以后（扩展） |
| 15 | `subscriptions` | 检测到的订阅 | P7 | Step 8 以后（扩展） |
| 16 | `budgets` | 预算设置 | P8 | Step 8 以后（扩展） |
| 17 | `monthly_summaries` | 月末汇总的生成・配信记录 | P8 | Step 8 以后（扩展） |

### 2.1 实装阶段的划分思路

根据 Step 2 的反馈，Step 3 只实装**上表中的 6 张表**。
有了这个范围，前端（Step 4〜6）就能以「手动输入的记账本」做动作验证，
Step 7 以后基础设施构建所需的「能跑起来的后端」也成立。

| 表 | 纳入 Step 3 核心的理由 |
|---|---|
| `users` | 全部业务表的多租户边界 |
| `mail_accounts` | Google OAuth 登录时保存账号信息。**邮件取得的执行放到 Step 8 以后** |
| `transactions` | 记账本的核心。手动输入的 CRUD 在这里成立 |
| `merchants` / `payment_methods` / `categories` | `transactions` 用外键引用的主数据，没有它们就登记不了交易 |

邮件连携・解析批处理・名寄せ（P1〜P5 的自动化类），以及通知・订阅・预算・月末汇总
（P4.5 / P7 / P8）对应的表，等核心跑通后分阶段追加。
API 侧对应的阶段划分见 [step2-api-spec.md](./step2-api-spec.md) 的 5.1。

> **迁移顺序**
> `transactions` 对 `users` / `merchants` / `payment_methods` / `categories` 有外键，
> 所以要先建被引用方。顺序为 `users` → `categories` / `payment_methods` / `merchants` → `mail_accounts` → `transactions`。

---

## 3. 各表详细

### 3.1 `users`

app 的利用者。认证委托给 Google OAuth，因此不保存密码。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | 用户 ID |
| google_sub | varchar(255) | ○ | UNIQUE | Google 账号的唯一标识（`sub` claim） |
| email | varchar(255) | ○ | UNIQUE | 邮箱地址 |
| display_name | varchar(100) | | | 显示名 |
| timezone | varchar(50) | ○ | default `Asia/Tokyo` | 汇总的基准时区 |
| created_at | timestamptz | ○ | | 创建时刻 |
| updated_at | timestamptz | ○ | | 更新时刻 |

**索引**

不定义额外索引。
登录时定位本人所用的 `google_sub` 检索，
由 PostgreSQL 为 `UNIQUE` 约束自动生成的唯一索引承担。

---

### 3.2 `mail_accounts`

连携的邮箱账号。**用 `provider` 列切换取込方式**，
这样 Gmail API / GAS / IMAP 选哪个都能用同一张表处理。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | ID |
| user_id | bigint | ○ | FK → users.id (CASCADE) | 所属用户 |
| email_address | varchar(255) | ○ | | 连携的邮箱地址 |
| provider | varchar(20) | ○ | | 取込方式（`gmail_api` / `gas` / `imap`） |
| credential_encrypted | text | ○ | | 加密后的认证信息（refresh token 等） |
| credential_expires_at | timestamptz | | | 认证信息的有效期 |
| sync_cursor | varchar(255) | | | 上次同步位置（Gmail 的 historyId 等） |
| backfilled_until | timestamptz | | | 已完成回填的起点 |
| last_synced_at | timestamptz | | | 最后同步时刻。**同步成功时即使 0 封也要更新**（死活监视要用，见 3.3） |
| status | varchar(20) | ○ | default `active` | `active` / `reauth_required` / `disabled`（**解除连携用这个值表达，不删行**。见下面的删除方针） |
| created_at | timestamptz | ○ | | 创建时刻 |
| updated_at | timestamptz | ○ | | 更新时刻 |

**约束**

- `UNIQUE(user_id, email_address)`：防止同一地址重复连携

**索引**

| 索引名 | 对象列 | 用途 |
|---|---|---|
| idx_mail_accounts_status | status | 批处理筛出待同步的账号 |

> 取用户的连携一览（`WHERE user_id = ?`）由 `UNIQUE(user_id, email_address)` 的
> 自动生成索引的左前缀承担，不定义专用索引。

**删除方针**

「解除连携」和「注销账号」当作**两个不同的操作**处理。

| 操作 | 执行内容 | 影响 |
|---|---|---|
| 解除连携 | 更新为 `status = 'disabled'`，并**清空 `credential_encrypted`** | 停止同步，但 `email_messages` 及之后的数据全部保留 |
| 注销账号 | `DELETE FROM users WHERE id = ?` | 靠 FK 的 CASCADE 连锁删除该用户的全部数据 |

> **理由**：`email_messages.mail_account_id` 是 CASCADE，
> 所以解除连携时若物理删除 `mail_accounts` 的行，邮件原始数据也会一起消失，
> 3.4 的「保留全量重算的起点」这一方针就不成立了。
> 用户点解除连携时期待的是「别再读我的邮箱了」，而不是「把我的家計簿删了」，
> 所以设计成保留行。
>
> 另一方面「别再读了」这个诉求也必须满足，因此
> **解除时清空 `credential_encrypted`，不再持有 refresh token**。

---

### 3.3 `sync_jobs`

取邮件 job 的执行历史。为了**可观测性（检测无声故障）**，必须有。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | ID |
| mail_account_id | bigint | ○ | FK → mail_accounts.id (CASCADE) | 对象账号 |
| trigger | varchar(20) | ○ | | `scheduled` / `manual` / `backfill` / `push`（迁移到 Push 通知时使用） |
| started_at | timestamptz | ○ | | 开始时刻 |
| finished_at | timestamptz | | | 结束时刻 |
| status | varchar(20) | ○ | | `running` / `success` / `partial` / `failed` |
| fetched_count | integer | ○ | default 0 | 取到的邮件数 |
| parsed_count | integer | ○ | default 0 | 抽出成功数 |
| failed_count | integer | ○ | default 0 | 抽出失败数 |
| cursor_before | varchar(255) | | | 执行前的游标 |
| cursor_after | varchar(255) | | | 执行后的游标 |
| error_message | text | | | 错误内容 |
| created_at | timestamptz | ○ | | 创建时刻 |

**索引**

| 索引名 | 对象列 | 用途 |
|---|---|---|
| idx_sync_jobs_account_started | (mail_account_id, started_at DESC) | 同步历史画面显示 |

**运营方针**

| # | 方针 | 内容 |
|---|---|---|
| 1 | 记录粒度 | 1 分钟跑一次，照单全收一天 1440 行。绝大多数是「没有新邮件」，没有记录价值，所以**`fetched_count = 0` 且 `status = success` 的执行不记入 `sync_jobs`** |
| 2 | 死活监视 | **只要执行成功，即使 0 封也必须更新 `mail_accounts.last_synced_at`**（见下） |
| 3 | 保留期间 | 超过 90 天的行定期删除。但 `status IN ('failed', 'partial')` 的行为了故障分析而保留 |

> **方针 2 的理由**：只有方针 1 的话，「没有新邮件」和「批处理本身停了」
> 在表里都表现为「`sync_jobs` 里没有行」，无法区分。
> `sync_jobs` 存在的目的就是检测无声故障，这样就失去了本来的意义。
>
> 成功时更新 `last_synced_at`，就能不增加行、只用 1 次 UPDATE 保住「最后成功时刻」。
> 死活监视用下面这条查询，有行就判定批处理已停止。
>
> ```sql
> SELECT id, email_address, last_synced_at
> FROM mail_accounts
> WHERE status = 'active'
>   AND last_synced_at < now() - interval '5 minutes';
> ```

> **設計判断：取邮件为什么选轮询**
>
> Gmail 有 Push 通知（`users.watch` + Cloud Pub/Sub），延迟 1~2 秒、零空转，
> 本来它才是合适的方案。初期实装仍选轮询，理由如下。
>
> | 观点 | 判断 |
> |---|---|
> | 配额 | 差分取得不是全量扫收件箱，而是用 `users.history.list`，一天 1440 次也只消耗数千 quota 单位，相对 project 日上限可以忽略 |
> | 延迟 | 最坏 60 秒。卡公司发出速报邮件本身就要几十秒到几分钟，放在整条链路里看差距很小 |
> | 额外运营成本 | Push 需要搭 Pub/Sub、公开 HTTPS 端点，以及**7 天就失效的 `watch` 的续期批处理**。这些运营成本和本课题的主题（解析支付邮件）无关 |
> | 可靠性 | Pub/Sub 是 at-least-once，即使迁移到 Push，也仍要并用低频轮询来兜底防漏 |
>
> **迁移到 Push 时几乎不需要改 schema。**
> Push 推来的同样是 `historyId`，`mail_accounts.sync_cursor` 的语义不变。
> 因为「取得的触发方式」和「取得后的处理」是分离的，
> 只要给 `sync_jobs.trigger` 加一个 `push` 值，两种方式就能用同一张表处理。
>
> **迁移的判断基准**：即时通知的延迟成为用户体验上的问题时。

---

### 3.4 `email_messages`

取到的邮件原始数据。**通常运营下不删。**
是后段（抽出・正规化・名寄せ）逻辑变更时做全量重算的起点。
解除邮箱连携也不会删除（见 3.2 的删除方针）。

另外，`transactions` 不引用这张表而是持有自己的字段，
所以即使因为隐私要求删掉这张表，**家計簿数据本身也不会丢失**
（丢的只是重算能力和溯源）。

> **正文的保留期限**：当前不删。将来只把正文（`body_text` / `body_html`）
> 以 90 天〜180 天为目标更新成 `NULL`。**行本身不删**
> （为了维持下面 `UNIQUE` 约束带来的取込 幂等性）。详见 6.4。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | ID |
| user_id | bigint | ○ | FK → users.id (CASCADE) | 所属用户 |
| mail_account_id | bigint | ○ | FK → mail_accounts.id (CASCADE) | 取得来源账号 |
| provider_message_id | varchar(255) | ○ | | Gmail 的 message ID |
| thread_id | varchar(255) | | | 会话 ID |
| from_address | varchar(255) | ○ | | 发件人地址 |
| from_name | varchar(255) | | | 发件人名 |
| subject | text | | | 标题 |
| received_at | timestamptz | ○ | | 接收时刻 |
| body_text | text | | | 正文（纯文本） |
| body_html | text | | | 正文（HTML） |
| classification | varchar(20) | ○ | default `unknown` | `payment` / `not_payment` / `unknown` |
| classified_at | timestamptz | | | 判定时刻 |
| matched_template_id | bigint | | FK → parser_templates.id (SET NULL) | 匹配上的模板 |
| parse_status | varchar(20) | ○ | default `pending` | `pending` / `success` / `failed` / `skipped` |
| parse_error | text | | | 抽出失败的内容 |
| created_at | timestamptz | ○ | | 创建时刻 |
| updated_at | timestamptz | ○ | | 更新时刻 |

**约束**

- `UNIQUE(mail_account_id, provider_message_id)`
  **← 保证取込幂等性的核心约束。** 同一封邮件重复取也不会多出行。

**索引**

| 索引名 | 对象列 | 用途 |
|---|---|---|
| idx_email_messages_user_received | (user_id, received_at DESC) | 接收一览显示 |
| idx_email_messages_queue | (user_id, classification, parse_status) | 筛出未判定・未处理的邮件（P2/P3 的队列） |
| idx_email_messages_from | from_address | 按发件人统计接收件数（中断检测） |

---

### 3.5 `parser_templates`

根据发件人・标题判定「是不是支付邮件」，并从正文抽出各项目的规则。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | ID |
| user_id | bigint | | FK → users.id (CASCADE) | NULL 表示全用户共通模板 |
| name | varchar(100) | ○ | | 模板名（例：楽天カード 利用速報） |
| from_pattern | varchar(255) | ○ | | 发件人匹配条件（正则） |
| subject_pattern | varchar(255) | | | 标题匹配条件（用来排除促销邮件） |
| event_type | varchar(20) | ○ | | 生成的支付事件种别（见 3.6） |
| payment_method_id | bigint | | FK → payment_methods.id (SET NULL) | 支付手段固定时 |
| body_format | varchar(10) | ○ | default `text` | `text` / `html` |
| extraction_rules | jsonb | ○ | | 各项目的抽出规则（见下） |
| is_multi_event | boolean | ○ | default false | 一封是否产生多条事件 |
| priority | integer | ○ | default 100 | 评估顺序（越小越优先） |
| is_active | boolean | ○ | default true | 有效标志 |
| created_by | varchar(20) | ○ | default `user` | `system` / `user` / `llm`（追踪 LLM 生成是否已 review） |
| created_at | timestamptz | ○ | | 创建时刻 |
| updated_at | timestamptz | ○ | | 更新时刻 |

**`extraction_rules` 的结构例**

```json
{
  "occurred_at": { "regex": "ご利用日時：(.+?)\\n", "format": "2006/01/02 15:04" },
  "amount":      { "regex": "ご利用金額：([0-9,]+)円" },
  "merchant":    { "regex": "ご利用先：(.+?)\\n" },
  "card_last4":  { "regex": "カード番号：\\*+([0-9]{4})" }
}
```

**约束**

- `UNIQUE(user_id, name)`

**索引**

| 索引名 | 对象列 | 用途 |
|---|---|---|
| idx_parser_templates_match | (is_active, priority) | 判定时按顺序取 |
| idx_parser_templates_from | from_pattern | 从发件人检索模板 |

> **設計判断**：抽出规则不展开成列而用 `jsonb`，理由是
> 各发件人要抽的项目不一样（有的有卡尾 4 位、有的没有；有的有货币、有的没有），
> 用列会全是 NULL，而且每加一个项目就要迁移。
> 反过来，**判定条件（`from_pattern` / `subject_pattern`）是检索对象，所以用列**。

---

### 3.6 `payment_events`

从一封邮件抽出的支付事件。**一封邮件可能产生多行**
（银行的出入金汇总通知、卡公司的月度确定明细等）。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | ID |
| user_id | bigint | ○ | FK → users.id (CASCADE) | 所属用户 |
| email_message_id | bigint | ○ | FK → email_messages.id (CASCADE) | 抽出来源邮件 |
| parser_template_id | bigint | | FK → parser_templates.id (SET NULL) | 使用的模板 |
| transaction_id | bigint | | FK → transactions.id (SET NULL) | **名寄せ 结果。未处理时为 NULL** |
| link_type | varchar(20) | | | `auto` / `manual`（名寄せ 的路径） |
| link_score | numeric(5,2) | | | 自动 名寄せ 时的分数 |
| linked_at | timestamptz | | | 名寄せ 时刻 |
| event_type | varchar(20) | ○ | | 事件种别（见下表） |
| occurred_at | timestamptz | ○ | | 支付时刻 |
| amount_minor | bigint | ○ | | 金额（最小单位） |
| currency | char(3) | ○ | default `JPY` | 货币代码 |
| amount_jpy_minor | bigint | ○ | | 日元换算额（最小单位） |
| raw_merchant_text | varchar(255) | | | 原始店铺名字符串 |
| merchant_id | bigint | | FK → merchants.id (SET NULL) | 正规化后的店铺 |
| payment_method_id | bigint | | FK → payment_methods.id (SET NULL) | 支付手段 |
| card_last4 | varchar(4) | | | 卡尾 4 位 |
| normalized_at | timestamptz | | | 正规化完成时刻 |
| notified_at | timestamptz | | | **即时通知已发送时刻。保证通知的幂等性** |
| created_at | timestamptz | ○ | | 创建时刻 |
| updated_at | timestamptz | ○ | | 更新时刻 |

**`event_type` 的取值**

| 值 | 例 | 金额可信度 | 即时通知 |
|---|---|---|---|
| `order_confirm` | 电商订单确认 | 低 | 不发（钱还没动） |
| `usage_notice` | 卡利用速报 | 中 | 发 |
| `settlement` | 卡确定明细 | 高 | 不发（速报时已通知） |
| `receipt` | 订阅收据 | 高 | 发 |
| `bank_debit` | 银行扣款通知 | 高 | 发 |

**索引**

| 索引名 | 对象列 | 用途 |
|---|---|---|
| idx_payment_events_user_occurred | (user_id, occurred_at DESC) | 一览・交易详情时参照 |
| **idx_payment_events_reconcile** | **(user_id, amount_jpy_minor, occurred_at)** | **名寄せ（P5）的候选生成。为「金额一致 + 日时接近」筛选专门建的复合索引** |
| idx_payment_events_transaction | transaction_id | 取交易挂着的事件 |
| idx_payment_events_unnotified | (user_id, notified_at) | 筛出未通知事件（部分索引：`WHERE notified_at IS NULL`） |
| idx_payment_events_email | email_message_id | 从邮件取事件 |

---

### 3.7 `transactions`

名寄せ 后的实际交易。**一次支出 = 一行**。

**持有自己的字段**，而不是引用 `payment_events`。
这样改了 名寄せ 逻辑做全量重算时，用户的手动修改不会丢。

现金等没有通知手段的支付不经过 `payment_events` 直接创建，
所以**挂 0 条 `payment_events` 的行也是正常的**。

**只有这张表用逻辑删除（`deleted_at`）。** 理由见下面的删除方针。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | 交易 ID |
| user_id | bigint | ○ | FK → users.id (CASCADE) | 所属用户 |
| occurred_at | timestamptz | ○ | | 支付时刻 |
| amount_minor | bigint | ○ | | 金额（最小单位） |
| currency | char(3) | ○ | default `JPY` | 货币代码 |
| amount_jpy_minor | bigint | ○ | | 日元换算额（汇总用这列） |
| merchant_id | bigint | | FK → merchants.id (SET NULL) | 店铺 |
| category_id | bigint | | FK → categories.id (SET NULL) | 分类 |
| payment_method_id | bigint | | FK → payment_methods.id (SET NULL) | 支付手段 |
| category_source | varchar(20) | ○ | default `default` | `rule` / `place_type` / `user` / `default`（防止用规则覆盖用户的修改） |
| source | varchar(20) | ○ | | `email` / `manual` |
| status | varchar(20) | ○ | default `confirmed` | `pending`（只有速报） / `confirmed`（有确定明细） |
| is_user_edited | boolean | ○ | default false | **true 的行不参与重算** |
| is_possible_duplicate | boolean | ○ | default false | 名寄せ 分数中等，等用户确认 |
| merged_into_id | bigint | | FK → transactions.id (SET NULL) | **被合并进的目标交易 ID（自引用）。非 NULL 表示已被合并** |
| note | text | | | 备注 |
| deleted_at | timestamptz | | | **逻辑删除的执行时刻。只有 NULL 的行有效**（见下面的删除方针） |
| created_at | timestamptz | ○ | | 创建时刻 |
| updated_at | timestamptz | ○ | | 更新时刻 |

**索引**

| 索引名 | 对象列 | 用途 |
|---|---|---|
| idx_transactions_user_occurred | (user_id, occurred_at DESC) | 交易一览的主轴（期间筛选・新到顺）。部分索引：`WHERE deleted_at IS NULL` |
| idx_transactions_user_category_occurred | (user_id, category_id, occurred_at) | 月末汇总的分类别聚合。部分索引：`WHERE deleted_at IS NULL` |
| idx_transactions_merchant | merchant_id | 店铺别聚合・订阅检测 |
| idx_transactions_review | (user_id, is_possible_duplicate) | 取待确认队列（部分索引：`WHERE is_possible_duplicate AND deleted_at IS NULL`） |
| idx_transactions_merged | merged_into_id | 解除合并时定位合并来源（部分索引：`WHERE merged_into_id IS NOT NULL`） |

> 查询系一律用 `deleted_at IS NULL` 筛选，所以主要索引都加了这个条件做成**部分索引**。
> 已删除的行不进索引，普通的一览和聚合不受删除件数影响。

**删除方针**

`transactions` 用**逻辑删除**（往 `deleted_at` 里写时刻）。
**逻辑删除只用在这一张表**，其他表保持物理删除。

| 操作 | 执行内容 |
|---|---|
| 用户删除 | `deleted_at = now()` |
| 合并（合并来源） | `deleted_at = now()` 且 `merged_into_id = <合并目标的交易 ID>` |
| 解除合并 | 把 `deleted_at` 和 `merged_into_id` 置回 NULL，并把支付事件挪回合并来源 |

查询时一律用 `deleted_at IS NULL` 筛选。

> **理由 1：物理删除的话，删掉的交易会复活**
>
> 交易一旦物理删除，`payment_events.transaction_id` 会 SET NULL，
> 事件回到「未名寄せ」状态。
> 下次 名寄せ 批处理（P5）捡到它们，**又会生成一模一样的交易**。
> 逻辑删除的话行还在、继续攥着支付事件，批处理能判断「已处理」，就不会复活。
>
> **理由 2：解除合并才成立**
>
> 合并来源一旦物理删除，`unmerge` 就恢复不回来。
> 事件可以置回 NULL，但被删掉的交易行、以及上面的用户手动修改（备注・分类）
> 是复原不了的。
> 用 `merged_into_id` 记下合并目标再逻辑删除，只要把两列置回 NULL 就能复原。
>
> **理由 3：误删可以救回来**
>
> 家計簿 是不希望发生不可挽回删除的数据，留出提供「撤销」的余地。
>
> **代价**：所有查询都要带 `deleted_at IS NULL`，漏一次就会让已删除的交易出现在画面上。
> 用 GORM 的 `gorm.DeletedAt` 类型可以自动加上，防止遗漏。
>
> **为什么不推广到其他表**：`categories` 等主数据若改成逻辑删除，
> 已删除的行会一直占着 `UNIQUE(user_id, name)`，用户没法用同名重新创建。
> 而且主数据的 FK 本来就是 `SET NULL`，物理删除也不会产生不一致。

---

### 3.8 `merchants`

正规化后的店铺主数据。**一行 = 一个支店**，用 `brand_name` 绑定连锁。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | 店铺 ID |
| user_id | bigint | ○ | FK → users.id (CASCADE) | 所属用户 |
| name | varchar(255) | ○ | | 正规化后的显示名（例：セブン-イレブン渋谷道玄坂店） |
| brand_name | varchar(100) | | | 连锁名（例：セブン-イレブン） |
| is_online | boolean | ○ | default false | 是否电商等线上支付 |
| address | varchar(255) | | | 地址 |
| latitude | numeric(9,6) | | | 纬度 |
| longitude | numeric(9,6) | | | 经度 |
| place_id | varchar(255) | | | Places API 的场所 ID |
| place_types | varchar(255) | | | 店铺种别（逗号分隔。用于分类推定） |
| geocode_status | varchar(20) | ○ | default `pending` | `pending` / `success` / `not_found` / `skipped` |
| geocoded_at | timestamptz | | | 地理编码执行时刻 |
| created_at | timestamptz | ○ | | 创建时刻 |
| updated_at | timestamptz | ○ | | 更新时刻 |

**约束**

- `UNIQUE(user_id, name)`

**索引**

| 索引名 | 对象列 | 用途 |
|---|---|---|
| idx_merchants_user_brand | (user_id, brand_name) | 品牌单位的聚合・分类规则的适用 |
| idx_merchants_geocode_queue | geocode_status | 筛出未地理编码的店铺（部分索引：`WHERE geocode_status = 'pending'`） |

---

### 3.9 `merchant_aliases`

把店铺名的写法差异映射到 `merchants`。
表达「アマゾン ジャパン」「AMAZON.CO.JP」「AMZN Mktp JP」指向同一家店。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | ID |
| user_id | bigint | ○ | FK → users.id (CASCADE) | 所属用户 |
| merchant_id | bigint | ○ | FK → merchants.id (CASCADE) | 对应的店铺 |
| alias | varchar(255) | ○ | | 正规化后的匹配键 |
| raw_sample | varchar(255) | | | 原始字符串的样例（debug 用） |
| source | varchar(20) | ○ | default `auto` | `auto` / `user`（是否用户手动关联） |
| created_at | timestamptz | ○ | | 创建时刻 |

**约束**

- `UNIQUE(user_id, alias)`：防止同一写法指向多家店铺。
  **这条约束的自动生成索引同时承担正规化（P4）的主要查找。**
  每处理一封邮件都要查，是调用最多的路径，但不需要专用索引。

**索引**

| 索引名 | 对象列 | 用途 |
|---|---|---|
| idx_merchant_aliases_merchant | merchant_id | 取店铺挂着的写法差异一览 |

---

### 3.10 `payment_methods`

支付手段主数据。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | ID |
| user_id | bigint | ○ | FK → users.id (CASCADE) | 所属用户 |
| name | varchar(100) | ○ | | 显示名（例：楽天カード、PayPay、現金） |
| kind | varchar(20) | ○ | | `credit_card` / `qr` / `bank` / `cash` / `other` |
| issuer | varchar(100) | | | 发行公司 |
| card_last4 | varchar(4) | | | 卡尾 4 位（用于从邮件定位支付手段） |
| is_active | boolean | ○ | default true | 使用中标志 |
| created_at | timestamptz | ○ | | 创建时刻 |
| updated_at | timestamptz | ○ | | 更新时刻 |

**约束**

- `UNIQUE(user_id, name)`

**索引**

| 索引名 | 对象列 | 用途 |
|---|---|---|
| idx_payment_methods_last4 | (user_id, card_last4) | 从邮件里的尾 4 位定位支付手段 |

---

### 3.11 `categories`

支出分类主数据。可以有父子关系。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | 分类 ID |
| user_id | bigint | ○ | FK → users.id (CASCADE) | 所属用户 |
| name | varchar(50) | ○ | | 分类名（例：食費、交通費） |
| parent_id | bigint | | FK → categories.id (SET NULL) | 父分类 |
| sort_order | integer | ○ | default 0 | 显示顺序 |
| is_system | boolean | ○ | default false | 是否初始创建的分类（含 `未分類`） |
| created_at | timestamptz | ○ | | 创建时刻 |
| updated_at | timestamptz | ○ | | 更新时刻 |

**约束**

- `UNIQUE(user_id, name)`

**索引**

| 索引名 | 对象列 | 用途 |
|---|---|---|
| idx_categories_user | (user_id, sort_order) | 分类一览显示 |

---

### 3.12 `category_rules`

分类自动判定规则。**用户修正交易分类时，会 upsert 到这里**（学习）。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | ID |
| user_id | bigint | ○ | FK → users.id (CASCADE) | 所属用户 |
| category_id | bigint | ○ | FK → categories.id (CASCADE) | 要适用的分类 |
| match_type | varchar(20) | ○ | | `merchant` / `brand` / `place_type` / `keyword` |
| match_value | varchar(255) | ○ | | 匹配值（店铺 ID、品牌名、店铺种别、关键词） |
| priority | integer | ○ | default 100 | 评估顺序（越小越优先） |
| source | varchar(20) | ○ | default `user` | `user` / `default` |
| created_at | timestamptz | ○ | | 创建时刻 |
| updated_at | timestamptz | ○ | | 更新时刻 |

**约束**

- `UNIQUE(user_id, match_type, match_value)`：防止同一条件指向多个分类。
  **这条约束的自动生成索引同时承担分类（P6）时的匹配。**

**索引**

| 索引名 | 对象列 | 用途 |
|---|---|---|
| idx_category_rules_priority | (user_id, priority) | 取评估顺序 |

> **設計判断**：准备了 `match_type = brand`，
> 所以「セブン-イレブン = 食費」教一次就对全支店生效。
> `place_type` 是从 Places API 拿到的店铺种别，**一次都没去过的店铺也能第一次就分对**。

---

### 3.13 `notification_settings`

通知设置。一个用户一行。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | ID |
| user_id | bigint | ○ | FK → users.id (CASCADE), UNIQUE | 所属用户 |
| channel | varchar(20) | ○ | default `email` | `email` / `slack` |
| email_to | varchar(255) | | | 配信目标邮箱 |
| slack_webhook_encrypted | text | | | 加密的 Slack Webhook URL |
| instant_enabled | boolean | ○ | default true | 即时通知开关 |
| instant_min_amount_minor | bigint | ○ | default 0 | 低于此金额不发即时通知 |
| quiet_hours_start | time | | | 静音时段开始 |
| quiet_hours_end | time | | | 静音时段结束 |
| budget_alert_enabled | boolean | ○ | default true | 预算超支通知开关 |
| monthly_summary_enabled | boolean | ○ | default true | 月末汇总开关 |
| monthly_summary_send_at | time | ○ | default `08:00` | 月末汇总的配信时刻 |
| created_at | timestamptz | ○ | | 创建时刻 |
| updated_at | timestamptz | ○ | | 更新时刻 |

**约束**

- `UNIQUE(user_id)`

---

### 3.14 `notifications`

通知的发送历史。即时通知・预算超支通知・月末汇总**三种用一张表处理**。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | ID |
| user_id | bigint | ○ | FK → users.id (CASCADE) | 所属用户 |
| kind | varchar(20) | ○ | | `instant` / `budget_alert` / `monthly_summary` |
| channel | varchar(20) | ○ | | `email` / `slack` |
| payment_event_id | bigint | | FK → payment_events.id (SET NULL) | 即时通知的对象事件 |
| dedupe_key | varchar(255) | ○ | | **防止重复发送的键**（见下） |
| subject | varchar(255) | | | 标题 |
| body | text | | | 正文 |
| status | varchar(20) | ○ | default `pending` | `pending` / `sent` / `failed` |
| sent_at | timestamptz | | | 发送时刻 |
| error_message | text | | | 发送失败的内容 |
| created_at | timestamptz | ○ | | 创建时刻 |

**`dedupe_key` 的设计**

| kind | dedupe_key 的例子 | 含义 |
|---|---|---|
| `instant` | `instant:event:12345` | 同一支付事件不通知两次 |
| `budget_alert` | `budget:2026-09:cat:3:80` | 同月・同分类・同阈值不通知两次 |
| `monthly_summary` | `monthly:2026-09` | 同一月的汇总不发两次 |

**约束**

- `UNIQUE(user_id, dedupe_key)`
  **← 用这一条约束防住三种通知的重复发送。** 批处理每分钟重跑也安全。

**索引**

| 索引名 | 对象列 | 用途 |
|---|---|---|
| idx_notifications_user_created | (user_id, created_at DESC) | 通知历史画面显示 |
| idx_notifications_pending | status | 筛出待重试的发送（部分索引：`WHERE status <> 'sent'`） |

---

### 3.15 `subscriptions`（扩展）

检测到的订阅。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | ID |
| user_id | bigint | ○ | FK → users.id (CASCADE) | 所属用户 |
| merchant_id | bigint | ○ | FK → merchants.id (CASCADE) | 店铺 |
| amount_minor | bigint | ○ | | 扣款额 |
| currency | char(3) | ○ | default `JPY` | 货币代码 |
| cycle | varchar(20) | ○ | | `monthly` / `yearly` / `weekly` |
| occurrence_count | integer | ○ | | 用于检测的扣款次数 |
| first_charged_at | timestamptz | ○ | | 首次扣款日 |
| last_charged_at | timestamptz | ○ | | 最后扣款日 |
| next_expected_at | timestamptz | | | 下次扣款的预测日 |
| status | varchar(20) | ○ | default `active` | `active` / `suspected_stopped` / `cancelled` |
| detected_at | timestamptz | ○ | | 检测时刻 |
| created_at | timestamptz | ○ | | 创建时刻 |
| updated_at | timestamptz | ○ | | 更新时刻 |

**约束**

- `UNIQUE(user_id, merchant_id, amount_minor, cycle)`

**索引**

| 索引名 | 对象列 | 用途 |
|---|---|---|
| idx_subscriptions_user_status | (user_id, status) | 取持续中订阅一览 |
| idx_subscriptions_next_expected | next_expected_at | 判定疑似停止 |

---

### 3.16 `budgets`（扩展）

预算设置。`category_id` 为 NULL 的行表示整体预算。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | ID |
| user_id | bigint | ○ | FK → users.id (CASCADE) | 所属用户 |
| category_id | bigint | | FK → categories.id (CASCADE) | NULL 为整体预算 |
| period | varchar(10) | ○ | default `monthly` | 预算的周期 |
| amount_minor | bigint | ○ | | 预算额 |
| alert_thresholds | varchar(50) | ○ | default `80,100` | 通知的达成率（%，逗号分隔） |
| is_active | boolean | ○ | default true | 有效标志 |
| created_at | timestamptz | ○ | | 创建时刻 |
| updated_at | timestamptz | ○ | | 更新时刻 |

**约束**

- `UNIQUE(user_id, category_id, period)`

---

### 3.17 `monthly_summaries`（扩展）

月末汇总的生成・配信记录。

| 列名 | 类型 | 必需 | 约束 | 说明 |
|---|---|---|---|---|
| id | bigint | ○ | PK, auto increment | ID |
| user_id | bigint | ○ | FK → users.id (CASCADE) | 所属用户 |
| year_month | char(7) | ○ | | 对象月（例：`2026-09`） |
| total_amount_minor | bigint | ○ | | 当月合计 |
| prev_total_amount_minor | bigint | | | 上月合计（算环比用） |
| breakdown | jsonb | ○ | | 分类别明细 |
| active_subscription_count | integer | ○ | default 0 | 持续中订阅数 |
| notification_id | bigint | | FK → notifications.id (SET NULL) | 配信的通知 |
| generated_at | timestamptz | ○ | | 生成时刻 |
| created_at | timestamptz | ○ | | 创建时刻 |

**约束**

- `UNIQUE(user_id, year_month)`：同一月的汇总不重复生成

---

## 4. 索引设计的思路

| 观点 | 对应索引 | 理由 |
|---|---|---|
| **画面主导线** | `idx_transactions_user_occurred` | 交易一览的基本操作就是「按期间筛选 + 新到顺」 |
| **聚合查询** | `idx_transactions_user_category_occurred` | 月末汇总的分类别明细 |
| **算法专用** | `idx_payment_events_reconcile` | 为支撑 名寄せ 的候选生成（金额一致 + 日时接近）建的复合索引 |
| **批处理取队列** | `idx_email_messages_queue`, `idx_merchants_geocode_queue`, `idx_payment_events_unnotified` | 各工序要快速捡出「只有未处理的」。**符合的行很少，所以用部分索引** |
| **正规化查找** | `merchant_aliases` 的 `UNIQUE(user_id, alias)` | 每封邮件必查、调用次数最多，但靠约束的自动生成索引就够 |
| **可观测性** | `idx_email_messages_from` | 按发件人统计接收件数，检测中断 |

> **不和约束自动生成的索引重复**
>
> PostgreSQL 会为 `PRIMARY KEY` 和 `UNIQUE` 自动生成唯一索引。
> 复合 `UNIQUE(A, B)` 也能用于 `WHERE A = ?` 的检索（左前缀），
> 所以**和这些重叠的专用索引一律不定义**。
> 重复定义会白占空间，而且每次写入都要更新两个内容相同的索引。
>
> 另一方面要注意，**`FOREIGN KEY` 不会自动生成索引**。
> 删除被引用方（主数据）的行时会导致引用方全表扫描，
> 但本服务里主数据的删除频率很低，所以没有专门建索引。

---

## 5. 主要設計判断

1. **三层分割（`email_messages` → `payment_events` → `transactions`）**
   保留原始数据，就能在抽出・名寄せ 逻辑变更时做全量重算。

2. **`email_messages` 1 — N `payment_events`**
   银行的汇总通知等，一封邮件里能读出多笔支付。

3. **名寄せ 没用中间表**
   一条支付事件所属的交易必定只有一个，所以关系是 N — 1。
   判断中间表不需要，改用 `payment_events.transaction_id` 外键表达。
   解除合并靠把这一列，以及合并来源的 `deleted_at` / `merged_into_id` 一起还原来实现（见 3.7）。

4. **`transactions` 持有自己的字段**
   如果设计成引用「代表事件」，重算时会丢掉用户的手动修改。
   用 `is_user_edited` 把这些行排除在重算之外。

5. **把 `payment_events.event_type` 作为核心列**
   名寄せ 后采用哪个金额的判定、以及即时通知发不发的判定，两处都用它。

6. **`merchants` 按支店单位 + `brand_name` 用列**
   地图显示要支店单位，聚合和分类规则要品牌单位。
   没拆 `brands` 表是考虑表数量和实装周期的范围取舍。

7. **用 `notifications.dedupe_key` 统一三种通知的幂等性**
   因为批处理每分钟重跑，必须防止重复发送。
   用 `UNIQUE(user_id, dedupe_key)` 一条约束覆盖三种。

8. **`parser_templates.extraction_rules` 用 `jsonb`**
   各发件人要抽的项目不同。判定条件因为要检索所以用列。

9. **金额用 `bigint` 最小单位保存**
   避免浮点误差。外币用「原货币 + 原金额 + 日元换算额」保存。

10. **多租户设计**
    运营是单用户，但所有业务表都带 `user_id`。事后追加的成本太高。

11. **取邮件选轮询，同时给 Push 通知留了迁移余地**
    初期实装是 1 分钟间隔的轮询。Push（`users.watch` + Cloud Pub/Sub）因为
    `watch` 续期批处理等主题外的运营成本太大而暂缓。
    但 Push 推来的也是 `historyId`、`sync_cursor` 的语义不变，
    所以设计成只要给 `sync_jobs.trigger` 加一个 `push` 就能迁移（见 3.3）。

12. **`sync_jobs` 做了间引，死活监视交给 `last_synced_at`**
    不记录无新邮件的执行，就会分不清「没有新邮件」和「批处理停了」。
    通过「成功时即使 0 件也更新 `mail_accounts.last_synced_at`」，
    在不增加行数的前提下让停止检测成立。

13. **解除连携不做物理删除**
    `email_messages` 通过 CASCADE 挂在 `mail_accounts` 下面，
    解除连携时若删行，邮件原始数据也会一起没了，重算能力随之丢失。
    解除连携用 `status = 'disabled'` + 销毁 token 表达，
    全部数据的删除拆成「注销（删除 `users`）」这个另外的操作（见 3.2）。

14. **只有 `transactions` 用逻辑删除**
    物理删除会让 `payment_events.transaction_id` 变成 SET NULL，
    下次 名寄せ 批处理又把同一笔交易生成一遍（**删掉的交易会复活**）。
    用 `deleted_at` 保留行、继续攥着支付事件，就挡住了这个问题。
    同时加上 `merged_into_id` 保留合并来源，让解除合并也能成立。
    主数据改成逻辑删除会占着 `UNIQUE` 约束，所以**例外只有这一张表**（见 3.7）。

---

## 6. 纠结的地方与 メンタリング 的确认结果

| # | 相谈事项 | 状态 |
|---|---|---|
| 1 | `sync_jobs` 的记录粒度 | **已确认**（总评中评价为妥当。维持现状） |
| 2 | 要不要有 `transactions.status` | **未确认** |
| 3 | `merchants` 按支店单位的判断 | **未确认** |
| 4 | `email_messages.body_text` / `body_html` 的保存 | **已确认**（追加了方针） |
| 5 | 大量使用部分索引 | **已确认**（总评中评价为妥当。维持现状） |

### 1. `sync_jobs` 的记录粒度 —— 已确认（维持现状）

> **相谈内容**：采用了「无新邮件的执行不记录，死活监视靠更新 `last_synced_at`」的方针（见 3.3）。
> 行数确实压下来了，但执行历史会变得断断续续，排查故障时会不会不方便

**结论：维持现状方针。** 反馈的总评里，
「不是每次都记录 `sync_jobs`，而是成功时即使 0 件也更新 `mail_accounts.last_synced_at`，
从而在压低行数的同时仍能检测批处理停止的运维方针」被列为做得好的点。

故障排查方面，靠 `status IN ('failed', 'partial')` 的行超过 90 天也保留的方针
（3.3 运维方针 3）来保障。**不记录的只有「成功且 0 件」的执行**，
会成为调查对象的异常系历史不会断。

### 2. 要不要有 `transactions.status`（`pending` / `confirmed`）—— 未确认

> **相谈内容**：作为「速报和确定明细金额会变」的对策合适吗

反馈里没有提及，下次 メンタリング 再谈。
现状按 3.7 保持 `pending`（只有速报）/ `confirmed`（有确定明细）的设计。

### 3. `merchants` 按支店单位的判断 —— 未确认

> **相谈内容**：是不是该拆 `brands` 表

反馈里没有提及，下次 メンタリング 再谈。
现状按 3.8 做成 **1 行 = 1 支店**，用 `brand_name` 列把连锁束起来。

### 4. `email_messages.body_text` / `body_html` 的保存 —— 已确认（追加了方针）

> **相谈内容**：重解析需要，但有隐私和容量的顾虑。
> 靠三层结构，**即使删掉这张表家計簿数据也还在**（丢的只是重算能力和溯源），
> 所以有余地设一个「正文保存 N 个月后删除」的保留期限。多长合适

**结论：方向上设保留期限，上限以 90 天〜180 天为目标。
但当前先一直保留。**

多亏三层结构，删掉正文家計簿数据本身也还在
（丢的只是重解析和溯源），所以设保留期限本身没问题。
另一方面，**改解析模板后想重新解析的场景还看不清**的期间，
删掉就等于丢了重算的起点。

**运维方针**

| # | 方针 | 内容 |
|---|---|---|
| 1 | 当前 | **不删。** 观察解析模板的变更频率和重解析的必要性 |
| 2 | 将来 | 只把正文（`body_text` / `body_html`）更新成 `NULL`。**行本身不删**（为维持 `UNIQUE(mail_account_id, provider_message_id)` 带来的取込 幂等性） |
| 3 | 上限 | 以 90 天〜180 天为目标。定下来要看重解析的实绩 |

> **不整行删除的理由**：3.4 的 `UNIQUE(mail_account_id, provider_message_id)`
> 是保障取込 幂等性的核心约束，删了行的话**同一封邮件再取一次就会多出重复行**。
> 只把正文置 `NULL` 的话，容量和隐私的顾虑都能解决，
> 同时「这封邮件已取过」这个事实还留着。

### 5. 大量使用部分索引 —— 已确认（维持现状）

> **相谈内容**：变成了 PostgreSQL 前提的设计，有问题吗

**结论：维持现状设计。** 反馈的总评里，
「把交易列表的主轴索引做成 `WHERE deleted_at IS NULL` 的部分索引，
从索引侧吸收删除条数增长带来的影响」被列为做得好的点。

以 PostgreSQL 为前提这点在 1 章的共通方针里已明示，
Step 7 以后的基础设施也是按 PostgreSQL 搭，所以移植性的顾虑不会显现。
