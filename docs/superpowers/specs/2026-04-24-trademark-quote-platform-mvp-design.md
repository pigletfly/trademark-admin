# 国际商标智能报价与业务管理平台 MVP — 设计规格

**日期**：2026-04-24
**作者**：brainstorming session（与 Claude 协同）
**状态**：待实施

---

## 1. 背景与目标

当前 `trademark-admin` 仓库是基于 `shadcn-admin` 模板的空壳。需按 `docs/国际商标智能报价与国际业务管理平台描述.pdf` 的业务需求，建设一个内部工具，服务三类角色：

- **业务员（salesperson）**：录入客户商标需求，生成报价方案，预览、调价、导出。
- **国际部商务（reviewer）**：审核报价、维护成本数据、认领工单。
- **系统管理员（admin）**：维护字典数据、管理用户、查看审计日志。

本 spec 覆盖 MVP 的端到端范围，包含前端、后端、数据库、导出管线。

## 2. MVP 范围

以 PDF 描述中的子系统为单位拆解后，MVP 合并以下四个子项目：

| 子项 | 产出 |
|---|---|
| P0 基础数据与角色 | JWT 认证、角色权限、国家/尼斯类别字典（seed） |
| P1 成本与模板管理 | 二维成本模板（国家 × 注册方式 × 费用项），immutable 版本化 |
| P2 报价生成器 | 向导式表单、纯函数计算引擎、预览、自定义调价、双语 PDF + Word 导出 |
| P3 审核工作流 | Mode A 线上审核（draft → submitted → reviewing → approved/rejected → downloaded） |
| 附加：客户档案 | 轻量 CRUD + 搜索 |

**不包含在本 MVP**：市场需求分析看板、供应商管理、企业微信/邮件通知集成、多币种、多租户、i18n UI。

## 3. 技术栈与仓库结构

### 3.1 技术选型

| 层 | 选型 |
|---|---|
| 前端 | React 19 + Vite + TanStack Router + TanStack Query + Shadcn/Radix + TailwindCSS 4 + Zustand |
| 表单/校验 | React Hook Form + Zod |
| 后端 | Go + Gin + GORM |
| 数据库 | PostgreSQL 16 |
| 认证 | 自建 JWT（access + refresh），httpOnly + SameSite=Lax cookie |
| 导出 | PDF：HTML 模板 + chromedp headless Chrome；Word：.docx 模板 + unioffice |
| 类型共享 | `openapi-typescript` 从 OpenAPI yaml 生成 TS 类型 |
| 本地开发 | docker-compose（postgres + api + web） |
| 端到端测试 | Playwright（已在 package.json） |

**移除**：Clerk（改为自建 JWT）、`@clerk/react` 依赖。

### 3.2 仓库布局（Monorepo）

```
trademark-admin/
├── apps/
│   ├── web/                      # React 前端（从仓库根迁入）
│   │   ├── src/
│   │   │   ├── features/         # 按业务域组织
│   │   │   ├── lib/api-client/   # 生成的 TS 类型
│   │   │   ├── stores/
│   │   │   └── routes/           # TanStack Router file-based
│   │   ├── package.json
│   │   └── vite.config.ts
│   │
│   └── api/                      # Go 后端（新建）
│       ├── cmd/
│       │   ├── server/main.go    # API 入口
│       │   ├── migrate/main.go   # 迁移 CLI
│       │   └── seed/main.go      # 字典数据 seed
│       ├── internal/
│       │   ├── auth/             # 登录、JWT、RBAC
│       │   ├── catalog/          # 国家、尼斯类别字典
│       │   ├── customer/         # 客户档案
│       │   ├── pricing/          # 成本模板 + 版本化 + 计算引擎
│       │   ├── quotation/        # 报价单 + 状态机 + 审核流
│       │   ├── export/           # PDF/Word 双语导出
│       │   └── platform/         # 中间件、日志、错误码、分页、审计
│       ├── pkg/
│       │   ├── database/         # GORM 封装
│       │   └── httpx/            # Gin 辅助
│       ├── migrations/*.sql
│       ├── seed/*.json           # 字典初始数据
│       ├── go.mod
│       └── Dockerfile
│
├── packages/
│   └── shared/
│       ├── openapi.yaml          # API 契约
│       └── types/                # 生成的 TS 类型
│
├── docker-compose.yml
├── pnpm-workspace.yaml
└── package.json                  # 根 workspace
```

