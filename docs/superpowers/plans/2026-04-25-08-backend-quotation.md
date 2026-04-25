# Backend Quotation + Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `quotations` backend with a strict status state machine (`draft → submitted → approved | rejected`), integrating `pricing.Calculate` for signature-snapshotting at submit time and wiring full audit trail coverage.

**Architecture:** New `internal/quotation` package mirroring the `customer`/`pricing` layout (model/dto/repo/service/handler/router). Quotations carry a frozen snapshot — a JSONB column `snapshot_json` holding `{lines, total_cny_cents}` plus a separate `signature` column — captured at the moment of submit so later pricing edits cannot mutate the deal. Transitions are guarded in `service.go` with per-role checks: salesperson owns draft/submit, reviewer owns approve/reject, cancel is allowed on draft by owner. All transitions write to `audit_logs` via the existing middleware plus a dedicated domain-event log line.

**Tech Stack:** Go 1.25, Gin v1.10, GORM v1.25.12, pgx v5, PostgreSQL 16, uuid/v6, testcontainers-go, golang-migrate/v4.

---

## File Structure

- Create: `apps/api/migrations/000004_quotations.up.sql` — `quotations` table + `quotation_status_history` audit table + indexes
- Create: `apps/api/migrations/000004_quotations.down.sql`
- Create: `apps/api/internal/quotation/model.go` — GORM models (Quotation, StatusHistory)
- Create: `apps/api/internal/quotation/dto.go` — request/response DTOs
- Create: `apps/api/internal/quotation/repository.go` — CRUD + list + transactional status transitions
- Create: `apps/api/internal/quotation/service.go` — state machine + role checks + calls `pricing.Calculate`
- Create: `apps/api/internal/quotation/handler.go` — Gin handlers
- Create: `apps/api/internal/quotation/router.go` — route registration
- Create: `apps/api/internal/quotation/service_test.go` — state machine unit tests (table-driven, no DB)
- Create: `apps/api/internal/quotation/repository_test.go` — testcontainers integration tests
- Modify: `apps/api/cmd/server/main.go` — wire quotation package into router groups

---

### Task 1: Migration for quotations + status history

**Files:**
- Create: `apps/api/migrations/000004_quotations.up.sql`
- Create: `apps/api/migrations/000004_quotations.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- apps/api/migrations/000004_quotations.up.sql

CREATE TABLE IF NOT EXISTS quotations (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id       UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
  country_id        UUID NOT NULL REFERENCES countries(id) ON DELETE RESTRICT,
  service_tier      TEXT NOT NULL,
  status            TEXT NOT NULL DEFAULT 'draft',
  -- Snapshot captured at submit time. NULL while draft.
  snapshot_json     JSONB,
  total_cny_cents   BIGINT,
  signature         TEXT,
  submitted_at      TIMESTAMPTZ,
  reviewed_at       TIMESTAMPTZ,
  reviewed_by       UUID REFERENCES users(id) ON DELETE SET NULL,
  review_comment    TEXT,
  notes             TEXT,
  created_by        UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at        TIMESTAMPTZ,

  CONSTRAINT chk_quotations_tier
    CHECK (service_tier IN ('basic','standard','premium')),
  CONSTRAINT chk_quotations_status
    CHECK (status IN ('draft','submitted','approved','rejected','cancelled')),
  -- Non-draft statuses must carry the snapshot.
  CONSTRAINT chk_quotations_snapshot_when_nondraft
    CHECK (
      status = 'draft'
      OR (snapshot_json IS NOT NULL AND total_cny_cents IS NOT NULL AND signature IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_quotations_customer ON quotations(customer_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_quotations_status   ON quotations(status)      WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_quotations_created_by_status ON quotations(created_by, status) WHERE deleted_at IS NULL;

-- Status transitions log. Append-only — rows are never updated or deleted.
CREATE TABLE IF NOT EXISTS quotation_status_history (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  quotation_id    UUID NOT NULL REFERENCES quotations(id) ON DELETE CASCADE,
  from_status     TEXT NOT NULL,
  to_status       TEXT NOT NULL,
  actor_id        UUID REFERENCES users(id) ON DELETE SET NULL,
  comment         TEXT,
  at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_quotation_history_qid ON quotation_status_history(quotation_id, at DESC);
```

- [ ] **Step 2: Write the down migration**

```sql
-- apps/api/migrations/000004_quotations.down.sql
DROP TABLE IF EXISTS quotation_status_history;
DROP TABLE IF EXISTS quotations;
```

- [ ] **Step 3: Build the API and verify migration compiles**

Run: `cd apps/api && go build ./...`
Expected: compiles (no code change yet; migrations are embedded so the embed FS just picks up new files on next startup)

- [ ] **Step 4: Commit**

```bash
git add apps/api/migrations/000004_quotations.up.sql apps/api/migrations/000004_quotations.down.sql
git commit -m "feat(api): migration 000004 quotations + status history"
```

---

### Task 2: Model + DTOs

**Files:**
- Create: `apps/api/internal/quotation/model.go`
- Create: `apps/api/internal/quotation/dto.go`

- [ ] **Step 1: Write `model.go`**

```go
package quotation

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"
)

// Status enumerates the finite set of quotation states. Keep in sync with
// the CHECK constraint in migration 000004.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusSubmitted Status = "submitted"
	StatusApproved  Status = "approved"
	StatusRejected  Status = "rejected"
	StatusCancelled Status = "cancelled"
)

// Quotation mirrors the quotations table.
type Quotation struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	CustomerID      uuid.UUID  `gorm:"type:uuid;not null;index"`
	CountryID       uuid.UUID  `gorm:"type:uuid;not null"`
	ServiceTier     string     `gorm:"not null"`
	Status          Status     `gorm:"not null;default:draft"`
	SnapshotJSON    audit.JSONB `gorm:"type:jsonb"`
	TotalCNYCents   *int64
	Signature       *string
	SubmittedAt     *time.Time
	ReviewedAt      *time.Time
	ReviewedBy      *uuid.UUID `gorm:"type:uuid"`
	ReviewComment   *string
	Notes           *string
	CreatedBy       uuid.UUID `gorm:"type:uuid;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

func (Quotation) TableName() string { return "quotations" }

// StatusHistory mirrors quotation_status_history. Rows are append-only.
type StatusHistory struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	QuotationID uuid.UUID  `gorm:"type:uuid;not null;index"`
	FromStatus  Status     `gorm:"not null"`
	ToStatus    Status     `gorm:"not null"`
	ActorID     *uuid.UUID `gorm:"type:uuid"`
	Comment     *string
	At          time.Time
}

