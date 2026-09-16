# API 规格书

> 中文参考版。正本：[../step2-api-spec.md](../step2-api-spec.md)（日文，提交用）
> 路径、字段名、错误码保持原样不翻译。

服务名：決済メール解析による自動家計簿サービス
语言 / 框架：Go / net/http（标准库）+ GORM
对应规格书：[step2-db-spec.md](./step2-db-spec.md) / [step2-er.md](./step2-er.md)

> 已反映 メンタリング 的反馈。全部 28 个 endpoint 的详细规格均已记载。
> 行为同形的 endpoint，其响应结构等用
> **「与 API-0XX 同形式」的引用写法简化**（反馈中许可的做法）。

> **框架变更（Step 3 反映）**
> 原本打算用 Gin，但 Step 3 的实施要件指定了「Step 1・2 学到的 Web 框架
> （FastAPI 或 net/http）」，因此**改为标准库 `net/http`**。
> Go 1.22 以后的 `http.ServeMux` 支持 `"GET /api/v1/transactions/{id}"` 这种
> 方法 + 路径的模式匹配，本规格的路由用标准库即可满足。

---

## 1. API 概要

提供从支付通知邮件自动抽出的支出数据的 REST API。

处理的主要数据有四类：

| 数据 | 说明 |
|---|---|
| 交易（transaction） | 名寄せ 后的实际支出一件 |
| 支付事件（payment_event） | 从邮件抽出的原始支付一件。是交易的构成要素 |
| 店铺（merchant） | 正规化后的店铺，带地址和座标 |
| 通知（notification） | 即时通知・预算超支通知・月末汇总的发送历史 |

取邮件、解析、名寄せ、分类由后台批处理完成，
本 API 负责**查看和订正其结果**，以及管理设置。

---

## 2. 共通规格

### base URL

```
http://<your-host>/api/v1
```

### 日时格式

ISO 8601（例：`2026-09-09T12:34:56+09:00`）。**必须带时区偏移。**

### 金额

一律用**最小单位的整数**表示（日元 1 = 1 円）。不用小数。

```json
{ "amount_minor": 580, "currency": "JPY" }
```

### 分页

一览系的 endpoint 接受以下 query 参数。

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| limit | integer | 20 | 取得件数（最大 100） |
| offset | integer | 0 | 取得起始位置 |

响应的 `meta` 里包含总件数。

### 响应格式

成功时：

```json
{ "data": { }, "meta": { } }
```

错误时：

```json
{ "error": { "code": "ERROR_CODE", "message": "错误消息" } }
```

---

## 3. 认证方式

本服务的认证**分两层**。要明确分离，不能混为一谈。

| 层 | 对象 | 方式 |
|---|---|---|
| **app 认证** | 调用本 API 的权利 | Google OAuth 登录后发行的 **Bearer token（JWT / HS256，有效期 24 小时）** |
| **邮箱连携认证** | 读用户邮件的权利 | Google OAuth（readonly scope）等。取得的认证信息加密后存入 `mail_accounts`，**绝不用于 API 请求** |

除健康检查和 OAuth 的开始・回调外，所有请求都需要 Bearer token。

```
Authorization: Bearer <access_token>
```

---

## 4. 状态码一览

| 状态码 | 含义 |
|---|---|
| 200 | 请求成功 |
| 201 | 资源创建成功 |
| 202 | 已受理异步处理（手动同步等） |
| 204 | 成功，无返回内容 |
| 400 | 校验错误 / 业务逻辑错误 |
| 401 | 认证错误（token 不正・过期） |
| 403 | 权限错误（访问他人的资源） |
| 404 | 资源不存在 |
| 409 | 冲突（重复登记、状态不一致） |
| 500 | 服务器内部错误 |
| 503 | 服务不可用（健康检查中无法连通 DB 时） |

---

## 5. Endpoint 一览

| No. | API ID | API 名 | 方法 | Endpoint | 认证 | 实装阶段 |
|---|---|---|---|---|---|---|
| 1 | API-001 | 健康检查 | GET | `/health` | 不要 | **Step 3（核心）** |
| 2 | API-002 | Google OAuth 开始 | GET | `/api/v1/auth/google` | 不要 | **Step 3（核心）** |
| 3 | API-003 | Google OAuth 回调 | GET | `/api/v1/auth/google/callback` | 不要 | **Step 3（核心）** |
| 4 | API-004 | 取当前用户 | GET | `/api/v1/auth/me` | 必要 | **Step 3（核心）** |
| 5 | API-005 | 登出 | POST | `/api/v1/auth/logout` | 必要 | **Step 3（核心）** |
| 6 | API-006 | 连携邮箱一览 | GET | `/api/v1/mail-accounts` | 必要 | Step 8 以后（扩展） |
| 7 | API-007 | 解除连携 | DELETE | `/api/v1/mail-accounts/{id}` | 必要 | Step 8 以后（扩展） |
| 8 | API-008 | 执行手动同步 | POST | `/api/v1/mail-accounts/{id}/sync` | 必要 | Step 8 以后（扩展） |
| 9 | API-009 | 取同步历史 | GET | `/api/v1/sync-jobs` | 必要 | Step 8 以后（扩展） |
| 10 | API-010 | 交易一览 | GET | `/api/v1/transactions` | 必要 | **Step 3（核心）** |
| 11 | API-011 | 手动登记交易 | POST | `/api/v1/transactions` | 必要 | **Step 3（核心）** |
| 12 | API-012 | 交易详情 | GET | `/api/v1/transactions/{id}` | 必要 | **Step 3（核心）** |
| 13 | API-013 | 修正交易 | PUT | `/api/v1/transactions/{id}` | 必要 | **Step 3（核心）**※1 |
| 14 | API-014 | 删除交易 | DELETE | `/api/v1/transactions/{id}` | 必要 | **Step 3（核心）** |
| 15 | API-015 | 合并交易（名寄せ） | POST | `/api/v1/transactions/merge` | 必要 | Step 8 以后（扩展） |
| 16 | API-016 | 解除合并 | POST | `/api/v1/transactions/{id}/unmerge` | 必要 | Step 8 以后（扩展） |
| 17 | API-017 | 未解析邮件一览 | GET | `/api/v1/inbox/unparsed` | 必要 | Step 8 以后（扩展） |
| 18 | API-018 | 教示支付邮件判定 | POST | `/api/v1/inbox/{id}/classify` | 必要 | Step 8 以后（扩展） |
| 19 | API-019 | 分类一览 | GET | `/api/v1/categories` | 必要 | **Step 3（核心）** |
| 20 | API-020 | 创建分类 | POST | `/api/v1/categories` | 必要 | **Step 3（核心）** |
| 21 | API-021 | 更新 / 删除分类 | PUT / DELETE | `/api/v1/categories/{id}` | 必要 | **Step 3（核心）** |
| 22 | API-022 | 店铺一览 / 修正 | GET / PUT | `/api/v1/merchants[/{id}]` | 必要 | Step 8 以后（扩展） |
| 23 | API-023 | 取月末汇总 | GET | `/api/v1/summaries/monthly` | 必要 | Step 8 以后（扩展） |
| 24 | API-024 | 订阅一览 | GET | `/api/v1/subscriptions` | 必要 | Step 8 以后（扩展） |
| 25 | API-025 | 更新订阅状态 | PUT | `/api/v1/subscriptions/{id}` | 必要 | Step 8 以后（扩展） |
| 26 | API-026 | 预算取得 / 设置 | GET / PUT | `/api/v1/budgets` | 必要 | Step 8 以后（扩展） |
| 27 | API-027 | 通知设置取得 / 更新 | GET / PUT | `/api/v1/notification-settings` | 必要 | Step 8 以后（扩展） |
| 28 | API-028 | 通知历史一览 | GET | `/api/v1/notifications` | 必要 | Step 8 以后（扩展） |