**模块内部结构**（以 `internal/quotation/` 为例）：
```
model.go         GORM entities
repository.go    数据访问
service.go       领域逻辑
handler.go       Gin handlers
router.go        路由注册
dto.go           Request/Response DTO
service_test.go  单元测试
```

**跨模块约束**：`internal/<domain>/` 之间只通过导出的 service 接口互调，禁止直接访问对方 repository 或 model。

## 4. 系统架构

```
┌──────────────────────────────────────────────────────────────┐
│                    Browser (apps/web)                         │
│  React + TanStack Router + Shadcn + Zustand + Query           │
│  JWT 保存于 httpOnly cookie，禁止 localStorage 存敏感数据         │
└──────────────┬──────────────────────────────────┬────────────┘
               │ /api/v1/...                      │ /api/v1/auth/*
               ▼                                  ▼
┌──────────────────────────────────────────────────────────────┐
│                    Go API (apps/api)                          │
│           Gin + JWT middleware + RBAC middleware              │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │ auth │ catalog │ customer │ pricing │ quotation │ export│ │
│  └─────────────────────────────────────────────────────────┘ │
│                 GORM → pgx → PostgreSQL                       │
└──────────────┬────────────────────────────┬─────────────────┘
               ▼                            ▼
      ┌──────────────────┐         ┌──────────────────┐
      │   PostgreSQL     │         │ 本地 FS / OSS    │
      │  业务数据 + 审计  │         │  导出文件暂存 24h │
      └──────────────────┘         └──────────────────┘
```

- **无状态 API**：JWT 放在 httpOnly cookie，CSRF 通过自定义 header（`X-CSRF-Token`，从双令牌中取）或 SameSite=Lax 防御。
- **审计日志**：所有写操作由 `platform.AuditMiddleware` 写入 `audit_logs` 表。
- **导出文件**：生成后存本地（MVP）或对象存储（生产），返回一次性签名下载链接，过期 24 小时。

## 5. 核心数据模型

所有表主键为 `uuid`（除 `nice_categories.code`）。时间字段均含 `created_at`、`updated_at`（soft-deletable 的表加 `deleted_at`）。

### 5.1 身份与角色

```sql
users (
  id              UUID PRIMARY KEY,
  name            TEXT NOT NULL,
  phone           TEXT,
  email           CITEXT UNIQUE NOT NULL,
  password_hash   TEXT NOT NULL,
  password_updated_at TIMESTAMPTZ NOT NULL,
  role_id         UUID NOT NULL REFERENCES roles(id),
  status          TEXT NOT NULL DEFAULT 'active',  -- active|disabled
  created_at, updated_at
);

roles (
  id              UUID PRIMARY KEY,
  code            TEXT UNIQUE NOT NULL,  -- salesperson|reviewer|admin
  name            TEXT NOT NULL,
  description     TEXT
);
```

权限通过 `roles.code` 编码进中间件，不做细粒度 RBAC 表。

### 5.2 字典

```sql
countries (
  id                          UUID PRIMARY KEY,
  code                        TEXT UNIQUE NOT NULL,   -- ISO 3166-1 α-2
  name_zh                     TEXT NOT NULL,
  name_en                     TEXT NOT NULL,
  is_madrid_member            BOOLEAN NOT NULL,
  default_acceptance_days     INTEGER,
  default_registration_months INTEGER,
  requires_notarization       BOOLEAN NOT NULL DEFAULT FALSE,
  notes_zh                    TEXT,
  notes_en                    TEXT,
  sort_order                  INTEGER NOT NULL DEFAULT 0,
  enabled                     BOOLEAN NOT NULL DEFAULT TRUE
);

nice_categories (
  code            INTEGER PRIMARY KEY,           -- 1..45
  name_zh         TEXT NOT NULL,
  name_en         TEXT NOT NULL,
  description_zh  TEXT,
  description_en  TEXT
);
```

Seed 数据：`apps/api/seed/countries.json`（约 130 条）、`apps/api/seed/nice_categories.json`（45 条）。`cmd/seed/main.go` 在应用启动或手动运行时 upsert 这些数据。

### 5.3 客户

```sql
customers (
  id              UUID PRIMARY KEY,
  name            TEXT NOT NULL,
  industry        TEXT,
  is_returning    BOOLEAN NOT NULL DEFAULT FALSE,
  price_sensitive BOOLEAN NOT NULL DEFAULT FALSE,
  contact_name    TEXT,
  contact_phone   TEXT,
  contact_email   TEXT,
  notes           TEXT,
  created_by      UUID NOT NULL REFERENCES users(id),
  created_at, updated_at,
  deleted_at      TIMESTAMPTZ,
  UNIQUE (name, deleted_at)   -- 同名在未删除数据里唯一
);
```

