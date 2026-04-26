# M3 Quotation Wizard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the single-page `QuotationFormSheet` with a 5-step wizard (customer → country → tier → notes → preview) backed by a new `POST /quotations/preview` non-persistent API, with `zustand + persist` client-side draft persistence and a "resume draft" banner on re-entry.

**Architecture:** Frontend `zustand` store holds wizard state, persisted to `localStorage` keyed by user id. The preview step calls a new backend endpoint that reuses the existing `pricing.Calculate` engine without touching the DB. On final submit, the preview step calls the existing `POST /quotations` + `POST /quotations/:id/submit` in sequence (new create-then-submit hook). Edit mode loads server draft and finally calls `PATCH /quotations/:id` + optional `/submit`. Keeps the current 4-field quotation model intact; no migrations.

**Tech Stack:** Go 1.25 + Gin + GORM (backend); React 19 + TanStack Router + TanStack Query + zustand (with persist middleware) + shadcn/ui (no Stepper primitive exists — build minimal indicator from button + Progress-style dots).

**Spec:** `docs/superpowers/specs/2026-04-26-m3-quotation-wizard-design.md`

---

## File structure

**New backend files:** none (all additions go into existing files in `apps/api/internal/quotation/`)

**Modified backend files**
- `apps/api/internal/quotation/dto.go` — add `PreviewRequest`, `PreviewResponse`
- `apps/api/internal/quotation/service.go` — add `customerRepo` interface, inject it, add `Preview` method
- `apps/api/internal/quotation/service_test.go` — extend `fakeRepo` with a `fakeCustomerRepo`; 4 new `TestService_Preview_*`
- `apps/api/internal/quotation/handler.go` — add `Preview` handler
- `apps/api/internal/quotation/router.go` — register `POST /quotations/preview` on authed group
- `apps/api/internal/quotation/handler_test.go` — 4 integration tests for preview
- `apps/api/cmd/server/main.go` — wire `customer.Repository` into `quotation.NewService`

**New frontend files**
- `apps/web/src/features/quotation/wizard/wizard-store.ts`
- `apps/web/src/features/quotation/wizard/wizard-store.test.ts`
- `apps/web/src/features/quotation/wizard/quotation-wizard.tsx`
- `apps/web/src/features/quotation/wizard/resume-banner.tsx`
- `apps/web/src/features/quotation/wizard/steps/step-customer.tsx`
- `apps/web/src/features/quotation/wizard/steps/step-country.tsx`
- `apps/web/src/features/quotation/wizard/steps/step-tier.tsx`
- `apps/web/src/features/quotation/wizard/steps/step-notes.tsx`
- `apps/web/src/features/quotation/wizard/steps/step-preview.tsx`
- `apps/web/src/features/quotation/wizard/hooks/use-preview.ts`
- `apps/web/src/routes/_authenticated/quotations/new.tsx`
- `apps/web/src/routes/_authenticated/quotations/$id.edit.tsx`

**Modified frontend files**
- `apps/web/src/features/quotation/hooks/use-quotation-mutations.ts` — add `useCreateAndSubmit`, `useUpdateAndSubmit`
- `apps/web/src/features/quotation/index.tsx` — list page "新建报价" button becomes `<Link to="/quotations/new">`
- `apps/web/src/features/quotation/detail.tsx` — "编辑草稿" becomes `<Link to="/quotations/$id/edit">`
- `apps/web/src/features/quotation/components/quotation-action-bar.tsx` — if this is where "编辑草稿" lives, same Link change
- `apps/web/src/features/quotation/quotation.integration.test.tsx` — 5 new scenarios
- `apps/web/src/test-utils/msw/handlers.ts` — MSW handler for `POST /quotations/preview`

**Deleted frontend files**
- `apps/web/src/features/quotation/components/quotation-form-sheet.tsx` (after routes swap)

---

## Task list (16 tasks)

Execution order: **T1 → T2 → T3** (backend foundation) → **T4 → T5 → T6** (frontend store + hooks) → **T7–T11** (5 step components) → **T12** (resume banner) → **T13** (wizard shell) → **T14 → T15 → T16** (routes + entry swap + integration tests).

---

### Task 1: Preview DTO

**Files:**
- Modify: `apps/api/internal/quotation/dto.go`

- [ ] **Step 1: Add `PreviewRequest` + `PreviewResponse`**

Append at the bottom of `dto.go`, after `HistoryEntry`:

```go
// PreviewRequest is the body of POST /quotations/preview — a non-persistent
// pricing lookup used by the wizard before the quotation row exists.
// Validation tags mirror CreateRequest so bad bodies are rejected before
// reaching the service.
type PreviewRequest struct {
	CustomerID  uuid.UUID `json:"customer_id"  binding:"required"`
	CountryID   uuid.UUID `json:"country_id"   binding:"required"`
	ServiceTier string    `json:"service_tier" binding:"required"`
}

// PreviewResponse is the shape returned by POST /quotations/preview.
// Intentionally mirrors the quotation Snapshot so the frontend can reuse
// the same rendering component (QuotationSnapshotView).
type PreviewResponse struct {
	Lines         []SnapshotLine `json:"lines"`
	TotalCNYCents int64          `json:"total_cny_cents"`
	Signature     string         `json:"signature"`
}
```

- [ ] **Step 2: Build**

Run: `cd apps/api && go build ./...`
Expected: compiles cleanly.

- [ ] **Step 3: Commit**

```bash
git add apps/api/internal/quotation/dto.go
git commit -m "$(cat <<'EOF'
feat(api): add PreviewRequest + PreviewResponse DTOs for M3

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: `Service.Preview` (TDD with fake repos)

**Files:**
- Modify: `apps/api/internal/quotation/service.go`
- Modify: `apps/api/internal/quotation/service_test.go`

- [ ] **Step 1: Add `customerRepo` interface to service.go**

Find the existing `repo` / `pricingRepo` interface block (near line 28–45) and add a third interface right after `pricingRepo`:

```go
// customerRepo is the subset of customer.Repository we need for the
// Preview endpoint. Get with ownerID=nil is an existence check (returns
// customer.ErrNotFound if the id isn't found). We depend on the
// interface rather than the concrete type to keep the service testable
// with fakes.
type customerRepo interface {
	Get(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*customer.Customer, error)
}
```

Add the import for `customer`:

```go
import (
	// ...existing imports...
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
)
```

- [ ] **Step 2: Update `Service` struct + constructor**

Change the struct and `NewService` signature:

```go
type Service struct {
	repo         repo
	pricingRepo  pricingRepo
	customerRepo customerRepo
}

func NewService(r repo, p pricingRepo, c customerRepo) *Service {
	return &Service{repo: r, pricingRepo: p, customerRepo: c}
}
```

- [ ] **Step 3: Write failing tests**

Find the end of `service_test.go`'s `fakeRepo` implementation and add a fake customer repo + 4 tests:

```go
// fakeCustomerRepo lets service tests control whether "customer exists"
// without touching Postgres. A nil entry for an id means "not found".
type fakeCustomerRepo struct{ byID map[uuid.UUID]*customer.Customer }

func newFakeCustomerRepo() *fakeCustomerRepo {
	return &fakeCustomerRepo{byID: map[uuid.UUID]*customer.Customer{}}
}

func (f *fakeCustomerRepo) Get(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*customer.Customer, error) {
	c := f.byID[id]
	if c == nil {
		return nil, customer.ErrNotFound
	}
	return c, nil
}

// fakePricingRepo also already exists further below — add Preview setup
// alongside the existing fakes.