※1 API-013 中，**「作为副作用学习分类规则」这部分处理挪到 Step 8 以后**。
学习需要 `category_rules` 表，但该表不在 Step 3 的核心 6 表内。
Step 3 只做交易的更新，不实装对 `category_rules` 的写入。

### 5.1 实装阶段的划分思路

Step 2 的反馈中确认：在 Step 3 的期间内实装全部 endpoint 在工作量上很吃紧，
为了不阻塞后续的 Step 4〜6（前端）和 Step 7 以后（基础设施），
**先划出最低限度线**。本规格书用上表的 `实装阶段` 列明示这条线。

| 阶段 | 内容 | 件数 |
|---|---|---|
| **Step 3（核心）** | 能作为手动输入的记账本成立，且 Step 7 以后基础设施构建所需的「能跑起来的后端」成立的最小范围 | 13 行 / 14 操作 |
| Step 8 以后（扩展） | 邮件连携・解析批处理・名寄せ（P1〜P5），通知・订阅・预算・月末汇总（P4.5 / P7 / P8） | 15 行 |

核心范围的选定依据如下。

| API ID | 纳入核心的理由 |
|---|---|
| API-001 | Step 7 以后基础设施的健康检查要用 |
| API-002 / API-003 | 无法登录就发不出 Bearer token，其余所有 endpoint 都无法验证 |
| API-004 / API-005 | 认证相关的最小集合 |
| API-010〜014 | 记账本的核心。交易 CRUD 齐了前端就能做动作验证 |
| API-019〜021 | 交易分类所需的主数据；也是最先把 CRUD 形状定型下来的对象 |

> **Step 3 的实装对象 =「`实装阶段` 列为 `Step 3（核心）` 的全部 endpoint」。**
> Step 8 以后（扩展）的 endpoint 在本规格书中同样把设计定稿，等核心跑通后分阶段追加。

> **删除系 endpoint 的行为**
>
> `DELETE` 表达的是「从客户端视角这个资源消失」，**和服务器侧的物理删除不是一回事**。
> 本服务里这样处理：
>
> | Endpoint | 服务器侧的处理 | 状态码 |
> |---|---|---|
> | API-007 解除连携 | 更新 `mail_accounts.status = 'disabled'` 并销毁 `credential_encrypted`。**不删行**（否则 CASCADE 会把邮件原始数据一起删掉） | 204 |
> | API-014 删除交易 | `transactions.deleted_at = now()` 的**逻辑删除**。物理删除的话支付事件会回到未名寄せ 状态，下次批处理又生成一模一样的交易 | 204 |
> | API-021 删除分类 | 物理删除。引用它的交易的 `category_id` 靠 FK 变成 `NULL` | 204 |
>
> 详见 [step2-db-spec.md](./step2-db-spec.md) 的 3.2 / 3.7 删除方针。

**API 的类型**（详细规格见 6 章，共 28 件。下面是各形态的代表）

| 类型 | 代表 | 同形的 endpoint |
|---|---|---|
| 需要认证的最小取得系 | API-004 | API-027（取单条设置） |
| 一览系（分页 + 筛选） | API-010 | API-009 / API-017 / API-028 |
| 登记系（校验 + 201） | API-011 | API-020 |
| 更新系（**副作用是学习分类规则**） | API-013 | API-021 PUT / API-022 PUT / API-025 |
| 装不进 CRUD 的动作型 | API-015 | API-008 / API-016 / API-018 |
| 聚合系（不是直接返回 DB 记录） | API-023 | — |
| 删除系（服务器侧处理因 endpoint 而异） | API-014 | API-007 / API-021 DELETE |

---

## 6. 各 API 详细规格

> 除特别说明外，所有 endpoint 都可能返回 `401 INVALID_TOKEN`（token 不正）和
> `401 TOKEN_EXPIRED`（过期）。以下的错误表**省略这两个**。
> 指定他人的资源时一律返回 `403 FORBIDDEN`（不用 `404`。比起不让人推测资源是否存在，
> **明示「这不是你的数据」**在 UI 上更好处理）。

### API-001 健康检查

死活监视用。**无需认证**。由 Step 7 以后的基础设施（负载均衡、容器健康检查）调用。

```
GET /health
```

**请求参数**：无

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data.status | string | 固定 `ok` |
| data.version | string | 应用版本（构建时嵌入） |
| data.database | string | DB 连通结果（`ok`） |
| data.checked_at | string | 确认时刻（ISO 8601） |

**响应示例**

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

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 503 | DATABASE_UNAVAILABLE | 无法连通 DB |

> **把 DB 连通性纳入 `200` 条件的理由**：进程活着但连不上 DB 的话请求也处理不了。
> 健康检查要是一直返回 `200`，流量就会持续打到坏掉的实例上。

---

### API-002 Google OAuth 开始

重定向到 Google 的授权画面。**无需认证**。

```
GET /api/v1/auth/google
```

**请求参数（query）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| redirect_uri | string | - | 认证完成后跳回的前端 URL（不指定则用默认值） |

**响应（302 Found）**

不返回 JSON，用 `Location` 头重定向到 Google 的授权 endpoint。
发行 CSRF 对策用的 `state`，与 `redirect_uri` 一起在服务器侧临时保存（有效期 10 分钟）。

| 头名 | 说明 |
|---|---|
| Location | `https://accounts.google.com/o/oauth2/v2/auth?...&state=<发行的 state>` |

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | INVALID_REDIRECT_URI | `redirect_uri` 不在白名单里 |

> **要求的 scope**：登录只要 `openid` / `email` / `profile`。
> 读邮件用的 `gmail.readonly` **此时不要求**。
> 按 3 章所述认证分两层，邮件连携走用户另行明确授权的导线。

---

### API-003 Google OAuth 回调

接收 Google 的重定向，确定用户并发行 Bearer token。**无需认证**。
`users` 里没有对应记录就创建（用 `google_sub` 匹配）。

```
GET /api/v1/auth/google/callback
```

**请求参数（query）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| code | string | ○ | Google 发行的授权码 |
| state | string | ○ | API-002 发行的 `state` |

**响应（302 Found / 200 OK）**

| 条件 | 行为 |
|---|---|
| 默认（浏览器跳转） | 302 到 API-002 保存的 `redirect_uri`。token 放在 URL fragment（`#access_token=...`） |
| `Accept: application/json` | 200 返回下面的 JSON。**用于 curl 做动作确认** |

| 字段名 | 类型 | 说明 |
|---|---|---|
| data.access_token | string | Bearer token（JWT / HS256） |
| data.token_type | string | 固定 `Bearer` |
| data.expires_in | integer | 有效期（秒。86400） |
| data.user.id | integer | 用户 ID |
| data.user.email | string | 邮箱地址 |
| meta.is_new_user | boolean | 本次登录是否新建了 `users` |

**响应示例**

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

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | INVALID_STATE | `state` 未发行・过期・不一致（疑似 CSRF） |
| 400 | OAUTH_EXCHANGE_FAILED | 授权码换 token 失败 |
| 403 | ACCESS_DENIED | 用户在 Google 同意画面点了拒绝 |

> **新用户创建时的初始数据**：在 `categories` 里建 `is_system = true` 的初始分类
> （含 `未分類`），在 `payment_methods` 里建 `現金`。
> 因为一条分类都没有的话就登记不了交易。

---

### API-004 取当前用户

```
GET /api/v1/auth/me
```

**请求参数**：无

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data.id | integer | 用户 ID |
| data.email | string | 邮箱地址 |
| data.display_name | string / null | 显示名 |
| data.timezone | string | 汇总的基准时区 |
| data.mail_accounts_count | integer | 已连携的邮箱数 |
| data.needs_reauth | boolean | 是否有需要重新认证的连携 |

