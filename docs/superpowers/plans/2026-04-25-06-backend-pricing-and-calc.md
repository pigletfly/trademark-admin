# Plan 6: 后端定价模板 + 计算引擎 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付一个版本化、不可变的 `pricing_entries` 定价表 + CRUD API + 纯函数计算引擎。后端围绕"二维定价表（country × service_tier × fee_item）"的 MVP：同一维度任一时刻只有一条 active 条目（`effective_to IS NULL`），历史上的价格通过 effective_from/to 记录。Admin 可新增 / 替换（自动废止旧 active）/ 废止；reviewer 和 admin 可读；salesperson 不可见本模块。

**Architecture:**
- `internal/pricing/{model,dto,repository,service,handler,router,calc}.go` — 单域单目录，沿用 catalog / customer 已有的分层约定。
- 不可变 + 版本化：所有写操作都追加新行；`Deprecate` 只改一行的 `effective_to`；不允许任何人物理删除定价条目。维度内替换通过 `ReplaceActive` 事务：deprecate 旧 active + insert new，保证任一时刻只有一条 active。
- 纯函数计算：`Calculate(entries []PricingEntry, input CalcInput) (CalcResult, error)` 放 `internal/pricing/calc.go`，**不依赖 DB**，方便单测和未来把 quotation 的金额签名塞进 audit。
- 价格单位：CNY 分（`amount_cny_cents BIGINT`），避免浮点累加误差；前端显示时除以 100。
- 角色门卫：read = authed 且 role ∈ {reviewer, admin}；write = admin。Salesperson 走到 `/pricing-entries*` 应直接拿到 403。

**Tech Stack:** Go 1.25 + Gin + GORM + Postgres 16（pgcrypto 已装，`gen_random_uuid()` 可用）+ golang-migrate + testcontainers-go。无新增第三方依赖。

---

## File Structure

### Create

- `apps/api/migrations/000003_pricing_entries.up.sql`
- `apps/api/migrations/000003_pricing_entries.down.sql`
- `apps/api/internal/pricing/model.go` — `PricingEntry` GORM 映射
- `apps/api/internal/pricing/dto.go` — 请求 / 响应 DTO + `toDTO`
- `apps/api/internal/pricing/repository.go` — `Repository` + ErrNotFound + 事务型 `ReplaceActive`
- `apps/api/internal/pricing/service.go` — 业务逻辑薄层，吃 `CreateOrReplace` / `Deprecate` / `ListActive` / `ListHistory`
- `apps/api/internal/pricing/handler.go` — HTTP 处理
- `apps/api/internal/pricing/router.go` — `RegisterRoutes(reviewerAdminGroup, adminGroup, h)`
- `apps/api/internal/pricing/calc.go` — 纯函数计算引擎 + 签名
- `apps/api/internal/pricing/repository_test.go` — 基于 testcontainers 的 repo 积分测试
- `apps/api/internal/pricing/calc_test.go` — 纯单测

### Modify

- `apps/api/cmd/server/main.go` — 新增 `reviewerAdminGroup := v1.Group("")` 并绑定 `RequireRole("reviewer","admin")`（`RequireRole` 已是可变参数，见 auth/middleware）；在 adminGroup 上注册 write 路由；wire pricing 包。
- `apps/api/internal/auth/middleware.go` — **若** `RequireRole` 目前只接受单字符串，扩展成 `func RequireRole(roles ...string)`。（先 grep 确认）

---

## Task 1: Migration — `pricing_entries` 表

**Files:**
- Create: `apps/api/migrations/000003_pricing_entries.up.sql`
- Create: `apps/api/migrations/000003_pricing_entries.down.sql`

- [ ] **Step 1: up migration**

Create `apps/api/migrations/000003_pricing_entries.up.sql`:

```sql
-- service_tier + fee_item 用 CHECK 约束而非 ENUM：CHECK 放宽/扩容时只需 ALTER TABLE
-- 单条语句；ENUM 改值要 DROP & recreate，在生产上痛苦。
CREATE TABLE pricing_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    country_id UUID NOT NULL REFERENCES countries(id) ON DELETE RESTRICT,
    service_tier TEXT NOT NULL
        CHECK (service_tier IN ('basic', 'standard', 'premium')),
    fee_item TEXT NOT NULL
        CHECK (length(fee_item) BETWEEN 1 AND 80),
    amount_cny_cents BIGINT NOT NULL CHECK (amount_cny_cents >= 0),
    notes TEXT,
    effective_from DATE NOT NULL,
    effective_to DATE
        CHECK (effective_to IS NULL OR effective_to > effective_from),
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 同一 (country, tier, fee_item) 任一时刻只能有一条 effective_to IS NULL 的 active 行
CREATE UNIQUE INDEX idx_pricing_active_unique
    ON pricing_entries (country_id, service_tier, fee_item)
    WHERE effective_to IS NULL;

-- 历史查询：按维度查所有版本，倒序
CREATE INDEX idx_pricing_history
    ON pricing_entries (country_id, service_tier, fee_item, effective_from DESC);

-- 列表查询：admin 查 active by country+tier 时走这个覆盖
CREATE INDEX idx_pricing_active_lookup
    ON pricing_entries (country_id, service_tier)
    WHERE effective_to IS NULL;
```

- [ ] **Step 2: down migration**

Create `apps/api/migrations/000003_pricing_entries.down.sql`:

```sql
DROP TABLE IF EXISTS pricing_entries;
```

