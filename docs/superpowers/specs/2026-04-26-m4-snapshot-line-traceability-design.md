# M4: SnapshotLine 溯源 + 历史价回查 — 设计规格

> **对标 roadmap**：`docs/superpowers/plans/2026-04-25-mvp-p0-roadmap.md` 的 M4 里程碑
> **前置**：无（不阻塞其他 milestone）
> **交付 DoD**：强化 #7 —— 审计能看见每条 snapshot line 出自哪条 pricing entry
> **日期**：2026-04-26

## 1. 背景与目标

当前 `quotation.SnapshotLine` 只有 `FeeItem` + `AmountCNYCents`，submit/adjust/preview 三条入口把 `pricing.Calculate` 产出的 `[]CalcLine` 拷到 snapshot 时**丢弃了 pricing entry 的主键**。审计视角下，一份冻结的报价无法回答"这条 ¥1200 的代理费，用的是哪一版定价"——尤其在 admin 多次 `CreateOrReplace` 覆盖老版本之后。

M4 的目标是：
1. 让 `SnapshotLine` 携带 `source_pricing_entry_id`
2. 开一个 `GET /pricing-entries/:id` 端点，让前端（或审计脚本）从 snapshot 线直接反查原 entry（包括 deprecated 的历史版本）
3. 不动签名、不动 UI、不迁移旧数据——MVP 增量最小

---

## 2. 与原 spec（§8.2 / §10）的偏差

| spec 原文 | 本 M4 决策 | 理由 |
|---|---|---|
| 独立 `quotation_items` 表 + `source_pricing_entry_id` 列 + FK | **保留现有 JSONB snapshot，在 `SnapshotLine` 里加字段** | 已有 snapshot+total+signature 的 CHECK 约束能冻结金额；再建独立表 + FK 收益边际，且 FK 会阻止 deprecate 老 entry。路线图已锁定此决策 |
| SnapshotLine 溯源字段 non-nullable | **`*uuid.UUID`（允许 null）** | Adjust 是 reviewer 手工覆写，按"orphan"语义处理；硬约束 non-null 等于强迫 reviewer UX 做"选条目"下拉，违背覆写的自由度 |
| 签名包含所有快照字段 | **签名仍为 v2（`fee_item + amount + total`），source_id 不计入** | 老快照无需重算；source_id 是元数据，不影响"金额是否被改过"的判定语义 |
| 数据迁移脚本回填历史 snapshot | **不迁移，旧 snapshot source=null** | MVP 阶段实际历史数据少；"按 submitted_at 匹配 effective 窗口"的脚本逻辑复杂且风险高，收益小 |

---

## 3. 范围

### 3.1 包含

- **后端**：
  - `pricing.CalcLine` 新增 `SourcePricingEntryID uuid.UUID`（non-null：CalcLine 必由 pricing entry 产生）
  - `pricing.Calculate` 循环体里把 `entry.ID` 填进 `CalcLine`
  - `quotation.SnapshotLine` 新增 `SourcePricingEntryID *uuid.UUID`（nullable：adjust orphan 行、旧 snapshot）
  - `Submit`、`Preview` 把 `calc.Lines[i].SourcePricingEntryID` 拷到 `snap.Lines[i].SourcePricingEntryID`
  - `Adjust`：请求体有就存，没有就 nil（无需 service 层加 validation）
  - 新端点 `GET /api/v1/pricing-entries/:id` + `Repository.GetByID` + `Service.GetByID` + `Handler.GetByID`
- **前端**：`types.ts` 的 `SnapshotLine` 加 `source_pricing_entry_id?: string | null`；MSW submit/preview handler 的 mock snapshot 带上 source_id 维持集成测试数据真实性
- **测试**：pricing calc + GetByID 单元测试；quotation submit/preview/adjust 的溯源传播测试；旧 snapshot 反序列化回归测试

### 3.2 不包含