func (StatusHistory) TableName() string { return "quotation_status_history" }
```

- [ ] **Step 2: Write `dto.go`**

```go
package quotation

import (
	"time"

	"github.com/google/uuid"
)

// CreateRequest is the POST /quotations body. Creates a new draft.
type CreateRequest struct {
	CustomerID  uuid.UUID `json:"customer_id" binding:"required"`
	CountryID   uuid.UUID `json:"country_id"  binding:"required"`
	ServiceTier string    `json:"service_tier" binding:"required"`
	Notes       *string   `json:"notes"`
}

// UpdateDraftRequest patches a draft's editable fields. Only applicable
// while status == draft.
type UpdateDraftRequest struct {
	CustomerID  *uuid.UUID `json:"customer_id"`
	CountryID   *uuid.UUID `json:"country_id"`
	ServiceTier *string    `json:"service_tier"`
	Notes       *string    `json:"notes"`
}

// ReviewRequest is the body for approve/reject. Comment is optional but
// strongly recommended when rejecting.
type ReviewRequest struct {
	Comment *string `json:"comment"`
}

// SnapshotLine is one priced fee item. Shape mirrors pricing.CalcLine.
type SnapshotLine struct {
	FeeItem        string `json:"fee_item"`
	AmountCNYCents int64  `json:"amount_cny_cents"`
}

// Snapshot is what's persisted in snapshot_json. Signature + total live
// in their own columns for indexing, but are duplicated here so the
// JSONB blob is self-contained for exports later.
type Snapshot struct {
	Lines         []SnapshotLine `json:"lines"`
	TotalCNYCents int64          `json:"total_cny_cents"`
	Signature     string         `json:"signature"`
}