**响应例**

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

**错误响应**

只有共通的 `401`（见开头注记）。

---

### API-005 登出

```
POST /api/v1/auth/logout
```

**请求参数**：无

**响应（204 No Content）**

无 body。

**错误响应**

只有共通的 `401`。

> **服务器侧做了什么**
> 本 API 采用**无状态的 JWT**，服务器不持有失效列表。
> 登出的实体是「客户端丢弃 token」，本 endpoint **只是受理这个完成并返回 `204`**。
>
> 代价是泄露的 token 最长 24 小时（`expires_in`）内仍然有效。
> 需要即时失效时，改为在 JWT 里带 `jti` 并加一张失效列表表来应对
> （有效期是 24 小时，所以列表也只需保留 24 小时）。
> 现阶段判断为：这张表加上每次请求都要查一次的成本，不值当。

---

### API-006 连携邮箱一览

```
GET /api/v1/mail-accounts
```

**请求参数**：无

> 每个用户预计只有几条，**不设分页**。

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data[].id | integer | 连携 ID |
| data[].email_address | string | 连携的邮箱地址 |
| data[].provider | string | `gmail_api` / `gas` / `imap` |
| data[].status | string | `active` / `reauth_required` / `disabled` |
| data[].last_synced_at | string / null | 最终同步时刻。**`status = active` 却很旧就说明批处理停了** |
| data[].credential_expires_at | string / null | 认证信息的有效期 |
| data[].backfilled_until | string / null | 过去邮件已取完的起点 |
| meta.total | integer | 件数 |

**响应示例**

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

**错误响应**

只有共通的 `401`。

> `credential_encrypted` **任何情况下都不放进响应**。

---

### API-007 解除连携

**不删行。** 更新 `status = 'disabled'` 并把 `credential_encrypted` 清空。
`email_messages` 以后的数据全部保留（见 DB 规格书 3.2 的删除方针）。

```
DELETE /api/v1/mail-accounts/{id}
```

**请求参数（path）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| id | integer | ○ | 连携 ID |

**响应（204 No Content）**

无 body。

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 403 | FORBIDDEN | 他人的连携 |
| 404 | MAIL_ACCOUNT_NOT_FOUND | 连携不存在 |
| 409 | ALREADY_DISABLED | 已经解除过了 |

---

### API-008 执行手动同步

即时执行取邮件批处理。**异步**，所以用 `202` 只返回受理。
进度用 API-009 查。

```
POST /api/v1/mail-accounts/{id}/sync
```

**请求参数（path）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| id | integer | ○ | 连携 ID |

**请求参数（body）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| mode | string | - | `incremental`（默认，增量取得） / `backfill`（回溯取历史邮件） |

**请求示例**

```json
{ "mode": "incremental" }
```

**响应（202 Accepted）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data.sync_job_id | integer | 创建的同步 job ID（用 API-009 查询） |
| data.status | string | 固定 `running` |
| data.trigger | string | 固定 `manual` |
| data.started_at | string | 开始时刻 |

**响应示例**

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

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 403 | FORBIDDEN | 他人的连携 |
| 404 | MAIL_ACCOUNT_NOT_FOUND | 连携不存在 |
| 409 | SYNC_ALREADY_RUNNING | 同一连携的同步正在执行中 |
| 409 | ACCOUNT_DISABLED | 连携已解除 |
| 409 | REAUTH_REQUIRED | 认证信息已失效，需要重新认证 |

> **返回 `202` 的理由**：取得件数多的时候要几十秒，不等同步完成。
> 先在 `sync_jobs` 建行再启动 job，把该 ID 返回，客户端就能追踪进度。

---

### API-009 取同步历史

一览系，参数和分页的处理**与 API-010 同形式**。

```
GET /api/v1/sync-jobs
```

**请求参数（query）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| mail_account_id | integer | - | 按连携筛选 |
| status | string | - | `running` / `success` / `partial` / `failed` |
| limit | integer | - | 取得件数（默认 20，最大 100） |
| offset | integer | - | 取得起始位置（默认 0） |

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data[].id | integer | job ID |
| data[].mail_account_id | integer | 对象连携 |
| data[].trigger | string | `scheduled` / `manual` / `backfill` / `push` |
| data[].started_at | string | 开始时刻 |
| data[].finished_at | string / null | 结束时刻（执行中为 `null`） |
| data[].status | string | `running` / `success` / `partial` / `failed` |
| data[].fetched_count | integer | 取得的邮件数 |
| data[].parsed_count | integer | 抽出成功数 |
| data[].failed_count | integer | 抽出失败数 |
| data[].error_message | string / null | 错误内容 |
| meta.total | integer | 符合条件的总件数 |

**响应示例**

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

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | LIMIT_TOO_LARGE | `limit` 超过 100 |

> **这个一览里不会出现「无新邮件」的执行。** 按 DB 规格书 3.3 的方针，
> `fetched_count = 0` 且 `status = success` 的执行不记录进 `sync_jobs`。
> 批处理是否在跑要看 API-006 的 `last_synced_at`。

---

### API-010 交易一览

```
GET /api/v1/transactions
```

**请求参数（query）**

| 参数名 | 类型 | 必需 | 说明 |
|---|---|---|---|
| from | string | - | 对象期间的开始日（`YYYY-MM-DD`，JST 基准） |
| to | string | - | 对象期间的结束日（`YYYY-MM-DD`，JST 基准） |
| category_id | integer | - | 按分类筛选 |
| payment_method_id | integer | - | 按支付手段筛选 |
| merchant_id | integer | - | 按店铺筛选 |
| needs_review | boolean | - | `true` 时只返回带疑似重复标记的交易 |
| q | string | - | 店铺名・备注的部分匹配检索 |
| limit | integer | - | 取得件数（默认 20，最大 100） |
| offset | integer | - | 取得起始位置（默认 0） |

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data[].id | integer | 交易 ID |
| data[].occurred_at | string | 支付时刻（ISO 8601） |
| data[].amount_minor | integer | 金额（最小单位） |
| data[].currency | string | 货币代码 |
| data[].merchant | object / null | 店铺信息 |
| data[].merchant.id | integer | 店铺 ID |
| data[].merchant.name | string | 店铺名 |
| data[].merchant.address | string / null | 地址（已地理编码时） |
| data[].category | object / null | 分类信息 |
| data[].payment_method | object / null | 支付手段 |
| data[].source | string | `email` / `manual` |
| data[].status | string | `pending` / `confirmed` |
| data[].is_possible_duplicate | boolean | 是否在等疑似重复的确认 |
| data[].event_count | integer | 挂着的支付事件数 |
| meta.total | integer | 符合条件的总件数 |
| meta.total_amount_minor | integer | 符合条件的交易合计金额 |

**响应例**

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

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | INVALID_DATE_RANGE | `from` 晚于 `to` |
| 400 | LIMIT_TOO_LARGE | `limit` 超过 100 |

---

### API-011 手动登记交易

登记现金等不会产生通知邮件的支付。
不经过支付事件，直接在 `transactions` 创建（`source = manual`）。

```
POST /api/v1/transactions
```

**请求参数（body）**

| 参数名 | 类型 | 必需 | 说明 |
|---|---|---|---|
| occurred_at | string | ○ | 支付时刻（ISO 8601） |
| amount_minor | integer | ○ | 金额（最小单位，1 以上） |
| currency | string | - | 货币代码（默认 `JPY`） |
| merchant_name | string | - | 店铺名（不存在则新建） |
| category_id | integer | - | 分类 ID |
| payment_method_id | integer | - | 支付手段 ID |
| note | string | - | 备注（500 字以内） |