- 任何 snapshot 渲染 UI（"查看来源" popover、diff 增强等全部推迟）
- PDF/DOCX 导出模板更新
- 签名升级到 v3
- 旧 snapshot 数据回填脚本
- Reviewer Adjust UX 增加"选 pricing entry 下拉"
- 批量查询端点 `GET /pricing-entries?ids=...`
- JSONB 内 `source_pricing_entry_id` 的外键约束（JSONB 字段本身不能建 FK；且希望 deprecated entry 仍可被历史 snapshot 引用）

---

## 4. 架构

### 4.1 数据模型变更

**`apps/api/internal/pricing/calc.go`**

```go
// CalcLine is one fee item included in the total.
type CalcLine struct {
    FeeItem              string    `json:"fee_item"`
    AmountCNYCents       int64     `json:"amount_cny_cents"`
    SourcePricingEntryID uuid.UUID `json:"source_pricing_entry_id"`
}
```

`Calculate` 的 append 改为：

```go
lines = append(lines, CalcLine{
    FeeItem:              e.FeeItem,
    AmountCNYCents:       e.AmountCNYCents,
    SourcePricingEntryID: e.ID,
})
```

**`apps/api/internal/quotation/dto.go`**

```go
// SnapshotLine is one priced fee item. Shape mirrors pricing.CalcLine,
// except SourcePricingEntryID is nullable — reviewer-adjusted lines
// (manual override) have no source entry, and legacy snapshots written
// before M4 also decode to nil.
type SnapshotLine struct {
    FeeItem              string     `json:"fee_item"`
    AmountCNYCents       int64      `json:"amount_cny_cents"`
    SourcePricingEntryID *uuid.UUID `json:"source_pricing_entry_id,omitempty"`
}
```

指针 + `omitempty`：
- 老 JSONB 缺字段 → 反序列化为 nil（无需改 Decode）
- Submit/Preview 写入 → 取地址赋非 nil
- Adjust 的请求没带就是 nil

### 4.2 传播链

```
Submit / Preview:
  pricing.Calculate(entries) → []CalcLine{SourcePricingEntryID: entry.ID}
    └─ for loop copy → []SnapshotLine{SourcePricingEntryID: &calcLine.ID}
    └─ json.Marshal → JSONB 存库

Adjust:
  AdjustRequest.Lines (来自前端 JSON)
    └─ 每行 SourcePricingEntryID 字段,前端有就有、没有就 nil
    └─ 直接拷进 snapshot,不做 validation
```

**签名（不变）**：

```go
// apps/api/internal/pricing/calc.go - signature()
fmt.Fprintf(h, "v2|%s|%s|", in.CountryID, in.ServiceTier)
for _, l := range lines {
    fmt.Fprintf(h, "%d:%s=%d;", len(l.FeeItem), l.FeeItem, l.AmountCNYCents)
}
fmt.Fprintf(h, "=%d", total)
```

CalcLine 多了 `SourcePricingEntryID` 但 `signature()` 里不引用它——源 ID 不计入摘要（决策 D2）。

### 4.3 新端点：`GET /api/v1/pricing-entries/:id`

```
apps/api/internal/pricing/
├── repository.go   +  GetByID(ctx, id) (*PricingEntry, error)
├── service.go      +  GetByID(ctx, id) (*PricingEntry, error)  (thin wrapper)
├── handler.go      +  GetByID gin handler
└── router.go       +  router.GET("/pricing-entries/:id", h.GetByID)
```

**认证**：`_authenticated` 下，任何 role 可调。历史回查是读操作，不限制 role。

**响应 200**：
```json
{
  "id": "uuid",
  "country_id": "uuid",
  "service_tier": "basic",
  "fee_item": "代理费",
  "amount_cny_cents": 120000,
  "notes": null,
  "effective_from": "2025-01-01",
  "effective_to": "2026-03-15",   // deprecate 后非 null
  "created_by": "uuid",
  "created_at": "2025-01-01T10:00:00Z",
  "updated_at": "2025-01-01T10:00:00Z"
}
```

**错误码**：

| 场景 | code | status |
|---|---|---|
| :id 非法 UUID | `ERR_INVALID_ID` | 400 |
| 不存在 | `ERR_NOT_FOUND` | 404 |
| 未登录 | `ERR_UNAUTHORIZED` | 401 |

