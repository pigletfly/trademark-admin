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
	byID    map[uuid.UUID]*Quotation
	history map[uuid.UUID][]StatusHistory
	nextErr error
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