**请求例**

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

**响应（201 Created）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data.id | integer | 创建的交易 ID |
| data.occurred_at | string | 支付时刻 |
| data.amount_minor | integer | 金额 |
| data.merchant | object / null | 店铺（含新建的情况） |
| data.category | object / null | 分类 |
| data.source | string | 恒为 `manual` |
| data.is_user_edited | boolean | 恒为 `true`（不参与重算） |

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | 必需项缺失、格式不正 |
| 400 | INVALID_AMOUNT | 金额 0 以下 |
| 404 | CATEGORY_NOT_FOUND | 指定的分类不存在 |
| 404 | PAYMENT_METHOD_NOT_FOUND | 指定的支付手段不存在 |

---

### API-012 交易详情

返回一览（API-010）里没有的**备注・日元换算额・关联的支付事件**。

```
GET /api/v1/transactions/{id}
```

**请求参数（path）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| id | integer | ○ | 交易 ID |

**响应（200 OK）**

`data` 的基本结构**与 API-010 的 `data[]` 同形式**。另外还有：

| 字段名 | 类型 | 说明 |
|---|---|---|
| data.amount_jpy_minor | integer | 日元换算额（聚合用这个值） |
| data.category_source | string | `rule` / `place_type` / `user` / `default` |
| data.is_user_edited | boolean | 用户是否手改过（`true` 不参与重算） |
| data.note | string / null | 备注 |
| data.payment_events[] | array | 关联的支付事件。**手动登记（`source = manual`）时为空数组** |
| data.payment_events[].id | integer | 支付事件 ID |
| data.payment_events[].event_type | string | `order_confirm` / `usage_notice` / `settlement` / `receipt` / `bank_debit` |
| data.payment_events[].occurred_at | string | 支付时刻 |
| data.payment_events[].amount_minor | integer | 金额 |
| data.payment_events[].raw_merchant_text | string / null | 正规化前的原始店铺名 |
| data.payment_events[].link_type | string / null | `auto` / `manual`（名寄せ 的路径） |
| data.payment_events[].link_score | number / null | 自动名寄せ 时的分数 |
| data.payment_events[].email_message_id | integer | 抽出源邮件的 ID |

**响应示例**

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

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 403 | FORBIDDEN | 他人的交易 |
| 404 | TRANSACTION_NOT_FOUND | 交易不存在，或已逻辑删除 |

> 已逻辑删除（`deleted_at IS NOT NULL`）的交易返回 `404`。
> 从客户端视角它已经被删了，所以当作不存在处理。

---

### API-013 修正交易

**重点在于它有副作用。** 变更分类时，会对该店铺的品牌 upsert 一条 `category_rules`，
让之后同品牌的交易自动分对。
另外，修正过的交易 `is_user_edited = true`，从此不参与 名寄せ 的重算。

> **Step 3 的实装范围**（5.1 的 ※1）
> **分类规则的学习挪到 Step 8 以后。** 学习需要 `category_rules` 表，
> 但该表不在 Step 3 的核心 6 表内。
> Step 3 只做交易的更新，`learn_category` 接收但忽略，
> `meta.learned_rule` 固定返回 `null`，`meta.affected_future` 固定返回 `false`。
> **请求・响应的形状不变**，所以 Step 8 补上学习时前端不用改。

```
PUT /api/v1/transactions/{id}
```

**请求参数（path）**

| 参数名 | 类型 | 必需 | 说明 |
|---|---|---|---|
| id | integer | ○ | 交易 ID |

**请求参数（body）**

| 参数名 | 类型 | 必需 | 说明 |
|---|---|---|---|
| occurred_at | string | - | 支付时刻 |
| amount_minor | integer | - | 金额（最小单位） |
| merchant_id | integer | - | 店铺 ID |
| category_id | integer | - | 分类 ID |
| payment_method_id | integer | - | 支付手段 ID |
| note | string | - | 备注 |
| learn_category | boolean | - | `true`（默认）时学习分类规则。`false` 只改这一笔 |

**请求例**

```json
{
  "category_id": 7,
  "learn_category": true
}
```

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data.id | integer | 交易 ID |
| data.category | object | 更新后的分类 |
| data.category_source | string | 恒为 `user` |
| data.is_user_edited | boolean | 恒为 `true` |
| meta.learned_rule | object / null | 学习到的分类规则 |
| meta.learned_rule.match_type | string | `brand` / `merchant` |
| meta.learned_rule.match_value | string | 匹配值 |
| meta.affected_future | boolean | 是否适用于今后的交易 |

**响应例**

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

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | 格式不正 |
| 403 | FORBIDDEN | 他人的交易 |
| 404 | TRANSACTION_NOT_FOUND | 交易不存在 |

---

### API-014 删除交易

**逻辑删除**（`deleted_at = now()`）。不物理删除的理由：支付事件会回到「未名寄せ」，
下次名寄せ 批处理会**把同一笔交易再生成一遍**（见 DB 规格书 3.7 的删除方针）。

```
DELETE /api/v1/transactions/{id}
```

**请求参数（path）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| id | integer | ○ | 交易 ID |

**响应（204 No Content）**

无 body。

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 403 | FORBIDDEN | 他人的交易 |
| 404 | TRANSACTION_NOT_FOUND | 交易不存在 |
| 409 | ALREADY_MERGED | 它是合并的来源，不能单独删（先用 API-016 解除） |

> **删除后的行为**：之后从 API-010 / API-012 就看不到了（查询系一律用 `deleted_at IS NULL` 筛）。
> 关联的 `payment_events.transaction_id` **原样保留**。
> 这样名寄せ 批处理会判定为「已处理」，删掉的交易不会复活。

---

### API-015 合并交易（名寄せ）

把作为疑似重复提示的两笔交易，作为同一次购买合并成一笔。
支付事件挂到合并目标上，**合并来源做逻辑删除**
（`deleted_at = now()`、`merged_into_id = 合并目标的 ID`）。
采用哪边的金额，按 `event_type` 的可信度决定。

**不物理删除合并来源，是为了让 API-016 `unmerge` 能复原。**
物理删除的话，合并来源上的备注和分类等手动修改就没了，「撤销合并」不成立。

```
POST /api/v1/transactions/merge
```

> **路径设计（メンタリング 的确认结果）**
> 原本是 `POST /transactions/{id}/merge`，**改成两个 ID 都在 body 里指定**。
>
> | 理由 | 内容 |
> |---|---|
> | 操作对象 | 合并不是对某一个资源的操作，而是**把多笔交易束成一笔、面向整个集合的操作**，没必然理由要在 URL 里放其中一方的 ID |
> | 可读性 | `{id}/merge` 从 URL 读不出是「把对象 merge 掉」还是「把对象和什么 merge」，而且合并源还是合并目标放 `{id}` 各实现容易摇摆 |
> | 扩展性 | 将来扩展到 3 笔以上合并时，body 指定更容易留余地 |
>
> 「哪边留下」由 `data.id` 和 `meta.merged_transaction_id` 明示，不放 URL 也不损失信息。

**请求参数（body）**

| 参数名 | 类型 | 必需 | 说明 |
|---|---|---|---|
| target_transaction_id | integer | ○ | 合并目标的交易 ID（**这笔留下**） |
| source_transaction_id | integer | ○ | 合并来源的交易 ID（**这笔会被逻辑删除**） |

**请求例**

```json
{
  "target_transaction_id": 1024,
  "source_transaction_id": 1025
}
```

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data.id | integer | 合并后的交易 ID |
| data.amount_minor | integer | 采用的金额 |
| data.event_count | integer | 合并后挂着的支付事件数 |
| data.is_possible_duplicate | boolean | 恒为 `false` |
| meta.merged_transaction_id | integer | 被逻辑删除的合并来源交易 ID（传给 `unmerge` 用） |
| meta.amount_source_event_type | string | 金额的采用来源事件种别 |