func TestService_Preview_Success(t *testing.T) {
	custID, countryID := uuid.New(), uuid.New()
	custRepo := newFakeCustomerRepo()
	custRepo.byID[custID] = &customer.Customer{ID: custID, Name: "Acme"}
	pricingRepo := &fakePricingRepo{entries: []pricing.PricingEntry{
		{ID: uuid.New(), CountryID: countryID, ServiceTier: "basic", FeeItem: "application", AmountCNYCents: 50000},
	}}
	svc := NewService(newFakeRepo(), pricingRepo, custRepo)

	resp, err := svc.Preview(context.Background(), PreviewRequest{
		CustomerID:  custID,
		CountryID:   countryID,
		ServiceTier: "basic",
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(resp.Lines) != 1 || resp.Lines[0].FeeItem != "application" || resp.Lines[0].AmountCNYCents != 50000 {
		t.Fatalf("lines = %+v, want one application line", resp.Lines)
	}
	if resp.TotalCNYCents != 50000 {
		t.Fatalf("total = %d, want 50000", resp.TotalCNYCents)
	}
	if len(resp.Signature) != 64 {
		t.Fatalf("signature len = %d, want 64", len(resp.Signature))
	}
}

func TestService_Preview_InvalidTier(t *testing.T) {
	svc := NewService(newFakeRepo(), &fakePricingRepo{}, newFakeCustomerRepo())
	_, err := svc.Preview(context.Background(), PreviewRequest{
		CustomerID:  uuid.New(),
		CountryID:   uuid.New(),
		ServiceTier: "bogus",
	})
	if !errors.Is(err, ErrInvalidTier) {
		t.Fatalf("err = %v, want ErrInvalidTier", err)
	}
}

func TestService_Preview_CustomerNotFound(t *testing.T) {
	svc := NewService(newFakeRepo(), &fakePricingRepo{}, newFakeCustomerRepo())
	_, err := svc.Preview(context.Background(), PreviewRequest{
		CustomerID:  uuid.New(),
		CountryID:   uuid.New(),
		ServiceTier: "basic",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestService_Preview_MissingPricing(t *testing.T) {
	custID, countryID := uuid.New(), uuid.New()
	custRepo := newFakeCustomerRepo()
	custRepo.byID[custID] = &customer.Customer{ID: custID, Name: "Acme"}
	// No pricing entries seeded → Calculate returns ErrNoMatchingEntries.
	svc := NewService(newFakeRepo(), &fakePricingRepo{}, custRepo)

	_, err := svc.Preview(context.Background(), PreviewRequest{
		CustomerID:  custID,
		CountryID:   countryID,
		ServiceTier: "basic",
	})
	if !errors.Is(err, ErrMissingPricing) {
		t.Fatalf("err = %v, want ErrMissingPricing", err)
	}
}
```

Add import at the top of `service_test.go`:

```go
import (
	// ...existing imports...
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
)
```

Also update every existing `NewService(...)` call site in `service_test.go` to pass a third argument. Search for `NewService(` in that file and append `, newFakeCustomerRepo()` to each call.

- [ ] **Step 4: Run tests — verify fail**

Run: `cd apps/api && go test ./internal/quotation/ -run TestService_Preview -count=1`
Expected: FAIL with `Service.Preview undefined` (or compile error — that's fine).

- [ ] **Step 5: Implement `Service.Preview`**

Append to `service.go`, placed after `List`:

```go
// Preview computes a snapshot for the (customer, country, tier) triple
// WITHOUT touching the quotations table. Used by the 5-step wizard to
// show the business user what pricing they're about to freeze before
// they commit to creating a draft row.
//
// Contract:
//   - tier must be in the valid enum → ErrInvalidTier
//   - customer_id must exist → ErrNotFound
//   - at least one active pricing entry must match (country, tier)
//     → ErrMissingPricing otherwise
//
// No side effects. Safe for any authenticated role.
func (s *Service) Preview(ctx context.Context, req PreviewRequest) (*PreviewResponse, error) {
	if !pricing.IsValidServiceTier(req.ServiceTier) {
		return nil, ErrInvalidTier
	}
	cust, err := s.customerRepo.Get(ctx, req.CustomerID, nil)
	if err != nil {
		if errors.Is(err, customer.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("quotation: customer lookup: %w", err)
	}
	if cust == nil {
		return nil, ErrNotFound
	}

	entries, err := s.pricingRepo.ListActive(ctx, &req.CountryID)
	if err != nil {
		return nil, fmt.Errorf("quotation: list pricing: %w", err)
	}
	calc, err := pricing.Calculate(entries, pricing.CalcInput{
		CountryID:   req.CountryID,
		ServiceTier: req.ServiceTier,
	})
	if err != nil {
		if errors.Is(err, pricing.ErrNoMatchingEntries) {
			return nil, ErrMissingPricing
		}
		return nil, fmt.Errorf("quotation: pricing calculate: %w", err)
	}

	lines := make([]SnapshotLine, 0, len(calc.Lines))
	for _, l := range calc.Lines {
		lines = append(lines, SnapshotLine{FeeItem: l.FeeItem, AmountCNYCents: l.AmountCNYCents})
	}
	return &PreviewResponse{
		Lines:         lines,
		TotalCNYCents: calc.TotalCNYCents,
		Signature:     calc.Signature,
	}, nil
}
```

- [ ] **Step 6: Run tests — verify pass**

Run: `cd apps/api && go test ./internal/quotation/ -count=1`
Expected: all pass, including 4 new `TestService_Preview_*`.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/quotation/service.go apps/api/internal/quotation/service_test.go
git commit -m "$(cat <<'EOF'
feat(api): add Service.Preview for wizard pricing lookup

Non-persistent variant of Submit's snapshot computation — validates
tier, checks customer existence, reuses pricing.Calculate. Returns
ErrInvalidTier / ErrNotFound / ErrMissingPricing matching the Submit
error taxonomy.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Preview handler + router + main.go wiring + integration tests

**Files:**
- Modify: `apps/api/internal/quotation/handler.go`
- Modify: `apps/api/internal/quotation/router.go`
- Modify: `apps/api/internal/quotation/handler_test.go`
- Modify: `apps/api/cmd/server/main.go`

- [ ] **Step 1: Add `Preview` handler**

In `handler.go`, add a new handler method after `Copy` (around line 197):

```go
// POST /quotations/preview — non-persistent pricing lookup for the
// 5-step wizard. Any authenticated user may call; the service returns
// the same error taxonomy as Submit.
func (h *Handler) Preview(c *gin.Context) {
	var req PreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	resp, err := h.svc.Preview(c.Request.Context(), req)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
```

- [ ] **Step 2: Register the route**

In `router.go`, add a new line inside `RegisterAuthedRoutes` BEFORE the `:id` routes so Gin's path-matching doesn't treat `preview` as a UUID:

```go
func RegisterAuthedRoutes(g *gin.RouterGroup, h *Handler) {
	g.POST("/quotations", h.Create)
	g.POST("/quotations/preview", h.Preview)  // ← new
	g.GET("/quotations", h.List)
	g.GET("/quotations/:id", h.Get)
	g.GET("/quotations/:id/history", h.History)
	g.PATCH("/quotations/:id", h.Update)
	g.POST("/quotations/:id/submit", h.Submit)
	g.POST("/quotations/:id/cancel", h.Cancel)
	g.POST("/quotations/:id/withdraw", h.Withdraw)
	g.POST("/quotations/:id/copy", h.Copy)
}
```

- [ ] **Step 3: Update `main.go` wiring**

In `apps/api/cmd/server/main.go`, find the line `quotSvc := quotation.NewService(quotRepo, ...)` and update it to pass the customer repo. Search for the existing `customer.NewRepository(db)` line — it should already exist. The new call looks like:

```go
quotSvc := quotation.NewService(quotRepo, pricingAdapter, customerRepo)
```

Where `customerRepo` is the already-constructed `*customer.Repository`. If the existing code constructs customer inside the function, reuse that variable; if it's built later, hoist its construction above quotation's.

- [ ] **Step 4: Update `pricingRepoAdapter` in handler_test.go**

Look at `handler_test.go` — the existing `pricingRepoAdapter` is already defined. Add a compatible customer repo adapter (customer.Repository already matches the interface directly, no adapter needed). The existing tests call `quotation.NewService(quotRepo, pricingRepoAdapter{...})` — update every call to add a third argument.

Search for `quotation.NewService(` in `handler_test.go` and append `, customer.NewRepository(db)` to each call.

Add the import if missing:

```go
import (
	// ...
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
)
```

- [ ] **Step 5: Write first failing integration test**

Append to `handler_test.go`:

```go
func TestHandler_Preview_OK(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, salesID := seedCustomerCountryUser(t, db)

	// Seed one active pricing entry so the preview can produce lines.
	if err := db.Exec(
		`INSERT INTO pricing_entries
		 (id, country_id, service_tier, fee_item, amount_cny_cents, effective_from, created_by)
		 VALUES (?, ?, 'basic', 'application', 50000, ?, ?)`,
		uuid.New(), countryID, time.Now(), salesID,
	).Error; err != nil {
		t.Fatalf("seed pricing: %v", err)
	}

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc))

	body, _ := json.Marshal(map[string]any{
		"customer_id":  custID,
		"country_id":   countryID,
		"service_tier": "basic",
	})
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("preview: status %d body %s", w.Code, w.Body.String())
	}
	var resp quotation.PreviewResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.TotalCNYCents != 50000 || len(resp.Lines) != 1 {
		t.Fatalf("resp = %+v, want 1 line totalling 50000", resp)
	}
	if resp.Signature == "" {
		t.Fatalf("empty signature")
	}
}
```

- [ ] **Step 6: Run — verify pass**

Run: `cd apps/api && go test ./internal/quotation/ -run TestHandler_Preview_OK -count=1`
Expected: PASS.

- [ ] **Step 7: Add 3 more integration tests**

Append after `TestHandler_Preview_OK`:

```go
func TestHandler_Preview_BadBody(t *testing.T) {
	db, _ := bootPg(t)
	_, _, salesID := seedCustomerCountryUser(t, db)

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc))

	// Missing country_id.
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/preview", strings.NewReader(`{"customer_id":"00000000-0000-0000-0000-000000000001","service_tier":"basic"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ERR_INVALID_BODY") {
		t.Fatalf("body = %s, want ERR_INVALID_BODY", w.Body.String())
	}
}

func TestHandler_Preview_CustomerNotFound(t *testing.T) {
	db, _ := bootPg(t)
	_, countryID, salesID := seedCustomerCountryUser(t, db)

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc))

	body, _ := json.Marshal(map[string]any{
		"customer_id":  uuid.New(), // does NOT exist
		"country_id":   countryID,
		"service_tier": "basic",
	})
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Preview_MissingPricing(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, salesID := seedCustomerCountryUser(t, db)
	// No pricing entries seeded.

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc))

	body, _ := json.Marshal(map[string]any{
		"customer_id":  custID,
		"country_id":   countryID,
		"service_tier": "basic",
	})
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ERR_MISSING_PRICING") {
		t.Fatalf("body = %s, want ERR_MISSING_PRICING", w.Body.String())
	}
}
```

- [ ] **Step 8: Run all tests**

Run: `cd apps/api && go test ./internal/quotation/... -count=1`
Expected: all pass including existing `TestHandler_HappyPath_*` and 4 new `TestHandler_Preview_*`.

- [ ] **Step 9: Run full backend build + test**

Run: `cd apps/api && go build ./... && go test ./... -count=1`
Expected: everything green.

- [ ] **Step 10: Commit**

```bash
git add apps/api/internal/quotation/handler.go \
        apps/api/internal/quotation/router.go \
        apps/api/internal/quotation/handler_test.go \
        apps/api/internal/quotation/service_test.go \
        apps/api/cmd/server/main.go
