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
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
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
func (f *fakeRepo) TransitionWithHistory(ctx context.Context, q *Quotation, to Status, actorID uuid.UUID, comment *string, diffJSON []byte) error {
	return f.Transition(ctx, q, to, actorID, comment)
}
func (f *fakeRepo) Withdraw(ctx context.Context, q *Quotation, actorID uuid.UUID) error {
	if f.nextErr != nil {
		e := f.nextErr
		f.nextErr = nil
		return e
	}
	stored := f.byID[q.ID]
	if stored == nil {
		return ErrInvalidTransition
	}
	if stored.Status != StatusSubmitted {
		return ErrInvalidTransition
	}
	stored.Status = StatusDraft
	stored.SnapshotJSON = nil
	stored.TotalCNYCents = nil
	stored.Signature = nil
	f.history[q.ID] = append(f.history[q.ID], StatusHistory{
		ID: uuid.New(), QuotationID: q.ID,
		FromStatus: StatusSubmitted, ToStatus: StatusDraft, ActorID: &actorID,
		At: time.Now(),
	})
	return nil
}
func (f *fakeRepo) SubmitWithSerial(ctx context.Context, q *Quotation, actorID uuid.UUID, now time.Time) error {
	serial := "Q" + now.UTC().Format("20060102") + "0001"
	q.SerialNo = &serial
	return f.Transition(ctx, q, StatusSubmitted, actorID, nil)
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

// TestService_Withdraw_ErrorPaths exercises the three non-happy branches
// of Service.Withdraw: not-found, wrong-owner, wrong-status. The repo
// happy path is covered by TestRepository_WithdrawClearsSnapshotAndKeepsSerial.
func TestService_Withdraw_ErrorPaths(t *testing.T) {
	country := uuid.New()
	entries := []pricing.PricingEntry{
		{ID: uuid.New(), CountryID: country, ServiceTier: "basic", FeeItem: "f", AmountCNYCents: 1, EffectiveFrom: time.Now(), CreatedBy: uuid.New()},
	}
	owner := uuid.New()

	t.Run("not found", func(t *testing.T) {
		svc, _ := newService(entries)
		if _, err := svc.Withdraw(context.Background(), uuid.New(), owner); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("not owner", func(t *testing.T) {
		svc, _ := newService(entries)
		q, _ := svc.Create(context.Background(), owner, CreateRequest{
			CustomerID: uuid.New(), CountryID: country, ServiceTier: "basic",
		})
		if _, err := svc.Submit(context.Background(), q.ID, owner); err != nil {
			t.Fatalf("submit: %v", err)
		}
		intruder := uuid.New()
		if _, err := svc.Withdraw(context.Background(), q.ID, intruder); !errors.Is(err, ErrNotOwner) {
			t.Fatalf("want ErrNotOwner, got %v", err)
		}
	})

	t.Run("wrong status", func(t *testing.T) {
		svc, _ := newService(entries)
		q, _ := svc.Create(context.Background(), owner, CreateRequest{
			CustomerID: uuid.New(), CountryID: country, ServiceTier: "basic",
		})
		// Still a draft — withdrawing must be rejected.
		if _, err := svc.Withdraw(context.Background(), q.ID, owner); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("want ErrInvalidTransition, got %v", err)
		}
	})
}

// TestService_Copy_FreshDraftOwnedByActor verifies Copy clones a
// quotation's *input* fields into a new draft owned by the actor, with
// all output/review fields left zero. Status of the source is irrelevant
// (drafts, submitted, approved, rejected, cancelled are all copyable).
func TestService_Copy_FreshDraftOwnedByActor(t *testing.T) {
	t.Run("happy path clones input fields only", func(t *testing.T) {
		svc, repo := newService(nil)
		userA := uuid.New()
		userB := uuid.New()
		customerID := uuid.New()
		countryID := uuid.New()
		note := "src note"
		serial := "Q202604260001"
		total := int64(12345)
		sig := "sig-abc"
		submittedAt := time.Now().Add(-2 * time.Hour)
		reviewedAt := time.Now().Add(-1 * time.Hour)
		reviewer := uuid.New()
		reviewComment := "looks good"

		src := &Quotation{
			ID:            uuid.New(),
			CustomerID:    customerID,
			CountryID:     countryID,
			ServiceTier:   "basic",
			Status:        StatusSubmitted,
			SnapshotJSON:  []byte(`{"lines":[{"fee_item":"application","amount_cny_cents":10000}],"total_cny_cents":10000,"signature":"sig-abc"}`),
			TotalCNYCents: &total,
			Signature:     &sig,
			SerialNo:      &serial,
			SubmittedAt:   &submittedAt,
			ReviewedAt:    &reviewedAt,
			ReviewedBy:    &reviewer,
			ReviewComment: &reviewComment,
			Notes:         &note,
			CreatedBy:     userA,
		}
		repo.byID[src.ID] = src

		got, err := svc.Copy(context.Background(), src.ID, userB)
		if err != nil {
			t.Fatalf("copy: %v", err)
		}

		// Cloned input fields.
		if got.CustomerID != customerID {
			t.Errorf("CustomerID: got %v want %v", got.CustomerID, customerID)
		}
		if got.CountryID != countryID {
			t.Errorf("CountryID: got %v want %v", got.CountryID, countryID)
		}
		if got.ServiceTier != "basic" {
			t.Errorf("ServiceTier: got %q want basic", got.ServiceTier)
		}
		if got.Notes == nil || *got.Notes != "src note" {
			t.Errorf("Notes: got %v want &\"src note\"", got.Notes)
		}

		// Actor owns the copy, not the source's owner.
		if got.CreatedBy != userB {
			t.Errorf("CreatedBy: got %v want %v (actor)", got.CreatedBy, userB)
		}
		if got.Status != StatusDraft {
			t.Errorf("Status: got %q want draft", got.Status)
		}

		// Output/review/serial fields are zero.
		if len(got.SnapshotJSON) != 0 {
			t.Errorf("SnapshotJSON should be empty, got %s", string(got.SnapshotJSON))
		}
		if got.TotalCNYCents != nil {
			t.Errorf("TotalCNYCents should be nil, got %v", *got.TotalCNYCents)
		}
		if got.Signature != nil {
			t.Errorf("Signature should be nil, got %v", *got.Signature)
		}
		if got.SerialNo != nil {
			t.Errorf("SerialNo should be nil, got %v", *got.SerialNo)
		}
		if got.SubmittedAt != nil {
			t.Errorf("SubmittedAt should be nil, got %v", *got.SubmittedAt)
		}
		if got.ReviewedAt != nil {
			t.Errorf("ReviewedAt should be nil, got %v", *got.ReviewedAt)
		}
		if got.ReviewedBy != nil {
			t.Errorf("ReviewedBy should be nil, got %v", *got.ReviewedBy)
		}
		if got.ReviewComment != nil {
			t.Errorf("ReviewComment should be nil, got %v", *got.ReviewComment)
		}

		// ID is stamped by repo.Create and distinct from source.
		if got.ID == uuid.Nil {
			t.Error("ID should be set by repo.Create")
		}
		if got.ID == src.ID {
			t.Error("copy ID must differ from source ID")
		}

		// Source was not mutated (defensive check).
		if src.CreatedBy != userA {
			t.Errorf("source CreatedBy mutated: got %v want %v", src.CreatedBy, userA)
		}
		if src.Status != StatusSubmitted {
			t.Errorf("source Status mutated: got %q want submitted", src.Status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc, _ := newService(nil)
		if _, err := svc.Copy(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}