### 5.4 成本模板（Immutable 版本化）

```sql
pricing_entries (
  id                      UUID PRIMARY KEY,
  country_id              UUID NOT NULL REFERENCES countries(id),
  registration_method     TEXT NOT NULL,  -- single|madrid
  service_tier            TEXT NOT NULL,  -- a|b
  fee_item                TEXT NOT NULL,  -- official|agent|notary|translation|other
  calc_basis              TEXT NOT NULL,  -- per_class|fixed|per_item
  amount_cny              NUMERIC(12,2) NOT NULL,
  default_included_items  INTEGER,        -- 含多少商品项（可空）
  extra_item_fee          NUMERIC(12,2),  -- 超出每项加收（可空）
  valid_from              TIMESTAMPTZ NOT NULL,
  valid_until             TIMESTAMPTZ,    -- NULL = 当前生效
  created_by              UUID NOT NULL REFERENCES users(id),
  created_at              TIMESTAMPTZ NOT NULL,
  note                    TEXT
);
CREATE INDEX idx_pricing_lookup
  ON pricing_entries (country_id, registration_method, service_tier, fee_item, valid_from);
```

**版本化机制**：
- 修改一条成本时，不就地 UPDATE。
- 事务内：`UPDATE old SET valid_until = now WHERE id = ?` + `INSERT new (valid_from = now, valid_until = NULL, ...)`。
- 查询当前生效版本：
  ```sql
  WHERE valid_from <= now
    AND (valid_until IS NULL OR valid_until > now)
  ```
- 不允许硬删除，只能「deprecate」（设 `valid_until = now` 不插新）。

### 5.5 报价单

```sql
quotations (
  id                         UUID PRIMARY KEY,
  serial_no                  TEXT UNIQUE NOT NULL,   -- Q202604230001
  status                     TEXT NOT NULL,          -- draft|submitted|reviewing|approved|rejected|downloaded
  creator_id                 UUID NOT NULL REFERENCES users(id),
  reviewer_id                UUID REFERENCES users(id),
  customer_id                UUID REFERENCES customers(id),
  customer_snapshot          JSONB NOT NULL,         -- 冻结的客户信息
  trademark_name             TEXT NOT NULL,
  trademark_image_url        TEXT,
  nice_category_codes        INTEGER[] NOT NULL,     -- 多选
  goods_items                TEXT NOT NULL,
  country_ids                UUID[] NOT NULL,
  registration_method        TEXT NOT NULL,          -- single|madrid|both
  service_tier               TEXT NOT NULL,          -- a|b

  -- 报价结果聚合
  total_official_cny         NUMERIC(14,2) NOT NULL DEFAULT 0,
  total_agent_cny            NUMERIC(14,2) NOT NULL DEFAULT 0,
  total_notary_cny           NUMERIC(14,2) NOT NULL DEFAULT 0,
  total_other_cny            NUMERIC(14,2) NOT NULL DEFAULT 0,
  total_amount_cny           NUMERIC(14,2) NOT NULL DEFAULT 0,

  -- 调价
  adjustment_type            TEXT NOT NULL DEFAULT 'none',  -- none|amount|percent
  adjustment_value           NUMERIC(14,2) NOT NULL DEFAULT 0,
  adjustment_target          TEXT NOT NULL DEFAULT 'total', -- total|service_fee_only

  -- 工作流时间戳
  submitted_at               TIMESTAMPTZ,
  review_started_at          TIMESTAMPTZ,
  reviewed_at                TIMESTAMPTZ,
  exported_at                TIMESTAMPTZ,
  reject_reason              TEXT,

  created_at, updated_at
);
CREATE INDEX idx_quotation_status_creator ON quotations (status, creator_id);

quotation_items (
  id                       UUID PRIMARY KEY,
  quotation_id             UUID NOT NULL REFERENCES quotations(id) ON DELETE CASCADE,
  country_id               UUID NOT NULL REFERENCES countries(id),
  fee_item                 TEXT NOT NULL,
  calc_basis               TEXT NOT NULL,
  unit_amount_cny          NUMERIC(12,2) NOT NULL,
  quantity                 NUMERIC(10,2) NOT NULL,      -- 类别数 / 超出项数 / 1
  subtotal_cny             NUMERIC(14,2) NOT NULL,
  source_pricing_entry_id  UUID NOT NULL REFERENCES pricing_entries(id)  -- 溯源
);
```

