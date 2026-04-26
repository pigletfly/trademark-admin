# M4: SnapshotLine 溯源 + 历史价回查 — 实装计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `SnapshotLine` 携带 `source_pricing_entry_id`，新增 `GET /pricing-entries/:id` 端点用于历史回查；旧 snapshot 保持兼容，签名不变，不做 UI。

**Architecture:** 在现有 JSONB snapshot 基础上扩展字段（`pricing.CalcLine` 加非 null `SourcePricingEntryID`；`quotation.SnapshotLine` 加 nullable `*uuid.UUID`），Submit/Preview 自动传播。Adjust 的行信任请求体——前端没带就是 null（orphan）。新端点复用现有 `Repository.GetByID`（已不过滤 `effective_to`，天然支持回查 deprecated 条目）。

**Tech Stack:** Go 1.25 + Gin + GORM（后端）；TypeScript + React（前端 types-only）。测试：Go testing + testcontainers-go（Postgres）+ vitest MSW（前端数据一致性）。

---

## 与 spec 的偏差（重要，执行前对齐）

| spec §4.3 原文 | plan 实装 | 理由 |
|---|---|---|
| 端点注册在 `_authenticated` 下，任何 role 可调 | **注册在现有 `RegisterReadRoutes` 下（reviewer+admin）** | 现有 `GET /pricing-entries` + `/history` 均在 `reviewerAdminGroup`；保持一致最小摩擦。salesperson 目前没有 UI 会直接打 pricing 端点（他们用 `POST /quotations/preview`）。若未来销售侧需要回查，另起 milestone 调整权限组 |
| 新建 `Repository.GetByID` | **复用既有 `Repository.GetByID`** | 该方法已为 `Deprecate` 而存在（`pricing/repository.go:74-84`），天然 `WHERE id = ?`，不过滤 `effective_to`——正好满足"deprecated 可查"的需求 |

两处偏差均为现状最小侵入性选择，不改变功能。

---

## 文件结构

### 后端新增/修改

| 路径 | 职责 | 变更 |
|---|---|---|
| `apps/api/internal/pricing/calc.go` | `CalcLine` 结构 + `Calculate` 填值 | 加字段 `SourcePricingEntryID uuid.UUID` + 赋 `e.ID` |
| `apps/api/internal/pricing/calc_test.go` | Calculate 单测 | 扩展 `TestCalculate_FiltersDeprecatedAndOtherTier` 断言 source_id；新增 `TestCalculate_CarriesSourceIDs` |
| `apps/api/internal/pricing/service.go` | 领域服务 | 新增 `GetByID` 方法（thin wrapper） |
| `apps/api/internal/pricing/handler.go` | HTTP handler | 新增 `GetByID` handler |
| `apps/api/internal/pricing/router.go` | 路由注册 | `RegisterReadRoutes` 里追加 `g.GET("/:id", h.GetByID)` |
| `apps/api/internal/pricing/repository_test.go` | Repo 集成测试 | 新增 `TestRepo_GetByID_ReturnsDeprecatedEntry` |
| `apps/api/internal/quotation/dto.go` | `SnapshotLine` 结构 | 加 `SourcePricingEntryID *uuid.UUID` |
| `apps/api/internal/quotation/service.go` | Submit/Preview 拷贝循环 | `snap.Lines[i].SourcePricingEntryID = &calc.Lines[i].SourcePricingEntryID` |
| `apps/api/internal/quotation/service_test.go` | 单测 | 新增 `TestSubmit_CarriesSourceIDs`、`TestPreview_CarriesSourceIDs`、`TestAdjust_RequestSourcesPreserved` |
| `apps/api/internal/quotation/snapshot_test.go` | Decode 测试 | 新增 `TestDecodeLegacySnapshot_SourceNil` |
| `apps/api/internal/quotation/handler_test.go` | E2E 集成 | 新增 `TestHandler_SnapshotSourceIDs_LookupPricingEntry` |

### 前端修改

| 路径 | 职责 | 变更 |
|---|---|---|
| `apps/web/src/features/quotation/types.ts` | 类型定义 | `SnapshotLine` 加 `source_pricing_entry_id?: string \| null` |
| `apps/web/src/test-utils/msw/handlers.ts` | MSW mock | `/quotations/:id/submit` + `/quotations/preview` 的 mock snapshot 每行加 `source_pricing_entry_id: e.id` |

---

## Task 1: `pricing.CalcLine` 加 `SourcePricingEntryID` + `Calculate` 填值

**Files:**
- Modify: `apps/api/internal/pricing/calc.go`
- Modify: `apps/api/internal/pricing/calc_test.go`

- [ ] **Step 1: 写失败测试 — 断言 Calculate 输出的每个 CalcLine 带有源 entry ID**

在 `apps/api/internal/pricing/calc_test.go` 文件末尾追加：

```go
// TestCalculate_CarriesSourceIDs verifies every CalcLine in the result
// carries the originating PricingEntry.ID — M4 traceability requirement.
func TestCalculate_CarriesSourceIDs(t *testing.T) {
	c := uuid.New()
	a := mustEntry(t, c, "basic", "aa_fee", 1000, false)
	b := mustEntry(t, c, "basic", "bb_fee", 2000, false)

	res, err := Calculate([]PricingEntry{a, b}, CalcInput{c, "basic"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("lines: want 2, got %d", len(res.Lines))
	}
	// Lines sort by fee_item so aa_fee comes first.
	if res.Lines[0].SourcePricingEntryID != a.ID {
		t.Errorf("line[0] source: want %s, got %s", a.ID, res.Lines[0].SourcePricingEntryID)
	}
	if res.Lines[1].SourcePricingEntryID != b.ID {
		t.Errorf("line[1] source: want %s, got %s", b.ID, res.Lines[1].SourcePricingEntryID)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/pricing/ -run TestCalculate_CarriesSourceIDs -v
```
Expected: FAIL with `res.Lines[0].SourcePricingEntryID undefined (field not defined)`