// Response is the GET response. Shape is flat for easy consumption.
type Response struct {
	ID            uuid.UUID  `json:"id"`
	CustomerID    uuid.UUID  `json:"customer_id"`
	CountryID     uuid.UUID  `json:"country_id"`
	ServiceTier   string     `json:"service_tier"`
	Status        Status     `json:"status"`
	Snapshot      *Snapshot  `json:"snapshot,omitempty"`
	TotalCNYCents *int64     `json:"total_cny_cents,omitempty"`
	Signature     *string    `json:"signature,omitempty"`
	SubmittedAt   *time.Time `json:"submitted_at,omitempty"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy    *uuid.UUID `json:"reviewed_by,omitempty"`
	ReviewComment *string    `json:"review_comment,omitempty"`
	Notes         *string    `json:"notes,omitempty"`
	CreatedBy     uuid.UUID  `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// HistoryEntry is one row in the transition log, returned by the
// history endpoint.
type HistoryEntry struct {
	FromStatus Status     `json:"from_status"`
	ToStatus   Status     `json:"to_status"`
	ActorID    *uuid.UUID `json:"actor_id,omitempty"`
	Comment    *string    `json:"comment,omitempty"`
	At         time.Time  `json:"at"`
}
```

- [ ] **Step 3: Build the package**

Run: `cd apps/api && go build ./internal/quotation/...`
Expected: compiles

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/quotation/model.go apps/api/internal/quotation/dto.go
git commit -m "feat(api): quotation model + DTOs"
```

---

### Task 3: Service layer — state machine, unit-tested

**Files:**
- Create: `apps/api/internal/quotation/service.go`
- Create: `apps/api/internal/quotation/service_test.go`

- [ ] **Step 1: Write `service.go`**

```go
package quotation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"
	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
)

// Typed sentinel errors mapped to HTTP codes in the handler layer.
var (
	ErrInvalidTier      = errors.New("quotation: invalid service tier")
	ErrInvalidTransition = errors.New("quotation: invalid status transition")
	ErrNotOwner         = errors.New("quotation: not owner of quotation")
	ErrNotFound         = errors.New("quotation: not found")
	ErrMissingPricing   = errors.New("quotation: no active pricing entries for country+tier")
)

// repo is the subset of Repository methods Service depends on. Keeps the
// service testable with fakes when full DB isn't needed.
type repo interface {
	Create(ctx context.Context, q *Quotation) error
	Get(ctx context.Context, id uuid.UUID) (*Quotation, error)
	UpdateDraft(ctx context.Context, id uuid.UUID, patch map[string]any) error
	Transition(ctx context.Context, q *Quotation, to Status, actorID uuid.UUID, comment *string) error
	List(ctx context.Context, f ListFilter) ([]Quotation, int64, error)
	History(ctx context.Context, id uuid.UUID) ([]StatusHistory, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// pricingRepo is the subset of pricing.Repository we need. Injected so
// the service can compute the submit-time snapshot.
type pricingRepo interface {
	ListActive(ctx context.Context, countryID *uuid.UUID) ([]pricing.PricingEntry, error)
}

// Service owns quotation business rules. Role enforcement lives in the
// handler/middleware layer; Service assumes the caller has the right
// role and only checks *ownership* within that role (e.g. a salesperson
// may only edit their own drafts).
type Service struct {
	repo        repo
	pricingRepo pricingRepo
}

func NewService(r repo, p pricingRepo) *Service { return &Service{repo: r, pricingRepo: p} }

// ListFilter feeds Service.List / repo.List.
type ListFilter struct {
	OwnerID    *uuid.UUID // when set, only rows created_by this user
	CustomerID *uuid.UUID
	Status     *Status
	Page       int
	PageSize   int
}

// Create inserts a draft quotation. Service validates the tier only;
// customer/country existence is enforced by FK constraints.
func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, req CreateRequest) (*Quotation, error) {
	if !pricing.IsValidServiceTier(req.ServiceTier) {
		return nil, ErrInvalidTier
	}
	q := &Quotation{
		ID:          uuid.New(),
		CustomerID:  req.CustomerID,
		CountryID:   req.CountryID,
		ServiceTier: req.ServiceTier,
		Status:      StatusDraft,
		Notes:       req.Notes,
		CreatedBy:   ownerID,
	}
	if err := s.repo.Create(ctx, q); err != nil {
		return nil, err
	}
	return q, nil
}

// UpdateDraft patches a draft's editable fields. Only the owner can
// patch, and only while status == draft.
func (s *Service) UpdateDraft(ctx context.Context, id, actorID uuid.UUID, req UpdateDraftRequest) error {
	q, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if q == nil {
		return ErrNotFound
	}
	if q.CreatedBy != actorID {
		return ErrNotOwner
	}
	if q.Status != StatusDraft {
		return ErrInvalidTransition
	}
	patch := map[string]any{}
	if req.CustomerID != nil {
		patch["customer_id"] = *req.CustomerID
	}
	if req.CountryID != nil {
		patch["country_id"] = *req.CountryID
	}
	if req.ServiceTier != nil {
		if !pricing.IsValidServiceTier(*req.ServiceTier) {
			return ErrInvalidTier
		}
		patch["service_tier"] = *req.ServiceTier
	}
	if req.Notes != nil {
		patch["notes"] = *req.Notes
	}
	if len(patch) == 0 {
		return nil
	}
	patch["updated_at"] = time.Now()
	return s.repo.UpdateDraft(ctx, id, patch)
}

// Submit: draft → submitted. Snapshots the current active pricing for
// the quotation's (country, tier) into snapshot_json + total + signature.
// Only the owner may submit.
func (s *Service) Submit(ctx context.Context, id, actorID uuid.UUID) (*Quotation, error) {
	q, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, ErrNotFound
	}
	if q.CreatedBy != actorID {
		return nil, ErrNotOwner
	}
	if q.Status != StatusDraft {
		return nil, ErrInvalidTransition
	}

	// Fetch active pricing then compute deterministic snapshot.
	entries, err := s.pricingRepo.ListActive(ctx, &q.CountryID)
	if err != nil {
		return nil, err
	}
	calc, err := pricing.Calculate(entries, pricing.CalcInput{
		CountryID:   q.CountryID,
		ServiceTier: q.ServiceTier,
	})
	if err != nil {
		if errors.Is(err, pricing.ErrNoMatchingEntries) {
			return nil, ErrMissingPricing
		}
		return nil, fmt.Errorf("quotation: pricing calculate: %w", err)
	}

	snap := Snapshot{
		Lines:         make([]SnapshotLine, 0, len(calc.Lines)),
		TotalCNYCents: calc.TotalCNYCents,
		Signature:     calc.Signature,
	}
	for _, l := range calc.Lines {
		snap.Lines = append(snap.Lines, SnapshotLine{FeeItem: l.FeeItem, AmountCNYCents: l.AmountCNYCents})
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return nil, fmt.Errorf("quotation: marshal snapshot: %w", err)
	}
	now := time.Now()
	q.SnapshotJSON = audit.JSONB(raw)
	q.TotalCNYCents = &calc.TotalCNYCents
	sig := calc.Signature
	q.Signature = &sig
	q.SubmittedAt = &now

	if err := s.repo.Transition(ctx, q, StatusSubmitted, actorID, nil); err != nil {
		return nil, err
	}
	return q, nil
}

// Approve / Reject are reviewer transitions. They do NOT recompute the
// snapshot — whatever was captured at submit time is what gets approved.
func (s *Service) Review(ctx context.Context, id, actorID uuid.UUID, approve bool, comment *string) (*Quotation, error) {
	q, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, ErrNotFound
	}
	if q.Status != StatusSubmitted {
		return nil, ErrInvalidTransition
	}
	now := time.Now()
	q.ReviewedAt = &now
	q.ReviewedBy = &actorID
	q.ReviewComment = comment
	target := StatusRejected
	if approve {
		target = StatusApproved
	}
	if err := s.repo.Transition(ctx, q, target, actorID, comment); err != nil {
		return nil, err
	}
	return q, nil
}

// Cancel: draft → cancelled, owner only.
func (s *Service) Cancel(ctx context.Context, id, actorID uuid.UUID, comment *string) error {
	q, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if q == nil {
		return ErrNotFound
	}
	if q.CreatedBy != actorID {
		return ErrNotOwner
	}
	if q.Status != StatusDraft {
		return ErrInvalidTransition
	}
	return s.repo.Transition(ctx, q, StatusCancelled, actorID, comment)
}

// Get returns a single quotation. Callers enforce visibility (owner vs
// reviewer) above this.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Quotation, error) {
	q, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, ErrNotFound
	}
	return q, nil
}

// List delegates straight to the repo. OwnerID scoping for salespeople
// is set by the handler before calling.
func (s *Service) List(ctx context.Context, f ListFilter) ([]Quotation, int64, error) {
	return s.repo.List(ctx, f)
}

// History returns the transition log.
func (s *Service) History(ctx context.Context, id uuid.UUID) ([]StatusHistory, error) {
	return s.repo.History(ctx, id)
}

// ToResponse marshals a domain Quotation into its HTTP response shape.
// Snapshot JSONB is re-parsed so the client gets structured lines
// instead of a raw JSON blob.
func ToResponse(q *Quotation) Response {
	out := Response{
		ID: q.ID, CustomerID: q.CustomerID, CountryID: q.CountryID,
		ServiceTier: q.ServiceTier, Status: q.Status,
		TotalCNYCents: q.TotalCNYCents, Signature: q.Signature,
		SubmittedAt: q.SubmittedAt, ReviewedAt: q.ReviewedAt,
		ReviewedBy: q.ReviewedBy, ReviewComment: q.ReviewComment,
		Notes: q.Notes, CreatedBy: q.CreatedBy,
		CreatedAt: q.CreatedAt, UpdatedAt: q.UpdatedAt,
	}
	if len(q.SnapshotJSON) > 0 {
		var snap Snapshot
		if err := json.Unmarshal(q.SnapshotJSON, &snap); err == nil {
			out.Snapshot = &snap
		}
	}
	return out
}
```

- [ ] **Step 2: Write failing unit tests for state machine**

```go
// apps/api/internal/quotation/service_test.go
package quotation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
)

// fakeRepo is an in-memory repo implementation for testing the service
// layer without Postgres.
type fakeRepo struct {
	byID       map[uuid.UUID]*Quotation
	history    map[uuid.UUID][]StatusHistory
	nextErr    error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: map[uuid.UUID]*Quotation{}, history: map[uuid.UUID][]StatusHistory{}}
}

func (f *fakeRepo) Create(ctx context.Context, q *Quotation) error {
	if f.nextErr != nil {
		e := f.nextErr
		f.nextErr = nil
		return e
	}
	q.CreatedAt, q.UpdatedAt = time.Now(), time.Now()
	f.byID[q.ID] = q
	return nil
}
func (f *fakeRepo) Get(ctx context.Context, id uuid.UUID) (*Quotation, error) {
	return f.byID[id], nil
}
func (f *fakeRepo) UpdateDraft(ctx context.Context, id uuid.UUID, patch map[string]any) error {
	q := f.byID[id]
	if q == nil {
		return ErrNotFound
	}
	if v, ok := patch["notes"].(string); ok {
		q.Notes = &v
	}
	if v, ok := patch["service_tier"].(string); ok {
		q.ServiceTier = v
	}
	return nil
}
func (f *fakeRepo) Transition(ctx context.Context, q *Quotation, to Status, actorID uuid.UUID, comment *string) error {
	from := q.Status
	q.Status = to
	f.byID[q.ID] = q
	f.history[q.ID] = append(f.history[q.ID], StatusHistory{
		ID: uuid.New(), QuotationID: q.ID,
		FromStatus: from, ToStatus: to, ActorID: &actorID, Comment: comment,
		At: time.Now(),
	})
	return nil
}
func (f *fakeRepo) List(ctx context.Context, fl ListFilter) ([]Quotation, int64, error) {
	var out []Quotation
	for _, q := range f.byID {
		if fl.OwnerID != nil && q.CreatedBy != *fl.OwnerID {
			continue
		}
		out = append(out, *q)
	}
	return out, int64(len(out)), nil
}
func (f *fakeRepo) History(ctx context.Context, id uuid.UUID) ([]StatusHistory, error) {
	return f.history[id], nil
}
func (f *fakeRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	delete(f.byID, id)
	return nil
}

// fakePricingRepo always returns the configured entries regardless of
// country filter.
type fakePricingRepo struct {
	entries []pricing.PricingEntry
	err     error
}

func (f *fakePricingRepo) ListActive(ctx context.Context, _ *uuid.UUID) ([]pricing.PricingEntry, error) {
	return f.entries, f.err
}

func newService(entries []pricing.PricingEntry) (*Service, *fakeRepo) {
	r := newFakeRepo()
	return NewService(r, &fakePricingRepo{entries: entries}), r
}

func TestCreate_InvalidTier(t *testing.T) {
	svc, _ := newService(nil)
	_, err := svc.Create(context.Background(), uuid.New(), CreateRequest{
		CustomerID: uuid.New(), CountryID: uuid.New(), ServiceTier: "vip",
	})
	if !errors.Is(err, ErrInvalidTier) {
		t.Fatalf("want ErrInvalidTier, got %v", err)
	}
}

func TestSubmit_SnapshotsPricing(t *testing.T) {
	country := uuid.New()
	owner := uuid.New()
	from := time.Now().Add(-24 * time.Hour)
	entries := []pricing.PricingEntry{
		{ID: uuid.New(), CountryID: country, ServiceTier: "basic", FeeItem: "application", AmountCNYCents: 10000, EffectiveFrom: from, CreatedBy: owner},
		{ID: uuid.New(), CountryID: country, ServiceTier: "basic", FeeItem: "agent", AmountCNYCents: 5000, EffectiveFrom: from, CreatedBy: owner},
	}
	svc, repo := newService(entries)
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
	if submitted.Status != StatusSubmitted {
		t.Fatalf("status = %q, want submitted", submitted.Status)
	}
	if submitted.TotalCNYCents == nil || *submitted.TotalCNYCents != 15000 {
		t.Fatalf("total: want 15000, got %v", submitted.TotalCNYCents)
	}
	if submitted.Signature == nil || *submitted.Signature == "" {
		t.Fatal("signature should be set after submit")
	}
	hist, _ := repo.History(context.Background(), q.ID)
	if len(hist) != 1 || hist[0].ToStatus != StatusSubmitted {
		t.Fatalf("history: want one submit row, got %+v", hist)
	}
}

func TestSubmit_RejectsWhenNoPricing(t *testing.T) {
	svc, _ := newService(nil) // no entries
	owner := uuid.New()
	q, _ := svc.Create(context.Background(), owner, CreateRequest{
		CustomerID: uuid.New(), CountryID: uuid.New(), ServiceTier: "basic",
	})
	_, err := svc.Submit(context.Background(), q.ID, owner)
	if !errors.Is(err, ErrMissingPricing) {
		t.Fatalf("want ErrMissingPricing, got %v", err)
	}
}

func TestSubmit_OnlyOwner(t *testing.T) {
	svc, _ := newService([]pricing.PricingEntry{
		{ID: uuid.New(), CountryID: uuid.New(), ServiceTier: "basic", FeeItem: "f", AmountCNYCents: 1, EffectiveFrom: time.Now(), CreatedBy: uuid.New()},
	})
	owner := uuid.New()
	intruder := uuid.New()
	q, _ := svc.Create(context.Background(), owner, CreateRequest{
		CustomerID: uuid.New(), CountryID: uuid.New(), ServiceTier: "basic",
	})
	if _, err := svc.Submit(context.Background(), q.ID, intruder); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("want ErrNotOwner, got %v", err)
	}
}

func TestReview_OnlyFromSubmitted(t *testing.T) {
	svc, _ := newService([]pricing.PricingEntry{
		{ID: uuid.New(), CountryID: uuid.New(), ServiceTier: "basic", FeeItem: "f", AmountCNYCents: 1, EffectiveFrom: time.Now(), CreatedBy: uuid.New()},
	})
	owner := uuid.New()
	q, _ := svc.Create(context.Background(), owner, CreateRequest{
		CustomerID: uuid.New(), CountryID: uuid.New(), ServiceTier: "basic",
	})
	// Reviewing a draft must fail.
	if _, err := svc.Review(context.Background(), q.ID, uuid.New(), true, nil); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("review draft: want ErrInvalidTransition, got %v", err)
	}
}

func TestCancel_OnlyDraftByOwner(t *testing.T) {
	svc, _ := newService([]pricing.PricingEntry{
		{ID: uuid.New(), CountryID: uuid.New(), ServiceTier: "basic", FeeItem: "f", AmountCNYCents: 1, EffectiveFrom: time.Now(), CreatedBy: uuid.New()},
	})
	owner, other := uuid.New(), uuid.New()
	q, _ := svc.Create(context.Background(), owner, CreateRequest{
		CustomerID: uuid.New(), CountryID: uuid.New(), ServiceTier: "basic",
	})
	if err := svc.Cancel(context.Background(), q.ID, other, nil); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("want ErrNotOwner, got %v", err)
	}
	if err := svc.Cancel(context.Background(), q.ID, owner, nil); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if q2, _ := svc.Get(context.Background(), q.ID); q2.Status != StatusCancelled {
		t.Fatalf("status = %q, want cancelled", q2.Status)
	}
}

func TestSubmitThenApprove_HistoryRows(t *testing.T) {
	country := uuid.New()
	owner, reviewer := uuid.New(), uuid.New()
	entries := []pricing.PricingEntry{
		{ID: uuid.New(), CountryID: country, ServiceTier: "basic", FeeItem: "f", AmountCNYCents: 100, EffectiveFrom: time.Now().Add(-24 * time.Hour), CreatedBy: owner},
	}
	svc, repo := newService(entries)
	q, _ := svc.Create(context.Background(), owner, CreateRequest{
		CustomerID: uuid.New(), CountryID: country, ServiceTier: "basic",
	})
	if _, err := svc.Submit(context.Background(), q.ID, owner); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Review(context.Background(), q.ID, reviewer, true, strPtr("ok")); err != nil {
		t.Fatalf("approve: %v", err)
	}
	hist, _ := repo.History(context.Background(), q.ID)
	if len(hist) != 2 {
		t.Fatalf("want 2 history rows, got %d", len(hist))
	}
	if hist[0].ToStatus != StatusSubmitted || hist[1].ToStatus != StatusApproved {
		t.Fatalf("history order wrong: %+v", hist)
	}
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 3: Run tests, expect failure**

Run: `cd apps/api && go test ./internal/quotation/...`
Expected: build fails — no repo.go yet, so `Repository` not defined. Service compiles but will fail later. **Actually:** this task's service.go compiles because it only references the `repo` interface — so tests should compile and the fake will drive them. Run and expect PASS now, since the fakes fully implement `repo`.

- [ ] **Step 4: Fix any test failures**

If any fail, fix the logic in service.go before moving on.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/quotation/service.go apps/api/internal/quotation/service_test.go
git commit -m "feat(api): quotation service with state-machine unit tests"
```

---

### Task 4: Repository (GORM) + integration tests

**Files:**
- Create: `apps/api/internal/quotation/repository.go`
- Create: `apps/api/internal/quotation/repository_test.go`

- [ ] **Step 1: Write `repository.go`**

```go
package quotation

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository is the GORM-backed persistence layer.
type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, q *Quotation) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(q).Error
}

// Get returns the quotation or nil + nil err when not found.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Quotation, error) {
	var q Quotation
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&q).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// UpdateDraft applies a map of column updates. Caller guarantees status == draft.
func (r *Repository) UpdateDraft(ctx context.Context, id uuid.UUID, patch map[string]any) error {
	res := r.db.WithContext(ctx).
		Model(&Quotation{}).
		Where("id = ? AND status = ?", id, StatusDraft).
		Updates(patch)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrInvalidTransition
	}
	return nil
}

// Transition updates the quotation row and appends a StatusHistory row
// in a single transaction. `q` is assumed to already reflect the target
// state on all snapshot/reviewer fields — Transition only writes status
// itself plus the history row.
func (r *Repository) Transition(
	ctx context.Context,
	q *Quotation,
	to Status,
	actorID uuid.UUID,
	comment *string,
) error {
	from := q.Status
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update the main row with status + any set snapshot/reviewer fields.
		updates := map[string]any{
			"status":     to,
			"updated_at": time.Now(),
		}
		if q.SnapshotJSON != nil {
			updates["snapshot_json"] = q.SnapshotJSON
		}
		if q.TotalCNYCents != nil {
			updates["total_cny_cents"] = *q.TotalCNYCents
		}
		if q.Signature != nil {
			updates["signature"] = *q.Signature
		}
		if q.SubmittedAt != nil {
			updates["submitted_at"] = *q.SubmittedAt
		}
		if q.ReviewedAt != nil {
			updates["reviewed_at"] = *q.ReviewedAt
		}
		if q.ReviewedBy != nil {
			updates["reviewed_by"] = *q.ReviewedBy
		}
		if q.ReviewComment != nil {
			updates["review_comment"] = *q.ReviewComment
		}
		if err := tx.Model(&Quotation{}).
			Where("id = ? AND status = ?", q.ID, from).
			Updates(updates).Error; err != nil {
			return err
		}
		// Append the history row.
		h := StatusHistory{
			ID: uuid.New(), QuotationID: q.ID,
			FromStatus: from, ToStatus: to,
			ActorID: &actorID, Comment: comment,
			At: time.Now(),
		}
		return tx.Create(&h).Error
	})
}

// List returns quotations matching the filter plus the total count.
func (r *Repository) List(ctx context.Context, f ListFilter) ([]Quotation, int64, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 200 {
		f.PageSize = 20
	}
	q := r.db.WithContext(ctx).Model(&Quotation{})
	if f.OwnerID != nil {
		q = q.Where("created_by = ?", *f.OwnerID)
	}
	if f.CustomerID != nil {
		q = q.Where("customer_id = ?", *f.CustomerID)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []Quotation
	err := q.Order("created_at DESC").
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		Find(&rows).Error
	return rows, total, err
}

// History returns the append-only transition log, oldest first.
func (r *Repository) History(ctx context.Context, id uuid.UUID) ([]StatusHistory, error) {
	var rows []StatusHistory
	err := r.db.WithContext(ctx).
		Where("quotation_id = ?", id).
		Order("at ASC").
		Find(&rows).Error
	return rows, err
}

// SoftDelete sets deleted_at. Only draft quotations should be deletable
// — that rule lives at the service layer.
func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Quotation{}, "id = ?", id).Error
}
```

- [ ] **Step 2: Write integration tests using testcontainers**

```go
// apps/api/internal/quotation/repository_test.go
package quotation_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"
	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// bootPg spins up a fresh Postgres container, applies migrations, and
// returns a GORM handle + the raw DSN.
func bootPg(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("quot_test"),
		tcpostgres.WithUsername("quot"),
		tcpostgres.WithPassword("quot"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("postgres container: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	mig, err := migrator.New(api.Migrations, "migrations", dsn)
	if err != nil {
		t.Fatalf("migrator: %v", err)
	}
	if err := mig.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	_ = mig.Close()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	return db, dsn
}

// seedCustomerCountryUser inserts the minimum FK targets so a quotation
// row can exist. Returns (customerID, countryID, userID).
func seedCustomerCountryUser(t *testing.T, db *gorm.DB) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	custID := uuid.New()
	countryID := uuid.New()
	userID := uuid.New()
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO customers (id, name, created_by) VALUES (?, ?, ?)`,
		custID, "Test Customer", userID,
	).Error; err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO countries (id, code, name_zh, name_en) VALUES (?, ?, ?, ?)`,
		countryID, "XX", "测试国", "Testland",
	).Error; err != nil {
		t.Fatalf("seed country: %v", err)
	}
	// users table has role_id FK — look up salesperson role id.
	var roleID uuid.UUID
	if err := db.WithContext(ctx).Raw(
		`SELECT id FROM roles WHERE code = ?`, "salesperson",
	).Scan(&roleID).Error; err != nil {
		t.Fatalf("select role: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		userID, "Tester", "tester-"+userID.String()+"@example.com", "x", roleID,
	).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return custID, countryID, userID
}

func TestRepository_CreateGet(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, userID := seedCustomerCountryUser(t, db)
	r := quotation.NewRepository(db)

	q := &quotation.Quotation{
		CustomerID: custID, CountryID: countryID,
		ServiceTier: "basic", Status: quotation.StatusDraft,
		CreatedBy: userID,
	}
	if err := r.Create(context.Background(), q); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := r.Get(context.Background(), q.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.Status != quotation.StatusDraft {
		t.Fatalf("got %+v", got)
	}
}

func TestRepository_TransitionRecordsHistory(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, userID := seedCustomerCountryUser(t, db)
	r := quotation.NewRepository(db)

	// Create draft.
	q := &quotation.Quotation{
		CustomerID: custID, CountryID: countryID,
		ServiceTier: "basic", Status: quotation.StatusDraft,
		CreatedBy: userID,
	}
	if err := r.Create(context.Background(), q); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Transition to submitted with a fake snapshot payload.
	snap, _ := json.Marshal(quotation.Snapshot{
		Lines:         []quotation.SnapshotLine{{FeeItem: "f", AmountCNYCents: 1000}},
		TotalCNYCents: 1000, Signature: "sig",
	})
	q.SnapshotJSON = audit.JSONB(snap)
	total := int64(1000)
	sig := "sig"
	now := time.Now()
	q.TotalCNYCents = &total
	q.Signature = &sig
	q.SubmittedAt = &now
	if err := r.Transition(context.Background(), q, quotation.StatusSubmitted, userID, nil); err != nil {
		t.Fatalf("transition: %v", err)
	}
	got, _ := r.Get(context.Background(), q.ID)
	if got.Status != quotation.StatusSubmitted || got.Signature == nil || *got.Signature != "sig" {
		t.Fatalf("after submit: %+v", got)
	}

	hist, err := r.History(context.Background(), q.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(hist) != 1 || hist[0].FromStatus != quotation.StatusDraft || hist[0].ToStatus != quotation.StatusSubmitted {
		t.Fatalf("history: %+v", hist)
	}
}

func TestRepository_CheckConstraintBlocksSubmittedWithoutSnapshot(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, userID := seedCustomerCountryUser(t, db)
	r := quotation.NewRepository(db)

	// Directly attempt to insert a submitted quotation with no snapshot.
	q := &quotation.Quotation{
		CustomerID: custID, CountryID: countryID,
		ServiceTier: "basic", Status: quotation.StatusSubmitted,
		CreatedBy: userID,
	}
	if err := r.Create(context.Background(), q); err == nil {
		t.Fatal("expected CHECK violation on submitted without snapshot")
	}
}

func TestRepository_ListFilters(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, userID := seedCustomerCountryUser(t, db)
	r := quotation.NewRepository(db)

	// Insert 3 quotations — 2 owned by userID as draft, 1 by a different
	// user with status cancelled.
	for i := 0; i < 2; i++ {
		_ = r.Create(context.Background(), &quotation.Quotation{
			CustomerID: custID, CountryID: countryID,
			ServiceTier: "basic", Status: quotation.StatusDraft,
			CreatedBy: userID,
		})
	}
	// Seed a second user to own the third row.
	var roleID uuid.UUID
	_ = db.Raw(`SELECT id FROM roles WHERE code = 'salesperson'`).Scan(&roleID).Error
	other := uuid.New()
	_ = db.Exec(`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		other, "Other", "other-"+other.String()+"@example.com", "x", roleID).Error
	_ = r.Create(context.Background(), &quotation.Quotation{
		CustomerID: custID, CountryID: countryID,
		ServiceTier: "basic", Status: quotation.StatusDraft,
		CreatedBy: other,
	})

	// Owner filter → 2 rows.
	ownFilter := userID
	got, total, err := r.List(context.Background(), quotation.ListFilter{OwnerID: &ownFilter})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(got) != 2 {
		t.Fatalf("owner filter: total=%d got=%d", total, len(got))
	}

	// Status filter → all 3 are drafts.
	draftStatus := quotation.StatusDraft
	_, total, _ = r.List(context.Background(), quotation.ListFilter{Status: &draftStatus})
	if total != 3 {
		t.Fatalf("draft total: want 3, got %d", total)
	}
}