**响应例**

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

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | `target_transaction_id` / `source_transaction_id` 缺失或格式不正 |
| 400 | SAME_TRANSACTION | 合并来源和目标相同 |
| 403 | FORBIDDEN | 他人的交易 |
| 404 | TRANSACTION_NOT_FOUND | 有一方不存在 |
| 409 | ALREADY_MERGED | 合并来源已被合并进别的交易（`merged_into_id` 已有值） |
| 409 | ALREADY_DELETED | 有一方已被用户删除（`deleted_at` 已有值，且不是合并造成的） |

> 因为用逻辑删除把合并来源留下了，**这两个 `409` 才能从状态判断出来**。
> 改成物理删除的话行就没了，只能回 `404`，
> 「已经合并过了」和「本来就不存在」就区分不了。

---

### API-016 解除合并

把 API-015 合并掉的交易还原。
合并来源的 `deleted_at` 和 `merged_into_id` 置回 NULL，支付事件挂回合并来源。

**这是对某一笔已合并交易的操作**，所以与 API-015 不同，路径带 `{id}`。

```
POST /api/v1/transactions/{id}/unmerge
```

**请求参数（path）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| id | integer | ○ | **合并目标**的交易 ID |

**请求参数（body）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| merged_transaction_id | integer | ○ | 要还原的合并来源交易 ID（API-015 的 `meta.merged_transaction_id`） |

**请求例**

```json
{ "merged_transaction_id": 1025 }
```

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data.id | integer | 合并目标的交易 ID |
| data.amount_minor | integer | 还原后的金额 |
| data.event_count | integer | 留在合并目标上的支付事件数 |
| meta.restored_transaction_id | integer | 被还原的交易 ID |
| meta.restored_event_count | integer | 挂回合并来源的支付事件数 |

**响应例**

```json
{
  "data": { "id": 1024, "amount_minor": 1500, "event_count": 1 },
  "meta": { "restored_transaction_id": 1025, "restored_event_count": 1 }
}
```

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | `merged_transaction_id` 缺失或格式不正 |
| 403 | FORBIDDEN | 他人的交易 |
| 404 | TRANSACTION_NOT_FOUND | 有一方不存在 |
| 409 | NOT_MERGED | 指定的交易并没有合并进 `{id}`（`merged_into_id` 不一致） |
| 409 | USER_DELETED | 合并来源是被用户删的而不是合并造成的（不是还原对象） |

> 能区分这两个 `409`，是因为合并来源是**用逻辑删除留着的**。
> `merged_into_id` 有值就是合并造成的，为 NULL 就是用户删除。

---

### API-017 未解析邮件一览

列出判定・抽出还没完成的邮件。
用户用它找出「明明是支付邮件却没被取进来」的，再用 API-018 教示。

一览系，分页处理**与 API-010 同形式**。

```
GET /api/v1/inbox/unparsed
```

**请求参数（query）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| classification | string | - | `unknown`（默认） / `payment` / `not_payment` |
| parse_status | string | - | `pending` / `failed` / `skipped` |
| from_address | string | - | 按发件人筛选 |
| limit | integer | - | 取得件数（默认 20，最大 100） |
| offset | integer | - | 取得起始位置（默认 0） |

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data[].id | integer | 邮件 ID |
| data[].from_address | string | 发件人地址 |
| data[].from_name | string / null | 发件人名 |
| data[].subject | string / null | 标题 |
| data[].received_at | string | 接收时刻 |
| data[].classification | string | `payment` / `not_payment` / `unknown` |
| data[].parse_status | string | `pending` / `success` / `failed` / `skipped` |
| data[].parse_error | string / null | 抽出失败的内容 |
| data[].matched_template_id | integer / null | 命中的模板 |
| meta.total | integer | 符合条件的总件数 |

**响应示例**

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

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | LIMIT_TOO_LARGE | `limit` 超过 100 |
| 400 | INVALID_CLASSIFICATION | `classification` 是定义外的值 |

> **正文（`body_text` / `body_html`）不返回。** 一览里需要的只是
> 「哪个发件人、什么时候、什么标题的邮件没处理」，
> 带上正文传输量大，而且也没什么必要把邮件全文放进浏览器。

---

### API-018 教示支付邮件判定

自动判定错了、或判定不出来的邮件，由用户告诉它正确答案。
教示为 `payment` 时**把重新解析压进队列**（带副作用的动作型）。

```
POST /api/v1/inbox/{id}/classify
```

**请求参数（path）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| id | integer | ○ | 邮件 ID |

**请求参数（body）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| classification | string | ○ | `payment` / `not_payment` |
| reparse | boolean | - | `true`（默认）把重新解析压进队列。`classification = not_payment` 时忽略 |

**请求例**

```json
{ "classification": "payment", "reparse": true }
```

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data.id | integer | 邮件 ID |
| data.classification | string | 更新后的判定 |
| data.classified_at | string | 判定时刻 |
| data.parse_status | string | 压进重解析队列时为 `pending` |
| meta.reparse_queued | boolean | 是否压进了重解析队列 |

**响应示例**

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

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | INVALID_CLASSIFICATION | `classification` 不是 `payment` / `not_payment` |
| 403 | FORBIDDEN | 他人的邮件 |
| 404 | EMAIL_MESSAGE_NOT_FOUND | 邮件不存在 |

> **不做成 `PUT /inbox/{id}` 的理由**：这个操作不只是改写 `classification`，
> 还带着**把重解析压进队列的副作用**。塞进通用的 `PUT` 的话，
> 行为会随送了哪些字段而变，不好懂，所以拆成意图明确的动作型
> （和 7 章设计判断 1 是同一个思路）。

---

### API-019 分类一览

```
GET /api/v1/categories
```

**请求参数（query）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| include_counts | boolean | - | `true` 时带上各分类的交易件数（默认 `false`） |

> 是主数据，每用户预计几十条，**不设分页**。
> 排序按 `sort_order` 升序，相同则按 `id` 升序。

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data[].id | integer | 分类 ID |
| data[].name | string | 分类名 |
| data[].parent_id | integer / null | 父分类 ID |
| data[].sort_order | integer | 显示顺 |
| data[].is_system | boolean | 是否初始创建的分类（**`true` 不可删除**） |
| data[].transaction_count | integer | 交易件数（仅 `include_counts = true` 时） |
| meta.total | integer | 件数 |

**响应示例**

```json
{
  "data": [
    { "id": 3, "name": "食費", "parent_id": null, "sort_order": 1, "is_system": true },
    { "id": 7, "name": "日用品", "parent_id": null, "sort_order": 2, "is_system": false }
  ],
  "meta": { "total": 2 }
}
```

**错误响应**

只有共通的 `401`。

---

### API-020 创建分类

登记系，校验和 `201` 的处理**与 API-011 同形式**。

```
POST /api/v1/categories
```

**请求参数（body）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| name | string | ○ | 分类名（50 字以内） |
| parent_id | integer | - | 父分类 ID |
| sort_order | integer | - | 显示顺（默认 0） |

**请求例**

```json
{ "name": "交際費", "parent_id": null, "sort_order": 5 }
```

**响应（201 Created）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data.id | integer | 创建的分类 ID |
| data.name | string | 分类名 |
| data.parent_id | integer / null | 父分类 ID |
| data.sort_order | integer | 显示顺 |
| data.is_system | boolean | 恒为 `false` |

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | `name` 缺失、超过 50 字 |
| 404 | PARENT_NOT_FOUND | 指定的父分类不存在 |
| 409 | DUPLICATE_CATEGORY_NAME | 已有同名分类（`UNIQUE(user_id, name)`） |

