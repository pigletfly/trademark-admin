# M3: 报价向导 + Preview API — 设计规格

> **对标 roadmap**：`docs/superpowers/plans/2026-04-25-mvp-p0-roadmap.md` 的 M3 里程碑
> **前置**：M1（导出 PDF，已完成）+ M2（调价流，已完成）
> **交付 DoD**：#4 业务员能走完向导（5 步）创建 draft → 预览 → 提交
> **日期**：2026-04-26

## 1. 背景与目标

当前 `QuotationFormSheet` 是一个 4 字段的单页 Sheet（`customer_id / country_id / service_tier / notes`），业务员没有"先预览再提交"的能力——只能直接 `POST /quotations` 创建草稿，然后在详情页手动 `POST /quotations/:id/submit` 提交审核。

M3 的目标是把新建/编辑报价流程拆成 5 步向导，并在最后一步调用一个**不入库**的 `:preview` API 预览定价，让业务员能：
1. 按步骤填写报价要素（一次一个焦点，减少错填）
2. 在提交前看到将要冻结的 lines 和 total
3. 一键"保存为草稿"或"保存并提交"

---

## 2. 与原 spec（§10.2 / §10.1）的偏差

| spec 原文 | 本 M3 决策 | 理由 |
|---|---|---|
| `wizard/step-customer / step-trademark / step-countries / step-config / step-preview` | `step-customer / step-country / step-tier / step-notes / step-preview` | 后端 quotation 模型还没有 trademark / 多国 / nice_categories 字段；M3 只做"现有模型之上的向导"，不夹带后端字段扩展。trademark + 多国留给独立 milestone（M6/M7） |
| `POST /quotations/{id}:preview`（带 id） | `POST /quotations/preview`（**不带 id**） | 草稿持久化选了"完全客户端（zustand + localStorage）"——最后一步才落库，preview 时还没有 id |
| `routes/_authenticated/quotations/new.tsx` | `new.tsx` + `$id.edit.tsx`（新增） | 保留 spec 原路由结构；编辑 draft 从详情页的 Sheet 升级为独立路由，和新建共享 wizard 组件 |
| export dialog（`export-dialog.tsx`） | **不重做**，保留 M1 的 `quotation-export-actions.tsx`（dropdown 形式已满足） | M1 已交付完整导出 UX（PDF 三语、DOCX 双语），再做 Dialog 只是外观改动，不算 MVP 补齐 |

---

## 3. 范围

### 3.1 包含

- **后端**：`POST /quotations/preview` 端点 + `Service.Preview` 方法 + DTO
- **前端**：
  - 新路由 `/quotations/new` 和 `/quotations/$id/edit`，共享 `QuotationWizard` 组件
  - 5 个 step 组件：`step-customer` / `step-country` / `step-tier` / `step-notes` / `step-preview`
  - `wizard-store.ts`：zustand store + persist middleware，localStorage key 含 `user_id`（切换用户自动隔离）
  - "续写 banner"：进 `/quotations/new` 检测 localStorage 残留时，顶部提示"检测到未完成的草稿，是否继续？"
  - preview 步双按钮：**保存草稿** / **保存并提交**
  - 列表页 `新建报价` 按钮改为 `<Link to="/quotations/new">`
  - 详情页 `编辑草稿` 按钮改为 `<Link to="/quotations/$id/edit">`（仅 draft 可见）
- **测试**：后端单测 + 集成；前端 wizard-store 单测 + 5 场景集成测试

### 3.2 不包含

- quotation 表结构变更（trademark / 多国 / nice_categories）
- pricing.Calculate 的 per_class / 多国聚合
- export dialog 改造
- `:claim`（reviewer 认领）——spec 明确范围外
- pricing 乐观锁（preview 的 signature vs submit 重算后的 signature 比对校验）

---

## 4. 架构

### 4.1 后端

```
apps/api/internal/quotation/
├── dto.go         +  PreviewRequest, PreviewResponse
├── service.go     +  Preview(ctx, input) (*PreviewResult, error)
├── handler.go     +  Preview handler
└── router.go      +  POST /quotations/preview 注册到 RegisterAuthedRoutes
```

`Service.Preview` 流程：