- [ ] **Step 3: 修改 `CalcLine` 加字段**

在 `apps/api/internal/pricing/calc.go` 中修改 `CalcLine` 结构（当前在第 20-24 行）：

```go
// CalcLine is one fee item included in the total. SourcePricingEntryID
// is the ID of the PricingEntry row this line was derived from — lets
// downstream snapshot consumers trace the line back for audit.
type CalcLine struct {
	FeeItem              string    `json:"fee_item"`
	AmountCNYCents       int64     `json:"amount_cny_cents"`
	SourcePricingEntryID uuid.UUID `json:"source_pricing_entry_id"`
}
```

- [ ] **Step 4: 更新 `Calculate` 的 append 循环**

在同文件中，`Calculate` 函数的 append 调用（第 64 行）改为：

```go
lines = append(lines, CalcLine{
    FeeItem:              e.FeeItem,
    AmountCNYCents:       e.AmountCNYCents,
    SourcePricingEntryID: e.ID,
})
```

- [ ] **Step 5: 运行测试确认通过**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/pricing/ -v
```
Expected: 所有现有测试 + 新测试 PASS。现有 `TestCalculate_SignatureStableAcrossInputOrder` 和 `TestCalculate_SignatureChangesWithAmount` 不回归（签名函数不引用 SourcePricingEntryID）。

- [ ] **Step 6: 构建 + vet 全局**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go build ./... && go vet ./...
```
Expected: 成功。注意：`quotation/service.go` 里 Submit/Preview/Adjust 的拷贝循环现在丢弃了新字段，但编译仍能通过——T2 会补上传播。

- [ ] **Step 7: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/pricing/calc.go apps/api/internal/pricing/calc_test.go
git commit -m "$(cat <<'EOF'
feat(api): add SourcePricingEntryID to pricing.CalcLine

Calculate now carries the originating PricingEntry.ID into each
CalcLine so downstream snapshot consumers can trace a line back to
the specific pricing version it was derived from. Signature is
unchanged (source id is metadata).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: `quotation.SnapshotLine` 加源字段 + Submit/Preview 传播 + 4 个单测

**Files:**
- Modify: `apps/api/internal/quotation/dto.go`
- Modify: `apps/api/internal/quotation/service.go`
- Modify: `apps/api/internal/quotation/service_test.go`
- Modify: `apps/api/internal/quotation/snapshot_test.go`

- [ ] **Step 1: 写 Submit 失败测试**

在 `apps/api/internal/quotation/service_test.go` 文件末尾追加。先找到文件末尾并在那里插入；service_test.go 中 `TestSubmit_SnapshotsPricing` 附近的风格是好的参照。

```go
func TestSubmit_CarriesSourceIDs(t *testing.T) {
	country := uuid.New()
	owner := uuid.New()
	from := time.Now().Add(-24 * time.Hour)
	entryA := pricing.PricingEntry{
		ID: uuid.New(), CountryID: country, ServiceTier: "basic",
		FeeItem: "application", AmountCNYCents: 10000, EffectiveFrom: from, CreatedBy: owner,
	}
	entryB := pricing.PricingEntry{
		ID: uuid.New(), CountryID: country, ServiceTier: "basic",
		FeeItem: "agent", AmountCNYCents: 5000, EffectiveFrom: from, CreatedBy: owner,
	}
	svc, _ := newService([]pricing.PricingEntry{entryA, entryB})
	q, err := svc.Create(context.Background(), owner, CreateRequest{
		CustomerID: uuid.New(), CountryID: country, ServiceTier: "basic",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	submitted, err := svc.Submit(context.Background(), q.ID, owner)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	snap, err := submitted.DecodeSnapshot()
	if err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if len(snap.Lines) != 2 {
		t.Fatalf("lines: want 2, got %d", len(snap.Lines))
	}
	// Lines are sorted by fee_item — "agent" < "application".
	byItem := map[string]*uuid.UUID{}
	for i := range snap.Lines {
		byItem[snap.Lines[i].FeeItem] = snap.Lines[i].SourcePricingEntryID
	}
	if byItem["agent"] == nil || *byItem["agent"] != entryB.ID {
		t.Errorf("agent line source: want %s, got %v", entryB.ID, byItem["agent"])
	}
	if byItem["application"] == nil || *byItem["application"] != entryA.ID {
		t.Errorf("application line source: want %s, got %v", entryA.ID, byItem["application"])
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/quotation/ -run TestSubmit_CarriesSourceIDs -v
```
Expected: FAIL — `byItem["agent"]` 为 nil 或编译错误（`SourcePricingEntryID` 字段尚未定义）。

- [ ] **Step 3: 修改 `SnapshotLine` 结构加字段**

在 `apps/api/internal/quotation/dto.go` 中，`SnapshotLine` 结构（当前第 42-46 行）改为：

```go
// SnapshotLine is one priced fee item. Shape mirrors pricing.CalcLine,
// except SourcePricingEntryID is nullable — reviewer-adjusted lines
// (manual override) have no source entry, and legacy snapshots written
// before M4 decode to nil here (missing JSON key -> nil *uuid.UUID).
type SnapshotLine struct {
	FeeItem              string     `json:"fee_item"`
	AmountCNYCents       int64      `json:"amount_cny_cents"`
	SourcePricingEntryID *uuid.UUID `json:"source_pricing_entry_id,omitempty"`
}
```

- [ ] **Step 4: 更新 Submit 拷贝循环**

在 `apps/api/internal/quotation/service.go` 的 Submit 函数中，当前的拷贝循环（第 179-181 行）改为：

```go
for _, l := range calc.Lines {
    sourceID := l.SourcePricingEntryID
    snap.Lines = append(snap.Lines, SnapshotLine{
        FeeItem:              l.FeeItem,
        AmountCNYCents:       l.AmountCNYCents,
        SourcePricingEntryID: &sourceID,
    })
}
```