// This silences the unused-import warning if pricing is elided.
var _ = pricing.ServiceTiers
```

- [ ] **Step 3: Run integration tests**

Run: `cd apps/api && go test ./internal/quotation/... -run Repository`
Expected: all 4 repo tests pass (spins up postgres container).

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/quotation/repository.go apps/api/internal/quotation/repository_test.go
git commit -m "feat(api): quotation repository + testcontainer integration tests"
```

---

### Task 5: Handlers + router

**Files:**
- Create: `apps/api/internal/quotation/handler.go`
- Create: `apps/api/internal/quotation/router.go`

- [ ] **Step 1: Write `handler.go`**

```go
package quotation

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/httpx"
)

// Handler wires HTTP to Service. Role gating is enforced by the
// middleware at the router level — inside handlers we only do ownership
// checks (e.g. a salesperson can't read another salesperson's draft).
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Post /quotations — create draft. Any authenticated user may create.
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortBadRequest(c, "invalid body", "ERR_VALIDATION")
		return
	}
	user := auth.CurrentUser(c)
	q, err := h.svc.Create(c.Request.Context(), user.ID, req)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, ToResponse(q))
}

// Get /quotations/:id — read one. Salesperson may only read their own.
func (h *Handler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	q, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	if !h.canRead(c, q) {
		httpx.AbortForbidden(c, "forbidden")
		return
	}
	c.JSON(http.StatusOK, ToResponse(q))
}

// Get /quotations — list. Role shapes the scope:
//   - salesperson → only their own
//   - reviewer/admin → all
func (h *Handler) List(c *gin.Context) {
	user := auth.CurrentUser(c)
	f := ListFilter{
		Page:     atoiDefault(c.Query("page"), 1),
		PageSize: atoiDefault(c.Query("page_size"), 20),
	}
	if user.Role == "salesperson" {
		uid := user.ID
		f.OwnerID = &uid
	}
	if s := c.Query("status"); s != "" {
		st := Status(s)
		f.Status = &st
	}
	if cid := c.Query("customer_id"); cid != "" {
		if cuid, err := uuid.Parse(cid); err == nil {
			f.CustomerID = &cuid
		}
	}
	rows, total, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	items := make([]Response, 0, len(rows))
	for i := range rows {
		items = append(items, ToResponse(&rows[i]))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": f.Page, "page_size": f.PageSize})
}

// Patch /quotations/:id — update editable fields while draft.
func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req UpdateDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.AbortBadRequest(c, "invalid body", "ERR_VALIDATION")
		return
	}
	user := auth.CurrentUser(c)
	if err := h.svc.UpdateDraft(c.Request.Context(), id, user.ID, req); err != nil {
		h.writeServiceErr(c, err)
		return
	}
	q, _ := h.svc.Get(c.Request.Context(), id)
	c.JSON(http.StatusOK, ToResponse(q))
}

// Post /quotations/:id/submit — draft → submitted.
func (h *Handler) Submit(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	user := auth.CurrentUser(c)
	q, err := h.svc.Submit(c.Request.Context(), id, user.ID)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, ToResponse(q))
}

// Post /quotations/:id/approve | /reject.
func (h *Handler) review(approve bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req ReviewRequest
		_ = c.ShouldBindJSON(&req) // body optional
		user := auth.CurrentUser(c)
		q, err := h.svc.Review(c.Request.Context(), id, user.ID, approve, req.Comment)
		if err != nil {
			h.writeServiceErr(c, err)
			return
		}
		c.JSON(http.StatusOK, ToResponse(q))
	}
}

// Post /quotations/:id/cancel — owner cancels a draft.
func (h *Handler) Cancel(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req ReviewRequest // reusing shape for comment
	_ = c.ShouldBindJSON(&req)
	user := auth.CurrentUser(c)
	if err := h.svc.Cancel(c.Request.Context(), id, user.ID, req.Comment); err != nil {
		h.writeServiceErr(c, err)
		return
	}
	q, _ := h.svc.Get(c.Request.Context(), id)
	c.JSON(http.StatusOK, ToResponse(q))
}

// Get /quotations/:id/history.
func (h *Handler) History(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	q, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	if !h.canRead(c, q) {
		httpx.AbortForbidden(c, "forbidden")
		return
	}
	rows, err := h.svc.History(c.Request.Context(), id)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	out := make([]HistoryEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, HistoryEntry{
			FromStatus: r.FromStatus, ToStatus: r.ToStatus,
			ActorID: r.ActorID, Comment: r.Comment, At: r.At,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

// canRead returns true when the current user can read this quotation.
// Salespeople can only read their own; reviewer + admin read all.
func (h *Handler) canRead(c *gin.Context, q *Quotation) bool {
	u := auth.CurrentUser(c)
	switch u.Role {
	case "admin", "reviewer":
		return true
	case "salesperson":
		return q.CreatedBy == u.ID
	}
	return false
}

func (h *Handler) writeServiceErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.AbortNotFound(c, "not found")
	case errors.Is(err, ErrNotOwner):
		httpx.AbortForbidden(c, "not owner")
	case errors.Is(err, ErrInvalidTransition):
		httpx.AbortConflict(c, "invalid status transition", "ERR_INVALID_TRANSITION")
	case errors.Is(err, ErrInvalidTier):
		httpx.AbortBadRequest(c, "invalid service tier", "ERR_INVALID_TIER")
	case errors.Is(err, ErrMissingPricing):
		httpx.AbortUnprocessable(c, "no active pricing for country+tier", "ERR_MISSING_PRICING")
	default:
		httpx.AbortInternal(c, "quotation error")
	}
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.AbortBadRequest(c, "bad id", "ERR_VALIDATION")
		return uuid.Nil, false
	}
	return id, true
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}
```