---

### API-021 更新 / 删除分类

同一路径上有 `PUT` 和 `DELETE`。

```
PUT    /api/v1/categories/{id}
DELETE /api/v1/categories/{id}
```

**请求参数（path）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| id | integer | ○ | 分类 ID |

#### PUT（更新）

**请求参数（body）** —— 只更新指定的项

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| name | string | - | 分类名（50 字以内） |
| parent_id | integer / null | - | 父分类 ID（`null` 表示摘掉父级） |
| sort_order | integer | - | 显示顺 |

**请求例**

```json
{ "name": "食費・日用品", "sort_order": 1 }
```

**响应（200 OK）**

`data` 的结构**与 API-020 的 `data` 同形式**。

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | 格式不正 |
| 400 | INVALID_PARENT | 把自己或自己的子孙指定为父级（会成环） |
| 403 | FORBIDDEN | 他人的分类 |
| 404 | CATEGORY_NOT_FOUND | 分类不存在 |
| 404 | PARENT_NOT_FOUND | 指定的父分类不存在 |
| 409 | DUPLICATE_CATEGORY_NAME | 已有同名分类 |

#### DELETE（删除）

**物理删除**。引用它的交易的 `category_id` 靠 FK 的 `SET NULL` 变成 `NULL`。

**响应（204 No Content）**

无 body。

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | SYSTEM_CATEGORY_NOT_DELETABLE | `is_system = true` 的分类（`未分類` 等）不能删 |
| 403 | FORBIDDEN | 他人的分类 |
| 404 | CATEGORY_NOT_FOUND | 分类不存在 |

> **用物理删除的理由**：逻辑删除的话，已删的行会占着 `UNIQUE(user_id, name)`，
> **同名分类就再也建不出来了**（见 DB 规格书 3.7）。
> FK 是 `SET NULL`，物理删除也不会让交易侧产生不一致。
>
> **子分类的处理**：`parent_id` 也是 `SET NULL`，所以被删分类的子级会升到顶层。
> 不会出现树结构坏掉留下孤儿的情况。
>
> **关联的 `category_rules`** 因为 FK 是 `CASCADE` 会连锁删除。
> 留着指向不存在分类的规则，分类批处理每次都会失败。

---

### API-022 店铺一览 / 修正

对同一资源有 `GET`（集合）和 `PUT`（单条）。

```
GET /api/v1/merchants
PUT /api/v1/merchants/{id}
```

#### GET（一览）

一览系，分页处理**与 API-010 同形式**。

**请求参数（query）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| q | string | - | 店铺名・品牌名的部分匹配搜索 |
| brand_name | string | - | 按品牌筛选 |
| geocode_status | string | - | `pending` / `success` / `not_found` / `skipped` |
| limit | integer | - | 取得件数（默认 20，最大 100） |
| offset | integer | - | 取得起始位置（默认 0） |

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data[].id | integer | 店铺 ID |
| data[].name | string | 正规化后的显示名 |
| data[].brand_name | string / null | 连锁品牌名 |
| data[].is_online | boolean | 是否 EC 等线上支付 |
| data[].address | string / null | 地址 |
| data[].latitude | number / null | 纬度 |
| data[].longitude | number / null | 经度 |
| data[].place_types | array | 店铺种别（用于分类推定） |
| data[].geocode_status | string | 地理编码的状态 |
| data[].transaction_count | integer | 该店铺的交易件数 |
| meta.total | integer | 符合条件的总件数 |

**响应示例**

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

用户修正正规化错了的店铺名・品牌名。

**请求参数（body）** —— 只更新指定的项

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| name | string | - | 显示名（255 字以内） |
| brand_name | string / null | - | 连锁品牌名 |
| address | string / null | - | 地址 |
| is_online | boolean | - | 是否线上支付 |

**请求例**

```json
{ "brand_name": "セブン-イレブン" }
```

**响应（200 OK）**

`data` 的结构**与 GET 的 `data[]` 同形式**。另外还有：

| 字段名 | 类型 | 说明 |
|---|---|---|
| meta.regeocode_queued | boolean | 因为改了 `address`，是否要重新跑地理编码 |

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | 格式不正 |
| 403 | FORBIDDEN | 他人的店铺 |
| 404 | MERCHANT_NOT_FOUND | 店铺不存在 |
| 409 | DUPLICATE_MERCHANT_NAME | 已有同名店铺（`UNIQUE(user_id, name)`） |

> **改了显示名也不动 `merchant_aliases`。**
> `merchants.name` 是「显示在画面上的名字」，`merchant_aliases.alias` 是
> 「和邮件里原始字符串做匹配的键」，是两个不同的概念。
> 改显示名时顺手改了匹配键的话，**下一封邮件开始名寄せ 就对不上了**。
>
> **改 `brand_name` 会波及已有的 `category_rules`（`match_type = brand`）。**
> 改了品牌，挂在那个品牌上的分类规则就失效了，
> 所以画面侧要提示「这个变更会影响分类规则」。

---

### API-023 取月末汇总

不直接返回 DB 记录，而是返回聚合结果。
让月末配信邮件的内容在画面上也能看到。

```
GET /api/v1/summaries/monthly
```

**请求参数（query）**

| 参数名 | 类型 | 必需 | 说明 |
|---|---|---|---|
| month | string | - | 对象月（`YYYY-MM`，默认当月。JST 基准） |

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data.month | string | 对象月 |
| data.total_amount_minor | integer | 当月合计支出 |
| data.prev_total_amount_minor | integer / null | 上月合计支出 |
| data.diff_ratio | number / null | 环比（`1.0` 为持平） |
| data.transaction_count | integer | 交易件数 |
| data.breakdown[].category | object | 分类 |
| data.breakdown[].amount_minor | integer | 分类别合计 |
| data.breakdown[].ratio | number | 占整体的比例 |
| data.active_subscriptions[] | array | 持续中的订阅 |
| data.budget.amount_minor | integer / null | 当月预算 |
| data.budget.used_ratio | number / null | 预算消耗率 |
| meta.is_complete | boolean | 对象月是否已结束（当月为 `false`） |

**响应例**

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

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | INVALID_MONTH_FORMAT | `month` 格式不是 `YYYY-MM` |
| 404 | NO_DATA | 对象月没有数据 |

---

### API-024 订阅一览

一览系，分页处理**与 API-010 同形式**。

```
GET /api/v1/subscriptions
```

**请求参数（query）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| status | string | - | `active`（默认） / `suspected_stopped` / `cancelled` / `all` |
| limit | integer | - | 取得件数（默认 20，最大 100） |
| offset | integer | - | 取得起始位置（默认 0） |

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data[].id | integer | 订阅 ID |
| data[].merchant | object | 店铺（结构与 API-010 的 `merchant` 同形式） |
| data[].amount_minor | integer | 扣费额 |
| data[].currency | string | 货币码 |
| data[].cycle | string | `monthly` / `yearly` / `weekly` |
| data[].occurrence_count | integer | 用于检测的扣费次数 |
| data[].first_charged_at | string | 首次扣费日 |
| data[].last_charged_at | string | 最近扣费日 |
| data[].next_expected_at | string / null | 下次扣费的预测日 |
| data[].status | string | `active` / `suspected_stopped` / `cancelled` |
| meta.total | integer | 符合条件的总件数 |
| meta.monthly_total_amount_minor | integer | **换算成月额的合计**（`yearly` 按 1/12、`weekly` 按 52/12 换算） |

**响应示例**

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

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | LIMIT_TOO_LARGE | `limit` 超过 100 |
| 400 | INVALID_STATUS | `status` 是定义外的值 |