注意：用局部变量 `sourceID := l.SourcePricingEntryID` 再取地址，避免 for-range 循环变量的引用陷阱（Go 1.22+ 虽然每轮新变量，但显式拷贝更清晰）。

- [ ] **Step 5: 更新 Preview 拷贝循环**

在同文件 Preview 函数（第 429-433 行）改为：

```go
lines := make([]SnapshotLine, 0, len(calc.Lines))
for _, l := range calc.Lines {
    sourceID := l.SourcePricingEntryID
    lines = append(lines, SnapshotLine{
        FeeItem:              l.FeeItem,
        AmountCNYCents:       l.AmountCNYCents,
        SourcePricingEntryID: &sourceID,
    })
}
```

- [ ] **Step 6: 运行 Submit 测试确认通过**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/quotation/ -run TestSubmit_CarriesSourceIDs -v
```
Expected: PASS。

- [ ] **Step 7: 写 Preview 测试**

在 service_test.go 追加：

```go
func TestPreview_CarriesSourceIDs(t *testing.T) {
	country := uuid.New()
	caller := uuid.New()
	custID := uuid.New()
	from := time.Now().Add(-24 * time.Hour)
	entryA := pricing.PricingEntry{
		ID: uuid.New(), CountryID: country, ServiceTier: "basic",
		FeeItem: "fee_x", AmountCNYCents: 7000, EffectiveFrom: from, CreatedBy: caller,
	}
	svc, _ := newService([]pricing.PricingEntry{entryA})
	// Preview validates customer existence — register it in the fake.
	svc.customerRepo.(*fakeCustomerRepo).byID[custID] = &customer.Customer{ID: custID}

	res, err := svc.Preview(context.Background(), PreviewRequest{
		CustomerID: custID, CountryID: country, ServiceTier: "basic",
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(res.Lines) != 1 {
		t.Fatalf("lines: want 1, got %d", len(res.Lines))
	}
	if res.Lines[0].SourcePricingEntryID == nil || *res.Lines[0].SourcePricingEntryID != entryA.ID {
		t.Errorf("source id: want %s, got %v", entryA.ID, res.Lines[0].SourcePricingEntryID)
	}
}
```

注意：`svc.customerRepo.(*fakeCustomerRepo)` 依赖 `newService` 返回的 Service 暴露了 `customerRepo` 字段。如果 service.go 的 Service 结构体是大写 `CustomerRepo`，用它；如果是小写（package-private），由于测试在同包 `package quotation`，直接访问即可。先 grep 确认字段名：`grep -n "customerRepo\|CustomerRepo" apps/api/internal/quotation/service.go`。若字段不存在或名称不同，调整断言方式（可以改为通过 `newService` 增加第 3 个参数直接注入，或者把 fakeCustomerRepo 实例化后 seed，从 `newService` 内部传入）。具体看现有 newService 实现：

```go
// 现有 (service_test.go:145-148)
func newService(entries []pricing.PricingEntry) (*Service, *fakeRepo) {
	r := newFakeRepo()
	return NewService(r, &fakePricingRepo{entries: entries}, newFakeCustomerRepo()), r
}
```

改为同时返回 customer repo 以便测试 seed：

```go
func newServiceWithCustomer(entries []pricing.PricingEntry) (*Service, *fakeRepo, *fakeCustomerRepo) {
	r := newFakeRepo()
	custRepo := newFakeCustomerRepo()
	return NewService(r, &fakePricingRepo{entries: entries}, custRepo), r, custRepo
}
```

然后在 `TestPreview_CarriesSourceIDs` 里：

```go
svc, _, custRepo := newServiceWithCustomer([]pricing.PricingEntry{entryA})
custRepo.byID[custID] = &customer.Customer{ID: custID}
```

保留原 `newService` 不变以免影响已有测试。

- [ ] **Step 8: 运行测试**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/quotation/ -run TestPreview_CarriesSourceIDs -v
```
Expected: PASS。

- [ ] **Step 9: 写 Adjust 测试 — 请求体的 source_id 被保留**

在 service_test.go 追加：

```go
func TestAdjust_RequestSourcesPreserved(t *testing.T) {
	country := uuid.New()
	reviewer := uuid.New()
	owner := uuid.New()
	from := time.Now().Add(-24 * time.Hour)
	entry := pricing.PricingEntry{
		ID: uuid.New(), CountryID: country, ServiceTier: "basic",
		FeeItem: "application", AmountCNYCents: 10000, EffectiveFrom: from, CreatedBy: owner,
	}
	svc, _ := newService([]pricing.PricingEntry{entry})
	q, err := svc.Create(context.Background(), owner, CreateRequest{
		CustomerID: uuid.New(), CountryID: country, ServiceTier: "basic",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Submit(context.Background(), q.ID, owner); err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Reviewer passes a request with one line carrying source_id,
	// another line without (simulating the "orphan" override case).
	preservedID := uuid.New()
	adjusted, err := svc.Adjust(context.Background(), q.ID, reviewer, []SnapshotLine{
		{FeeItem: "preserved", AmountCNYCents: 1000, SourcePricingEntryID: &preservedID},
		{FeeItem: "orphan", AmountCNYCents: 2000}, // SourcePricingEntryID nil
	}, nil)
	if err != nil {
		t.Fatalf("adjust: %v", err)
	}
	snap, err := adjusted.DecodeSnapshot()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	byItem := map[string]*uuid.UUID{}
	for i := range snap.Lines {
		byItem[snap.Lines[i].FeeItem] = snap.Lines[i].SourcePricingEntryID
	}
	if byItem["preserved"] == nil || *byItem["preserved"] != preservedID {
		t.Errorf("preserved line source: want %s, got %v", preservedID, byItem["preserved"])
	}
	if byItem["orphan"] != nil {
		t.Errorf("orphan line source: want nil, got %v", byItem["orphan"])
	}
}
```

- [ ] **Step 10: 运行测试**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/quotation/ -run TestAdjust_RequestSourcesPreserved -v
```
Expected: PASS — Adjust 的 snapshot 构造（service.go 第 330 行）是 `nextSnap := Snapshot{Lines: lines, ...}`，直接存传入的 lines，所以 source_id 自然保留。

- [ ] **Step 11: 写 Decode 老 snapshot 测试**

在 `apps/api/internal/quotation/snapshot_test.go` 追加（文件当前是 `package quotation` + `computeAdjustSignature` 相关测试）：

```go
// TestDecodeLegacySnapshot_SourceNil verifies that a snapshot JSONB
// blob written before M4 (missing the source_pricing_entry_id key)
// decodes with Lines[i].SourcePricingEntryID == nil, without error.
// This is the behavior json.Unmarshal gives us for free on a pointer
// field tagged with omitempty — this test locks it against future
// regressions (e.g. if someone adds a required tag or a custom
// UnmarshalJSON).
func TestDecodeLegacySnapshot_SourceNil(t *testing.T) {
	legacy := []byte(`{"lines":[{"fee_item":"application","amount_cny_cents":10000}],"total_cny_cents":10000,"signature":"abc"}`)
	var s Snapshot
	if err := json.Unmarshal(legacy, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s.Lines) != 1 {
		t.Fatalf("lines: want 1, got %d", len(s.Lines))
	}
	if s.Lines[0].SourcePricingEntryID != nil {
		t.Errorf("legacy source: want nil, got %v", s.Lines[0].SourcePricingEntryID)
	}
	if s.Lines[0].FeeItem != "application" {
		t.Errorf("fee_item: want application, got %s", s.Lines[0].FeeItem)
	}
}
```

注意：`snapshot_test.go` 的现有 package 和 import 需要 `encoding/json` 和 `testing`。先读文件顶部确认 import：`head -10 apps/api/internal/quotation/snapshot_test.go`。若缺 `encoding/json`，补上。

- [ ] **Step 12: 运行测试**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/quotation/ -v
```
Expected: 全部 PASS，包括 T2 的 4 个新测试 + T1 的 calc 测试 + 既有所有测试。

- [ ] **Step 13: 构建 + vet**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go build ./... && go vet ./...
```
Expected: 成功。

- [ ] **Step 14: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/quotation/dto.go \
        apps/api/internal/quotation/service.go \
        apps/api/internal/quotation/service_test.go \
        apps/api/internal/quotation/snapshot_test.go
git commit -m "$(cat <<'EOF'
feat(api): propagate SourcePricingEntryID into SnapshotLine

SnapshotLine gains a nullable SourcePricingEntryID; Submit and Preview
copy it from CalcLine. Adjust passes request-body source ids through
unchanged (nil means manual override — D1 orphan semantics).

Legacy snapshots decode with nil source (missing JSON key handled by
default json.Unmarshal behavior on pointer+omitempty).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: pricing `Service.GetByID` + `Handler.GetByID` + 路由 + repo 测试

**Files:**
- Modify: `apps/api/internal/pricing/service.go`
- Modify: `apps/api/internal/pricing/handler.go`
- Modify: `apps/api/internal/pricing/router.go`
- Modify: `apps/api/internal/pricing/repository_test.go`

- [ ] **Step 1: 写 repo 失败测试 — deprecated entry 仍可查**

在 `apps/api/internal/pricing/repository_test.go` 文件末尾追加：

```go
// TestRepo_GetByID_ReturnsDeprecatedEntry locks in the M4 assumption
// that GetByID does NOT filter by effective_to — historical lookup
// needs to reach rows even after they've been deprecated by a newer
// version.
func TestRepo_GetByID_ReturnsDeprecatedEntry(t *testing.T) {
	db, countryID, userID := bootstrap(t)
	ctx := context.Background()
	repo := pricing.NewRepository(db)

	day1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	old, err := repo.ReplaceActive(ctx, pricing.NewEntry{
		CountryID:      countryID,
		ServiceTier:    "basic",
		FeeItem:        "application",
		AmountCNYCents: 10000,
		EffectiveFrom:  day1,
		CreatedBy:      userID,
	})
	require.NoError(t, err)

	// Replace deprecates `old` by setting effective_to = day2.
	_, err = repo.ReplaceActive(ctx, pricing.NewEntry{
		CountryID:      countryID,
		ServiceTier:    "basic",
		FeeItem:        "application",
		AmountCNYCents: 12000,
		EffectiveFrom:  day2,
		CreatedBy:      userID,
	})
	require.NoError(t, err)

	// GetByID on the now-deprecated old entry must still return it.
	got, err := repo.GetByID(ctx, old.ID)
	require.NoError(t, err)
	assert.Equal(t, old.ID, got.ID)
	require.NotNil(t, got.EffectiveTo)
	assert.Equal(t, int64(10000), got.AmountCNYCents)
}

// TestRepo_GetByID_NotFound returns ErrNotFound for a random UUID.
func TestRepo_GetByID_NotFound(t *testing.T) {
	db, _, _ := bootstrap(t)
	repo := pricing.NewRepository(db)
	_, err := repo.GetByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, pricing.ErrNotFound)
}
```

- [ ] **Step 2: 运行 repo 测试确认通过（既有行为）**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/pricing/ -run "TestRepo_GetByID" -v
```
Expected: PASS — 既有的 `GetByID` 实现已经满足（`WHERE id = ?` 不过滤 `effective_to`）。这俩测试锁定行为。

- [ ] **Step 3: 加 Service.GetByID**

在 `apps/api/internal/pricing/service.go` 末尾追加（紧随 Deprecate 之后）：

```go
// GetByID returns one pricing entry by id, regardless of whether
// it's been deprecated. Used by the traceability endpoint so snapshot
// lines can be resolved back to their source pricing row (including
// historical versions).
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*PricingEntryDTO, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	dto := toDTO(*row)
	return &dto, nil
}
```

- [ ] **Step 4: 加 Handler.GetByID**

在 `apps/api/internal/pricing/handler.go` 的 `GetHistory` handler 之后追加（注意现有 handler 里 ErrNotFound 的映射在 `PostDeprecate` 有先例）：

```go
// GetByID GET /pricing-entries/:id — returns one entry (active or
// deprecated). Used by M4 traceability: snapshot lines carry a
// source_pricing_entry_id; this endpoint lets the client expand that
// id back into full pricing context (effective window, amount, etc.).
func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID", "message": "invalid id"})
		return
	}
	dto, err := h.svc.GetByID(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": "failed to fetch pricing entry"})
		return
	}
	c.JSON(http.StatusOK, dto)
}
```

- [ ] **Step 5: 注册路由**

在 `apps/api/internal/pricing/router.go` 的 `RegisterReadRoutes` 函数里追加一行：

```go
// RegisterReadRoutes mounts read endpoints on a group restricted to
// reviewer+admin (caller wires the middleware).
func RegisterReadRoutes(group *gin.RouterGroup, h *Handler) {
	g := group.Group("/pricing-entries")
	g.GET("", h.GetActive)
	g.GET("/history", h.GetHistory)
	g.GET("/:id", h.GetByID)
}
```

**路由顺序说明**：`g.GET("/history", ...)` 必须在 `g.GET("/:id", ...)` **之前**注册。Gin 的路由树对静态路径和参数路径的匹配有优先级，但同级注册顺序也决定冲突处理。放置如上（history 在前，:id 在后）可避免 `GET /pricing-entries/history` 被当作 `:id = "history"` 错误命中。

- [ ] **Step 6: 构建 + vet**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go build ./... && go vet ./...
```
Expected: 成功。

- [ ] **Step 7: 运行所有 pricing 测试**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/pricing/ -v
```
Expected: 全绿（calc 测试 + repo 测试）。无新 handler 测试——handler 行为由 T4 的 E2E 集成覆盖。

- [ ] **Step 8: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/pricing/service.go \
        apps/api/internal/pricing/handler.go \
        apps/api/internal/pricing/router.go \
        apps/api/internal/pricing/repository_test.go
git commit -m "$(cat <<'EOF'
feat(api): add GET /pricing-entries/:id for historical lookup

New Service.GetByID + Handler.GetByID wrap the existing
Repository.GetByID (which already ignores effective_to). Registered
under RegisterReadRoutes (reviewer+admin) — consistent with existing
pricing read endpoints. Salesperson lookup can be enabled later if
UI requires it.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: E2E 集成测试 — 完整链路 submit → 读 snapshot source_id → 调 /pricing-entries/:id

**Files:**
- Modify: `apps/api/internal/quotation/handler_test.go`

- [ ] **Step 1: 在 handler_test.go 的 `buildRouter` 里补注册 pricing read 路由**

找到 `buildRouter` 函数（当前大约在 `handler_test.go:38-53`）。它目前只挂了 quotation 路由。在它里面加上 pricing handler 的注册，以便同一个 router 能同时服务 quotation 和 pricing 端点。

先在函数签名上增加一个 pricing handler 参数：

```go
// buildRouter wires up a Gin router with a synthetic auth middleware
// that injects the current user into Gin's context using the key the
// auth package already uses (`auth.currentUser`). Other handler tests
// in this repo follow the same pattern — see customer/handler_test.go.
func buildRouter(t *testing.T, quotHandler *quotation.Handler, pricingHandler *pricing.Handler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		uid, _ := uuid.Parse(c.GetHeader("X-Test-User-ID"))
		role := c.GetHeader("X-Test-Role")
		c.Set("auth.currentUser", auth.CurrentUserSummary{ID: uid, Role: role})
		c.Next()
	})
	authed := r.Group("/api/v1")
	quotation.RegisterAuthedRoutes(authed, quotHandler)
	reviewer := r.Group("/api/v1", auth.RequireRole("reviewer", "admin"))
	quotation.RegisterReviewerRoutes(reviewer, quotHandler)
	// Pricing reads are reviewer+admin in main.go — mirror that so the
	// traceability endpoint can be exercised in tests under role=admin
	// or role=reviewer.
	pricing.RegisterReadRoutes(reviewer, pricingHandler)
	return r
}
```

- [ ] **Step 2: 更新现有测试调用点**

buildRouter 多了一个参数，所有现有调用必须更新。找到 `TestHandler_HappyPath_SubmitThenApprove` 等测试里的 `r := buildRouter(t, quotation.NewHandler(svc))` 改为：

```go
pricingRepo := pricing.NewRepository(db) // 可能已存在于该测试的作用域
pricingSvc := pricing.NewService(pricingRepo)
pricingHandler := pricing.NewHandler(pricingSvc)
r := buildRouter(t, quotation.NewHandler(svc), pricingHandler)
```

如果某些测试此前没引入 `pricing.NewService` / `pricing.NewHandler`，加进来。运行 `grep -n "buildRouter(t" apps/api/internal/quotation/handler_test.go` 定位所有调用点，逐一更新。

- [ ] **Step 3: 运行现有测试确认不回归**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/quotation/ -v
```
Expected: 现有集成测试（SubmitThenApprove 等）仍然 PASS。

- [ ] **Step 4: 写 E2E 溯源测试**

在 `apps/api/internal/quotation/handler_test.go` 末尾追加：

```go
// TestHandler_SnapshotSourceIDs_LookupPricingEntry exercises the full
// traceability chain: submit a draft → read snapshot → extract
// source_pricing_entry_id from each line → hit GET /pricing-entries/:id
// and confirm we get the underlying entry back.
func TestHandler_SnapshotSourceIDs_LookupPricingEntry(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, salesID := seedCustomerCountryUser(t, db)

	// Seed two pricing entries so we have multiple lines to trace.
	appID := uuid.New()
	agentID := uuid.New()
	if err := db.Exec(
		`INSERT INTO pricing_entries
		 (id, country_id, service_tier, fee_item, amount_cny_cents, effective_from, created_by)
		 VALUES (?, ?, 'basic', 'application', 10000, ?, ?)`,
		appID, countryID, time.Now(), salesID,
	).Error; err != nil {
		t.Fatalf("seed pricing app: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO pricing_entries
		 (id, country_id, service_tier, fee_item, amount_cny_cents, effective_from, created_by)
		 VALUES (?, ?, 'basic', 'agent', 5000, ?, ?)`,
		agentID, countryID, time.Now(), salesID,
	).Error; err != nil {
		t.Fatalf("seed pricing agent: %v", err)
	}

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	pricingSvc := pricing.NewService(pricingRepo)
	pricingHandler := pricing.NewHandler(pricingSvc)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricingHandler)

	// Create a draft.
	body, _ := json.Marshal(map[string]any{
		"customer_id":  custID,
		"country_id":   countryID,
		"service_tier": "basic",
	})
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/quotations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status %d body %s", w.Code, w.Body.String())
	}
	var created quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	// Submit — freezes snapshot.
	req, _ = http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+created.ID.String()+"/submit", nil)
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("submit: status %d body %s", w.Code, w.Body.String())
	}
	var submitted quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &submitted)
	if submitted.Snapshot == nil {
		t.Fatal("submitted quotation has nil snapshot")
	}
	if len(submitted.Snapshot.Lines) != 2 {
		t.Fatalf("snapshot lines: want 2, got %d", len(submitted.Snapshot.Lines))
	}

	// Build a lookup admin for the reviewer-required GET endpoint.
	reviewerID, _ := ensureReviewer(t, db) // see note below
	// Trace each snapshot line back to its source pricing entry.
	for _, line := range submitted.Snapshot.Lines {
		if line.SourcePricingEntryID == nil {
			t.Errorf("line %s: source id is nil", line.FeeItem)
			continue
		}
		req, _ = http.NewRequestWithContext(context.Background(), "GET",
			"/api/v1/pricing-entries/"+line.SourcePricingEntryID.String(), nil)
		req.Header.Set("X-Test-User-ID", reviewerID.String())
		req.Header.Set("X-Test-Role", "reviewer")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("lookup line %s: status %d body %s", line.FeeItem, w.Code, w.Body.String())
		}
		var entry map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &entry)
		if entry["fee_item"] != line.FeeItem {
			t.Errorf("lookup mismatch: snapshot says %s, pricing entry says %v",
				line.FeeItem, entry["fee_item"])
		}
		gotAmount, _ := entry["amount_cny_cents"].(float64)
		if int64(gotAmount) != line.AmountCNYCents {
			t.Errorf("amount mismatch for %s: snapshot %d, entry %d",
				line.FeeItem, line.AmountCNYCents, int64(gotAmount))
		}
	}

	// Bonus: 404 on random UUID.
	req, _ = http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/pricing-entries/"+uuid.New().String(), nil)
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("random id lookup: want 404, got %d body %s", w.Code, w.Body.String())
	}

	// 400 on invalid UUID.
	req, _ = http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/pricing-entries/not-a-uuid", nil)
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid uuid: want 400, got %d body %s", w.Code, w.Body.String())
	}
}

// ensureReviewer inserts a reviewer user if needed and returns the id.
// Small helper scoped to this test file.
func ensureReviewer(t *testing.T, db *gorm.DB) (uuid.UUID, uuid.UUID) {
	t.Helper()
	var reviewerRoleID string
	require.NoError(t, db.Raw("SELECT id FROM roles WHERE code = ?", "reviewer").Scan(&reviewerRoleID).Error)
	rid, err := uuid.Parse(reviewerRoleID)
	require.NoError(t, err)
	uid := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		uid, "M4 Reviewer", "m4-reviewer-"+uid.String()+"@test.local", "hash", rid,
	).Error)
	return uid, rid
}
```

**注意**：
1. `bootPg(t)` 和 `seedCustomerCountryUser(t, db)` 是 handler_test.go 里既有的辅助函数——看文件上部（约 `handler_test.go:55` 附近的 `TestHandler_HappyPath_SubmitThenApprove` 用到）。直接复用。
2. `submitted.Snapshot.Lines[i].SourcePricingEntryID` 的字段路径依赖 T2 的 `Response.Snapshot` → `*Snapshot` → `Lines []SnapshotLine` → `.SourcePricingEntryID *uuid.UUID` 的完整链路。如果编译报 "undefined field"，说明 T2 没到位，回查 T2 的 dto.go 变更。
3. `require` 来自 `github.com/stretchr/testify/require`。handler_test.go 里可能没导入——如果没有，先 `go test` 看报错，再补 import。

- [ ] **Step 5: 运行新测试**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/quotation/ -run TestHandler_SnapshotSourceIDs_LookupPricingEntry -v
```
Expected: PASS。若 `bootPg` 需要 Docker，确保 Docker Desktop 已运行（testcontainers-go 需要）。

- [ ] **Step 6: 全量回归**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./... -v 2>&1 | tail -40
```
Expected: 全绿。

- [ ] **Step 7: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/quotation/handler_test.go
git commit -m "$(cat <<'EOF'
test(api): e2e traceability — submit → snapshot source_id → pricing lookup

New integration test exercises the full M4 chain: create draft,
submit, read back the frozen snapshot, and confirm each line's
source_pricing_entry_id resolves via GET /pricing-entries/:id to the
original PricingEntry row (fee_item + amount match).

Also covers 404 (random uuid) and 400 (malformed uuid) branches of
the new handler.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: 前端 `types.ts` 增字段 + MSW submit/preview handler 补 mock source_id

**Files:**
- Modify: `apps/web/src/features/quotation/types.ts`
- Modify: `apps/web/src/test-utils/msw/handlers.ts`

- [ ] **Step 1: types.ts 加字段**

在 `apps/web/src/features/quotation/types.ts`，`SnapshotLine` 接口（当前第 14-17 行）改为：

```ts
export interface SnapshotLine {
  fee_item: string
  amount_cny_cents: number
  /**
   * The pricing_entries.id that this line was derived from. null/undefined
   * means either a legacy snapshot (created before M4) or a reviewer-adjusted
   * line ("orphan" manual override). Set when the line came from
   * pricing.Calculate.
   */
  source_pricing_entry_id?: string | null
}
```

- [ ] **Step 2: MSW preview handler 加 source_id**

在 `apps/web/src/test-utils/msw/handlers.ts` 中，`/api/v1/quotations/preview` 的 lines 构造（当前第 346-348 行）改为：

```ts
const lines = matched
  .map((e) => ({
    fee_item: e.fee_item,
    amount_cny_cents: e.amount_cny_cents,
    source_pricing_entry_id: e.id,
  }))
  .sort((a, b) => a.fee_item.localeCompare(b.fee_item))
```

- [ ] **Step 3: MSW submit handler 加 source_id**

同文件 `/api/v1/quotations/:id/submit` 的 lines 构造（当前第 390-392 行）改为：

```ts
const lines = matching
  .map((p) => ({
    fee_item: p.fee_item,
    amount_cny_cents: p.amount_cny_cents,
    source_pricing_entry_id: p.id,
  }))
  .sort((a, b) => a.fee_item.localeCompare(b.fee_item))
```

- [ ] **Step 4: 搜索确认没有其他 mock snapshot 构造漏了字段**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/web
grep -rn "fee_item:" src/test-utils/msw/handlers.ts
```
Expected: 应该看到 preview、submit 已带 source_pricing_entry_id；adjust 等接口若也构造 snapshot，检查是否也要补（如第 745 行的 mock lines）。只要是写入 quotation.snapshot 的 mock，都补 source_id（或明确留 null 以模拟 orphan）。

对于第 745 行附近的 adjust mock：adjust 场景下"orphan"是合理的，所以**故意不带** source_id（模拟前端没传）。添加一行注释说明：

```ts
// Adjust-produced lines are "orphan" in M4 semantics — reviewer's
// manual override has no originating pricing entry. Intentionally
// omit source_pricing_entry_id here.
const lines = [{ fee_item: 'application', amount_cny_cents: total }]
```

（具体行号和上下文需执行时核对；搜 adjust handler 块）

- [ ] **Step 5: 跑前端 typecheck + lint**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/web
pnpm build
```
Expected: 构建通过。`types.ts` 加了可选字段不会让任何既有代码报错。

- [ ] **Step 6: 跑整个前端测试套件**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/web
pnpm vitest run src/features/quotation/quotation.integration.test.tsx --browser.headless
```
Expected: M3 的 11 个测试继续通过。MSW 的 snapshot 多一个字段，但 React 组件不渲染该字段（`QuotationSnapshotView` 不读它），所以 UI 无变化。

- [ ] **Step 7: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/features/quotation/types.ts \
        apps/web/src/test-utils/msw/handlers.ts
git commit -m "$(cat <<'EOF'
feat(web): add source_pricing_entry_id to SnapshotLine type + MSW mocks

Type is optional+nullable to tolerate legacy snapshots and adjust
orphan lines. MSW submit/preview handlers now populate the field
from the seeded pricing entry id so integration tests see the same
shape the real backend will produce after M4 backend lands.

No UI rendering change — field is purely data-layer metadata until
a future traceability UI milestone.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: 收尾 — Adjust 请求的 JSON 解析路径覆盖 + 文档注释

**Files:**
- Modify: `apps/api/internal/quotation/handler_test.go` (or a new small test)
- Modify: `docs/superpowers/specs/2026-04-26-m4-snapshot-line-traceability-design.md` (append deviation)

T1-T5 覆盖了 service 层面的所有行为（Submit/Preview 传播、Adjust 保留请求字段、Decode 老 snapshot）。剩下一个薄弱点：AdjustRequest 从 HTTP JSON 反序列化时，新增的 `source_pricing_entry_id` 字段是否正确 round-trip。service_test.go 直接构造 struct，跳过了 HTTP 解析。这里补一个端到端 adjust 的 handler-level 测试。

同时，补充 spec 偏差说明（T3 的 RegisterReadRoutes 决策 + Repository.GetByID 既存）。

- [ ] **Step 1: 写 handler-level adjust 溯源保持测试**

在 `apps/api/internal/quotation/handler_test.go` 末尾追加。复用 T4 的 pricingHandler setup 模式。

```go
// TestHandler_Adjust_PreservesSourceIDs covers the HTTP JSON round-trip:
// reviewer POSTs /:id/adjust with lines whose JSON carries
// source_pricing_entry_id, and we read back GET /:id and confirm the
// field persisted through json.Unmarshal → snapshot.Lines.
func TestHandler_Adjust_PreservesSourceIDs(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, salesID := seedCustomerCountryUser(t, db)
	reviewerID, _ := ensureReviewer(t, db)

	// Seed one pricing entry for the initial submit.
	entryID := uuid.New()
	if err := db.Exec(
		`INSERT INTO pricing_entries
		 (id, country_id, service_tier, fee_item, amount_cny_cents, effective_from, created_by)
		 VALUES (?, ?, 'basic', 'application', 10000, ?, ?)`,
		entryID, countryID, time.Now(), salesID,
	).Error; err != nil {
		t.Fatalf("seed pricing: %v", err)
	}

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	pricingSvc := pricing.NewService(pricingRepo)
	pricingHandler := pricing.NewHandler(pricingSvc)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricingHandler)

	// Create + submit as salesperson.
	body, _ := json.Marshal(map[string]any{
		"customer_id": custID, "country_id": countryID, "service_tier": "basic",
	})
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/quotations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var created quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	req, _ = http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+created.ID.String()+"/submit", nil)
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("submit: %d %s", w.Code, w.Body.String())
	}

	// Reviewer adjusts — carry one line with source_id, one without.
	preserved := uuid.New()
	adjustBody, _ := json.Marshal(map[string]any{
		"lines": []map[string]any{
			{"fee_item": "preserved", "amount_cny_cents": 500, "source_pricing_entry_id": preserved.String()},
			{"fee_item": "orphan", "amount_cny_cents": 700},
		},
	})
	req, _ = http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+created.ID.String()+"/adjust", bytes.NewReader(adjustBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("adjust: %d %s", w.Code, w.Body.String())
	}

	// Read back via GET /:id — snapshot should contain both lines with
	// their source_id state intact.
	req, _ = http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/quotations/"+created.ID.String(), nil)
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get: %d", w.Code)
	}
	var got quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Snapshot == nil {
		t.Fatal("snapshot nil")
	}
	byItem := map[string]*uuid.UUID{}
	for i := range got.Snapshot.Lines {
		byItem[got.Snapshot.Lines[i].FeeItem] = got.Snapshot.Lines[i].SourcePricingEntryID
	}
	if byItem["preserved"] == nil || *byItem["preserved"] != preserved {
		t.Errorf("preserved source: want %s, got %v", preserved, byItem["preserved"])
	}
	if byItem["orphan"] != nil {
		t.Errorf("orphan source: want nil, got %v", byItem["orphan"])
	}
}
```

- [ ] **Step 2: 运行新测试**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/quotation/ -run TestHandler_Adjust_PreservesSourceIDs -v
```
Expected: PASS。