- [ ] **Step 2: Write `router.go`**

```go
package quotation

import (
	"github.com/gin-gonic/gin"
)

// RegisterAuthedRoutes registers endpoints available to any authed user.
// Inside the handlers we apply finer-grained role/ownership checks.
func RegisterAuthedRoutes(g *gin.RouterGroup, h *Handler) {
	g.POST("/quotations", h.Create)
	g.GET("/quotations", h.List)
	g.GET("/quotations/:id", h.Get)
	g.GET("/quotations/:id/history", h.History)
	g.PATCH("/quotations/:id", h.Update)
	g.POST("/quotations/:id/submit", h.Submit)
	g.POST("/quotations/:id/cancel", h.Cancel)
}

// RegisterReviewerRoutes registers reviewer-only transitions. The group
// is expected to already chain RequireRole("reviewer","admin").
func RegisterReviewerRoutes(g *gin.RouterGroup, h *Handler) {
	g.POST("/quotations/:id/approve", h.review(true))
	g.POST("/quotations/:id/reject", h.review(false))
}
```

- [ ] **Step 3: Check httpx has the helpers used above**

Run: `grep -rn "AbortUnprocessable\|AbortConflict\|AbortForbidden\|AbortNotFound\|AbortBadRequest\|AbortInternal" apps/api/internal/platform/httpx/`
Expected: all 6 present. If any helper is missing, add it to `apps/api/internal/platform/httpx/errors.go` following the existing pattern. They must each accept `(c *gin.Context, msg string, [code string]...)` and call `c.AbortWithStatusJSON(<status>, gin.H{"error": msg, "code": code})`.