**快照原则**：报价 `submit` 时，服务层运行计算引擎并将对应 `pricing_entries` 的有效值拷贝到 `quotation_items`。之后成本表无论怎么改，这张报价的数字不变。

### 5.6 审核流水 + 审计

```sql
quotation_reviews (
  id             UUID PRIMARY KEY,
  quotation_id   UUID NOT NULL REFERENCES quotations(id) ON DELETE CASCADE,
  reviewer_id    UUID NOT NULL REFERENCES users(id),
  action         TEXT NOT NULL,  -- submit|withdraw|claim|adjust|approve|reject|copy|download
  from_status    TEXT,
  to_status      TEXT,
  comment        TEXT,
  diff_json      JSONB,          -- 若动作是 adjust
  created_at     TIMESTAMPTZ NOT NULL
);

audit_logs (
  id              UUID PRIMARY KEY,
  user_id         UUID REFERENCES users(id),
  action          TEXT NOT NULL,
  resource_type   TEXT NOT NULL,
  resource_id     TEXT NOT NULL,
  changes_json    JSONB,
  ip              INET,
  user_agent      TEXT,
  created_at      TIMESTAMPTZ NOT NULL
);
```

### 5.7 导出文件

```sql
export_files (
  id              UUID PRIMARY KEY,
  quotation_id    UUID NOT NULL REFERENCES quotations(id),
  format          TEXT NOT NULL,   -- pdf|docx
  language        TEXT NOT NULL,   -- zh|en|bilingual
  file_path       TEXT NOT NULL,
  file_size       BIGINT NOT NULL,
  sha256          TEXT,
  expires_at      TIMESTAMPTZ NOT NULL,
  created_by      UUID NOT NULL REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL
);
```

## 6. 报价计算引擎

**定位**：纯函数，输入 DTO，输出 DTO，无任何副作用。服务层编排调用它。

### 6.1 签名

```go
// internal/pricing/engine.go

type QuoteInput struct {
    NiceCategoryCodes  []int
    GoodsItemCount     int               // 数个商品/服务项
    Countries          []CountryRef
    RegistrationMethod string             // single|madrid|both
    ServiceTier        string             // a|b
    Adjustment         AdjustmentInput
    EffectiveAt        time.Time          // 默认 now
}

type QuoteResult struct {
    Warnings    []string                   // 如「XX 国不支持马德里」
    Items       []QuoteLineItem            // 每行 = 国家 × 费用项
    Totals      QuoteTotals
    AppliedAdj  AdjustmentApplied
}

func (e *Engine) Compute(ctx, input QuoteInput) (QuoteResult, error)
```

### 6.2 算法

1. **校验**：
   - 每个国家查 `countries.is_madrid_member`：不是成员国 + 选马德里 → 加 warning（不阻断）。
   - `requires_notarization` 为 true → 加 warning（提示附加公证费）。
2. **对每个 (country, registration_method) 组合**，查询当前生效的 `pricing_entries`：
   - 条件：`country_id` + `registration_method` + `service_tier` + `valid_from <= effective_at AND (valid_until IS NULL OR valid_until > effective_at)`
   - 遍历所有匹配的 `fee_item`（official / agent / notary / translation / other）
3. **对每条成本计算 subtotal**：
   - `fixed`：`subtotal = amount`
   - `per_class`：`subtotal = amount × len(nice_category_codes)`
   - `per_item`：若 `goods_items_count > default_included_items`，`subtotal = (goods_items_count - default_included_items) × extra_item_fee`；否则 0
4. **聚合**：按 fee_item 类别求和 → `total_official_cny` 等；`total_amount_cny = Σ all`。
5. **应用调价**：
   - `none`：无操作
   - `amount`：`total_amount += value`（也可负）
   - `percent`：`total_amount *= (1 + value / 100)`
   - `adjustment_target = service_fee_only` 时仅调整 `total_agent_cny`
6. **返回 QuoteResult**（不入库）。

### 6.3 保存与快照

`quotation.Service.Submit(id)`：
1. 加载 draft quotation 的全部输入
2. 调用引擎（当前时间的 pricing）
3. 开事务：
   - `UPDATE quotations SET status='submitted', submitted_at=now, total_* = ...`
   - 删除旧的 `quotation_items`（如果是重复 submit 的情况，见 §7.2 状态机）
   - 插入新的 `quotation_items`，每条带 `source_pricing_entry_id`
   - 写 `quotation_reviews` 一条