- [ ] **Step 3: 全量回归（后端 + 前端）**

```bash
cd /Users/adam/workspace/github/trademark-admin
(cd apps/api && go test ./... 2>&1 | tail -20)
(cd apps/web && pnpm build)
```
Expected: 后端全绿，前端构建成功。

- [ ] **Step 4: 给 spec 加一条偏差说明**

在 `docs/superpowers/specs/2026-04-26-m4-snapshot-line-traceability-design.md` 的 §2 表格末尾（`|---|---|---|` 之后的最后一行之后）追加两行：

```markdown
| 端点注册在 `_authenticated`(任何 role 可调) | **注册到现有 `RegisterReadRoutes`(reviewer+admin)** | 保持与 `GET /pricing-entries` 和 `/history` 的 middleware 组一致；salesperson 无 UI 消费本端点;若未来需要,另起 milestone 放宽 |
| 新建 `Repository.GetByID` | **复用既有 `Repository.GetByID`** | 该方法已为 Deprecate 而存在,且 `WHERE id = ?` 不过滤 `effective_to`,已满足"deprecated 可查"需求 |
```

- [ ] **Step 5: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/quotation/handler_test.go \
        docs/superpowers/specs/2026-04-26-m4-snapshot-line-traceability-design.md
git commit -m "$(cat <<'EOF'
test(api): cover adjust HTTP JSON round-trip for source_pricing_entry_id