**路由顺序**：`/pricing-entries/history` 已经注册，`/:id` 不会抢它的路径（Gin 优先匹配字面量）。但为了稳妥，在 `router.go` 的 `RegisterAuthedRoutes` 里把 `/:id` 放在 `/history` 之后注册。

### 4.4 前端

仅 `apps/web/src/features/quotation/types.ts`：

```ts
export interface SnapshotLine {
  fee_item: string
  amount_cny_cents: number
  source_pricing_entry_id?: string | null  // M4 新增
}
```

MSW 的 `POST /quotations/:id/submit` handler 现在 mock 出的 snapshot 里，每行也要带上 `source_pricing_entry_id`（用当时 seeded 的 pricing entry id；保持 M3 集成测试里的 snapshot 形状真实）。同样处理 `POST /quotations/preview` handler。

`QuotationSnapshotView` 不变。

---

## 5. API 契约

### `GET /api/v1/pricing-entries/:id` (新)

**Request**: URL 参数即 entry id，无 body。

**Response 200**: 完整 `PricingEntry` JSON（如 §4.3 示例）

**错误**:

| 场景 | code | status |
|---|---|---|
| :id 非 UUID | `ERR_INVALID_ID` | 400 |
| entry 不存在 | `ERR_NOT_FOUND` | 404 |
| 未登录 | `ERR_UNAUTHORIZED` | 401 |

### 已有端点的响应形状变化

- `POST /quotations/:id/submit`：响应体里 `snapshot.lines[i]` 多一个 `source_pricing_entry_id` 字段
- `POST /quotations/preview`：`lines[i]` 同上
- `POST /quotations/:id/adjust`：同上（若请求未带则缺失）
- `GET /quotations/:id`：同上（若老 snapshot 则缺失）

---

## 6. 错误处理与边界

- **老 snapshot 读取**：JSON 反序列化遇到缺字段 → `*uuid.UUID` 为 nil。`DecodeSnapshot` 逻辑无需改动。新增 `TestDecodeLegacySnapshot_SourceNil` 固化这个行为
- **Deprecated entry 查询**：`GET /:id` 不区分 `effective_to` 是否 null，deprecate 过的老 entry 仍能查到——这就是"历史价回查"的核心
- **Entry 被物理删除**：当前 `pricing` 模型不支持删除（只 deprecate），所以 ID 始终有效。若未来引入删除，snapshot 的 source_id 变成悬挂引用；暂不处理
- **Adjust 请求的 source 字段伪造**：reviewer 理论上能在 AdjustRequest 里塞一个伪造的 source_pricing_entry_id。Service 不做 validation，信任前端；因为 snapshot 是冻结的"事实记录"，reviewer 修改已经在 diff_json 里审计——伪造 source_id 不会让金额变、也不会让历史查不到（端点还会返回真实 entry）
- **前端读 null source**：未来的 UI（非 M4 范围）在渲染 snapshot line 时，若 `source_pricing_entry_id` 为 null/undefined，应显示"人工调整"或"未记录"，不触发 lookup

---

## 7. 测试策略

### 7.1 pricing 单元测试

**`calc_test.go`**（扩展现有）：
- `TestCalculate_CarriesSourceIDs` —— 新 case：2 条 entry 走 Calculate，断言每个 CalcLine 的 SourcePricingEntryID 等于对应 entry.ID
- 现有 `TestCalculate_SignatureStableAcrossInputOrder` / `TestCalculate_SignatureChangesWithAmount` 不变，证明 source_id 不影响签名

**`repository_test.go`**（新增 `TestRepo_GetByID_*`）：
- `TestRepo_GetByID_Success` —— 先 Create 一行，GetByID 返回一致
- `TestRepo_GetByID_NotFound` —— 随机 UUID，返回 `(nil, ErrNotFound)`
- `TestRepo_GetByID_Deprecated` —— Deprecate 后再 GetByID，仍能拿到（确认不过滤 effective_to）