4. 提交

## 7. 审核工作流

### 7.1 状态机

```
      submit        claim         approve
draft ────→ submitted ────→ reviewing ────→ approved ──┐
  ↑           │                 │                       │ download
  │           │ withdraw        │ reject                ▼
  └───────────┘                 ▼                   downloaded
                            rejected
```

### 7.2 动作定义

| 动作 | 授权 | 前置 | 后置 | 业务效果 |
|---|---|---|---|---|
| `submit` | 业务员（owner） | draft | submitted | 触发引擎计算 + 写入 quotation_items 快照 |
| `withdraw` | 业务员（owner） | submitted | draft | 清空 reviewer_id；保留 items 便于再 submit |
| `claim` | reviewer | submitted | reviewing | 写入 reviewer_id、review_started_at |
| `adjust` | reviewer（本人认领的） | reviewing | reviewing | 修改 `adjustment_*` 或逐项 `quotation_items`，diff_json 记录 |
| `approve` | reviewer（本人认领的） | reviewing | approved | 写 reviewed_at；可触发内部通知 |
| `reject` | reviewer（本人认领的） | reviewing | rejected | 要求 `reject_reason`；通知 creator |
| `copy` | 任何人（能看到原单） | 任意 | → 新 draft | 生成新 quotation（新 serial_no），拷贝输入字段，重置金额 |
| `download` | creator / reviewer | approved 或 downloaded | downloaded | 触发 export pipeline；首次转 downloaded |

### 7.3 实现要点

- 所有状态迁移封装在 `quotation.Service.Transition(ctx, id, action, actor, payload)`。
- 非法迁移返回 `ErrInvalidTransition` → HTTP 409。
- 授权检查在同一方法内（避免散落）。
- 每次成功迁移写 `quotation_reviews` 一行。

### 7.4 分配策略

MVP 采用**人工认领**（claim）：`submitted` 的报价进入「待审核池」，任一 reviewer 可认领。并发 claim 通过 `UPDATE ... WHERE status='submitted' AND reviewer_id IS NULL` 的原子性保证先到者胜。

## 8. 导出管道

### 8.1 技术选型

| 格式 | 方案 | 工具 |
|---|---|---|
| PDF | HTML 模板 → Chrome 打印 | `chromedp` headless |
| Word (.docx) | 模板占位符替换 | `unioffice` |

字体：镜像内打包思源黑体/思源宋体（Source Han Sans/Serif，开源），保证中英混排一致。

### 8.2 数据组装

```go
// internal/export/viewmodel.go
type ExportViewModel struct {
    Header    HeaderSection      // serial_no, date, creator, company logo
    Customer  CustomerSection
    Trademark TrademarkSection   // name, image, categories
    Countries []CountryRow       // 每行：name_zh/en, madrid, acceptance/cycle, 四类费用
    Totals    TotalsSection      // 各分类合计 + 总价
    Materials []MaterialItem     // 所需资料（营业执照、委托书等）
    Remarks   []string           // 备注（如 10 项含，超出加收）
    MethodSection   LocalizedText  // 注册方式说明
    AdvantageSection LocalizedText  // 我司优势
}
```

加载：
1. `quotation + quotation_items`
2. `countries`（JOIN 获取 name_zh/en、马德里成员标记、周期）
3. `nice_categories`（JOIN 获取类别名）
4. 静态文案：`apps/api/internal/export/locales/{zh,en}.yaml`
5. 如需经典案例：MVP 硬编码 3-5 段放在 YAML，按 industry 粗筛。

### 8.3 模板组织

```
apps/api/internal/export/
├── templates/
│   ├── quotation_zh.html.tpl      # 中文版 HTML 模板
│   ├── quotation_en.html.tpl
│   ├── quotation_bilingual.html.tpl
│   ├── quotation_zh.docx
│   ├── quotation_en.docx
│   ├── quotation_bilingual.docx
│   └── assets/
│       ├── logo.png
│       └── fonts/SourceHanSans-Regular.otf
├── locales/
│   ├── zh.yaml
│   └── en.yaml
├── engine_pdf.go      # chromedp 渲染
├── engine_docx.go     # unioffice 替换
└── service.go         # 组装 view model + 写入 export_files
```

### 8.4 下载链接