- [ ] **Step 4: Build**

Run: `cd apps/api && go build ./...`
Expected: compiles.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/quotation/handler.go apps/api/internal/quotation/router.go
# plus any httpx additions made in Step 3:
git add apps/api/internal/platform/httpx/ 2>/dev/null || true
git commit -m "feat(api): quotation HTTP handlers + routes"
```

---

### Task 6: Wire into `cmd/server/main.go`

**Files:**
- Modify: `apps/api/cmd/server/main.go`

- [ ] **Step 1: Add the import + construction block**

In the imports, add (after the pricing import):

```go
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
```

After the `pricing.RegisterAdminRoutes(adminGroup, pricingHandler)` line (just before `srv := &http.Server{...}`), add:

```go
	// Quotations — any authed user creates/lists own; reviewer+admin
	// sees all + approves/rejects. Salesperson ownership is enforced
	// inside the handler layer.
	quotRepo := quotation.NewRepository(db)
	quotSvc := quotation.NewService(quotRepo, pricingRepo)
	quotHandler := quotation.NewHandler(quotSvc)
	quotation.RegisterAuthedRoutes(authed, quotHandler)
	quotation.RegisterReviewerRoutes(reviewerAdminGroup, quotHandler)
```

- [ ] **Step 2: Build**

Run: `cd apps/api && go build ./...`
Expected: compiles.

- [ ] **Step 3: Run full backend test suite**

Run: `cd apps/api && go test ./...`
Expected: all existing packages still green; new quotation package shows 6 unit tests + 4 integration tests passing.

- [ ] **Step 4: Commit**

```bash
git add apps/api/cmd/server/main.go
git commit -m "feat(api): wire quotation package into server main"
```

---

### Task 7: Handler integration test (end-to-end through Gin)

**Files:**
- Create: `apps/api/internal/quotation/handler_test.go`

- [ ] **Step 1: Write the handler-level test**

Use the same testcontainer boot helper from repository_test.go via package-level reuse. This test exercises one full flow: salesperson creates draft → submits (snapshot captured) → reviewer approves.

```go
// apps/api/internal/quotation/handler_test.go
package quotation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
)