git commit -m "$(cat <<'EOF'
feat(api): add POST /quotations/preview endpoint for M3 wizard

Non-persistent pricing lookup; wires customer repo into quotation service
for the existence check. Returns 400/404/422 matching the service
error taxonomy.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Wizard store (zustand + persist, TDD)

**Files:**
- Create: `apps/web/src/features/quotation/wizard/wizard-store.ts`
- Create: `apps/web/src/features/quotation/wizard/wizard-store.test.ts`

- [ ] **Step 1: Write failing tests**

Create `apps/web/src/features/quotation/wizard/wizard-store.test.ts`:

```ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import {
  createWizardStore,
  isStepCustomerValid,
  isStepCountryValid,
  isStepTierValid,
  type WizardDraft,
} from './wizard-store'
import type { Quotation } from '../types'

const USER_A = '00000000-0000-0000-0000-00000000A001'
const USER_B = '00000000-0000-0000-0000-00000000B002'

const empty: WizardDraft = {
  customer_id: '',
  country_id: '',
  service_tier: 'basic',
  notes: '',
}

describe('wizard-store', () => {
  beforeEach(() => {
    localStorage.clear()
  })
  afterEach(() => {
    localStorage.clear()
  })

  it('starts empty with step 0 and no editingId', () => {
    const store = createWizardStore(USER_A)
    const s = store.getState()
    expect(s.currentStep).toBe(0)
    expect(s.editingId).toBeNull()
    expect(s.draft).toEqual(empty)
  })

  it('patchDraft merges fields and preserves the rest', () => {
    const store = createWizardStore(USER_A)
    store.getState().patchDraft({ customer_id: 'c1' })
    store.getState().patchDraft({ notes: 'hi' })
    const s = store.getState()
    expect(s.draft.customer_id).toBe('c1')
    expect(s.draft.notes).toBe('hi')
    expect(s.draft.service_tier).toBe('basic')
  })

  it('reset clears draft, step, and editingId', () => {
    const store = createWizardStore(USER_A)
    store.getState().patchDraft({ customer_id: 'c1', notes: 'x' })
    store.getState().setStep(2)
    store.getState().reset()
    const s = store.getState()
    expect(s.draft).toEqual(empty)
    expect(s.currentStep).toBe(0)
    expect(s.editingId).toBeNull()
  })

  it('loadForEdit sets editingId and fills draft from a Quotation', () => {
    const store = createWizardStore(USER_A)
    const q: Quotation = {
      id: 'q1', customer_id: 'c1', country_id: 'co1', service_tier: 'premium',
      status: 'draft', notes: 'edit-me', created_by: USER_A,
      created_at: '2026-04-26T00:00:00Z', updated_at: '2026-04-26T00:00:00Z',
    }
    store.getState().loadForEdit('q1', q)
    const s = store.getState()
    expect(s.editingId).toBe('q1')
    expect(s.draft.customer_id).toBe('c1')
    expect(s.draft.country_id).toBe('co1')
    expect(s.draft.service_tier).toBe('premium')
    expect(s.draft.notes).toBe('edit-me')
    expect(s.currentStep).toBe(0)
  })

  it('loadForEdit maps null notes to empty string', () => {
    const store = createWizardStore(USER_A)
    const q: Quotation = {
      id: 'q1', customer_id: 'c1', country_id: 'co1', service_tier: 'basic',
      status: 'draft', notes: null, created_by: USER_A,
      created_at: '2026-04-26T00:00:00Z', updated_at: '2026-04-26T00:00:00Z',
    }
    store.getState().loadForEdit('q1', q)
    expect(store.getState().draft.notes).toBe('')
  })

  it('different user ids have independent localStorage keys', () => {
    const a = createWizardStore(USER_A)
    a.getState().patchDraft({ customer_id: 'cA' })
    const b = createWizardStore(USER_B)
    expect(b.getState().draft.customer_id).toBe('')
    b.getState().patchDraft({ customer_id: 'cB' })
    // Re-opening USER_A's store should still see cA.
    const a2 = createWizardStore(USER_A)
    expect(a2.getState().draft.customer_id).toBe('cA')
  })

  it('isStepCustomerValid requires a uuid-ish non-empty customer_id', () => {
    expect(isStepCustomerValid({ ...empty })).toBe(false)
    expect(isStepCustomerValid({ ...empty, customer_id: 'c1' })).toBe(true)
  })

  it('isStepCountryValid requires a non-empty country_id', () => {
    expect(isStepCountryValid({ ...empty, customer_id: 'c1' })).toBe(false)
    expect(isStepCountryValid({ ...empty, customer_id: 'c1', country_id: 'co1' })).toBe(true)
  })

  it('isStepTierValid accepts any enum value', () => {
    expect(isStepTierValid({ ...empty, service_tier: 'basic' })).toBe(true)
    expect(isStepTierValid({ ...empty, service_tier: 'standard' })).toBe(true)
    expect(isStepTierValid({ ...empty, service_tier: 'premium' })).toBe(true)
  })
})
```

- [ ] **Step 2: Run tests — verify fail**

Run: `cd apps/web && pnpm vitest run src/features/quotation/wizard/wizard-store.test.ts`
Expected: FAIL — module doesn't exist.

- [ ] **Step 3: Implement the store**

Create `apps/web/src/features/quotation/wizard/wizard-store.ts`:

```ts
import { create, type StoreApi, type UseBoundStore } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'

import type { Quotation, ServiceTier } from '../types'

// WizardDraft carries the 4 editable quotation fields plus a default
// tier. Kept as a flat shape so zustand patch calls are trivial.
export interface WizardDraft {
  customer_id: string
  country_id: string
  service_tier: ServiceTier
  notes: string
}

export interface WizardState {
  currentStep: 0 | 1 | 2 | 3 | 4
  draft: WizardDraft
  editingId: string | null

  setStep: (step: 0 | 1 | 2 | 3 | 4) => void
  patchDraft: (patch: Partial<WizardDraft>) => void
  reset: () => void
  loadForEdit: (id: string, serverDraft: Quotation) => void
}

const EMPTY_DRAFT: WizardDraft = {
  customer_id: '',
  country_id: '',
  service_tier: 'basic',
  notes: '',
}

// createWizardStore is user-scoped — each authenticated user gets their
// own localStorage slot so logging in as a different user never sees
// the previous user's draft. The caller (route component) constructs
// the store lazily via useWizardStore().
export function createWizardStore(userId: string): UseBoundStore<StoreApi<WizardState>> {
  const storageKey = `quotation-wizard-draft:${userId}`
  return create<WizardState>()(
    persist(
      (set) => ({
        currentStep: 0,
        draft: { ...EMPTY_DRAFT },
        editingId: null,
        setStep: (step) => set({ currentStep: step }),
        patchDraft: (patch) => set((s) => ({ draft: { ...s.draft, ...patch } })),
        reset: () => set({ currentStep: 0, draft: { ...EMPTY_DRAFT }, editingId: null }),
        loadForEdit: (id, q) =>
          set({
            editingId: id,
            currentStep: 0,
            draft: {
              customer_id: q.customer_id,
              country_id: q.country_id,
              service_tier: q.service_tier,
              notes: q.notes ?? '',
            },
          }),
      }),
      {
        name: storageKey,
        storage: createJSONStorage(() => localStorage),
        // Only persist draft + currentStep + editingId — those are the
        // "user's in-progress work". Method references don't persist.
        partialize: (s) => ({
          currentStep: s.currentStep,
          draft: s.draft,
          editingId: s.editingId,
        }),
      },
    ),
  )
}

// Step validators — exported because the wizard shell uses them to
// enable/disable the "Next" button, and tests assert them directly.

export function isStepCustomerValid(d: WizardDraft): boolean {
  return d.customer_id.length > 0
}

export function isStepCountryValid(d: WizardDraft): boolean {
  return d.country_id.length > 0
}

export function isStepTierValid(d: WizardDraft): boolean {
  return d.service_tier === 'basic' || d.service_tier === 'standard' || d.service_tier === 'premium'
}

// isStepNotesValid: notes is optional, so always valid. Kept for symmetry
// in the step indicator.
export function isStepNotesValid(_d: WizardDraft): boolean {
  return true
}

// hasNonEmptyDraft is the "resume banner" trigger — any user-typed
// content means we should ask before silently reusing the state.
export function hasNonEmptyDraft(d: WizardDraft): boolean {
  return d.customer_id.length > 0 || d.country_id.length > 0 || d.notes.length > 0
}
```