- [ ] **Step 3: 验证 migration 可运行**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
# Postgres via docker compose 已在 apps/api/docker-compose.yml；若未启动：
docker compose up -d postgres
# 跑 migration
go run ./cmd/migrate up
# 验证表存在
docker compose exec postgres psql -U postgres -d trademark_admin -c "\d+ pricing_entries" | head -40
# 回滚确认 down 干净
go run ./cmd/migrate down 1
go run ./cmd/migrate up
```

Expected: `pricing_entries` 表结构与 up migration 一致；down 删掉后 up 再跑成功。

- [ ] **Step 4: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/migrations/000003_pricing_entries.up.sql apps/api/migrations/000003_pricing_entries.down.sql
git commit -m "$(cat <<'EOF'
feat(api): migration 000003 creates pricing_entries table

Versioned pricing rows keyed by (country, service_tier, fee_item).
Partial unique index enforces "at most one active row per dimension" so
the service layer can safely deprecate+insert to replace. effective_from/to
carry history forward without physical deletion. Amounts stored in CNY
cents (BIGINT) to avoid float accumulation.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Model + DTO

**Files:**
- Create: `apps/api/internal/pricing/model.go`
- Create: `apps/api/internal/pricing/dto.go`

- [ ] **Step 1: Model**

Create `apps/api/internal/pricing/model.go`:

```go
package pricing

import (
	"time"

	"github.com/google/uuid"
)

// PricingEntry mirrors the pricing_entries table.
// Entries are immutable except for effective_to, which is set when the
// entry is deprecated.
type PricingEntry struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	CountryID      uuid.UUID  `gorm:"type:uuid;not null;index"`
	ServiceTier    string     `gorm:"not null"`
	FeeItem        string     `gorm:"not null"`
	AmountCNYCents int64      `gorm:"not null"`
	Notes          *string
	EffectiveFrom  time.Time  `gorm:"type:date;not null"`
	EffectiveTo    *time.Time `gorm:"type:date"`
	CreatedBy      uuid.UUID  `gorm:"type:uuid;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (PricingEntry) TableName() string { return "pricing_entries" }

// ServiceTiers enumerates supported tiers; keep in sync with the
// CHECK constraint in migration 000003.
var ServiceTiers = []string{"basic", "standard", "premium"}

// IsValidServiceTier reports whether t is one of the allowed tiers.
func IsValidServiceTier(t string) bool {
	for _, v := range ServiceTiers {
		if v == t {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: DTO**

Create `apps/api/internal/pricing/dto.go`:

```go
package pricing

import (
	"time"

	"github.com/google/uuid"
)