Handler-level test seals the one code path skipped by service-level
tests: JSON body → c.ShouldBindJSON → AdjustRequest.Lines →
SnapshotLine.SourcePricingEntryID persisted in JSONB → readback via
GET /:id.

Also appends two deviation rows to the M4 spec: RegisterReadRoutes
scope and Repository.GetByID reuse.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## 执行顺序

T1 → T2（依赖 T1 的 `CalcLine.SourcePricingEntryID` 字段）→ T3（独立，可与 T2 并行）→ T4（依赖 T2 的 SnapshotLine + T3 的路由）→ T5（独立，可与 T3/T4 并行）→ T6（需要 T4 的 `buildRouter` 扩展）。

串行推荐：T1 → T2 → T3 → T4 → T5 → T6。并行机会：T3 和 T5 可并行（不相关），但 subagent-driven 串行也只多几分钟，不必冒交叉风险。

---

## 自检清单

- [x] **Spec §3.1 后端字段**：T1 + T2 覆盖 `CalcLine` + `SnapshotLine` 新字段
- [x] **Spec §3.1 Submit/Preview/Adjust 传播**：T2 step 4-5（Submit/Preview）+ T2 Adjust 测试（Adjust 天然 pass-through）
- [x] **Spec §3.1 GET /pricing-entries/:id**：T3
- [x] **Spec §3.1 Repository 既存**：T3 step 2 锁定行为
- [x] **Spec §3.2 不做 UI**：T5 仅 types.ts + MSW mock,`QuotationSnapshotView` 零改动
- [x] **Spec §4.2 签名不变**：T1 不触碰 signature();T2 不改 computeAdjustSignature
- [x] **Spec §4.3 路由顺序 (/history 先于 /:id)**：T3 step 5 明确说明
- [x] **Spec §6 老 snapshot decode**:T2 step 11 的 `TestDecodeLegacySnapshot_SourceNil`
- [x] **Spec §6 deprecated entry 可查**：T3 step 1 的 `TestRepo_GetByID_ReturnsDeprecatedEntry`
- [x] **Spec §6 Adjust 信任请求**：T2 step 9 的 `TestAdjust_RequestSourcesPreserved` + T6 的 handler-level round-trip
- [x] **Spec §7.1 pricing 单测**：T1（calc_test）+ T3（repo_test）
- [x] **Spec §7.2 quotation 单测**：T2 的 4 个新测试
- [x] **Spec §7.3 集成测试**：T4 的 E2E lookup + T6 的 adjust round-trip
- [x] **Spec §7.4 MSW 一致性**：T5 同步更新 submit/preview handlers
- [x] **类型一致性**：`SnapshotLine.SourcePricingEntryID *uuid.UUID`（nullable）贯穿 T2/T4/T6/T5 类型;`CalcLine.SourcePricingEntryID uuid.UUID`（non-null）贯穿 T1/T2