**`handler_test.go`**（新增 `TestHandler_GetByID_*`）：
- `TestHandler_GetByID_OK` —— 200 + body 完整
- `TestHandler_GetByID_InvalidID` —— 非 UUID → 400
- `TestHandler_GetByID_NotFound` —— 404
- `TestHandler_GetByID_Unauthenticated` —— 401

### 7.2 quotation 单元测试

**`service_test.go`**（新增）：
- `TestService_Submit_CarriesSourceIDs` —— 提交 draft 后 DecodeSnapshot，断言每行 SourcePricingEntryID 非 nil 且等于对应 pricing entry
- `TestService_Preview_CarriesSourceIDs` —— 同上，针对 PreviewResponse.Lines
- `TestService_Adjust_PreservesRequestSources` —— AdjustRequest 里带 source_id，snapshot 里存住；不带则 nil

**`snapshot_test.go`**（新增）：
- `TestDecodeLegacySnapshot_SourceNil` —— 手构造 `{"lines":[{"fee_item":"x","amount_cny_cents":100}],"total_cny_cents":100,"signature":"..."}` 的 JSONB，Decode 后 Lines[0].SourcePricingEntryID == nil

### 7.3 集成测试（backend）

**`quotation/handler_test.go`** 扩展 —— 完整链路：
- 创建 pricing entry → 创建 customer → 创建 draft → submit → GET /quotations/:id，抓出 snapshot.lines[0].source_pricing_entry_id → 用它调 `GET /pricing-entries/:id` → 返回 200 + 对应的 entry

### 7.4 前端

无新增测试。但 MSW 的 submit/preview handler 需要在 mock snapshot 的 lines 里带上 `source_pricing_entry_id` —— 用当时 seeded 的 pricing entry ID。确保 M3 的集成测试（`quotation.integration.test.tsx`）不回归。

---

## 8. 任务拆分（预算 6）

| # | 任务 | 文件/范围 |
|---|---|---|
| **T1** | `pricing.CalcLine` 加 `SourcePricingEntryID` + `Calculate` 填值 + 单测扩展 | `pricing/calc.go`、`pricing/calc_test.go` |
| **T2** | `quotation.SnapshotLine` 加 `SourcePricingEntryID *uuid.UUID` + Submit/Preview/Adjust 传播 + 3 个新单测 | `quotation/dto.go`、`service.go`、`service_test.go`、`snapshot_test.go` |
| **T3** | `pricing.Repository.GetByID` + `Service.GetByID` + `Handler.GetByID` + 路由 + 4 个测试（含 deprecated case） | `pricing/repository.go`、`service.go`、`handler.go`、`router.go`、对应 `_test.go` |
| **T4** | 端到端集成测试：create→submit→读 snapshot.source_id→GET /pricing-entries/:id | `quotation/handler_test.go` |
| **T5** | 前端 `types.ts` 增字段 + MSW submit/preview handler 的 mock snapshot 也填 source_id（用 seeded entry id） | `apps/web/src/features/quotation/types.ts`、`apps/web/src/test-utils/msw/handlers.ts` |
| **T6** | 旧 snapshot 反序列化回归测试 + 更新 ADR/`_test.go` 里的注释说明 M4 决策 | `quotation/snapshot_test.go`、README/注释 |

**执行顺序**：T1 → T2（依赖 T1 CalcLine 新字段）→ T3（独立，可与 T2 并行）→ T4（依赖 T2+T3）→ T5（独立，可与 T3/T4 并行）→ T6（收尾）

---

## 9. 自检清单

- [x] 所有偏差（§2）都有理由且与用户对齐（5 次 AskUserQuestion 锁定）
- [x] SnapshotLine 用 `*uuid.UUID` 保证老 snapshot 反序列化不出错
- [x] `pricing.CalcLine.SourcePricingEntryID` 非 nullable（CalcLine 必来自 entry）
- [x] 签名计算不引用 source_id（§4.2）
- [x] `GET /:id` 不过滤 effective_to，deprecated entry 可查
- [x] 前端仅改 types；现有组件/路由零改动
- [x] MSW submit/preview handler 更新 mock，避免集成测试回归
- [x] 测试覆盖：happy path / not-found / unauthorized / legacy-snapshot / deprecated-entry 5 类场景