> **返回 `meta.monthly_total_amount_minor` 的理由**：订阅一览里想知道的是
> 「每个月固定支出多少」，`cycle` 混在一起的金额单纯相加没有意义。
> 换算放在服务器侧做，可以避免各客户端的换算式出现偏差。

---

### API-025 更新订阅状态

更新系，处理**与 API-013 同形式**（但没有学习这个副作用）。

```
PUT /api/v1/subscriptions/{id}
```

**请求参数（path）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| id | integer | ○ | 订阅 ID |

**请求参数（body）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| status | string | ○ | 只能指定 `active` / `cancelled` |

**请求例**

```json
{ "status": "cancelled" }
```

**响应（200 OK）**

`data` 的结构**与 API-024 的 `data[]` 同形式**。

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | INVALID_STATUS | `status` 不是 `active` / `cancelled` |
| 403 | FORBIDDEN | 他人的订阅 |
| 404 | SUBSCRIPTION_NOT_FOUND | 订阅不存在 |

> **不让用户指定 `suspected_stopped` 的理由**：这个值是
> 「过了 `next_expected_at` 也没观测到扣费」这一**批处理的检测结果**，
> 不是用户的意思表示。用户能表达的只有「还在用（`active`）」和
> 「退订了（`cancelled`）」两种，所以可指定的值就限定为这两个。

---

### API-026 预算取得 / 设置

以集合为单位取得和批量更新。

```
GET /api/v1/budgets
PUT /api/v1/budgets
```

#### GET（取得）

**请求参数（query）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| month | string | - | 计算消化率的对象月（`YYYY-MM`，默认当月） |

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data[].id | integer | 预算 ID |
| data[].category | object / null | 分类。**`null` 表示总体预算** |
| data[].period | string | 预算周期（`monthly`） |
| data[].amount_minor | integer | 预算额 |
| data[].alert_thresholds | array | 要通知的到达率（%） |
| data[].is_active | boolean | 有效标志 |
| data[].used_amount_minor | integer | 对象月的消化额 |
| data[].used_ratio | number | 消化率（`1.0` 表示刚好用完） |
| meta.month | string | 对象月 |

**响应示例**

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

#### PUT（批量设置）

**请求参数（body）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| budgets[] | array | ○ | 要设置的预算数组 |
| budgets[].category_id | integer / null | ○ | 分类 ID。`null` 表示总体预算 |
| budgets[].amount_minor | integer | ○ | 预算额（0 以上） |
| budgets[].alert_thresholds | array | - | 要通知的到达率（默认 `[80, 100]`） |
| budgets[].is_active | boolean | - | 有效标志（默认 `true`） |

**请求例**

```json
{
  "budgets": [
    { "category_id": null, "amount_minor": 150000, "alert_thresholds": [80, 100] },
    { "category_id": 3, "amount_minor": 60000 }
  ]
}
```

**响应（200 OK）**

`data` 的结构**与 GET 的 `data[]` 同形式**（`used_amount_minor` / `used_ratio` 是当月的值）。

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | `budgets` 缺失、格式不正 |
| 400 | INVALID_AMOUNT | 预算额为负 |
| 400 | INVALID_THRESHOLD | 到达率超出 1〜200 范围，或有重复 |
| 400 | DUPLICATE_CATEGORY | 数组里同一个 `category_id` 出现多次 |
| 404 | CATEGORY_NOT_FOUND | 指定的分类不存在 |

> **不设单条的 `POST` / `DELETE`，而是对集合做 `PUT` 的理由**：
> 预算设置是「在画面上把数字排开编辑、一起保存」的操作，
> 拆成一条一条的请求会让前端不得不自己算创建・更新・删除的差分。
> 有 `UNIQUE(user_id, category_id, period)` 在，
> 以 `category_id` 为键做 upsert 的话一次请求就能搞定。
> 数组里没包含的既有预算**不动**（想删的话送 `is_active = false`）。

---

### API-027 通知设置取得 / 更新

每个用户只有 1 行，所以路径不带 ID。

```
GET /api/v1/notification-settings
PUT /api/v1/notification-settings
```

#### GET（取得）

**请求参数**：无

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data.channel | string | `email` / `slack` |
| data.email_to | string / null | 投递目标邮箱 |
| data.slack_webhook_configured | boolean | **Slack Webhook 是否已设置**（URL 本身不返回） |
| data.instant_enabled | boolean | 即时通知开关 |
| data.instant_min_amount_minor | integer | 低于这个金额不做即时通知 |
| data.quiet_hours_start | string / null | 静音时段开始（`HH:MM`） |
| data.quiet_hours_end | string / null | 静音时段结束（`HH:MM`） |
| data.budget_alert_enabled | boolean | 预算超支通知开关 |
| data.monthly_summary_enabled | boolean | 月末汇总开关 |
| data.monthly_summary_send_at | string | 月末汇总的投递时刻（`HH:MM`） |

**响应示例**

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

**请求参数（body）** —— 只更新指定的项

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| channel | string | - | `email` / `slack` |
| email_to | string / null | - | 投递目标邮箱 |
| slack_webhook | string / null | - | **只写不读。** Slack Webhook URL（加密保存。`null` 表示删除） |
| instant_enabled | boolean | - | 即时通知开关 |
| instant_min_amount_minor | integer | - | 即时通知的下限金额（0 以上） |
| quiet_hours_start | string / null | - | 静音时段开始（`HH:MM`） |
| quiet_hours_end | string / null | - | 静音时段结束（`HH:MM`） |
| budget_alert_enabled | boolean | - | 预算超支通知开关 |
| monthly_summary_enabled | boolean | - | 月末汇总开关 |
| monthly_summary_send_at | string | - | 月末汇总的投递时刻（`HH:MM`） |

**请求例**

```json
{ "instant_min_amount_minor": 3000, "quiet_hours_start": "22:30" }
```

**响应（200 OK）**

`data` 的结构**与 GET 的 `data` 同形式**。

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | VALIDATION_ERROR | 格式不正 |
| 400 | INVALID_TIME_FORMAT | 不是 `HH:MM` 格式 |
| 400 | EMAIL_TO_REQUIRED | `channel = email` 但 `email_to` 没设 |
| 400 | SLACK_WEBHOOK_REQUIRED | `channel = slack` 但 Webhook 没设 |

> **把 `slack_webhook` 做成只写的理由**：Webhook URL
> **本身就是带投稿权限的机密信息**，设好之后没有读出来的必要。
> GET 只返回 `slack_webhook_configured` 这个真假值，不返回 URL。
> DB 上也是作为 `slack_webhook_encrypted` 加密保存。
>
> **静音时段跨日**（`start = 23:00`、`end = 07:00`）时，
> 把 `start > end` 当作「跨日」处理。两个都是 `null` 表示不静音。
> 静音时段里发生的即时通知**不丢弃，等静音结束后再发**
> （有 `notifications.dedupe_key` 在，重跑也不会重复发送）。

---

### API-028 通知历史一览

一览系，分页处理**与 API-010 同形式**。

```
GET /api/v1/notifications
```

**请求参数（query）**

| 参数名 | 类型 | 必须 | 说明 |
|---|---|---|---|
| kind | string | - | `instant` / `budget_alert` / `monthly_summary` |
| status | string | - | `pending` / `sent` / `failed` |
| limit | integer | - | 取得件数（默认 20，最大 100） |
| offset | integer | - | 取得起始位置（默认 0） |

**响应（200 OK）**

| 字段名 | 类型 | 说明 |
|---|---|---|
| data[].id | integer | 通知 ID |
| data[].kind | string | `instant` / `budget_alert` / `monthly_summary` |
| data[].channel | string | `email` / `slack` |
| data[].subject | string / null | 标题 |
| data[].status | string | `pending` / `sent` / `failed` |
| data[].sent_at | string / null | 发送时刻 |
| data[].error_message | string / null | 发送失败的内容 |
| data[].created_at | string | 创建时刻 |
| meta.total | integer | 符合条件的总件数 |