- [ ] **Step 4: Run tests — verify pass**

Run: `cd apps/web && pnpm vitest run src/features/quotation/wizard/wizard-store.test.ts`
Expected: all 9 tests pass.

- [ ] **Step 5: Typecheck**

Run: `cd apps/web && pnpm tsc --noEmit`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/features/quotation/wizard/wizard-store.ts \
        apps/web/src/features/quotation/wizard/wizard-store.test.ts
git commit -m "$(cat <<'EOF'
feat(web): add wizard-store with per-user localStorage persistence

zustand + persist middleware, keyed by user id so cross-user data is
isolated. Exposes step validators used by the wizard shell.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: `useCreateAndSubmit` + `useUpdateAndSubmit` hooks

**Files:**
- Modify: `apps/web/src/features/quotation/hooks/use-quotation-mutations.ts`

- [ ] **Step 1: Add the two combined mutations**

Append to `use-quotation-mutations.ts`, after `useAdjustQuotation`:

```ts
/**
 * useCreateAndSubmit runs POST /quotations then POST /quotations/:id/submit.
 *
 * Failure semantics:
 * - If create fails: throws; caller shows toast and draft stays in localStorage
 *   so the user can retry.
 * - If create succeeds but submit fails: resolves with { id, submitted: false }.
 *   The caller should clear localStorage (draft exists on the server) and
 *   surface a toast like "草稿已创建,但提交失败,请在详情页重试".
 */
export function useCreateAndSubmit() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (
      body: CreateQuotationRequest,
    ): Promise<{ id: string; submitted: boolean }> => {
      const created = await api.post<Quotation>('/quotations', body)
      try {
        await api.post<Quotation>(`/quotations/${created.data.id}/submit`)
        return { id: created.data.id, submitted: true }
      } catch {
        return { id: created.data.id, submitted: false }
      }
    },
    onSuccess: (result) => {
      invalidate(qc, result.id)
      if (result.submitted) {
        toast.success('报价已提交待审核')
      } else {
        toast.warning('草稿已创建,但提交失败,请在详情页重试')
      }
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}

/**
 * useUpdateAndSubmit runs PATCH /quotations/:id then POST /quotations/:id/submit.
 * Same failure semantics as useCreateAndSubmit.
 */
export function useUpdateAndSubmit() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (args: {
      id: string
      body: UpdateDraftRequest
    }): Promise<{ id: string; submitted: boolean }> => {
      await api.patch<Quotation>(`/quotations/${args.id}`, args.body)
      try {
        await api.post<Quotation>(`/quotations/${args.id}/submit`)
        return { id: args.id, submitted: true }
      } catch {
        return { id: args.id, submitted: false }
      }
    },
    onSuccess: (result) => {
      invalidate(qc, result.id)
      if (result.submitted) {
        toast.success('报价已提交待审核')
      } else {
        toast.warning('草稿已更新,但提交失败,请在详情页重试')
      }
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}
```

- [ ] **Step 2: Typecheck**

Run: `cd apps/web && pnpm tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/features/quotation/hooks/use-quotation-mutations.ts
git commit -m "$(cat <<'EOF'
feat(web): add useCreateAndSubmit + useUpdateAndSubmit mutations

Two-step API calls for the wizard's 'save and submit' button, with
partial-failure semantics: if submit fails after create succeeded, the
mutation resolves with submitted=false so the caller can surface a
dedicated toast.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: `use-preview` hook

**Files:**
- Create: `apps/web/src/features/quotation/wizard/hooks/use-preview.ts`

- [ ] **Step 1: Create the hook**

```ts
import { useQuery } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { api } from '@/lib/api'
import type { ServiceTier } from '../../types'

export interface PreviewRequest {
  customer_id: string
  country_id: string
  service_tier: ServiceTier
}

export interface PreviewLine {
  fee_item: string
  amount_cny_cents: number
}

export interface PreviewResponse {
  lines: PreviewLine[]
  total_cny_cents: number
  signature: string
}

export const PREVIEW_QUERY_KEY = ['quotations', 'preview'] as const

// usePreview fetches pricing for the wizard's current triple. Returns
// an idle/empty state when any required field is missing. Cached 5
// minutes so a round-trip to an earlier step + back doesn't hammer
// the API.
export function usePreview(req: PreviewRequest) {
  const enabled = Boolean(req.customer_id && req.country_id && req.service_tier)
  return useQuery<PreviewResponse, AxiosError>({
    queryKey: [...PREVIEW_QUERY_KEY, req.customer_id, req.country_id, req.service_tier],
    queryFn: async () => {
      const { data } = await api.post<PreviewResponse>('/quotations/preview', req)
      return data
    },
    enabled,
    staleTime: 5 * 60_000,
    retry: false,
  })
}
```

- [ ] **Step 2: Typecheck**

Run: `cd apps/web && pnpm tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/features/quotation/wizard/hooks/use-preview.ts
git commit -m "$(cat <<'EOF'
feat(web): add usePreview query hook for wizard step 5

5-min staleTime so moving back a step and returning reuses the cached
response. Disabled until all three required fields are present.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Step 1 — Customer picker

**Files:**
- Create: `apps/web/src/features/quotation/wizard/steps/step-customer.tsx`

- [ ] **Step 1: Implement**

```tsx
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useCustomersList } from '@/features/customers/hooks'
import type { WizardState } from '../wizard-store'

interface Props {
  state: WizardState
}

// Step 1: select the customer. Reuses the existing customers list hook;
// for now a Select of the first 100 is fine. If in the future users
// need a searchable combobox, swap to cmdk's Command — layout is ready.
export function StepCustomer({ state }: Props) {
  const { data } = useCustomersList({ page: 1, page_size: 100 })
  return (
    <div className='flex flex-col gap-3'>
      <div className='space-y-1.5'>
        <Label htmlFor='wizard-customer'>客户 / Customer</Label>
        <Select
          value={state.draft.customer_id}
          onValueChange={(v) => state.patchDraft({ customer_id: v })}
        >
          <SelectTrigger id='wizard-customer' className='w-full'>
            <SelectValue placeholder='请选择客户' />
          </SelectTrigger>
          <SelectContent>
            {data?.items.map((c) => (
              <SelectItem key={c.id} value={c.id}>
                {c.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Typecheck**

Run: `cd apps/web && pnpm tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/features/quotation/wizard/steps/step-customer.tsx
git commit -m "feat(web): add step-customer for quotation wizard

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Step 2 — Country picker

**Files:**
- Create: `apps/web/src/features/quotation/wizard/steps/step-country.tsx`

- [ ] **Step 1: Implement**

```tsx
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useCountries } from '@/features/catalog/hooks/use-countries'
import type { WizardState } from '../wizard-store'

interface Props {
  state: WizardState
}

export function StepCountry({ state }: Props) {
  const { data } = useCountries()
  return (
    <div className='flex flex-col gap-3'>
      <div className='space-y-1.5'>
        <Label htmlFor='wizard-country'>国家 / Country</Label>
        <Select
          value={state.draft.country_id}
          onValueChange={(v) => state.patchDraft({ country_id: v })}
        >
          <SelectTrigger id='wizard-country' className='w-full'>
            <SelectValue placeholder='请选择国家' />
          </SelectTrigger>
          <SelectContent>
            {data?.map((c) => (
              <SelectItem key={c.id} value={c.id}>
                {c.name_zh}（{c.code}）
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Typecheck + Commit**

```bash
cd apps/web && pnpm tsc --noEmit
git add apps/web/src/features/quotation/wizard/steps/step-country.tsx
git commit -m "feat(web): add step-country for quotation wizard

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: Step 3 — Service tier

**Files:**
- Create: `apps/web/src/features/quotation/wizard/steps/step-tier.tsx`

- [ ] **Step 1: Implement**

