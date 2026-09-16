# Step2：バックエンド2 —— 待办清单 & 交付物目录

> 中文参考版。正本：[../step2-checklist.md](../step2-checklist.md)（日文）

出处：[Step2: バックエンド2](https://devopscamp.reheartcloud.com/final-challenge/app/step2) / [設計サンプル](https://devopscamp.reheartcloud.com/final-challenge/app/design-samples)
选题：[通过解析支付邮件实现的自动记账服务](./theme-kakeibo.md)（Go）
设计根据：[step0-pipeline.md](./step0-pipeline.md)

> Step2 的目标：**不写代码**。作为 Step3（后端实现）的地基，
> 把 **API 仕様書 / DB 仕様書 / ER 図** 三件作为设计草案定下来。
> ※ 课题提交和メンタリング都是プレミアムプラン限定

---

## 全局（三块）

| # | 块 | 内容 | 交付物 |
|---|---|---|---|
| A | 讲座履修 | Go 讲座后半 7 章 | （无提交物・可跳过） |
| B | 课题实施 | 设计文档制作 | API 仕様書 / DB 仕様書 / ER 図 / 設計判断 / まとめ |
| C | メンタリング報告 | 口头报告 | 讲座回顾 / 提交课题说明 |

---

## A. 讲座履修（Go 讲座后半）

有同等知识可跳过。

- [ ] [Goでデータベースを操作しよう](https://devopscamp.reheartcloud.com/go/database-operation) —— Go 连 DB、CRUD 实现
- [ ] [GORM入門](https://devopscamp.reheartcloud.com/go/gorm-intro) —— 表和结构体的映射
- [ ] [net／http入門](https://devopscamp.reheartcloud.com/go/net-http-intro) —— 基本 endpoint
- [ ] [Gin入門](https://devopscamp.reheartcloud.com/go/gin-intro) —— 路由・请求处理
- [ ] [GoでREST APIを作ろう](https://devopscamp.reheartcloud.com/go/rest-api) —— Gin + GORM 做 CRUD 型 REST API
- [ ] [Goで静的解析をしよう](https://devopscamp.reheartcloud.com/go/static-analysis) —— 引入 golangci-lint
- [ ] [Goで自動テストをしよう](https://devopscamp.reheartcloud.com/go/automated-testing) —— testing 包

---

## B. 课题实施（交付物）

### 提交表单的 5 项

- [ ] **① API 仕様書** —— 附件 or 仓库/文档 URL
- [ ] **② データベース仕様書** —— 附件 or URL
- [ ] **③ ER 図** —— 用 draw.io 之类制作，图片附件 or URL
- [ ] **④ 設計判断** —— endpoint 粒度、表拆分、关系设计、索引选定等
- [ ] **⑤ まとめ** —— 辛苦的地方、纠结的判断、学到的东西

---

### ① API 仕様書 —— 目录

格式自由（Markdown / 表格 / Excel）。

```
1. API 概要
   - 是什么功能的 API
   - 对象数据是什么
2. 共通仕様
   - base URL
   - 日时格式
   - 分页规格
   - 响应格式（成功 / 错误的 JSON 信封）
3. 认证方式
4. 状态码一览
5. Endpoint 一览（No. / API ID / API名 / 方法 / 路径 / 认证）
6. 各 API 的详细规格  ← 全部 endpoint 必填
   6.x API-00X <API名>
       - 请求参数（参数名 / 类型 / 必需 / 说明）
       - 请求 body 的 JSON 例
       - 响应（字段名 / 类型 / 说明）
       - 响应的 JSON 例
       - 错误响应（状态码 / 错误码 / 错误内容）
```

#### 本选题的 Endpoint 一览（草案）

| No. | API ID | API名 | 方法 | Endpoint | 认证 |
|---|---|---|---|---|---|
| 1 | API-001 | 健康检查 | GET | `/health` | 不要 |
| 2 | API-002 | Google OAuth 开始 | GET | `/api/v1/auth/google` | 不要 |
| 3 | API-003 | Google OAuth 回调 | GET | `/api/v1/auth/google/callback` | 不要 |
| 4 | API-004 | 取当前用户 | GET | `/api/v1/auth/me` | 必要 |
| 5 | API-005 | 登出 | POST | `/api/v1/auth/logout` | 必要 |
| 6 | API-006 | 连携邮箱一览 | GET | `/api/v1/mail-accounts` | 必要 |
| 7 | API-007 | 解除连携 | DELETE | `/api/v1/mail-accounts/{id}` | 必要 |
| 8 | API-008 | 执行手动同步 | POST | `/api/v1/mail-accounts/{id}/sync` | 必要 |
| 9 | API-009 | 取同步历史 | GET | `/api/v1/sync-jobs` | 必要 |
| 10 | API-010 | 交易一览 | GET | `/api/v1/transactions` | 必要 |
| 11 | API-011 | 手动登记交易（现金等） | POST | `/api/v1/transactions` | 必要 |
| 12 | API-012 | 交易详情 | GET | `/api/v1/transactions/{id}` | 必要 |
| 13 | API-013 | 修正交易 | PUT | `/api/v1/transactions/{id}` | 必要 |
| 14 | API-014 | 删除交易 | DELETE | `/api/v1/transactions/{id}` | 必要 |
| 15 | API-015 | 手动合并（名寄せ） | POST | `/api/v1/transactions/{id}/merge` | 必要 |
| 16 | API-016 | 解除合并 | POST | `/api/v1/transactions/{id}/unmerge` | 必要 |
| 17 | API-017 | 未解析邮件一览 | GET | `/api/v1/inbox/unparsed` | 必要 |
| 18 | API-018 | 教示支付邮件判定 | POST | `/api/v1/inbox/{id}/classify` | 必要 |
| 19 | API-019 | 分类一览 | GET | `/api/v1/categories` | 必要 |
| 20 | API-020 | 创建分类 | POST | `/api/v1/categories` | 必要 |
| 21 | API-021 | 更新／删除分类 | PUT / DELETE | `/api/v1/categories/{id}` | 必要 |
| 22 | API-022 | 店铺（正规化名）一览・修正 | GET / PUT | `/api/v1/merchants[/{id}]` | 必要 |
| 23 | API-023 | 取月末汇总 | GET | `/api/v1/summaries/monthly?month=YYYY-MM` | 必要 |
| 24 | API-024 | 订阅一览 | GET | `/api/v1/subscriptions` | 必要 |
| 25 | API-025 | 更新订阅状态（已解约等） | PUT | `/api/v1/subscriptions/{id}` | 必要 |
| 26 | API-026 | 预算取得・设置 | GET / PUT | `/api/v1/budgets` | 必要 |
| 27 | API-027 | 通知设置取得・更新 | GET / PUT | `/api/v1/notification-settings` | 必要 |
| 28 | API-028 | 通知历史一览 | GET | `/api/v1/notifications` | 必要 |

---

### ② データベース仕様書 —— 目录

```
1. 数据库概要
2. 表一览（No. / 表名 / 概要）
3. 各表的详细规格  ← 全部表必填
   3.x <表名>
       - 表概要
       - 列定义（列名 / 类型 / 必需 / 约束(PK/FK/UNIQUE) / 说明）
       - 约束（复合 UNIQUE 等）
       - 索引（索引名 / 对象列 / 用途）
```

#### 本选题的表一览（草案）

**核心（Step3 必须实现）**

| No. | 表名 | 概要 |
|---|---|---|
| 1 | `users` | app 用户・认证信息 |
| 2 | `mail_accounts` | 连携的 Gmail 账号和 token |
| 3 | `sync_jobs` | 取邮件 job 的执行历史 |
| 4 | `email_messages` | 取到的邮件原始数据（用 Gmail message ID 保证幂等） |
| 5 | `parser_templates` | 发件人＋标题模式 → 解析规则 |
| 6 | `payment_events` | 从一封邮件抽出的支付事件（名寄せ 前） |
| 7 | `transactions` | 名寄せ 后的实际交易（一次购买 = 一行） |
| 8 | `transaction_links` | `payment_events` ↔ `transactions` 的对应 |
| 9 | `merchants` | 正规化店铺主数据（支店单位 + `brand_name` + 位置信息） |
| 10 | `merchant_aliases` | 写法差异 → `merchants` 的映射 |
| 11 | `payment_methods` | 支付手段（卡 / QR / 银行 / 现金） |
| 12 | `categories` | 分类主数据 |
| 13 | `category_rules` | 品牌・店铺种别・关键词 → 分类规则 |
| 14 | `notification_settings` | 通知渠道・最低金额・静音时段 |
| 15 | `notifications` | 通知发送历史（即时 / 预算超支 / 月末汇总共用） |

**扩展（有余力再做）**

| No. | 表名 | 概要 |
|---|---|---|
| 16 | `subscriptions` | 检测到的订阅 |
| 17 | `budgets` | 预算设置（预算超支通知用） |
| 18 | `monthly_summaries` | 月末汇总的生成・配信记录 |

---

### ③ ER 図

- [ ] 用 draw.io（[app.diagrams.net](https://app.diagrams.net/)）制作
- [ ] 用记号表达 1对1 / 1对多 / 多对多
- [ ] 导出成图片提交
- 特别想标明的关系：
  - `email_messages` 1 — N `payment_events`
  - `payment_events` N — N `transactions`（经由 `transaction_links`）← **本选题的核心**
  - `merchants` 1 — N `merchant_aliases`
  - `merchants` 1 — N `transactions`
  - `categories` 1 — N `transactions`
  - `payment_events` 1 — N `notifications`（即时通知）
  - `users` 1 — N 所有用户所属数据

---

### ④ 設計判断（提交项目・メンタリング 也要说明）

要写的论点：

- [ ] **`email_messages` / `payment_events` / `transactions` 三层分割** —— 保留原始数据，改了 名寄せ 逻辑也能全量重算
- [ ] **`email_messages` 1 — N `payment_events`** —— 银行汇总通知等，一封会出多条事件
- [ ] **用中间表（`transaction_links`）表达 名寄せ 的理由** —— 处理电商侧和卡侧的 N:N，并支持解除合并
- [ ] **把 `payment_events.event_type` 作为核心列的理由** —— 金额可信度判定（P5）和即时通知对象筛选（P4.5）两处都用
- [ ] **`merchants` 用支店单位 + `brand_name` 列的理由** —— 地图按支店，汇总和分类规则按品牌。没拆 `brands` 表是范围判断
- [ ] **金额用 `bigint`（最小单位）保存** —— 避免浮点误差
- [ ] **Gmail message ID 加 UNIQUE 约束** —— 保证重取时的幂等性
- [ ] **防止通知重复发送** —— 靠 `payment_events.notified_at` 和 `notifications` 保证幂等
- [ ] **Endpoint 粒度** —— 为什么把 `merge` / `unmerge` / `classify` 做成专用 endpoint 而不是通用 PUT
- [ ] **索引选定** —— `(user_id, occurred_at)` 复合、`(user_id, category_id, occurred_at)`、`merchant_aliases.alias` 等
- [ ] **多租户设计** —— 运营是单用户，为什么所有业务表还是带 `user_id`
- [ ] **订阅用表存还是每次检测**
- [ ] **认证有两种** —— app 自身的认证（Bearer/JWT）和 Gmail 连携的 Google OAuth 分离

### ⑤ まとめ（提交项目）

- [ ] 设计中辛苦的地方
- [ ] 纠结的判断
- [ ] 学到的东西

---

## C. メンタリング報告

### C-1. 讲座回顾

下面 5 个要素 × 3 个观点（**概要 / 能做什么 / 使用时要注意什么**）来报告。

- [ ] 数据库操作
- [ ] REST API 框架（net/http・Gin）
- [ ] ORM（GORM）
- [ ] 静的解析（golangci-lint）
- [ ] 自动测试（testing）

### C-2. 提交课题的说明

- [ ] API 和数据库的设计判断（endpoint 设计・表设计・关系・索引）
- [ ] 设计中纠结的点、判断上犹豫的地方