```
Input: {CustomerID, CountryID, ServiceTier}
  └─ 校验 tier ∈ {basic,standard,premium}           → ErrInvalidTier (400)
  └─ 校验 customer 存在(读 customer repo)            → ErrNotFound    (404)
  └─ 读 active pricing entries (by country+tier)    
  └─ pricing.Calculate(entries, input)              → ErrMissingPricing (422) if no lines
  └─ 返回 CalcResult (lines, total, signature)
```

- **权限**：任何已认证用户可调（salesperson/reviewer/admin），因为 preview 无副作用 + 数据源已是 pricing 字典
- **不落库**：这条 API 只读 pricing + customer 校验，不写任何表
- **错误处理复用**：`ErrInvalidTier` / `ErrNotFound` / `ErrMissingPricing` 都已存在，`writeServiceErr` 已映射

### 4.2 前端

```
apps/web/src/features/quotation/
├── wizard/                                    (M3 新增)
│   ├── wizard-store.ts                        zustand + persist(localStorage)
│   ├── quotation-wizard.tsx                   Stepper 外壳,上/下步+进度条
│   ├── resume-banner.tsx                      "检测到未完成的草稿"
│   ├── steps/
│   │   ├── step-customer.tsx
│   │   ├── step-country.tsx
│   │   ├── step-tier.tsx
│   │   ├── step-notes.tsx
│   │   └── step-preview.tsx                   调 :preview + 两按钮
│   └── hooks/
│       └── use-preview.ts                     TanStack Query useQuery
├── hooks/use-quotation-mutations.ts           (+)useCreateAndSubmit 组合
└── ...(其余不动)

apps/web/src/routes/_authenticated/quotations/
├── index.tsx              (已有)
├── new.tsx                (新增) mode="create"
├── $id.tsx                (已有)
├── $id.edit.tsx           (新增) mode="edit",initial={server draft}
└── $id.print.tsx          (已有)
```

### 4.3 Wizard 状态管理

```ts
// wizard-store.ts
interface WizardDraft {
  customer_id: string
  country_id: string
  service_tier: ServiceTier   // default 'basic'
  notes: string
}

interface WizardState {
  currentStep: 0 | 1 | 2 | 3 | 4
  draft: WizardDraft
  editingId: string | null    // null=create mode, set=edit mode

  setStep(step: number): void
  patchDraft(patch: Partial<WizardDraft>): void
  reset(): void
  loadForEdit(id: string, serverDraft: Quotation): void
}

// persist middleware key: `quotation-wizard-draft:${userId}`
// userId 取自 useAuthStore.getState().auth.user?.id;匿名/未登录时禁用 persist
// key 天然按用户隔离,登出不主动清理(同一用户再登录恢复草稿是合理 UX)
```

**Store 生命周期**：
- **新建模式**（`/quotations/new`，editingId=null）：persist 到 localStorage，支持跨会话续写
- **编辑模式**（`/quotations/$id/edit`，editingId=id）：`$id.edit.tsx` 的 `useEffect` 在 unmount 时调 `reset()`，避免"未保存的 edit 修改"泄漏到下次进 new 页。刷新 edit 页时 `loadForEdit` 从 server 重新覆盖（丢失会话内未保存的改动，这是预期行为，参见 §7）

**步骤切换**：`setStep` 做"不倒退验证"——用户要前进第 N 步必须满足前 N-1 步的字段（customer_id、country_id、service_tier 非空）。点回上一步无限制。

**续写 banner**：`new.tsx` 进入时：
- 若 `draft` 非全空（customer_id 或 country_id 或 notes 非空），顶部显示 banner：`[继续 | 放弃]`
- `放弃` → `reset()` → banner 消失
- `继续` → banner 消失，wizard 直接用 localStorage 的值

**编辑模式**：`$id.edit.tsx` 进入时：
- `useQuotation(id)` 拉取当前 quotation
- 若 `status !== 'draft'`，跳回 `/quotations/$id`（编辑只对 draft 开放）
- `loadForEdit(id, quotation)` 覆盖 localStorage 里的任何残留

**提交成功后**：任何路径下 `reset()` + `localStorage.removeItem` + 跳 `/quotations/$id`

---

## 5. API 契约

### `POST /quotations/preview` (新)

**Request**:
```json
{
  "customer_id": "uuid",
  "country_id": "uuid",
  "service_tier": "basic"
}
```