// buildRouter wires up a Gin router with a synthetic auth middleware
// that injects the current user from headers — mirrors how the real
// middleware populates context but sidesteps JWT plumbing.
func buildRouter(t *testing.T, quotHandler *quotation.Handler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		uid, _ := uuid.Parse(c.GetHeader("X-Test-User-ID"))
		role := c.GetHeader("X-Test-Role")
		auth.SetCurrentUser(c, auth.User{ID: uid, Role: role})
		c.Next()
	})
	authed := r.Group("/api/v1")
	quotation.RegisterAuthedRoutes(authed, quotHandler)
	reviewer := r.Group("/api/v1")
	quotation.RegisterReviewerRoutes(reviewer, quotHandler)
	return r
}

func TestHandler_HappyPath_SubmitThenApprove(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, salesID := seedCustomerCountryUser(t, db)

	// Seed a single active pricing entry so submit can snapshot.
	_ = db.Exec(
		`INSERT INTO pricing_entries
		 (id, country_id, service_tier, fee_item, amount_cny_cents, effective_from, created_by)
		 VALUES (?, ?, 'basic', 'application', 10000, ?, ?)`,
		uuid.New(), countryID, time.Now(), salesID,
	).Error

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepo)
	r := buildRouter(t, quotation.NewHandler(svc))

	// Salesperson creates draft.
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

	// Salesperson submits.
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
	if submitted.Status != quotation.StatusSubmitted || submitted.Snapshot == nil {
		t.Fatalf("submit result: %+v", submitted)
	}
	if submitted.TotalCNYCents == nil || *submitted.TotalCNYCents != 10000 {
		t.Fatalf("total = %v, want 10000", submitted.TotalCNYCents)
	}

	// Seed a reviewer user too.
	var roleID uuid.UUID
	_ = db.Raw(`SELECT id FROM roles WHERE code = 'reviewer'`).Scan(&roleID).Error
	reviewerID := uuid.New()
	_ = db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		reviewerID, "Rev", "rev-"+reviewerID.String()+"@example.com", "x", roleID,
	).Error

	// Reviewer approves.
	req, _ = http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+created.ID.String()+"/approve", bytes.NewReader([]byte(`{"comment":"ok"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("approve: status %d body %s", w.Code, w.Body.String())
	}
	var approved quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &approved)
	if approved.Status != quotation.StatusApproved {
		t.Fatalf("status = %q, want approved", approved.Status)
	}

	// History endpoint returns 2 rows.
	req, _ = http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/quotations/"+created.ID.String()+"/history", nil)
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("history: status %d body %s", w.Code, w.Body.String())
	}
	var hist struct {
		Items []quotation.HistoryEntry `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &hist)
	if len(hist.Items) != 2 {
		t.Fatalf("history items = %d, want 2", len(hist.Items))
	}
}

