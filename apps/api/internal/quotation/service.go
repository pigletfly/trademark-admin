package quotation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"
	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
)

// Typed sentinel errors mapped to HTTP codes in the handler layer.
var (
	ErrInvalidTier       = errors.New("quotation: invalid service tier")
	ErrInvalidTransition = errors.New("quotation: invalid status transition")
	ErrNotOwner          = errors.New("quotation: not owner of quotation")
	ErrNotFound          = errors.New("quotation: not found")
	ErrMissingPricing    = errors.New("quotation: no active pricing entries for country+tier")
	ErrEmptyAdjust       = errors.New("quotation: adjust requires at least one line")
)

// repo is the subset of Repository methods Service depends on. Keeps the
// service testable with fakes when full DB isn't needed.
type repo interface {
	Create(ctx context.Context, q *Quotation) error
	Get(ctx context.Context, id uuid.UUID) (*Quotation, error)
	UpdateDraft(ctx context.Context, id uuid.UUID, patch map[string]any) error
	Transition(ctx context.Context, q *Quotation, to Status, actorID uuid.UUID, comment *string) error
	TransitionWithHistory(ctx context.Context, q *Quotation, to Status, actorID uuid.UUID, comment *string, diffJSON []byte) error
	Withdraw(ctx context.Context, q *Quotation, actorID uuid.UUID) error
	SubmitWithSerial(ctx context.Context, q *Quotation, actorID uuid.UUID, now time.Time) error
	List(ctx context.Context, f ListFilter) ([]Quotation, int64, error)
	History(ctx context.Context, id uuid.UUID) ([]StatusHistory, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// pricingRepo is the subset of pricing.Repository we need. Injected so
// the service can compute the submit-time snapshot.
type pricingRepo interface {
	ListActive(ctx context.Context, countryID *uuid.UUID) ([]pricing.PricingEntry, error)
}

// customerRepo is the subset of customer.Repository we need for the
// Preview endpoint. Get with ownerID=nil is an existence check (returns
// customer.ErrNotFound if the id isn't found). We depend on the
// interface rather than the concrete type to keep the service testable
// with fakes.
type customerRepo interface {
	Get(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*customer.Customer, error)
}

// Service owns quotation business rules. Role enforcement lives in the
// handler/middleware layer; Service assumes the caller has the right
// role and only checks *ownership* within that role (e.g. a salesperson
// may only edit their own drafts).
type Service struct {
	repo         repo
	pricingRepo  pricingRepo
	customerRepo customerRepo
}

func NewService(r repo, p pricingRepo, c customerRepo) *Service {
	return &Service{repo: r, pricingRepo: p, customerRepo: c}
}

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
		sourceID := l.SourcePricingEntryID
		snap.Lines = append(snap.Lines, SnapshotLine{
			FeeItem:              l.FeeItem,
			AmountCNYCents:       l.AmountCNYCents,
			SourcePricingEntryID: &sourceID,
		})
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

	if err := s.repo.SubmitWithSerial(ctx, q, actorID, now); err != nil {
		return nil, err
	}
	q.Status = StatusSubmitted
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
	q.Status = target
	return q, nil
}

// Withdraw returns a submitted quotation to draft. Owner-only; reviewers
// should Reject rather than withdraw. Clears snapshot/total/signature to
// satisfy chk_quotations_snapshot_when_nondraft for the draft state.
// serial_no is preserved — a future Submit will reuse or overwrite it.
func (s *Service) Withdraw(ctx context.Context, id, actorID uuid.UUID) (*Quotation, error) {
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
	if q.Status != StatusSubmitted {
		return nil, ErrInvalidTransition
	}
	if err := s.repo.Withdraw(ctx, q, actorID); err != nil {
		return nil, err
	}
	q.Status = StatusDraft
	q.SnapshotJSON = nil
	q.TotalCNYCents = nil
	q.Signature = nil
	return q, nil
}

// Copy clones a quotation's input fields (customer, country, tier, notes)
// into a fresh draft owned by actor. The new draft has no snapshot/total/
// signature — those will be computed when the new draft is itself Submitted
// (against pricing entries that may have changed since the source was
// submitted). serial_no is NOT copied; a draft has no serial. The source's
// status is irrelevant: drafts, submitted, approved, rejected, and
// cancelled are all copyable. Visibility of the source is enforced by the
// handler layer; the service performs no ownership check on the actor.
func (s *Service) Copy(ctx context.Context, sourceID, actorID uuid.UUID) (*Quotation, error) {
	src, err := s.repo.Get(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, ErrNotFound
	}
	q := &Quotation{
		CustomerID:  src.CustomerID,
		CountryID:   src.CountryID,
		ServiceTier: src.ServiceTier,
		Status:      StatusDraft,
		Notes:       src.Notes,
		CreatedBy:   actorID,
	}
	if err := s.repo.Create(ctx, q); err != nil {
		return nil, err
	}
	return q, nil
}

// Adjust mutates a submitted quotation's snapshot in place. Role-gated
// by the router (reviewer/admin); Service enforces only the status
// predicate — ownership is deliberately NOT checked because reviewers
// need to edit other people's submissions.
//
// The quotation stays in `submitted` status; the status_history row
// records from=submitted,to=submitted with a non-null diff_json
// distinguishing it from plain submit. The guarded UPDATE in
// transitionInTx (WHERE id = ? AND status = ?) still fires because
// from == to == submitted, making it a snapshot-rewrite transition.
//
// Lines come straight from the caller — Adjust does NOT run through
// pricing.Calculate (the reviewer is overriding whatever pricing
// produced), so it computes its own total and a signature via
// computeAdjustSignature over the canonical sorted form.
func (s *Service) Adjust(
	ctx context.Context,
	id, actorID uuid.UUID,
	lines []SnapshotLine,
	comment *string,
) (*Quotation, error) {
	if len(lines) == 0 {
		return nil, ErrEmptyAdjust
	}
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
	prevSnap, err := q.DecodeSnapshot()
	if err != nil {
		return nil, fmt.Errorf("quotation: decode prior snapshot: %w", err)
	}

	var total int64
	for _, l := range lines {
		total += l.AmountCNYCents
	}
	sig := computeAdjustSignature(lines)
	nextSnap := Snapshot{Lines: lines, TotalCNYCents: total, Signature: sig}

	diff := computeSnapshotDiff(prevSnap, nextSnap)
	diffJSON, err := json.Marshal(diff)
	if err != nil {
		return nil, fmt.Errorf("quotation: marshal diff: %w", err)
	}
	nextSnapJSON, err := json.Marshal(nextSnap)
	if err != nil {
		return nil, fmt.Errorf("quotation: marshal snapshot: %w", err)
	}

	q.SnapshotJSON = audit.JSONB(nextSnapJSON)
	q.TotalCNYCents = &total
	q.Signature = &sig
	if err := s.repo.TransitionWithHistory(ctx, q, StatusSubmitted, actorID, comment, diffJSON); err != nil {
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
		sourceID := l.SourcePricingEntryID
		lines = append(lines, SnapshotLine{
			FeeItem:              l.FeeItem,
			AmountCNYCents:       l.AmountCNYCents,
			SourcePricingEntryID: &sourceID,
		})
	}
	return &PreviewResponse{
		Lines:         lines,
		TotalCNYCents: calc.TotalCNYCents,
		Signature:     calc.Signature,
	}, nil
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
		SerialNo: q.SerialNo,
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