**响应示例**

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

**错误响应**

| 状态码 | 错误码 | 错误内容 |
|---|---|---|
| 400 | LIMIT_TOO_LARGE | `limit` 超过 100 |
| 400 | INVALID_KIND | `kind` 是定义外的值 |

> **正文（`body`）和 `dedupe_key` 不返回。** 历史一览需要的只是
> 「什么时候・哪个种别・发出去没有」，正文看已发送的邮件就够了。
> `dedupe_key` 是保证幂等性的内部值，对客户端没有意义。

---

## 7. 主要設計判断

1. **准备了动作型 endpoint**
   `merge` / `unmerge` / `classify` 不只是改资源状态，还**伴随副作用**
   （事件的重新挂载、规则的学习）。
   塞进通用 `PUT` 的话，行为会随「改了哪个字段」而变，难以理解，
   所以分离成意图明确的动作型 endpoint。

2. **`PUT /transactions/{id}` 的副作用用 `meta` 返回**
   修正分类时把学到的规则作为 `meta.learned_rule` 返回，
   就能在画面上告诉用户「这次修正对今后也有效」。
   另外准备了 `learn_category` 标志，给不想让它学习的场合。

3. **金额用最小单位整数返回**
   防止客户端侧的舍入误差。显示时的千分位由前端负责。

4. **一览的 `meta` 里含合计金额**
   每次改筛选条件就不用另发一个请求取合计，减少请求数。

5. **认证分成两层**
   调 API 的认证和读邮件的认证，目的和有效期都不同。
   后者的 token 绝不用于 API 请求，加密后存 DB。

6. **批处理相关的 API 做到最少**
   取込・解析・名寄せ 由批处理做，所以 API 只提供
   `POST /mail-accounts/{id}/sync`（202 受理）和 `GET /sync-jobs`（查历史）。

7. **`DELETE` 的含义按 endpoint 逐个定义**
   `DELETE` 只表示「从客户端看它消失了」，不代表服务器一定物理删除。
   本服务里 `transactions` 用逻辑删除、`mail_accounts` 用 `status = 'disabled'`、
   `categories` 用物理删除，各不相同，对照表写在第 5 章。
   如果都用物理删除，**删掉的交易会在下次 名寄せ 批处理时复活**（详见 DB 规格书 3.7）。

8. **`merge` 保留合并来源，让 `unmerge` 得以成立**
   合并来源一旦物理删除，上面的用户手动修改就没了，「撤销合并」无法实现。
   用 `merged_into_id` 记下合并目标再逻辑删除，
   只要把两列置回 NULL 就能复原。
   副产品是 `409 ALREADY_MERGED` 变得能从状态判断出来。

9. **`merge` 和 `unmerge` 的路径形状不同**
   `merge` 是把多笔交易束起来、面向集合的操作，所以是 `POST /transactions/merge`（两边都放 body）；
   `unmerge` 是**对某一笔已合并交易的操作**，所以是 `POST /transactions/{id}/unmerge`。
   形状不同是因为对象不同，不是摇摆。

---

## 8. 纠结的地方与 メンタリング 的确认结果

Step 2 提交时列出的 5 个相谈事项中，**4 个在反馈里有了结论**。
下面逐条记载确认结果和在规格书里的反映位置。

| # | 相谈事项 | 状态 |
|---|---|---|
| 1 | `merge` 的路径设计 | **已确认**（有变更） |
| 2 | 月末汇总的聚合方式 | **已确认**（维持现状） |
| 3 | `GET /inbox/unparsed` 的定位 | **未确认** |
| 4 | 错误码的粒度 | **已确认**（维持现状） |
| 5 | 分页方式 | **已确认**（维持现状） |

### 1. `merge` 的路径设计 —— 已确认（有变更）

> **相谈内容**：`POST /transactions/{id}/merge` 和
> `POST /transactions/merge`（两边都放 body 里）哪个更合适

**结论：采用 `POST /transactions/merge`。**

合并不是对某一个资源的操作，而是把多笔交易束成一笔的
**面向集合的操作**，所以没必然理由要在 URL 里放其中一方的 ID。
另外 `{id}/merge` 的方向从 URL 读不出来，各实现容易摇摆。
将来扩展到 3 笔以上合并也更容易留余地。

**反映位置**：5 章的 endpoint 一览、6 章的 API-015、7 章的设计判断 9。

### 2. 月末汇总的聚合方式 —— 已确认（维持现状）

> **相谈内容**：每次用 API 现算，还是返回 `monthly_summaries` 里存好的结果

**结论：当前维持每次现算的设计。**

`monthly_summaries` **只作为投递历史保留**，查询系的 query 用
`idx_transactions_user_category_occurred` 做 `GROUP BY`。
这样在一致性（改了交易汇总立刻跟上）和简单性（不做双重管理）之间平衡最好。

**迁移的判断基准**：等性能成为问题时再考虑物化视图化。
提前做成返回存好的结果，反而要自己去解决
「改了交易但汇总还是旧的」这种不一致。

**反映位置**：6 章的 API-023（按每次现算为前提的响应定义）。

### 3. `GET /inbox/unparsed` 的定位 —— 未确认

> **相谈内容**：邮件该作为资源公开，还是该做成「待处理队列」这个另外的概念

**这一条反馈里没有提及，下次 メンタリング 再谈。**

现阶段**设计成「`email_messages` 的筛选后一览」**（6 章的 API-017）。
用 `classification` / `parse_status` 做筛选，正文（`body_text` / `body_html`）不放进响应。
相当于把邮件作为资源公开，同时用 query 参数表达「只看未处理的」这条导线的折衷方案。

**论点**：把「待处理队列」做成另外的概念的话，
用 `GET /inbox/queue` 这样的 endpoint 能让**服务器侧决定处理顺序**，
但另需一个单独引用某封邮件的手段。
现状它是 Step 8 以后（扩展）的 endpoint，实装前定下来就来得及。

### 4. 错误码的粒度 —— 已确认（维持现状）

> **相谈内容**：归到 `VALIDATION_ERROR` 一个，还是按项目分开

**结论：维持现状粒度（按前端想分支的单位分开）。**

归到 `VALIDATION_ERROR` 的话，为了告诉对方是哪个项目不正，
**响应里就需要带字段级别的详情结构**。
按前端想分支的单位拆错误码——也就是现在的立场——从 UI 角度更好处理。

因此本规格书里，像格式不正这种「任何项目都可能发生」的归到 `VALIDATION_ERROR`，
而 `INVALID_AMOUNT` / `INVALID_DATE_RANGE` / `LIMIT_TOO_LARGE` /
`DUPLICATE_CATEGORY_NAME` 这种**画面上想出不同提示的**就给独立的码。

**反映位置**：6 章各 endpoint 的错误响应表。

### 5. 分页方式 —— 已确认（维持现状）

> **相谈内容**：先用 offset，交易件数变多后要不要换成 cursor 方式

**结论：当前维持 offset 方式。**

**迁移的判断基准**：交易件数**超过数万件的阶段**再考虑切换到 cursor 方式
（`occurred_at` + `id` 的组合）。
因为交易列表的主轴索引已经统一为 `(user_id, occurred_at DESC)`，
切换时的实装成本很小。

返回 `meta.total`（总件数）的设计也是以 offset 方式为前提的。
迁移到 cursor 方式时，要不要继续返回总件数
（每次都跑 `COUNT`，件数越多越重）也要一并判断。

**反映位置**：2 章的共通规格、6 章的一览系 endpoint。