func TestHandler_SalespersonCannotReadAnothersQuotation(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, aliceID := seedCustomerCountryUser(t, db)

	// Seed bob too.
	var roleID uuid.UUID
	_ = db.Raw(`SELECT id FROM roles WHERE code = 'salesperson'`).Scan(&roleID).Error
	bobID := uuid.New()
	_ = db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		bobID, "Bob", "bob-"+bobID.String()+"@example.com", "x", roleID,
	).Error

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepo)
	r := buildRouter(t, quotation.NewHandler(svc))

	// Alice creates a quotation.
	body, _ := json.Marshal(map[string]any{
		"customer_id": custID, "country_id": countryID, "service_tier": "basic",
	})
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", aliceID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status %d", w.Code)
	}
	var created quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	// Bob tries to read it → 403.
	req, _ = http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/quotations/"+created.ID.String(), nil)
	req.Header.Set("X-Test-User-ID", bobID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("bob read alice's quote: want 403, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Check auth package exposes `SetCurrentUser` + `User{ID,Role}`**

Run: `grep -n "SetCurrentUser\|type User\|CurrentUser" apps/api/internal/auth/*.go | head -20`
Expected: both `SetCurrentUser(c, auth.User)` and `CurrentUser(c) auth.User` are exported with `.ID` and `.Role` fields. If `SetCurrentUser` doesn't exist yet, add it as a companion to `CurrentUser` — store the user in a well-known context key and expose a setter.

- [ ] **Step 3: Run handler test**

Run: `cd apps/api && go test ./internal/quotation/... -run TestHandler`
Expected: both tests pass.

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/quotation/handler_test.go
# plus any auth additions from Step 2:
git add apps/api/internal/auth/ 2>/dev/null || true
git commit -m "test(api): quotation handler integration tests (salesperson + reviewer)"
```

---

### Task 8: Full suite verification

- [ ] **Step 1: Run the entire backend test suite**

Run: `cd apps/api && go test ./...`
Expected: every package green — auth, catalog, customer, platform/audit, platform/config, platform/httpx, pricing, quotation, pkg/seeder.

- [ ] **Step 2: Lint / static analysis (if configured)**

Run: `cd apps/api && go vet ./...`
Expected: no findings.

- [ ] **Step 3: If anything drifted in go.sum during `go test`, revert it**

Run: `git diff apps/api/go.sum`
If shown drift is only about already-used packages, revert: `git checkout -- apps/api/go.sum`. If genuinely new deps appeared, include go.sum in the final commit.

---