**Response 200**:
```json
{
  "lines": [
    { "fee_item": "代理费", "amount_cny_cents": 120000 },
    { "fee_item": "申请费", "amount_cny_cents": 30000 }
  ],
  "total_cny_cents": 150000,
  "signature": "sha256-hex-64-chars"
}
```

**错误**:

| 场景 | code | status |
|---|---|---|
| body 缺字段 / 非法 UUID | `ERR_INVALID_BODY` | 400 |
| tier 不在枚举 | `ERR_INVALID_TIER` | 400 |
| customer_id 不存在 | `ERR_NOT_FOUND` | 404 |
| country_id 没有活跃 pricing | `ERR_MISSING_PRICING` | 422 |
| 未登录 | `ERR_UNAUTHORIZED` | 401 |

---

## 6. Preview 步交互

### 6.1 数据获取

- 使用 `useQuery`（非 mutation），`queryKey = ['quotation-preview', customer_id, country_id, service_tier]`
- `staleTime: 5 * 60_000` —— 相同输入 5 分钟缓存，切回上一步修改后自动重新拉取
- `enabled: customer_id && country_id && service_tier` —— 缺字段不发

### 6.2 UI

```
┌─────────────────────────────────────┐
│ 预览报价 / Preview                   │
├─────────────────────────────────────┤
│ 客户: 张三公司                       │
│ 国家: 美国 (US)                      │
│ 级别: standard                       │
│ 备注: 仅中文版                        │
├─────────────────────────────────────┤
│ 明细:                                │
│   代理费                 ¥1,200.00   │
│   申请费                 ¥300.00     │
│ ───────────────────────────────────  │
│ 合计:                    ¥1,500.00   │
│ 签名: a1b2c3d4...（前 8 位）          │
├─────────────────────────────────────┤
│        [← 上一步]                    │
│    [保存草稿]  [保存并提交]           │
└─────────────────────────────────────┘
```

- **加载中**：spinner + "计算中…"
- **出错**：显示错误消息 + `[重试]` 按钮；两个保存按钮禁用
- **成功**：显示 lines + total，两个保存按钮启用

### 6.3 双按钮语义

**新建模式**：

| 按钮 | 行为 |
|---|---|
| 保存草稿 | `POST /quotations` → 成功后 `reset() + 跳 /quotations/$id` + toast "草稿已创建" |
| 保存并提交 | `POST /quotations` → `POST /quotations/:id/submit` → 跳 `/quotations/$id` + toast "报价已提交待审核" |

**编辑模式**：

| 按钮 | 行为 |
|---|---|
| 保存修改 | `PATCH /quotations/:id` → 跳 `/quotations/$id` + toast "草稿已保存" |
| 保存并提交 | `PATCH /quotations/:id` → `POST /quotations/:id/submit` → 跳 `/quotations/$id` + toast "报价已提交待审核" |

组合 API 调用在 `useCreateAndSubmit` / `useUpdateAndSubmit` hook 里封装，失败时前者回滚（若第一步成功第二步失败，保留 draft 让用户重试提交）。

---

## 7. 错误处理与边界

- **preview `ERR_MISSING_PRICING`**：toast "该国家/级别暂无定价，请联系管理员"；保留 wizard 状态，让用户回上一步改 country/tier
- **保存草稿失败**：toast 错误，**不清 localStorage**，用户重试或改内容后重试
- **保存并提交 - 第 1 步失败（POST /quotations）**：同上
- **保存并提交 - 第 2 步失败（POST /submit）**：跳 `/quotations/$id`（已有 draft）+ toast "草稿已创建，但提交失败，请在详情页重试"。localStorage 清掉（因为 server 上有 draft 了）
- **编辑模式下 status 变了**（用户打开 edit 页时 reviewer 已认领/通过）：`$id.edit.tsx` 的 `useEffect` 监视 `q.status`，非 draft 时立刻跳回 `/quotations/$id` + toast "报价状态已变更，无法编辑"
- **续写 banner 的边界**：用户先在 user A 下留了草稿，退出登录，换 user B 登录，进 `/quotations/new` 看到的 draft 是 B 的空草稿（localStorage key 含 user_id）。A 的草稿保留在 localStorage 但被隔离

---

## 8. 测试策略

### 8.1 后端