```tsx
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import type { ServiceTier } from '../../types'
import type { WizardState } from '../wizard-store'

interface Props {
  state: WizardState
}

const OPTIONS: { value: ServiceTier; zh: string; desc: string }[] = [
  { value: 'basic', zh: '基础', desc: '标准申请流程' },
  { value: 'standard', zh: '标准', desc: '含审查反馈跟进' },
  { value: 'premium', zh: '尊享', desc: '全流程专员支持' },
]

export function StepTier({ state }: Props) {
  return (
    <div className='flex flex-col gap-3'>
      <Label>服务级别 / Service Tier</Label>
      <RadioGroup
        value={state.draft.service_tier}
        onValueChange={(v) => state.patchDraft({ service_tier: v as ServiceTier })}
        className='flex flex-col gap-3'
      >
        {OPTIONS.map((o) => (
          <label
            key={o.value}
            className='flex items-start gap-3 rounded-md border p-3 cursor-pointer hover:bg-muted/40'
          >
            <RadioGroupItem value={o.value} id={`tier-${o.value}`} />
            <div className='flex flex-col'>
              <span className='font-medium'>{o.zh} ({o.value})</span>
              <span className='text-xs text-muted-foreground'>{o.desc}</span>
            </div>
          </label>
        ))}
      </RadioGroup>
    </div>
  )
}
```

- [ ] **Step 2: Typecheck + Commit**

```bash
cd apps/web && pnpm tsc --noEmit
git add apps/web/src/features/quotation/wizard/steps/step-tier.tsx
git commit -m "feat(web): add step-tier for quotation wizard

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: Step 4 — Notes

**Files:**
- Create: `apps/web/src/features/quotation/wizard/steps/step-notes.tsx`

- [ ] **Step 1: Implement**

```tsx
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import type { WizardState } from '../wizard-store'

interface Props {
  state: WizardState
}