// PricingEntryDTO is the wire shape of a pricing row.
type PricingEntryDTO struct {
	ID             uuid.UUID  `json:"id"`
	CountryID      uuid.UUID  `json:"country_id"`
	ServiceTier    string     `json:"service_tier"`
	FeeItem        string     `json:"fee_item"`
	AmountCNYCents int64      `json:"amount_cny_cents"`
	Notes          *string    `json:"notes,omitempty"`
	EffectiveFrom  string     `json:"effective_from"`           // YYYY-MM-DD
	EffectiveTo    *string    `json:"effective_to,omitempty"`   // YYYY-MM-DD or null
	CreatedBy      uuid.UUID  `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CreateOrReplaceRequest creates a new active entry for the given
// dimension. If an active entry already exists for (country, tier, item),
// the service layer deprecates it (effective_to = new.effective_from)
// and inserts the new row atomically.
type CreateOrReplaceRequest struct {
	CountryID      uuid.UUID `json:"country_id" binding:"required"`
	ServiceTier    string    `json:"service_tier" binding:"required"`
	FeeItem        string    `json:"fee_item" binding:"required"`
	AmountCNYCents int64     `json:"amount_cny_cents" binding:"gte=0"`
	Notes          *string   `json:"notes,omitempty"`
	EffectiveFrom  string    `json:"effective_from" binding:"required"` // YYYY-MM-DD
}

// DeprecateRequest retires the active entry at :id. effective_to
// defaults to today if omitted.
type DeprecateRequest struct {
	EffectiveTo *string `json:"effective_to,omitempty"` // YYYY-MM-DD
}

func toDTO(e PricingEntry) PricingEntryDTO {
	dto := PricingEntryDTO{
		ID:             e.ID,
		CountryID:      e.CountryID,
		ServiceTier:    e.ServiceTier,
		FeeItem:        e.FeeItem,
		AmountCNYCents: e.AmountCNYCents,
		Notes:          e.Notes,
		EffectiveFrom:  e.EffectiveFrom.Format("2006-01-02"),
		CreatedBy:      e.CreatedBy,
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
	}
	if e.EffectiveTo != nil {
		s := e.EffectiveTo.Format("2006-01-02")
		dto.EffectiveTo = &s
	}
	return dto
}
```

- [ ] **Step 3: 快速编译**

```bash
cd /Users/adam/workspace/github/trademark-admin
go -C apps/api build ./internal/pricing
```

Expected: exit 0.

- [ ] **Step 4: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/pricing/model.go apps/api/internal/pricing/dto.go
git commit -m "$(cat <<'EOF'
feat(api): pricing model + DTO — versioned immutable rows

PricingEntry mirrors pricing_entries with amounts stored as int64 CNY
cents. Tiers basic/standard/premium exposed via ServiceTiers slice so
the handler can validate without re-opening the migration. Dates marshal
as YYYY-MM-DD in DTO since that is what the API consumers need.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Repository — 事务型 ReplaceActive

**Files:**
- Create: `apps/api/internal/pricing/repository.go`

- [ ] **Step 1: Repository**

Create `apps/api/internal/pricing/repository.go`:

```go
package pricing

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrNotFound is returned when the repo can't find a row.
var ErrNotFound = errors.New("pricing: not found")

// ErrNoActive is returned by Deprecate when the entry at :id is already
// deprecated (effective_to already set).
var ErrNoActive = errors.New("pricing: entry already deprecated")

// Repository wraps DB access for pricing entries.
type Repository struct{ db *gorm.DB }

// NewRepository wires a Repository.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// ActiveFilter narrows ListActive.
type ActiveFilter struct {
	CountryID   *uuid.UUID
	ServiceTier *string
}

// ListActive returns all entries where effective_to IS NULL, filtered
// optionally by country and/or tier. Ordered by country_id, tier, fee_item
// so the frontend can render a 2-D table deterministically.
func (r *Repository) ListActive(ctx context.Context, f ActiveFilter) ([]PricingEntry, error) {
	q := r.db.WithContext(ctx).
		Where("effective_to IS NULL").
		Order("country_id, service_tier, fee_item")
	if f.CountryID != nil {
		q = q.Where("country_id = ?", *f.CountryID)
	}
	if f.ServiceTier != nil {
		q = q.Where("service_tier = ?", *f.ServiceTier)
	}
	var rows []PricingEntry
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// HistoryFilter queries every version of a single dimension.
type HistoryFilter struct {
	CountryID   uuid.UUID
	ServiceTier string
	FeeItem     string
}

// ListHistory returns every version of the (country, tier, item) tuple
// newest first.
func (r *Repository) ListHistory(ctx context.Context, f HistoryFilter) ([]PricingEntry, error) {
	var rows []PricingEntry
	err := r.db.WithContext(ctx).
		Where("country_id = ? AND service_tier = ? AND fee_item = ?",
			f.CountryID, f.ServiceTier, f.FeeItem).
		Order("effective_from DESC, created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetByID fetches a single row by id; used by Deprecate.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*PricingEntry, error) {
	var row PricingEntry
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// NewEntry carries the fields required to insert an entry.
type NewEntry struct {
	CountryID      uuid.UUID
	ServiceTier    string
	FeeItem        string
	AmountCNYCents int64
	Notes          *string
	EffectiveFrom  time.Time
	CreatedBy      uuid.UUID
}

// ReplaceActive inserts a new active entry for (country, tier, item),
// deprecating the existing active row (if any) by setting
// effective_to = newEntry.EffectiveFrom. Runs in a single transaction so
// readers never observe two active rows at once.
//
// Returns the inserted entry.
func (r *Repository) ReplaceActive(ctx context.Context, n NewEntry) (*PricingEntry, error) {
	var inserted *PricingEntry
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Deprecate any existing active row for this dimension.
		if err := tx.Model(&PricingEntry{}).
			Where("country_id = ? AND service_tier = ? AND fee_item = ? AND effective_to IS NULL",
				n.CountryID, n.ServiceTier, n.FeeItem).
			Updates(map[string]any{
				"effective_to": n.EffectiveFrom,
				"updated_at":   gorm.Expr("NOW()"),
			}).Error; err != nil {
			return err
		}
		row := PricingEntry{
			ID:             uuid.New(),
			CountryID:      n.CountryID,
			ServiceTier:    n.ServiceTier,
			FeeItem:        n.FeeItem,
			AmountCNYCents: n.AmountCNYCents,
			Notes:          n.Notes,
			EffectiveFrom:  n.EffectiveFrom,
			CreatedBy:      n.CreatedBy,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		inserted = &row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return inserted, nil
}

// Deprecate sets effective_to on the entry at :id. Returns ErrNoActive if
// the row is already deprecated. Returns ErrNotFound if no such row.
func (r *Repository) Deprecate(ctx context.Context, id uuid.UUID, effectiveTo time.Time) (*PricingEntry, error) {
	row, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.EffectiveTo != nil {
		return nil, ErrNoActive
	}
	if !effectiveTo.After(row.EffectiveFrom) {
		return nil, errors.New("pricing: effective_to must be after effective_from")
	}
	if err := r.db.WithContext(ctx).
		Model(&PricingEntry{}).Where("id = ?", id).
		Updates(map[string]any{
			"effective_to": effectiveTo,
			"updated_at":   gorm.Expr("NOW()"),
		}).Error; err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}
```

- [ ] **Step 2: 快速编译**

```bash
cd /Users/adam/workspace/github/trademark-admin
go -C apps/api build ./internal/pricing
```

- [ ] **Step 3: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/pricing/repository.go
git commit -m "$(cat <<'EOF'
feat(api): pricing repository with transactional ReplaceActive

ReplaceActive deprecates the existing active row (if any) and inserts the
replacement in a single transaction, so the partial unique index never
sees two active rows. Deprecate sets effective_to; rejects already-
deprecated rows and past-dated effective_to. ListActive/ListHistory are
simple read paths keyed on the partial + history indices from the
migration.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Service 薄层

**Files:**
- Create: `apps/api/internal/pricing/service.go`

- [ ] **Step 1: Service**

Create `apps/api/internal/pricing/service.go`:

```go
package pricing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidTier is returned when service_tier isn't in the allowed set.
var ErrInvalidTier = errors.New("pricing: invalid service_tier")

// Service orchestrates the repository and validates request shape.
type Service struct{ repo *Repository }

// NewService wires a Service.
func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// ListActive delegates to the repository.
func (s *Service) ListActive(ctx context.Context, f ActiveFilter) ([]PricingEntryDTO, error) {
	rows, err := s.repo.ListActive(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]PricingEntryDTO, len(rows))
	for i, r := range rows {
		out[i] = toDTO(r)
	}
	return out, nil
}

// ListHistory returns every version of one dimension.
func (s *Service) ListHistory(ctx context.Context, f HistoryFilter) ([]PricingEntryDTO, error) {
	rows, err := s.repo.ListHistory(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]PricingEntryDTO, len(rows))
	for i, r := range rows {
		out[i] = toDTO(r)
	}
	return out, nil
}

// CreateOrReplace validates the request and delegates to repo.
// callerID must be a valid user; it ends up in created_by.
func (s *Service) CreateOrReplace(ctx context.Context, callerID uuid.UUID, req CreateOrReplaceRequest) (*PricingEntryDTO, error) {
	if !IsValidServiceTier(req.ServiceTier) {
		return nil, ErrInvalidTier
	}
	if req.FeeItem == "" {
		return nil, errors.New("pricing: fee_item required")
	}
	if req.AmountCNYCents < 0 {
		return nil, errors.New("pricing: amount_cny_cents must be >= 0")
	}
	eff, err := time.Parse("2006-01-02", req.EffectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("pricing: invalid effective_from: %w", err)
	}
	row, err := s.repo.ReplaceActive(ctx, NewEntry{
		CountryID:      req.CountryID,
		ServiceTier:    req.ServiceTier,
		FeeItem:        req.FeeItem,
		AmountCNYCents: req.AmountCNYCents,
		Notes:          req.Notes,
		EffectiveFrom:  eff,
		CreatedBy:      callerID,
	})
	if err != nil {
		return nil, err
	}
	dto := toDTO(*row)
	return &dto, nil
}

// Deprecate retires a single entry.
func (s *Service) Deprecate(ctx context.Context, id uuid.UUID, req DeprecateRequest) (*PricingEntryDTO, error) {
	var effTo time.Time
	if req.EffectiveTo == nil {
		// Default: tomorrow (so it is strictly after any effective_from that
		// could have been today).
		effTo = time.Now().UTC().Add(24 * time.Hour).Truncate(24 * time.Hour)
	} else {
		t, err := time.Parse("2006-01-02", *req.EffectiveTo)
		if err != nil {
			return nil, fmt.Errorf("pricing: invalid effective_to: %w", err)
		}
		effTo = t
	}
	row, err := s.repo.Deprecate(ctx, id, effTo)
	if err != nil {
		return nil, err
	}
	dto := toDTO(*row)
	return &dto, nil
}
```

- [ ] **Step 2: 编译**

```bash
cd /Users/adam/workspace/github/trademark-admin
go -C apps/api build ./internal/pricing
```

- [ ] **Step 3: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/pricing/service.go
git commit -m "$(cat <<'EOF'
feat(api): pricing service validates + wraps repo calls

CreateOrReplace parses effective_from, validates tier, and delegates to
ReplaceActive. Deprecate defaults effective_to to tomorrow when the
client omits it (so it is strictly after any effective_from that could
be today, satisfying the DB CHECK).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: 纯函数计算引擎 + 单测

**Files:**
- Create: `apps/api/internal/pricing/calc.go`
- Create: `apps/api/internal/pricing/calc_test.go`

- [ ] **Step 1: calc.go**

Create `apps/api/internal/pricing/calc.go`:

```go
package pricing

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
)

// CalcInput feeds Calculate. At MVP we price per (country, tier); future
// plans add multi-country aggregation at the quotation layer.
type CalcInput struct {
	CountryID   uuid.UUID `json:"country_id"`
	ServiceTier string    `json:"service_tier"`
}

// CalcLine is one fee item included in the total.
type CalcLine struct {
	FeeItem        string `json:"fee_item"`
	AmountCNYCents int64  `json:"amount_cny_cents"`
}

// CalcResult is the deterministic output of Calculate.
type CalcResult struct {
	Lines         []CalcLine `json:"lines"`
	TotalCNYCents int64      `json:"total_cny_cents"`
	// Signature is a SHA-256 over input + sorted lines + total — lets the
	// quotation layer detect tampering when it re-calls the engine later.
	Signature string `json:"signature"`
}

// ErrNoMatchingEntries is returned when the filtered slice produces no
// lines — usually the country/tier combination is unpriced.
var ErrNoMatchingEntries = errors.New("pricing: no active entries for input")

// Calculate deterministically reduces entries → CalcResult.
//
// Rules:
//   - Only active entries (effective_to == nil) matching input are used.
//   - Lines are sorted by fee_item ascending so signature is stable.
//   - Total is simple sum — no discounts / promos at MVP.
//
// Calculate does NOT touch the DB. Callers fetch entries (usually via
// repo.ListActive with a CountryID filter) and hand the slice in.
func Calculate(entries []PricingEntry, input CalcInput) (CalcResult, error) {
	if !IsValidServiceTier(input.ServiceTier) {
		return CalcResult{}, ErrInvalidTier
	}
	var lines []CalcLine
	var total int64
	for _, e := range entries {
		if e.EffectiveTo != nil {
			continue
		}
		if e.CountryID != input.CountryID {
			continue
		}
		if e.ServiceTier != input.ServiceTier {
			continue
		}
		lines = append(lines, CalcLine{FeeItem: e.FeeItem, AmountCNYCents: e.AmountCNYCents})
		total += e.AmountCNYCents
	}
	if len(lines) == 0 {
		return CalcResult{}, ErrNoMatchingEntries
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].FeeItem < lines[j].FeeItem })
	sig := signature(input, lines, total)
	return CalcResult{Lines: lines, TotalCNYCents: total, Signature: sig}, nil
}

func signature(in CalcInput, lines []CalcLine, total int64) string {
	h := sha256.New()
	fmt.Fprintf(h, "v1|%s|%s|", in.CountryID, in.ServiceTier)
	for _, l := range lines {
		fmt.Fprintf(h, "%s=%d;", l.FeeItem, l.AmountCNYCents)
	}
	fmt.Fprintf(h, "=%d", total)
	return hex.EncodeToString(h.Sum(nil))
}
```

- [ ] **Step 2: calc_test.go**

Create `apps/api/internal/pricing/calc_test.go`:

```go
package pricing

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func mustEntry(t *testing.T, c uuid.UUID, tier, item string, cents int64, deprecated bool) PricingEntry {
	t.Helper()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e := PricingEntry{
		ID:             uuid.New(),
		CountryID:      c,
		ServiceTier:    tier,
		FeeItem:        item,
		AmountCNYCents: cents,
		EffectiveFrom:  from,
		CreatedBy:      uuid.New(),
	}
	if deprecated {
		to := from.Add(24 * time.Hour)
		e.EffectiveTo = &to
	}
	return e
}

func TestCalculate_InvalidTier(t *testing.T) {
	_, err := Calculate(nil, CalcInput{CountryID: uuid.New(), ServiceTier: "vip"})
	if err != ErrInvalidTier {
		t.Fatalf("want ErrInvalidTier, got %v", err)
	}
}

func TestCalculate_NoMatchingEntries(t *testing.T) {
	c := uuid.New()
	entries := []PricingEntry{
		mustEntry(t, uuid.New(), "basic", "application", 10000, false), // wrong country
	}
	_, err := Calculate(entries, CalcInput{CountryID: c, ServiceTier: "basic"})
	if err != ErrNoMatchingEntries {
		t.Fatalf("want ErrNoMatchingEntries, got %v", err)
	}
}

func TestCalculate_FiltersDeprecatedAndOtherTier(t *testing.T) {
	c := uuid.New()
	entries := []PricingEntry{
		mustEntry(t, c, "basic", "application", 10000, false),
		mustEntry(t, c, "basic", "agent", 5000, false),
		mustEntry(t, c, "basic", "deprecated_fee", 9999, true),  // deprecated
		mustEntry(t, c, "premium", "application", 20000, false), // wrong tier
	}
	res, err := Calculate(entries, CalcInput{CountryID: c, ServiceTier: "basic"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.TotalCNYCents != 15000 {
		t.Fatalf("total: want 15000, got %d", res.TotalCNYCents)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("lines: want 2, got %d", len(res.Lines))
	}
	// Lines must be sorted alphabetically by fee_item so signature is
	// stable regardless of input order.
	if res.Lines[0].FeeItem != "agent" || res.Lines[1].FeeItem != "application" {
		t.Fatalf("expected [agent, application] order, got %+v", res.Lines)
	}
}

func TestCalculate_SignatureStableAcrossInputOrder(t *testing.T) {
	c := uuid.New()
	a := mustEntry(t, c, "standard", "aa_fee", 1000, false)
	b := mustEntry(t, c, "standard", "bb_fee", 2000, false)

	r1, _ := Calculate([]PricingEntry{a, b}, CalcInput{c, "standard"})
	r2, _ := Calculate([]PricingEntry{b, a}, CalcInput{c, "standard"})

	if r1.Signature != r2.Signature {
		t.Fatalf("signature differs across input order: %s vs %s", r1.Signature, r2.Signature)
	}
	if r1.TotalCNYCents != 3000 {
		t.Fatalf("total: want 3000, got %d", r1.TotalCNYCents)
	}
}

func TestCalculate_SignatureChangesWithAmount(t *testing.T) {
	c := uuid.New()
	a1 := mustEntry(t, c, "standard", "fee", 1000, false)
	a2 := mustEntry(t, c, "standard", "fee", 1001, false)

	r1, _ := Calculate([]PricingEntry{a1}, CalcInput{c, "standard"})
	r2, _ := Calculate([]PricingEntry{a2}, CalcInput{c, "standard"})

	if r1.Signature == r2.Signature {
		t.Fatal("signature should change when amount changes")
	}
}
```

- [ ] **Step 3: 跑 calc 单测**

```bash
cd /Users/adam/workspace/github/trademark-admin
go -C apps/api test ./internal/pricing -run Calculate -v
```

Expected: 4 PASS。

- [ ] **Step 4: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/pricing/calc.go apps/api/internal/pricing/calc_test.go
git commit -m "$(cat <<'EOF'
feat(api): pure pricing calc engine with tamper-detection signature

Calculate filters matching active entries for a (country, tier) input,
sums to CNY cents, and emits a stable SHA-256 signature over sorted
lines + total. Signature lets the future quotation layer snapshot a
priced result and detect tampering on later re-calculation. No DB
touch: caller passes the slice in. Four unit tests cover tier-validation,
empty-result, filter correctness, and signature stability across input
order and amount deltas.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Handler + Router + 扩展 RequireRole 支持多角色

**Files:**
- Create: `apps/api/internal/pricing/handler.go`
- Create: `apps/api/internal/pricing/router.go`
- Modify (if needed): `apps/api/internal/auth/middleware.go`
- Modify: `apps/api/cmd/server/main.go`

### Step 1: 先确认 `RequireRole` 是否已支持多角色

```bash
cd /Users/adam/workspace/github/trademark-admin
grep -n "func RequireRole" apps/api/internal/auth/*.go
```

预期看到形如 `func RequireRole(role string) gin.HandlerFunc` 或 `func RequireRole(roles ...string) gin.HandlerFunc`。

- 若已是 variadic（`roles ...string`）：跳到 Step 3。
- 若是单参数：进 Step 2。

### Step 2（仅当需要时）: 扩展 RequireRole

打开当前 `middleware.go` 看现有签名。把 `role string` 改成 `roles ...string`，将比较改为 `for _, r := range roles { if u.Role == r { c.Next(); return } }`，最后 `c.AbortWithStatusJSON(http.StatusForbidden, ...)`。保留现有单字符串调用点（`auth.RequireRole("admin")` 仍然合法）。确保现有测试仍过：

```bash
go -C apps/api test ./internal/auth
```

Expected: PASS.

### Step 3: handler.go

Create `apps/api/internal/pricing/handler.go`:

```go
package pricing

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

// Handler exposes pricing HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler wires a Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// GetActive GET /pricing-entries?country_id=...&service_tier=...
func (h *Handler) GetActive(c *gin.Context) {
	var f ActiveFilter
	if s := c.Query("country_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_QUERY", "message": "invalid country_id"})
			return
		}
		f.CountryID = &id
	}
	if s := c.Query("service_tier"); s != "" {
		if !IsValidServiceTier(s) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_QUERY", "message": "invalid service_tier"})
			return
		}
		f.ServiceTier = &s
	}
	rows, err := h.svc.ListActive(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": "failed to list pricing entries"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// GetHistory GET /pricing-entries/history?country_id=...&service_tier=...&fee_item=...
func (h *Handler) GetHistory(c *gin.Context) {
	cID, err := uuid.Parse(c.Query("country_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_QUERY", "message": "invalid country_id"})
		return
	}
	tier := c.Query("service_tier")
	if !IsValidServiceTier(tier) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_QUERY", "message": "invalid service_tier"})
		return
	}
	item := c.Query("fee_item")
	if item == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_QUERY", "message": "fee_item required"})
		return
	}
	rows, err := h.svc.ListHistory(c.Request.Context(), HistoryFilter{
		CountryID:   cID,
		ServiceTier: tier,
		FeeItem:     item,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": "failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// PostCreateOrReplace POST /pricing-entries (admin).
func (h *Handler) PostCreateOrReplace(c *gin.Context) {
	var req CreateOrReplaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	u := auth.CurrentUser(c)
	if u.ID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "ERR_UNAUTHORIZED"})
		return
	}
	dto, err := h.svc.CreateOrReplace(c.Request.Context(), u.ID, req)
	if errors.Is(err, ErrInvalidTier) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_TIER", "message": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, dto)
}

// PostDeprecate POST /pricing-entries/:id/deprecate (admin).
func (h *Handler) PostDeprecate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID"})
		return
	}
	var req DeprecateRequest
	// Body is optional; ignore binding errors when body is empty.
	_ = c.ShouldBindJSON(&req)
	dto, err := h.svc.Deprecate(c.Request.Context(), id, req)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND"})
		return
	}
	if errors.Is(err, ErrNoActive) {
		c.JSON(http.StatusConflict, gin.H{"code": "ERR_ALREADY_DEPRECATED", "message": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto)
}
```

### Step 4: router.go

Create `apps/api/internal/pricing/router.go`:

```go
package pricing

import "github.com/gin-gonic/gin"

// RegisterReadRoutes mounts read endpoints on a group restricted to
// reviewer+admin (caller wires the middleware).
func RegisterReadRoutes(group *gin.RouterGroup, h *Handler) {
	g := group.Group("/pricing-entries")
	g.GET("", h.GetActive)
	g.GET("/history", h.GetHistory)
}

// RegisterAdminRoutes mounts write endpoints on an admin-only group.
func RegisterAdminRoutes(admin *gin.RouterGroup, h *Handler) {
	g := admin.Group("/pricing-entries")
	g.POST("", h.PostCreateOrReplace)
	g.POST("/:id/deprecate", h.PostDeprecate)
}
```

### Step 5: 修 main.go — 新建 reviewerAdminGroup + wire pricing

打开 `apps/api/cmd/server/main.go`。找到 adminGroup 块，在它上面加一个 `reviewerAdminGroup` 并 wire pricing read；在 adminGroup 下注册 pricing write。

追加以下代码（放在 `customer.RegisterRoutes(authed, custHandler)` 之后，`adminGroup :=` 之前）：

```go
	// Pricing entries — reviewer+admin read, admin write.
	pricingRepo := pricing.NewRepository(db)
	pricingSvc := pricing.NewService(pricingRepo)
	pricingHandler := pricing.NewHandler(pricingSvc)

	reviewerAdminGroup := v1.Group("")
	reviewerAdminGroup.Use(auth.RequireAuth([]byte(cfg.JWTAccessSecret)),
		auth.RequireRole("reviewer", "admin"),
		auth.CSRF(),
		auditMW,
	)
	pricing.RegisterReadRoutes(reviewerAdminGroup, pricingHandler)
```

并在 adminGroup 块里（`adminGroup.Use(...)` 之后）加：

```go
	pricing.RegisterAdminRoutes(adminGroup, pricingHandler)
```

在 import 里加：

```go
	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
```

### Step 6: 构建 + 启动 smoke

```bash
cd /Users/adam/workspace/github/trademark-admin
go -C apps/api build ./...
go -C apps/api test ./internal/auth
```

Expected: 两条都通过。

### Step 7: 提交

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/pricing/handler.go \
        apps/api/internal/pricing/router.go \
        apps/api/cmd/server/main.go
# include middleware.go only if step 2 modified it:
git add apps/api/internal/auth/middleware.go 2>/dev/null || true
git commit -m "$(cat <<'EOF'
feat(api): pricing HTTP handlers + routes wired with role gates

Reviewer+admin can GET /pricing-entries[?country_id=&service_tier=] and
/pricing-entries/history?country_id=&service_tier=&fee_item=. Admin can
POST /pricing-entries (create-or-replace, idempotent per active
dimension) and POST /pricing-entries/:id/deprecate. Salesperson gets
403. RequireRole now accepts variadic roles so we can say
RequireRole("reviewer", "admin") without listing two middlewares.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Repository 积分测试（testcontainers Postgres）

**Files:**
- Create: `apps/api/internal/pricing/repository_test.go`

沿用 catalog / customer 的 testcontainers 模式，跑真实 Postgres + migration + 测 CRUD。

- [ ] **Step 1: 找一个已存在的 testcontainers helper 作参考**

```bash
grep -l "testcontainers" apps/api/internal/ -r | head -5
```

预期命中 `internal/customer/repository_test.go` 或 `internal/catalog/repository_test.go`。抄它的 setup（startPostgres / runMigrations helper）到 pricing 包里同样的文件名。注意包名要是 `pricing`。

- [ ] **Step 2: 写 repository_test.go**

Create `apps/api/internal/pricing/repository_test.go`:

```go
package pricing

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	// local pkg used to run migrations; mirrors customer/catalog tests
	apiroot "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
)

// startDB boots a Postgres container, runs all migrations, and returns
// a GORM handle + teardown func. It mirrors the pattern used in
// internal/customer/repository_test.go and internal/catalog/repository_test.go.
func startDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(postgres.BasicWaitStrategies()),
	)
	require.NoError(t, err)

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	mig, err := migrator.New(apiroot.Migrations, "migrations", dsn)
	require.NoError(t, err)
	require.NoError(t, mig.Up())
	_ = mig.Close()

	db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	return db, func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
		_ = pg.Terminate(ctx)
	}
}

// seedCountryUser inserts one country + one user so pricing_entries'
// FKs are satisfied. Returns the ids.
func seedCountryUser(t *testing.T, db *gorm.DB) (uuid.UUID, uuid.UUID) {
	t.Helper()
	var countryID, userID uuid.UUID
	// Pick an existing seeded country (code='CN' should come from seed data).
	err := db.Raw(`SELECT id FROM countries WHERE code = 'CN'`).Row().Scan(&countryID)
	require.NoError(t, err, "countries seed missing CN — seed data may have changed")
	// Create a minimal user row. Users table has NOT NULL columns for
	// email / password_hash / name / role; we bypass GORM structs and
	// insert raw so we aren't coupled to the auth package.
	err = db.Exec(`INSERT INTO users (id, email, password_hash, name, role, status)
		VALUES (gen_random_uuid(), 'pricing-test@example.com', 'x', 'Test',
		        (SELECT id FROM roles WHERE code = 'admin'), 'active')
		RETURNING id`).
		Row().Scan(&userID)
	// Above RETURNING inside Exec isn't supported by all drivers; fall back to SELECT MAX.
	if err != nil || userID == uuid.Nil {
		require.NoError(t, db.Exec(`
			INSERT INTO users (id, email, password_hash, name, role, status)
			VALUES ($1, $2, $3, $4, (SELECT id FROM roles WHERE code='admin'), 'active')`,
			uuid.New(), "pricing-test@example.com", "x", "Pricing Test").Error)
		var s string
		require.NoError(t, db.Raw(`SELECT id::text FROM users WHERE email = ?`, "pricing-test@example.com").Row().Scan(&s))
		userID = uuid.MustParse(s)
	}
	return countryID, userID
}

func TestReplaceActive_InsertsNewAndDeprecatesOld(t *testing.T) {
	db, cleanup := startDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewRepository(db)

	countryID, userID := seedCountryUser(t, db)
	day1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	// First insert.
	e1, err := repo.ReplaceActive(ctx, NewEntry{
		CountryID: countryID, ServiceTier: "basic", FeeItem: "application",
		AmountCNYCents: 10000, EffectiveFrom: day1, CreatedBy: userID,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, e1.ID)
	assert.Nil(t, e1.EffectiveTo)

	// Replace — must deprecate e1 and insert e2, all in one tx.
	e2, err := repo.ReplaceActive(ctx, NewEntry{
		CountryID: countryID, ServiceTier: "basic", FeeItem: "application",
		AmountCNYCents: 12000, EffectiveFrom: day2, CreatedBy: userID,
	})
	require.NoError(t, err)
	assert.Nil(t, e2.EffectiveTo)

	// After the replace, ListHistory returns both rows, newest first;
	// the old row has effective_to = day2.
	hist, err := repo.ListHistory(ctx, HistoryFilter{
		CountryID: countryID, ServiceTier: "basic", FeeItem: "application",
	})
	require.NoError(t, err)
	require.Len(t, hist, 2)
	assert.Equal(t, e2.ID, hist[0].ID)
	assert.Nil(t, hist[0].EffectiveTo)
	assert.Equal(t, e1.ID, hist[1].ID)
	require.NotNil(t, hist[1].EffectiveTo)
	assert.True(t, hist[1].EffectiveTo.Equal(day2), "old entry's effective_to should equal new's effective_from")

	// ListActive returns only e2.
	active, err := repo.ListActive(ctx, ActiveFilter{CountryID: &countryID})
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, e2.ID, active[0].ID)
}

func TestDeprecate_AlreadyDeprecated(t *testing.T) {
	db, cleanup := startDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewRepository(db)
	countryID, userID := seedCountryUser(t, db)

	day1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	e1, err := repo.ReplaceActive(ctx, NewEntry{
		CountryID: countryID, ServiceTier: "basic", FeeItem: "renewal",
		AmountCNYCents: 5000, EffectiveFrom: day1, CreatedBy: userID,
	})
	require.NoError(t, err)

	_, err = repo.Deprecate(ctx, e1.ID, day2)
	require.NoError(t, err)

	// Second deprecate must error.
	_, err = repo.Deprecate(ctx, e1.ID, day2.Add(24*time.Hour))
	assert.ErrorIs(t, err, ErrNoActive)
}

func TestDeprecate_EffectiveToMustBeAfterFrom(t *testing.T) {
	db, cleanup := startDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewRepository(db)
	countryID, userID := seedCountryUser(t, db)

	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	e, err := repo.ReplaceActive(ctx, NewEntry{
		CountryID: countryID, ServiceTier: "standard", FeeItem: "x",
		AmountCNYCents: 1000, EffectiveFrom: from, CreatedBy: userID,
	})
	require.NoError(t, err)

	// effective_to = effective_from is invalid
	_, err = repo.Deprecate(ctx, e.ID, from)
	require.Error(t, err)

	// past date also invalid
	_, err = repo.Deprecate(ctx, e.ID, from.Add(-24*time.Hour))
	require.Error(t, err)
}

func TestListActive_FiltersByTier(t *testing.T) {
	db, cleanup := startDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewRepository(db)
	countryID, userID := seedCountryUser(t, db)

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	for _, tier := range []string{"basic", "standard", "premium"} {
		_, err := repo.ReplaceActive(ctx, NewEntry{
			CountryID: countryID, ServiceTier: tier, FeeItem: "application",
			AmountCNYCents: 1000, EffectiveFrom: from, CreatedBy: userID,
		})
		require.NoError(t, err)
	}

	tier := "standard"
	got, err := repo.ListActive(ctx, ActiveFilter{CountryID: &countryID, ServiceTier: &tier})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "standard", got[0].ServiceTier)
}
```

- [ ] **Step 3: 跑测试**

```bash
cd /Users/adam/workspace/github/trademark-admin
go -C apps/api test ./internal/pricing -v
```

Expected: 4 calc tests + 4 repo tests = 8 PASS（Repo 测试跑 Postgres，单次可能 20-40s）。

- [ ] **Step 4: `go mod tidy` 可能会把 testcontainers 的间接依赖改成直接依赖**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go mod tidy
git diff go.mod go.sum | head -30
```

如果 `go.mod require` 区加了 `github.com/testcontainers/testcontainers-go` 或 `github.com/stretchr/testify`（已经在），stage 并提交。

- [ ] **Step 5: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/pricing/repository_test.go
# 若 go.mod / go.sum 有变化：
git add apps/api/go.mod apps/api/go.sum 2>/dev/null || true
git commit -m "$(cat <<'EOF'
test(api): pricing repository integration tests (testcontainers)

Covers the four behaviours that really matter: ReplaceActive atomically
deprecates old+inserts new, double-deprecate errors, deprecate refuses
past/equal effective_to, and ListActive respects tier filter. Setup
reuses the migrator to run all migrations inside the container so the
CHECK constraints are exercised for real.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: 端到端 smoke test —— 跑服务器，admin 起一个 entry，reviewer 能读，salesperson 拿 403

**Files:**
- (无新增 — 纯 bash / curl 验证)

- [ ] **Step 1: 启 Postgres + 跑迁移 + 启服务器**

```bash
cd /Users/adam/workspace/github/trademark-admin
docker compose -f apps/api/docker-compose.yml up -d postgres
# 终端 A
go -C apps/api run ./cmd/server
```

等到日志里出现 `api listening`。

- [ ] **Step 2: 登录 admin，记下 cookie jar**

```bash
# 终端 B
cd /Users/adam/workspace/github/trademark-admin
curl -s -c /tmp/tm-admin.cookies -b /tmp/tm-admin.cookies \
  -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"change-me-on-first-login"}' | head -40