**`service_test.go`**（新增 `TestService_Preview_*`）：
- `TestService_Preview_Success` —— 有匹配 pricing，返回非空 lines + total 匹配
- `TestService_Preview_InvalidTier` —— tier="foo" → ErrInvalidTier
- `TestService_Preview_CustomerNotFound` —— 随机 UUID → ErrNotFound
- `TestService_Preview_NoActivePricing` —— country 没有 active entries → ErrMissingPricing

**`handler_test.go`**（新增 `TestHandler_Preview_*`）：
- `TestHandler_Preview_OK` —— 200 + body shape 正确
- `TestHandler_Preview_BadBody` —— 缺字段 → 400 ERR_INVALID_BODY
- `TestHandler_Preview_NotFoundCustomer` —— 404
- `TestHandler_Preview_MissingPricing` —— 422
- `TestHandler_Preview_Unauthenticated` —— 401

### 8.2 前端

**`wizard-store.test.ts`**（新增，vitest unit）：
- `reset()` 清所有字段、步骤归零
- `patchDraft()` 保留其他字段
- `loadForEdit()` 覆盖 draft + 设置 editingId
- 不同 userId 的 store 实例 localStorage 互不干扰

**`quotation.integration.test.tsx`**（扩展，vitest-browser-react）：
- `新建 → 5 步 → 保存草稿` —— list 出现新 draft，详情页状态=草稿
- `新建 → 5 步 → 保存并提交` —— 详情页状态=已提交，snapshot 存在
- `编辑 → 改 tier → 保存并提交` —— 详情页 snapshot 用新 tier 的价格
- `续写 banner` —— localStorage 预置残留 → 进 /quotations/new → 看到 banner → 点"放弃" → 表单清空
- `Preview 错误` —— MSW stub `:preview` 返回 422 → 显示 retry + 两按钮禁用

### 8.3 手动验收（post-commit smoke）

完整链路：
1. `docker compose up -d postgres gotenberg`
2. `go run ./cmd/server` + `pnpm vite`
3. 以 salesperson 登录 → `/quotations/new` → 走完 5 步 → 保存并提交 → 详情页
4. 切到 reviewer → 列表看到 submitted → 通过 → 回 salesperson → 导出 PDF
5. 回到 salesperson，进 `/quotations/$id/edit`（选一个 draft）→ 改 tier → 保存修改 → 详情页 status=草稿

---

## 9. 预估任务数

| 组 | 任务 |
|---|---|
| 后端 (3) | T1 Preview DTO / T2 Service.Preview + 单测 / T3 Handler + Router + 单测 |
| 前端 store (2) | T4 wizard-store.ts + 单测 / T5 useCreateAndSubmit + useUpdateAndSubmit hooks |
| 前端路由 + 入口 (3) | T6 routes/new.tsx / T7 routes/$id.edit.tsx / T8 列表页 + 详情页入口改 Link,并删除旧 `quotation-form-sheet.tsx` |
| 前端 wizard 骨架 (1) | T9 QuotationWizard 外壳 + 步进条 + 上/下步按钮 |
| 前端 steps (5) | T10-T14 step-customer / step-country / step-tier / step-notes / step-preview |
| 前端 preview 集成 (2) | T15 use-preview hook / T16 resume-banner |
| 测试 (2) | T17 wizard-store 单测 / T18 integration 5 场景 |

约 **18 个任务**（roadmap 预算 16；超出的 2 个是 `wizard-store` 单测和续写 banner，都是"保证体验完整"的增量）。

执行顺序：**T1-T3（后端）→ T4-T5（store + hooks）→ T6-T9（路由骨架）→ T10-T14（steps）→ T15-T16（preview 细节）→ T17-T18（测试）**

---

## 10. 自检清单

- [x] 所有偏差（§2）都有 justification 且与用户对齐决策
- [x] localStorage key 含 user_id，切换用户自动隔离；edit 模式 unmount 时 reset 避免泄漏
- [x] 编辑模式下，非 draft 状态立刻跳回详情页（不给用户"打开编辑页看到被审核中的数据"的机会）
- [x] 续写 banner 有"放弃"出口，避免用户被卡在过期草稿里
- [x] preview 错误不污染 wizard 状态
- [x] 保存并提交 - 提交失败后 draft 已存在时，清 localStorage 避免"幽灵草稿"
- [x] 测试覆盖成功/失败/权限/边界 4 类场景
