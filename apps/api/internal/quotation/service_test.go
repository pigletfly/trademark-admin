// apps/api/internal/quotation/service_test.go
package quotation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
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

// fakeCustomerRepo lets service tests control whether "customer exists"
// without touching Postgres. A nil entry for an id means "not found".
type fakeCustomerRepo struct {
	byID map[uuid.UUID]*customer.Customer
}

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

// fakePricingRepo always returns the configured entries regardless of
// country filter.
type fakePricingRepo struct {
	entries            []pricing.PricingEntry
	madridEntries      []pricing.MadridPricingEntry
	singleClassEntries []pricing.SingleClassPricingEntry
	err                error
}

func (f *fakePricingRepo) ListActive(ctx context.Context, _ *uuid.UUID) ([]pricing.PricingEntry, error) {
	return f.entries, f.err
}

func (f *fakePricingRepo) ListActiveMadrid(ctx context.Context, filter pricing.MadridActiveFilter) ([]pricing.MadridPricingEntry, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []pricing.MadridPricingEntry
	for _, entry := range f.madridEntries {
		if entry.EffectiveTo != nil {
			continue
		}
		if entry.IsBaseFee {
			if filter.IncludeBase {
				out = append(out, entry)
			}
			continue
		}
		if filter.CountryID != nil {
			if entry.CountryID != nil && *entry.CountryID == *filter.CountryID {
				out = append(out, entry)
			}
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

func (f *fakePricingRepo) ListActiveSingleClass(ctx context.Context, filter pricing.SingleClassActiveFilter) ([]pricing.SingleClassPricingEntry, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []pricing.SingleClassPricingEntry
	for _, entry := range f.singleClassEntries {
		if entry.EffectiveTo != nil {
			continue
		}
		if filter.CountryID != nil && entry.CountryID != *filter.CountryID {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

func newService(entries []pricing.PricingEntry) (*Service, *fakeRepo) {
	r := newFakeRepo()
	return NewService(r, &fakePricingRepo{entries: entries}, newFakeCustomerRepo()), r
}

// newServiceWithCustomer returns the fake customer repo too so tests
// that exercise customer-dependent service methods (Preview, Create's
// customer validation) can seed records.
func newServiceWithCustomer(entries []pricing.PricingEntry) (*Service, *fakeRepo, *fakeCustomerRepo) {
	r := newFakeRepo()
	custRepo := newFakeCustomerRepo()
	return NewService(r, &fakePricingRepo{entries: entries}, custRepo), r, custRepo
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

func TestCreate_PersistsExtendedFormFields(t *testing.T) {
	countryA := uuid.New()
	countryB := uuid.New()
	owner := uuid.New()
	svc, _ := newService(nil)

	q, err := svc.Create(context.Background(), owner, CreateRequest{
		CustomerID:          uuid.New(),
		CountryID:           countryA,
		CountryIDs:          []uuid.UUID{countryA, countryB},
		NiceCategoryCodes:   []int{9, 35},
		RegistrationMethods: []string{"madrid", "single"},
		AgentLevel:          "agent_b",
		ServiceTier:         "standard",
		InfoSections:        []string{"acceptance_time", "real_cases"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if q.CountryID != countryA {
		t.Fatalf("primary country = %s, want %s", q.CountryID, countryA)
	}
	assertUUIDJSONB(t, q.CountryIDs, []uuid.UUID{countryA, countryB})
	assertIntJSONB(t, q.NiceCategoryCodes, []int{9, 35})
	assertStringJSONB(t, q.RegistrationMethods, []string{"madrid", "single"})
	if q.AgentLevel != "agent_b" {
		t.Fatalf("agent level = %q, want agent_b", q.AgentLevel)
	}
	assertStringJSONB(t, q.InfoSections, []string{"acceptance_time", "real_cases"})
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

func assertUUIDJSONB(t *testing.T, raw []byte, want []uuid.UUID) {
	t.Helper()
	var got []uuid.UUID
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal uuid jsonb: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("uuid jsonb length = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("uuid jsonb[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func assertIntJSONB(t *testing.T, raw []byte, want []int) {
	t.Helper()
	var got []int
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal int jsonb: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("int jsonb length = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("int jsonb[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func assertStringJSONB(t *testing.T, raw []byte, want []string) {
	t.Helper()
	var got []string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal string jsonb: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("string jsonb length = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("string jsonb[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

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

func TestService_Preview_MultiCountryAggregates(t *testing.T) {
	custID := uuid.New()
	countryA := uuid.New()
	countryB := uuid.New()
	custRepo := newFakeCustomerRepo()
	custRepo.byID[custID] = &customer.Customer{ID: custID, Name: "Acme"}
	pricingRepo := &fakePricingRepo{entries: []pricing.PricingEntry{
		{ID: uuid.New(), CountryID: countryA, ServiceTier: "basic", FeeItem: "application", AmountCNYCents: 50000},
		{ID: uuid.New(), CountryID: countryB, ServiceTier: "basic", FeeItem: "application", AmountCNYCents: 70000},
	}}
	svc := NewService(newFakeRepo(), pricingRepo, custRepo)

	resp, err := svc.Preview(context.Background(), PreviewRequest{
		CustomerID:  custID,
		CountryID:   countryA,
		CountryIDs:  []uuid.UUID{countryA, countryB},
		ServiceTier: "basic",
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(resp.Lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(resp.Lines))
	}
	if resp.TotalCNYCents != 120000 {
		t.Fatalf("total = %d, want 120000", resp.TotalCNYCents)
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

func TestPreview_CarriesSourceIDs(t *testing.T) {
	country := uuid.New()
	caller := uuid.New()
	custID := uuid.New()
	from := time.Now().Add(-24 * time.Hour)
	entryA := pricing.PricingEntry{
		ID: uuid.New(), CountryID: country, ServiceTier: "basic",
		FeeItem: "fee_x", AmountCNYCents: 7000, EffectiveFrom: from, CreatedBy: caller,
	}
	svc, _, custRepo := newServiceWithCustomer([]pricing.PricingEntry{entryA})
	custRepo.byID[custID] = &customer.Customer{ID: custID}

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

func TestPreview_UsesSingleClassPricingForSingleRegistrationMethod(t *testing.T) {
	country := uuid.New()
	custID := uuid.New()
	sourceID := uuid.New()
	pricingRepo := &fakePricingRepo{
		singleClassEntries: []pricing.SingleClassPricingEntry{{
			ID:                             sourceID,
			CountryID:                      country,
			Continent:                      "Asia",
			CountryArea:                    "Singapore",
			FirstClassFeeCNYCents:          360000,
			FirstClassFeeTax6CNYCents:      381600,
			FirstClassFeeTax1CNYCents:      363600,
			AdditionalClassFeeCNYCents:     270000,
			AdditionalClassFeeTax6CNYCents: 286200,
			AdditionalClassFeeTax1CNYCents: 272700,
		}},
	}
	custRepo := newFakeCustomerRepo()
	custRepo.byID[custID] = &customer.Customer{ID: custID}
	svc := NewService(newFakeRepo(), pricingRepo, custRepo)

	resp, err := svc.Preview(context.Background(), PreviewRequest{
		CustomerID:          custID,
		CountryID:           country,
		NiceCategoryCodes:   []int{9, 35},
		RegistrationMethods: []string{"single"},
		ServiceTier:         "basic",
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if resp.TotalCNYCents != 630000 {
		t.Fatalf("total = %d, want 630000", resp.TotalCNYCents)
	}
	if len(resp.Lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(resp.Lines))
	}
	if resp.Lines[0].RegistrationMethod != "single" || resp.Lines[0].SourcePricingTable != pricing.SingleClassPricingTable {
		t.Fatalf("line source mismatch: %+v", resp.Lines[0])
	}
	if resp.Lines[0].SourcePricingID == nil || *resp.Lines[0].SourcePricingID != sourceID {
		t.Fatalf("line source id mismatch: %+v", resp.Lines[0])
	}
}

func TestSubmit_UsesMadridPricingForMadridRegistrationMethod(t *testing.T) {
	country := uuid.New()
	owner := uuid.New()
	baseID := uuid.New()
	countrySourceID := uuid.New()
	svc, _ := newService(nil)
	svc.pricingRepo = &fakePricingRepo{
		madridEntries: []pricing.MadridPricingEntry{
			{
				ID:                  baseID,
				CountryArea:         "Basic registration fee - black and white mark",
				OfficialFeeCHFCents: 65300,
				AgencyFeeCNYCents:   400000,
				IsBaseFee:           true,
			},
			{
				ID:                  countrySourceID,
				CountryID:           &country,
				CountryArea:         "Singapore",
				OfficialFeeCHFCents: 26100,
				AgencyFeeCNYCents:   40000,
			},
		},
	}
	q, err := svc.Create(context.Background(), owner, CreateRequest{
		CustomerID:          uuid.New(),
		CountryID:           country,
		NiceCategoryCodes:   []int{9},
		RegistrationMethods: []string{"madrid"},
		ServiceTier:         "basic",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	submitted, err := svc.Submit(context.Background(), q.ID, owner)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.TotalCNYCents == nil || *submitted.TotalCNYCents != 1244320 {
		t.Fatalf("total = %v, want 1244320", submitted.TotalCNYCents)
	}
	snap, err := submitted.DecodeSnapshot()
	if err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if len(snap.Lines) != 4 {
		t.Fatalf("lines = %d, want 4", len(snap.Lines))
	}
	if snap.Lines[2].RegistrationMethod != "madrid" || snap.Lines[2].SourcePricingTable != pricing.MadridPricingTable {
		t.Fatalf("line source mismatch: %+v", snap.Lines[2])
	}
	if snap.Lines[2].SourcePricingID == nil || *snap.Lines[2].SourcePricingID != countrySourceID {
		t.Fatalf("country source id mismatch: %+v", snap.Lines[2])
	}
}

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

	preservedID := uuid.New()
	adjusted, err := svc.Adjust(context.Background(), q.ID, reviewer, []SnapshotLine{
		{FeeItem: "preserved", AmountCNYCents: 1000, SourcePricingEntryID: &preservedID},
		{FeeItem: "orphan", AmountCNYCents: 2000},
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