1. 生成后文件存 `<EXPORT_STORAGE_ROOT>/quotations/<id>/<timestamp>-<format>-<lang>.pdf`
2. `export_files` 记录，含 `expires_at = now + 24h`
3. 下载接口 `GET /exports/{id}/download` 校验签名 + 过期 + 权限
4. 过期后由定时任务（MVP 可用数据库标记，先不跑定时清理；生产再加 cron）清理

## 9. API 设计

REST + `:action` 子路径风格。OpenAPI 3.1 定义在 `packages/shared/openapi.yaml`。

### 9.1 路由清单

```
/api/v1/
├── auth/
│   ├── POST  /login               {email, password} → Set-Cookie
│   ├── POST  /logout
│   ├── POST  /refresh
│   └── GET   /me
│
├── catalog/
│   ├── GET   /countries
│   ├── GET   /nice-categories
│   ├── PATCH /countries/{id}       (admin)
│   └── GET   /countries/{id}/pricing?effective_at=
│
├── customers/
│   ├── GET    /customers?q=&page=
│   ├── POST   /customers
│   ├── GET    /customers/{id}
│   └── PATCH  /customers/{id}
│
├── pricing/
│   ├── GET    /pricing-entries?country_id=&registration_method=&tier=&effective_at=
│   ├── POST   /pricing-entries       (reviewer/admin) — 创建新版本
│   ├── GET    /pricing-entries/{id}/history
│   └── POST   /pricing-entries/{id}:deprecate   (reviewer/admin)
│
├── quotations/
│   ├── POST   /quotations
│   ├── GET    /quotations?status=&mine=&page=
│   ├── GET    /quotations/{id}
│   ├── PATCH  /quotations/{id}                 (draft only)
│   ├── POST   /quotations/{id}:preview          → 不入库
│   ├── POST   /quotations/{id}:submit
│   ├── POST   /quotations/{id}:withdraw
│   ├── POST   /quotations/{id}:claim
│   ├── POST   /quotations/{id}:adjust
│   ├── POST   /quotations/{id}:approve
│   ├── POST   /quotations/{id}:reject
│   ├── POST   /quotations/{id}:copy
│   └── POST   /quotations/{id}:export           → {format, language}
│
├── exports/
│   └── GET   /exports/{id}/download             (签名 + 过期校验)
│
└── dashboard/
    └── GET   /dashboard/home                     (分角色聚合视图)
```

### 9.2 响应规约

列表：
```json
{
  "items": [...],
  "page": 1,
  "page_size": 20,
  "total": 143
}
```

错误：
```json
{
  "code": "ERR_INVALID_TRANSITION",
  "message": "不能从 approved 转到 draft",
  "details": { "from": "approved", "to": "draft" }
}
```

HTTP 状态码：`400` 参数错、`401` 未认证、`403` 无权限、`404` 不存在、`409` 状态冲突、`422` 业务规则校验失败、`500` 内部错误。

## 10. 前端结构

### 10.1 路由树（TanStack Router file-based）

```
apps/web/src/routes/
├── __root.tsx
├── (auth)/
│   └── sign-in.tsx
└── _authenticated/
    ├── route.tsx                  # 未登录 → /sign-in
    ├── index.tsx                  # dashboard 首页
    ├── quotations/
    │   ├── index.tsx              # 列表（按 status/mine 筛选）
    │   ├── new.tsx                # wizard 向导式
    │   └── $id.tsx                # 详情 + 审核
    ├── customers/
    │   ├── index.tsx
    │   └── $id.tsx
    ├── pricing/                   # reviewer/admin
    │   ├── index.tsx
    │   └── $id.tsx
    └── catalog/                   # admin only
        ├── countries.tsx
        └── nice-categories.tsx
```

路由守卫在 `_authenticated/route.tsx` 的 `beforeLoad` 判断登录态与角色；无权访问的路由渲染 `403` 错误页。

### 10.2 feature 目录

```
apps/web/src/features/
├── auth/
│   ├── sign-in-form.tsx
│   └── use-auth.ts              # zustand store: user, role, logout
├── dashboard/
│   ├── index.tsx
│   ├── pending-reviews-card.tsx
│   ├── recent-quotations-card.tsx
│   └── quick-actions.tsx
├── customers/
│   ├── list.tsx
│   ├── detail.tsx
│   └── customer-form.tsx
├── catalog/
│   └── countries-table.tsx
├── pricing/
│   ├── entries-table.tsx
│   ├── entry-form.tsx
│   └── history-drawer.tsx
└── quotations/
    ├── wizard/
    │   ├── step-customer.tsx
    │   ├── step-trademark.tsx
    │   ├── step-countries.tsx
    │   ├── step-config.tsx
    │   └── step-preview.tsx
    ├── list.tsx
    ├── detail.tsx
    ├── review-actions.tsx
    ├── price-breakdown.tsx
    ├── adjustment-panel.tsx
    └── export-dialog.tsx
```