export function StepNotes({ state }: Props) {
  return (
    <div className='flex flex-col gap-3'>
      <div className='space-y-1.5'>
        <Label htmlFor='wizard-notes'>备注 / Notes（可选）</Label>
        <Textarea
          id='wizard-notes'
          rows={6}
          placeholder='商标描述、客户特别要求、提交给 reviewer 的补充说明…'
          value={state.draft.notes}
          onChange={(e) => state.patchDraft({ notes: e.target.value })}
        />
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Typecheck + Commit**

```bash
cd apps/web && pnpm tsc --noEmit
git add apps/web/src/features/quotation/wizard/steps/step-notes.tsx
git commit -m "feat(web): add step-notes for quotation wizard

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: Step 5 — Preview

**Files:**
- Create: `apps/web/src/features/quotation/wizard/steps/step-preview.tsx`

- [ ] **Step 1: Implement**

```tsx
import { useNavigate } from '@tanstack/react-router'
import { AxiosError } from 'axios'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { AlertCircle, Loader2 } from 'lucide-react'
import {
  useCreateQuotation,
  useCreateAndSubmit,
  useUpdateQuotationDraft,
  useUpdateAndSubmit,
} from '../../hooks/use-quotation-mutations'
import { usePreview } from '../hooks/use-preview'
import type { WizardState } from '../wizard-store'

interface Props {
  state: WizardState
  onExit: () => void // called after a successful submit to clear the store
}

// Step 5: preview and submit. Two paths:
//   - 保存草稿 → POST /quotations (or PATCH in edit mode)
//   - 保存并提交 → POST + POST /submit (or PATCH + POST /submit)
// After any successful action, onExit() resets the store and the caller
// navigates to the detail page.
export function StepPreview({ state, onExit }: Props) {
  const navigate = useNavigate()
  const preview = usePreview({
    customer_id: state.draft.customer_id,
    country_id: state.draft.country_id,
    service_tier: state.draft.service_tier,
  })
  const createMut = useCreateQuotation()
  const createSubmitMut = useCreateAndSubmit()
  const updateMut = useUpdateQuotationDraft()
  const updateSubmitMut = useUpdateAndSubmit()

  const isEdit = state.editingId !== null
  const busy =
    createMut.isPending ||
    createSubmitMut.isPending ||
    updateMut.isPending ||
    updateSubmitMut.isPending
  const canSubmit = preview.isSuccess && !busy

  const body = {
    customer_id: state.draft.customer_id,
    country_id: state.draft.country_id,
    service_tier: state.draft.service_tier,
    notes: state.draft.notes ? state.draft.notes : null,
  }

  const saveDraft = async () => {
    if (isEdit && state.editingId) {
      await updateMut.mutateAsync({ id: state.editingId, body })
      onExit()
      navigate({ to: '/quotations/$id', params: { id: state.editingId } })
    } else {
      const q = await createMut.mutateAsync(body)
      onExit()
      navigate({ to: '/quotations/$id', params: { id: q.id } })
    }
  }

  const saveAndSubmit = async () => {
    if (isEdit && state.editingId) {
      const result = await updateSubmitMut.mutateAsync({ id: state.editingId, body })
      onExit()
      navigate({ to: '/quotations/$id', params: { id: result.id } })
    } else {
      const result = await createSubmitMut.mutateAsync(body)
      onExit()
      navigate({ to: '/quotations/$id', params: { id: result.id } })
    }
  }

  // Error path: show a retry button. The two save buttons stay
  // disabled because saving without a valid signature+total is the
  // same thing as submitting a broken quotation.
  if (preview.isError) {
    const code = (preview.error as AxiosError<{ code?: string }>)?.response?.data?.code
    const message =
      code === 'ERR_MISSING_PRICING'
        ? '该国家/级别暂无定价,请联系管理员或回到上一步选择其他国家'
        : code === 'ERR_NOT_FOUND'
          ? '客户不存在,请回到第 1 步重新选择'
          : '预览失败,请稍后重试'
    return (
      <div className='flex flex-col gap-3'>
        <Alert variant='destructive'>
          <AlertCircle className='h-4 w-4' />
          <AlertTitle>预览失败 / Preview failed</AlertTitle>
          <AlertDescription>{message}</AlertDescription>
        </Alert>
        <Button variant='outline' onClick={() => preview.refetch()}>
          重试 / Retry
        </Button>
        <div className='flex justify-end gap-2'>
          <Button disabled variant='outline'>
            {isEdit ? '保存修改' : '保存草稿'}
          </Button>
          <Button disabled>
            保存并提交
          </Button>
        </div>
      </div>
    )
  }

  if (preview.isLoading || !preview.data) {
    return (
      <div className='flex items-center gap-2 text-sm text-muted-foreground'>
        <Loader2 className='h-4 w-4 animate-spin' /> 计算中 / Computing…
      </div>
    )
  }

  const { lines, total_cny_cents, signature } = preview.data
  return (
    <div className='flex flex-col gap-4'>
      <div className='rounded-md border p-4'>
        <div className='mb-2 text-sm font-medium'>明细 / Line items</div>
        <div className='flex flex-col gap-1'>
          {lines.map((l) => (
            <div key={l.fee_item} className='flex items-center justify-between text-sm'>
              <span>{l.fee_item}</span>
              <span className='font-mono'>¥{(l.amount_cny_cents / 100).toFixed(2)}</span>
            </div>
          ))}
          <Separator className='my-2' />
          <div className='flex items-center justify-between font-medium'>
            <span>合计 / Total</span>
            <span className='font-mono'>¥{(total_cny_cents / 100).toFixed(2)}</span>
          </div>
        </div>
        <div className='mt-3 text-xs text-muted-foreground font-mono'>
          签名 / Signature: {signature.slice(0, 12)}…
        </div>
      </div>
      <div className='flex justify-end gap-2'>
        <Button variant='outline' disabled={!canSubmit} onClick={saveDraft}>
          {isEdit ? '保存修改' : '保存草稿'}
        </Button>
        <Button disabled={!canSubmit} onClick={saveAndSubmit}>
          保存并提交
        </Button>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Typecheck**

Run: `cd apps/web && pnpm tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/features/quotation/wizard/steps/step-preview.tsx
git commit -m "$(cat <<'EOF'
feat(web): add step-preview with save draft + save & submit actions

Uses usePreview to fetch pricing; renders lines+total via a simple
table. Error state maps ERR_MISSING_PRICING / ERR_NOT_FOUND to Chinese
guidance and exposes a retry button.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 12: Resume banner

**Files:**
- Create: `apps/web/src/features/quotation/wizard/resume-banner.tsx`

- [ ] **Step 1: Implement**

```tsx
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { RotateCcw, X } from 'lucide-react'

interface Props {
  onContinue: () => void
  onDiscard: () => void
}

// ResumeBanner appears at the top of /quotations/new when localStorage
// already has a non-empty draft. The two buttons are 继续 (keep the draft)
// and 放弃 (reset the store). The banner itself doesn't own state —
// the parent component controls visibility by not rendering it after
// the user picks either option.
export function ResumeBanner({ onContinue, onDiscard }: Props) {
  return (
    <Alert className='mb-4'>
      <RotateCcw className='h-4 w-4' />
      <AlertTitle>检测到未完成的草稿 / Unfinished draft detected</AlertTitle>
      <AlertDescription className='flex items-center justify-between gap-3'>
        <span>要继续上次的草稿,还是重新开始?</span>
        <div className='flex gap-2'>
          <Button size='sm' variant='outline' onClick={onDiscard}>
            <X className='mr-1 h-4 w-4' /> 放弃
          </Button>
          <Button size='sm' onClick={onContinue}>
            继续
          </Button>
        </div>
      </AlertDescription>
    </Alert>
  )
}
```

- [ ] **Step 2: Typecheck + Commit**

```bash
cd apps/web && pnpm tsc --noEmit
git add apps/web/src/features/quotation/wizard/resume-banner.tsx
git commit -m "feat(web): add resume-banner component

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 13: Wizard shell — orchestrates steps, step indicator, nav

**Files:**
- Create: `apps/web/src/features/quotation/wizard/quotation-wizard.tsx`

- [ ] **Step 1: Implement**

```tsx
import { useMemo } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { cn } from '@/lib/utils'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import {
  createWizardStore,
  isStepCustomerValid,
  isStepCountryValid,
  isStepTierValid,
  type WizardState,
} from './wizard-store'
import { StepCustomer } from './steps/step-customer'
import { StepCountry } from './steps/step-country'
import { StepTier } from './steps/step-tier'
import { StepNotes } from './steps/step-notes'
import { StepPreview } from './steps/step-preview'

// The wizard store is user-scoped, so we construct it lazily per-user
// and memoize by user id. A pool is cheaper than recreating across
// re-renders, and matches the "one user = one persisted slot" contract.
const stores: Record<string, ReturnType<typeof createWizardStore>> = {}
function useWizardStoreForUser(userId: string) {
  if (!stores[userId]) {
    stores[userId] = createWizardStore(userId)
  }
  return stores[userId]
}

interface Props {
  mode: 'create' | 'edit'
  // When mode==='edit', the outer route component is responsible for
  // calling loadForEdit() BEFORE rendering this component. In 'create'
  // mode the outer component is responsible for showing/hiding the
  // ResumeBanner and calling reset() if the user picks 放弃.
}

const STEPS: {
  key: string
  label: string
  isValid: (d: WizardState['draft']) => boolean
}[] = [
  { key: 'customer', label: '客户', isValid: isStepCustomerValid },
  { key: 'country', label: '国家', isValid: isStepCountryValid },
  { key: 'tier', label: '级别', isValid: isStepTierValid },
  { key: 'notes', label: '备注', isValid: () => true },
  { key: 'preview', label: '预览', isValid: () => true },
]

export function QuotationWizard({ mode }: Props) {
  const userId = useAuthStore((s) => s.auth.user?.id) ?? ''
  const useStore = useWizardStoreForUser(userId)
  const state = useStore()

  const canProceed = useMemo(() => {
    // To move FROM step N to N+1 the step N's validator must pass.
    return STEPS[state.currentStep]?.isValid(state.draft) ?? true
  }, [state.currentStep, state.draft])

  const goNext = () => {
    if (canProceed && state.currentStep < 4) {
      state.setStep((state.currentStep + 1) as WizardState['currentStep'])
    }
  }
  const goBack = () => {
    if (state.currentStep > 0) {
      state.setStep((state.currentStep - 1) as WizardState['currentStep'])
    }
  }

  const stepContent = (() => {
    switch (state.currentStep) {
      case 0:
        return <StepCustomer state={state} />
      case 1:
        return <StepCountry state={state} />
      case 2:
        return <StepTier state={state} />
      case 3:
        return <StepNotes state={state} />
      case 4:
        return <StepPreview state={state} onExit={() => state.reset()} />
    }
  })()

  return (
    <div className='flex flex-col gap-4'>
      <StepIndicator current={state.currentStep} />
      <Card>
        <CardHeader>
          <CardTitle>
            {mode === 'edit' ? '编辑报价' : '新建报价'} — 第 {state.currentStep + 1} 步:{' '}
            {STEPS[state.currentStep].label}
          </CardTitle>
          <CardDescription>
            {state.currentStep < 4
              ? '填写以下字段后点"下一步"。数据会自动保存在本地。'
              : '确认信息无误后可保存草稿或直接提交审核。'}
          </CardDescription>
        </CardHeader>
        <CardContent>{stepContent}</CardContent>
      </Card>
      {state.currentStep < 4 && (
        <div className='flex justify-between'>
          <Button
            variant='ghost'
            disabled={state.currentStep === 0}
            onClick={goBack}
          >
            <ChevronLeft className='mr-1 h-4 w-4' /> 上一步
          </Button>
          <Button disabled={!canProceed} onClick={goNext}>
            下一步 <ChevronRight className='ml-1 h-4 w-4' />
          </Button>
        </div>
      )}
      {state.currentStep === 4 && (
        <div className='flex justify-start'>
          <Button variant='ghost' onClick={goBack}>
            <ChevronLeft className='mr-1 h-4 w-4' /> 上一步
          </Button>
        </div>
      )}
    </div>
  )
}

// StepIndicator renders a dotted pill-strip "0/1/2/3/4" with the
// current step highlighted. No shadcn Stepper primitive exists; this
// is the minimal visual affordance — a dot per step.
function StepIndicator({ current }: { current: number }) {
  return (
    <div className='flex items-center gap-2'>
      {STEPS.map((s, i) => (
        <div key={s.key} className='flex items-center gap-2'>
          <div
            className={cn(
              'flex h-8 w-8 items-center justify-center rounded-full border text-xs font-medium',
              i < current && 'bg-primary text-primary-foreground border-primary',
              i === current && 'border-primary text-primary',
              i > current && 'text-muted-foreground',
            )}
            aria-current={i === current ? 'step' : undefined}
          >
            {i + 1}
          </div>
          <span
            className={cn(
              'text-sm',
              i === current ? 'font-medium' : 'text-muted-foreground',
            )}
          >
            {s.label}
          </span>
          {i < STEPS.length - 1 && <div className='mx-1 h-px w-6 bg-border' />}
        </div>
      ))}
    </div>
  )
}

// Re-export a convenience hook so routes don't need to import createWizardStore
// directly — they use getStore(userId).reset() for mode transitions.
export function getWizardStore(userId: string) {
  return useWizardStoreForUser(userId)
}

// __resetWizardStorePool is for tests ONLY. Vitest runs multiple
// scenarios in one process; localStorage.clear() between tests does
// NOT clear the in-memory stores cached in `stores`. Call this in
// beforeEach to guarantee a clean slate.
export function __resetWizardStorePool() {
  for (const k of Object.keys(stores)) {
    stores[k].getState().reset()
    delete stores[k]
  }
}

// hasNonEmptyDraft / reset etc. are re-exported from the store module
// for routes that do resume-banner handling.
export { hasNonEmptyDraft } from './wizard-store'
```

- [ ] **Step 2: Typecheck + Commit**

```bash
cd apps/web && pnpm tsc --noEmit
git add apps/web/src/features/quotation/wizard/quotation-wizard.tsx
git commit -m "$(cat <<'EOF'
feat(web): add QuotationWizard shell with step indicator + nav

Per-user store pool keyed by user id; simple dotted step indicator
(no shadcn Stepper primitive exists). Exposes getWizardStore so routes
can drive reset/loadForEdit from their lifecycle.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 14: `/quotations/new` route

**Files:**
- Create: `apps/web/src/routes/_authenticated/quotations/new.tsx`

- [ ] **Step 1: Implement**

```tsx
import { useState } from 'react'
import { createFileRoute, Link } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { useAuthStore } from '@/stores/auth-store'
import { getWizardStore, QuotationWizard, hasNonEmptyDraft } from '@/features/quotation/wizard/quotation-wizard'
import { ResumeBanner } from '@/features/quotation/wizard/resume-banner'

export function NewQuotationPage() {
  const userId = useAuthStore((s) => s.auth.user?.id) ?? ''
  const store = getWizardStore(userId)
  const draft = store((s) => s.draft)
  const editingId = store((s) => s.editingId)
  const reset = store.getState().reset

  // Banner shows only when:
  //   - mount: draft is non-empty AND not an edit-session leftover
  //   - user hasn't explicitly dismissed it this render
  const initiallyShow = hasNonEmptyDraft(draft) || editingId !== null
  const [showBanner, setShowBanner] = useState(initiallyShow)

  // If we're in new mode but the store still has a stale editingId from
  // a prior /edit visit, wipe it so we don't accidentally PATCH a
  // stranger's quotation. We do this eagerly on mount.
  if (editingId !== null) {
    reset()
  }

  return (
    <>
      <Header fixed>
        <Button asChild variant='ghost' size='sm' className='me-auto'>
          <Link to='/quotations'>← 返回列表</Link>
        </Button>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        <h2 className='text-2xl font-bold'>新建报价</h2>
        {showBanner && (
          <ResumeBanner
            onContinue={() => setShowBanner(false)}
            onDiscard={() => {
              reset()
              setShowBanner(false)
            }}
          />
        )}
        <QuotationWizard mode='create' />
      </Main>
    </>
  )
}

export const Route = createFileRoute('/_authenticated/quotations/new')({
  component: NewQuotationPage,
})
```

- [ ] **Step 2: Regenerate route tree (TanStack Router codegen)**

If the project uses `@tanstack/router-vite-plugin` with codegen, vite will auto-generate `routeTree.gen.ts` on the next dev start. Verify by running:

Run: `cd apps/web && pnpm vite build`
Expected: build completes; `src/routeTree.gen.ts` now lists `/_authenticated/quotations/new`.

If build errors mention missing routes, run `pnpm tsr generate` (the plugin name depends on the setup — check `vite.config.ts`).

- [ ] **Step 3: Typecheck + Commit**

```bash
cd apps/web && pnpm tsc --noEmit
git add apps/web/src/routes/_authenticated/quotations/new.tsx \
        apps/web/src/routeTree.gen.ts
git commit -m "$(cat <<'EOF'
feat(web): add /quotations/new route for wizard create flow

Wires resume-banner (fires when localStorage has a non-empty draft) +
QuotationWizard. Eagerly resets the store if a stale editingId is
present on mount.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 15: `/quotations/$id/edit` route

**Files:**
- Create: `apps/web/src/routes/_authenticated/quotations/$id.edit.tsx`

- [ ] **Step 1: Implement**

```tsx
import { useEffect } from 'react'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { useQuotation, quotationDetailQueryOptions } from '@/features/quotation/hooks'
import { getWizardStore, QuotationWizard } from '@/features/quotation/wizard/quotation-wizard'

export function EditQuotationPage() {
  const { id } = Route.useParams()
  const navigate = useNavigate()
  const userId = useAuthStore((s) => s.auth.user?.id) ?? ''
  const store = getWizardStore(userId)

  const { data: quotation } = useQuotation(id)

  // Load server draft into store once, on first render where quotation
  // is available. The store's loadForEdit overwrites any localStorage
  // residue (new-mode draft or stale edit-mode state from another id).
  useEffect(() => {
    if (quotation) {
      store.getState().loadForEdit(id, quotation)
    }
  }, [quotation, id, store])

  // If status leaves 'draft' while the user has this page open (a
  // reviewer approved/rejected/adjusted in another tab), bounce to
  // the detail page — editing a non-draft would fail anyway.
  useEffect(() => {
    if (quotation && quotation.status !== 'draft') {
      toast.info('报价状态已变更,无法编辑')
      navigate({ to: '/quotations/$id', params: { id } })
    }
  }, [quotation, id, navigate])

  // On unmount, clear the store so going back to /quotations/new
  // doesn't see a "zombie" edit state.
  useEffect(() => {
    return () => {
      store.getState().reset()
    }
  }, [store])

  return (
    <>
      <Header fixed>
        <Button asChild variant='ghost' size='sm' className='me-auto'>
          <Link to='/quotations/$id' params={{ id }}>
            ← 返回详情
          </Link>
        </Button>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        <h2 className='text-2xl font-bold'>编辑报价 — {quotation?.serial_no ?? id.slice(0, 8)}</h2>
        {quotation && quotation.status === 'draft' && <QuotationWizard mode='edit' />}
      </Main>
    </>
  )
}

export const Route = createFileRoute('/_authenticated/quotations/$id/edit')({
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(quotationDetailQueryOptions(params.id)),
  component: EditQuotationPage,
})
```

- [ ] **Step 2: Build + typecheck**

Run: `cd apps/web && pnpm vite build && pnpm tsc --noEmit`
Expected: success; `routeTree.gen.ts` now includes `/_authenticated/quotations/$id/edit`.

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/routes/_authenticated/quotations/$id.edit.tsx \
        apps/web/src/routeTree.gen.ts
git commit -m "$(cat <<'EOF'
feat(web): add /quotations/$id/edit route for wizard edit flow

loadForEdit from server draft on mount; bounces if status !== draft;
unmount resets store to avoid zombie state leaking into /new.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 16: Swap entry points + delete legacy Sheet + integration tests

**Files:**
- Modify: `apps/web/src/features/quotation/index.tsx`
- Modify: `apps/web/src/features/quotation/detail.tsx`
- Modify: `apps/web/src/features/quotation/components/quotation-action-bar.tsx` (if "编辑草稿" button lives here)
- Delete: `apps/web/src/features/quotation/components/quotation-form-sheet.tsx`
- Modify: `apps/web/src/features/quotation/quotation.integration.test.tsx`
- Modify: `apps/web/src/test-utils/msw/handlers.ts`

- [ ] **Step 1: Inspect where "编辑草稿" button actually lives**

Run: `cd apps/web && grep -rn "编辑草稿\|QuotationFormSheet" src/features/quotation`
Expected output: shows the actual files importing/using the Sheet.

Based on what you find, apply the Link swap to those exact components.

- [ ] **Step 2: Swap list page "新建报价" to a Link**

In `apps/web/src/features/quotation/index.tsx`, replace the existing `<Button onClick={() => setCreateOpen(true)}>新建报价</Button>` with:

```tsx
<Button asChild>
  <Link to='/quotations/new'>新建报价</Link>
</Button>
```

Add `import { Link } from '@tanstack/react-router'` at the top. Remove `useState`-based `createOpen` state and the `<QuotationFormSheet ... />` render at the bottom. Remove the `import { QuotationFormSheet } ...` line.

- [ ] **Step 3: Swap detail page's "编辑草稿" to a Link**

In whichever file holds the "编辑草稿" button (check Step 1's grep), replace the `<Button onClick={() => setEditOpen(true)}>` with a Link-wrapped Button. Example (adjust to file):

```tsx
<Button asChild size='sm'>
  <Link to='/quotations/$id/edit' params={{ id: quotation.id }}>
    编辑草稿
  </Link>
</Button>
```

Also remove the `editOpen` state and the `<QuotationFormSheet ... />` render in `detail.tsx` if present.

- [ ] **Step 4: Delete the legacy Sheet**

```bash
git rm apps/web/src/features/quotation/components/quotation-form-sheet.tsx
```

- [ ] **Step 5: Verify nothing still imports the Sheet**

Run: `cd apps/web && grep -rn "QuotationFormSheet\|quotation-form-sheet" src/`
Expected: no results.

- [ ] **Step 6: Add MSW handler for POST /quotations/preview**

In `apps/web/src/test-utils/msw/handlers.ts`, add a new handler inside `defaultHandlers` array (near other `/quotations` handlers). It reads `customer_id`, `country_id`, `service_tier` from the body and returns a pricing snapshot based on the seeded `pricingEntries` store:

```ts
http.post('/api/v1/quotations/preview', async ({ request }) => {
  const body = (await request.json()) as {
    customer_id: string
    country_id: string
    service_tier: 'basic' | 'standard' | 'premium'
  }
  if (!customers.find((c) => c.id === body.customer_id)) {
    return HttpResponse.json({ code: 'ERR_NOT_FOUND', message: 'customer not found' }, { status: 404 })
  }
  const matched = pricingEntries.filter(
    (e) =>
      e.country_id === body.country_id &&
      e.service_tier === body.service_tier &&
      e.effective_to === null,
  )
  if (matched.length === 0) {
    return HttpResponse.json(
      { code: 'ERR_MISSING_PRICING', message: 'no pricing entries' },
      { status: 422 },
    )
  }
  const lines = matched
    .map((e) => ({ fee_item: e.fee_item, amount_cny_cents: e.amount_cny_cents }))
    .sort((a, b) => a.fee_item.localeCompare(b.fee_item))
  const total = lines.reduce((s, l) => s + l.amount_cny_cents, 0)
  // Deterministic mock signature — doesn't need to match backend hash.
  const signature = `mock-${body.country_id}-${body.service_tier}-${total}`.padEnd(64, '0').slice(0, 64)
  return HttpResponse.json({ lines, total_cny_cents: total, signature })
}),
```

- [ ] **Step 7: Add 5 integration tests**

Append to `quotation.integration.test.tsx`. First, import the two wizard pages and the store-pool reset helper at the top of the test file:

```tsx
import { NewQuotationPage } from '@/routes/_authenticated/quotations/new'
import { EditQuotationPage } from '@/routes/_authenticated/quotations/$id.edit'
import { __resetWizardStorePool } from '@/features/quotation/wizard/quotation-wizard'
```

Extend `buildRouter` to register the two wizard routes. Find the existing `routeTree: rootRoute.addChildren([listRoute, detailRoute])` call and update it:

```tsx
const newRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/quotations/new',
  component: NewQuotationPage,
})
const editRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/quotations/$id/edit',
  component: EditQuotationPage,
})
const router = createRouter({
  routeTree: rootRoute.addChildren([listRoute, detailRoute, newRoute, editRoute]),
  history: createMemoryHistory({ initialEntries: [initialPath] }),
  context: { queryClient },
})
```

Add this new `describe` block at the end of the file (don't replace existing tests):

```tsx
describe('quotation wizard M3', () => {
  beforeAll(async () => {
    await worker.start({ onUnhandledRequest: 'bypass' })
  })
  beforeEach(() => {
    resetMswState()
    __resetAuthInterceptorState()
    useAuthStore.getState().auth.reset()
    localStorage.clear()
    // Critical: clear the in-memory wizard store pool — localStorage.clear()
    // alone leaves cached zustand instances populated with last test's state.
    __resetWizardStorePool()
  })
  afterAll(() => {
    worker.stop()
  })

  async function seedWizardPrereqs() {
    const custId = seedCustomer({ name: 'Acme 国际' })
    seedPricingEntry({
      country_id: COUNTRY_CN_ID,
      service_tier: 'basic',
      fee_item: 'application',
      amount_cny_cents: 50000,
    })
    return { custId }
  }

  it('new → 5 steps → save draft → list shows new row', async () => {
    await seedWizardPrereqs()
    const { router, queryClient } = buildRouter('salesperson', '/quotations/new')
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    )

    // Step 1 — customer.
    await userEvent.click(screen.getByRole('combobox', { name: /客户/ }))
    await userEvent.click(screen.getByRole('option', { name: /Acme/ }))
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))

    // Step 2 — country.
    await userEvent.click(screen.getByRole('combobox', { name: /国家/ }))
    await userEvent.click(screen.getByRole('option').first())
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))

    // Step 3 — tier (basic is default).
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))

    // Step 4 — notes (skip).
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))

    // Step 5 — preview loads, click "保存草稿".
    await expect.element(screen.getByText(/application/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '保存草稿' }))

    // After navigation the detail page shows the new draft.
    await expect.element(screen.getByText('草稿').first()).toBeInTheDocument()
  })

  it('new → 5 steps → save and submit → status becomes submitted', async () => {
    await seedWizardPrereqs()
    const { router, queryClient } = buildRouter('salesperson', '/quotations/new')
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    )

    // Same 4 clicks through steps 1-4, simplified:
    await userEvent.click(screen.getByRole('combobox', { name: /客户/ }))
    await userEvent.click(screen.getByRole('option', { name: /Acme/ }))
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))
    await userEvent.click(screen.getByRole('combobox', { name: /国家/ }))
    await userEvent.click(screen.getByRole('option').first())
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))

    await userEvent.click(screen.getByRole('button', { name: '保存并提交' }))
    await expect.element(screen.getByText('已提交').first()).toBeInTheDocument()
  })

  it('edit → change tier → save and submit → status=submitted', async () => {
    const { custId } = await seedWizardPrereqs()
    const draftId = seedQuotationDraft({
      customer_id: custId,
      country_id: COUNTRY_CN_ID,
      service_tier: 'basic',
    })

    const { router, queryClient } = buildRouter(
      'salesperson',
      `/quotations/${draftId}/edit`,
    )
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    )

    // Advance to tier step (customer + country already filled).
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))
    // Pick "premium".
    await userEvent.click(screen.getByRole('radio', { name: /尊享/ }))
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))
    // Preview → save and submit.
    await userEvent.click(screen.getByRole('button', { name: '保存并提交' }))
    await expect.element(screen.getByText('已提交').first()).toBeInTheDocument()
  })

  it('resume banner: pre-seeded localStorage → banner shows → discard clears form', async () => {
    await seedWizardPrereqs()
    // Pre-seed localStorage under ADMIN_ID (buildRouter's hard-coded user id).
    localStorage.setItem(
      `quotation-wizard-draft:${ADMIN_ID}`,
      JSON.stringify({
        state: {
          currentStep: 2,
          editingId: null,
          draft: {
            customer_id: 'stale-customer',
            country_id: COUNTRY_CN_ID,
            service_tier: 'standard',
            notes: 'stale notes',
          },
        },
        version: 0,
      }),
    )

    const { router, queryClient } = buildRouter('admin', '/quotations/new')
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    )

    await expect.element(screen.getByText(/未完成的草稿/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /放弃/ }))
    // After 放弃, banner disappears and wizard is on step 0 with empty draft.
    await expect.element(screen.getByText(/未完成的草稿/)).not.toBeInTheDocument()
  })

  it('preview error: ERR_MISSING_PRICING → retry button + both saves disabled', async () => {
    // Seed a customer but NO pricing entries.
    seedCustomer({ name: 'Acme 国际' })

    const { router, queryClient } = buildRouter('salesperson', '/quotations/new')
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    )

    await userEvent.click(screen.getByRole('combobox', { name: /客户/ }))
    await userEvent.click(screen.getByRole('option', { name: /Acme/ }))
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))
    await userEvent.click(screen.getByRole('combobox', { name: /国家/ }))
    await userEvent.click(screen.getByRole('option').first())
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))
    await userEvent.click(screen.getByRole('button', { name: /下一步/ }))

    await expect.element(screen.getByText(/该国家\/级别暂无定价/)).toBeInTheDocument()
    await expect.element(screen.getByRole('button', { name: '保存草稿' })).toBeDisabled()
    await expect.element(screen.getByRole('button', { name: '保存并提交' })).toBeDisabled()
    await expect.element(screen.getByRole('button', { name: /重试/ })).toBeInTheDocument()
  })
})
```

Note: `buildRouter` hard-codes `user.id: ADMIN_ID` for every role; that's why the banner test keys on ADMIN_ID, and the other tests implicitly use the same id. If `getByRole('option').first()` is ambiguous when multiple countries are seeded, narrow it with `getByRole('option', { name: /China|中国/ })` based on the actual seeded country name.

- [ ] **Step 8: Run integration tests**

Run: `cd apps/web && pnpm vitest run src/features/quotation/quotation.integration.test.tsx`
Expected: all tests pass (existing 2 + 5 new = 7 total).

If the test with `getByRole('option').first()` fails because multiple options exist, switch to `getByRole('option', { name: /China|中国/ })` or whatever country name is rendered.

- [ ] **Step 9: Run full frontend test + typecheck + build**

Run: `cd apps/web && pnpm tsc --noEmit && pnpm vitest run --browser.headless && pnpm vite build`
Expected: all green, build output produced.

- [ ] **Step 10: Commit**

```bash
git add apps/web/src/features/quotation/index.tsx \
        apps/web/src/features/quotation/detail.tsx \
        apps/web/src/features/quotation/components/quotation-action-bar.tsx \
        apps/web/src/features/quotation/quotation.integration.test.tsx \
        apps/web/src/test-utils/msw/handlers.ts \
        apps/web/src/routes/_authenticated/quotations/new.tsx \
        apps/web/src/routes/_authenticated/quotations/$id.edit.tsx
git rm apps/web/src/features/quotation/components/quotation-form-sheet.tsx
git commit -m "$(cat <<'EOF'
feat(web): swap Sheet entry points to wizard routes + integration tests

List page 新建 and detail page 编辑草稿 now navigate to the new wizard
routes. Delete legacy QuotationFormSheet. Adds MSW preview handler
and 5 wizard integration scenarios.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Post-completion smoke test

After all 16 tasks commit cleanly, exercise the full stack locally:

```bash
docker compose up -d postgres gotenberg
cd apps/api && go run ./cmd/server &
cd apps/web && pnpm vite
```

1. Log in as salesperson.
2. Navigate to `/quotations/new`. Walk the 5 steps. Click **保存并提交**.
3. Expect redirect to `/quotations/$id` with status "已提交".
4. Log in as reviewer in another browser; approve the quotation.
5. Back as salesperson, export PDF from the detail page.
6. As salesperson, create a new draft via `/quotations/new` and this time click **保存草稿**; then visit `/quotations/$id/edit`, change tier, click **保存并提交**.
7. Close the browser before finishing a wizard; reopen `/quotations/new` and confirm the resume banner appears. Click 放弃 and start fresh.

---

## Self-review checklist

- [x] Backend `Preview` covers all four error cases the spec lists (`ERR_INVALID_BODY` via gin binding, `ERR_INVALID_TIER`, `ERR_NOT_FOUND`, `ERR_MISSING_PRICING`)
- [x] Frontend store tests cover: empty init, patchDraft merge, reset, loadForEdit, null-notes coercion, cross-user key isolation, step validators
- [x] Integration tests cover: create+save-draft, create+save-submit, edit+save-submit, resume banner, preview error
- [x] Every task ends with a git commit; no task leaves working-tree changes uncommitted
- [x] Route files export the component so integration tests can import it without hitting TanStack Router's runtime
- [x] `new.tsx` eagerly wipes `editingId` on mount so stale edit state never leaks
- [x] `$id.edit.tsx` resets the store on unmount so leaving mid-edit doesn't leak into `/new`
- [x] `usePreview` is disabled when any required field is missing — no wasted API calls on steps 1–4
- [x] Partial-failure semantics on `useCreateAndSubmit` / `useUpdateAndSubmit` tell the user "draft saved, submit failed" instead of a silent confusing state
- [x] Type consistency: `WizardState['currentStep']` is `0 | 1 | 2 | 3 | 4` throughout; `WizardDraft.service_tier: ServiceTier`; no drift between store and steps