# 拿 CN 的 country id
CN_ID=$(curl -s -b /tmp/tm-admin.cookies http://localhost:8080/api/v1/catalog/countries | jq -r '.items[] | select(.code=="CN") | .id')
echo "CN=$CN_ID"
```

- [ ] **Step 3: 创建一个 pricing entry**

```bash
# admin 登录态下，带上 CSRF cookie
curl -s -b /tmp/tm-admin.cookies -c /tmp/tm-admin.cookies \
  -X POST http://localhost:8080/api/v1/pricing-entries \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $(awk '$6=="tm_csrf_token" {print $7}' /tmp/tm-admin.cookies)" \
  -d "{
    \"country_id\": \"$CN_ID\",
    \"service_tier\": \"basic\",
    \"fee_item\": \"application_fee\",
    \"amount_cny_cents\": 80000,
    \"effective_from\": \"2026-01-01\"
  }" | jq .
```

Expected: 201 + JSON 包含 `id`, `effective_to: null`, `signature: null (N/A here — only calc emits it)`.

- [ ] **Step 4: GET 列表**

```bash
curl -s -b /tmp/tm-admin.cookies http://localhost:8080/api/v1/pricing-entries | jq '.items | length'
```

Expected: 1.

- [ ] **Step 5: 再 POST 同 dimension 一条 — replace 生效**

```bash
curl -s -b /tmp/tm-admin.cookies \
  -X POST http://localhost:8080/api/v1/pricing-entries \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $(awk '$6=="tm_csrf_token" {print $7}' /tmp/tm-admin.cookies)" \
  -d "{
    \"country_id\": \"$CN_ID\",
    \"service_tier\": \"basic\",
    \"fee_item\": \"application_fee\",
    \"amount_cny_cents\": 90000,
    \"effective_from\": \"2026-07-01\"
  }" | jq .