### 10.3 状态管理

| 用途 | 工具 |
|---|---|
| 服务端数据（列表、详情） | `TanStack Query` |
| 当前登录用户、角色、权限 | `zustand` |
| 报价向导的暂存草稿（未提交） | `zustand` + localStorage 持久化 |
| 表单状态 | `react-hook-form` + `zod` |

## 11. 权限矩阵

| 资源 / 动作 | salesperson | reviewer | admin |
|---|:---:|:---:|:---:|
| 查看自建报价 | ✅ | ✅（全部） | ✅ |
| 创建/修改 draft 报价 | ✅（自建） | ✅（自建） | ✅ |
| 提交、撤回、复制自建报价 | ✅ | ✅ | ✅ |
| 认领/调整/通过/驳回（审核） | ❌ | ✅ | ✅ |
| 下载已通过报价 | ✅（自建） | ✅ | ✅ |
| 客户档案 CRUD | ✅（自建） | ✅（全部） | ✅ |
| 读成本模板 | ❌（但 preview 能看到计算结果） | ✅ | ✅ |
| 写成本模板（创建新版本 / deprecate） | ❌ | ✅ | ✅ |
| 维护字典（国家、类别） | ❌ | ❌ | ✅ |
| 用户管理 / 审计日志 | ❌ | ❌ | ✅ |

**实现**：Gin 路由组挂 `middleware.RequireRole(roles ...string)`，service 层对「自建」做 owner 校验。

## 12. 认证流程

1. `POST /auth/login` 校验 email+password → 签发 access token（15 分钟）+ refresh token（7 天）
2. 两个 token 均写入 httpOnly + SameSite=Lax cookie（生产加 Secure）
3. 前端请求时浏览器自动带 cookie；响应 401 时前端调 `/auth/refresh`；refresh 失败 → 登出
4. `POST /auth/logout` 清除 cookie + 后端记录 refresh token 黑名单（可选，MVP 不做）
5. 密码哈希使用 `argon2id`

**CSRF 防御**：同源 cookie + SameSite=Lax 已经防御大部分；对非 GET 请求要求 header `X-CSRF-Token`（从专门的 csrf cookie 读取，double-submit 模式）。

## 13. 审计日志

写入时机：所有非 GET 请求（除 `/auth/refresh`）；通过 `platform.AuditMiddleware` 统一写入。

字段：`user_id`、`action`（路径 + HTTP method）、`resource_type`、`resource_id`、`changes_json`（请求体过滤敏感字段）、`ip`、`user_agent`。

管理员在 `/audit` 路由页按资源、用户、时间筛选查看。

## 14. 测试策略

| 层级 | 工具 | 范围 | 目标 |
|---|---|---|---|
| 单元（后端） | `testing` + `testify` | 纯函数：pricing engine、状态迁移检查、调价算法 | 覆盖率 ≥ 80% |
| 集成（后端） | `testcontainers-go` + 真 PostgreSQL | repository、service、handler 全链路 | 关键路径全覆盖 |
| 单元（前端） | Vitest + Testing Library | 组件、hooks、zustand stores | 核心组件全覆盖 |
| 端到端 | Playwright | 关键场景 | 覆盖 §14.1 清单 |

### 14.1 必测场景

**后端**：
- 报价计算：单一 vs 马德里、tier A/B、多类别 per_class、超 10 项 per_item、调价（amount/percent/service_fee_only）
- 非马德里成员 + 马德里方式 → warning（不阻断）
- 成本版本切换：读特定 `effective_at` 得到历史价；改 pricing 后老 quotation 金额保持
- 所有合法状态迁移 + 所有非法迁移（断言 409）
- RBAC：每个受限路由各角色 allow/deny

**端到端**：
- 登录 → 新建客户 → 创建报价向导 5 步 → 预览 → 调价 → 提交 → reviewer 登录认领 → 通过 → 业务员下载 PDF

## 15. 配置与部署

### 15.1 本地开发