```

- [ ] **Step 6: GET history 看到 2 条**

```bash
curl -s -b /tmp/tm-admin.cookies \
  "http://localhost:8080/api/v1/pricing-entries/history?country_id=$CN_ID&service_tier=basic&fee_item=application_fee" \
  | jq '.items | length, .items[] | {effective_from, effective_to, amount_cny_cents}'
```

Expected: length 2，首行 `effective_to: null`，次行 `effective_to: "2026-07-01"`。

- [ ] **Step 7: Salesperson 角色验证 403**

这步需要一个非 admin 账号。用 admin 先创建一个：

```bash
curl -s -b /tmp/tm-admin.cookies \
  -X POST http://localhost:8080/api/v1/admin/users \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $(awk '$6=="tm_csrf_token" {print $7}' /tmp/tm-admin.cookies)" \
  -d '{"email":"sales@example.com","password":"salespass123","name":"Sales","role":"salesperson"}' | jq .
```

然后用 sales 登录，尝试 GET pricing-entries：

```bash
curl -s -c /tmp/tm-sales.cookies -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"sales@example.com","password":"salespass123"}'
curl -s -b /tmp/tm-sales.cookies -o /dev/null -w '%{http_code}\n' \
  http://localhost:8080/api/v1/pricing-entries
```

Expected: `403`.

- [ ] **Step 8: 清理、停服务**

```bash
# 终端 A 按 Ctrl-C
# 终端 B
rm -f /tmp/tm-admin.cookies /tmp/tm-sales.cookies
```

这一步不产生 commit —— 纯验收。若有任意失败，回到 Task 6 / Task 3 排查。

---

## Plan 6 Definition of Done

1. ✅ Migration 000003 上 + 下都可跑
2. ✅ `go -C apps/api build ./...` 干净
3. ✅ `go -C apps/api test ./...` 全绿（pricing 里新加 8 个测试）
4. ✅ Admin 能 POST /pricing-entries 创建 + 替换（同维度自动 deprecate 旧行）
5. ✅ Admin 能 POST /pricing-entries/:id/deprecate
6. ✅ Reviewer 能 GET /pricing-entries + /pricing-entries/history
7. ✅ Salesperson GET /pricing-entries 得到 403
8. ✅ Calculate 对给定 slice 输出 stable signature

## 下一步

Plan 7：前端定价 —— reviewer/admin 可见的 `/pricing/*` 页面：二维表格（country × service_tier），每格展开 fee_items 明细 + 编辑抽屉 + 历史时间线；sidebar 加"定价"入口（仅 reviewer/admin）。