`docker-compose.yml`：
```yaml
services:
  postgres:
    image: postgres:16
    ports: ["5432:5432"]
    volumes: [postgres_data:/var/lib/postgresql/data]
  api:
    build: apps/api
    ports: ["8080:8080"]
    depends_on: [postgres]
    environment:
      DATABASE_URL: postgres://...
      JWT_SECRET: ...
      EXPORT_STORAGE_ROOT: /data/exports
  web:
    build: apps/web
    ports: ["5173:80"]
```

根 `Makefile` 提供 `make dev` / `make seed` / `make test` 捷径。

### 15.2 环境变量

| 变量 | 用途 |
|---|---|
| `DATABASE_URL` | Postgres 连接串 |
| `JWT_ACCESS_SECRET` / `JWT_REFRESH_SECRET` | 两个独立 secret |
| `JWT_ACCESS_TTL` / `JWT_REFRESH_TTL` | 过期时间（默认 15m / 7d）|
| `EXPORT_STORAGE_ROOT` | 导出文件目录 |
| `EXPORT_TTL_HOURS` | 导出文件过期（默认 24）|
| `COOKIE_SECURE` | 生产 true |
| `CORS_ORIGINS` | 允许的源（开发 http://localhost:5173） |

`.env.example` 在仓库根维护模板。

## 16. 迁移路径（从现状到 monorepo）

1. **Phase 1 — 准备骨架**
   - 根 `package.json` 改为 workspace root
   - 创建 `pnpm-workspace.yaml`、`apps/`、`packages/`
2. **Phase 2 — 迁移前端**
   - `git mv src apps/web/src`，同移其他前端资源（vite.config.ts、tsconfig*、public/ 等）
   - 前端 `package.json` 落到 `apps/web/`
   - 验证 `pnpm -C apps/web dev` 启动
3. **Phase 3 — 新建后端骨架**
   - `apps/api/go.mod` 初始化
   - 最小 Gin 服务 + 健康检查 + Postgres 连接
   - `docker-compose.yml` 打通本地开发环境
4. **Phase 4 — 按模块实施**（详见 plan）
   - auth → catalog → customer → pricing → quotation → export → dashboard

## 17. 范围外（非 MVP）

- 市场需求分析看板（P6）
- 供应商（境外代理机构）管理（P5）
- 企业微信 / 邮件通知
- 多币种
- i18n UI
- 多租户
- 离线 / PWA
- 细粒度 RBAC 表
- 生产级部署（K8s、监控、日志聚合）

这些在后续 sub-project 的独立 spec 中展开。

## 18. 关键风险与缓解

| 风险 | 缓解 |
|---|---|
| 报价计算引擎复杂度爆炸（国家、注册方式、tier、类别数、项数的笛卡尔积） | 引擎做纯函数 + 大量单元测试 + 明确的 `calc_basis` 枚举；不支持过于复杂的公式 |
| 成本表历史版本查询性能 | `(country_id, registration_method, service_tier, fee_item, valid_from)` 复合索引 |
| PDF/Word 双语排版差异 | 模板预先做好示例，与 1-2 个业务同事对齐后再冻结；双语版以中英双表/双栏为主 |
| 审核并发 claim | 数据库层面 `UPDATE ... WHERE reviewer_id IS NULL` 原子保证，辅以前端乐观刷新 |
| 从 shadcn-admin 模板迁移到 monorepo 的历史遗留 | 旧模板 feature（chats/tasks/apps）在迁移后删除，不保留 |

---

## 验收标准（Definition of Done）

MVP 视为完成当以下全部满足：

1. ✅ 三类角色能登录（admin 预置，其余由 admin 创建）。
2. ✅ 管理员能维护国家字典（enable/disable 某个国家）；尼斯分类 seed 不可变。
3. ✅ reviewer/admin 能在 pricing 页创建/修改成本条目，所有修改都是版本化的；能看历史。
4. ✅ 业务员能走完向导（5 步）创建 draft → 预览 → 调价 → 提交。
5. ✅ reviewer 能在待审核池认领 → 调整 → 通过 / 驳回。
6. ✅ 通过后业务员能下载中英双语 PDF 和 Word，内容结构符合 PDF 描述。
7. ✅ 已提交报价的金额在成本表后续修改后保持不变。
8. ✅ 权限矩阵里所有 ❌ 项返回 403。
9. ✅ 关键路径端到端测试全部通过。
10. ✅ docker-compose up 本地能一键启动完整环境。
