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
	ErrInvalidFormInput  = errors.New("quotation: invalid extended form input")
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
	ListActiveMadrid(ctx context.Context, f pricing.MadridActiveFilter) ([]pricing.MadridPricingEntry, error)
	ListActiveSingleClass(ctx context.Context, f pricing.SingleClassActiveFilter) ([]pricing.SingleClassPricingEntry, error)
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
	selection, err := normalizeMethodCountrySelection(
		req.CountryID,
		req.CountryIDs,
		req.RegistrationMethods,
		req.MadridCountryIDs,
		req.SingleCountryIDs,
	)
	if err != nil {
		return nil, err
	}
	countryIDs := selection.countryIDs()
	niceCodes, err := normalizeNiceCategoryCodes(req.NiceCategoryCodes)
	if err != nil {
		return nil, err
	}
	registrationMethods := selection.registrationMethods()
	agentLevel, err := normalizeAgentLevel(req.AgentLevel)
	if err != nil {
		return nil, err
	}
	infoSections, err := normalizeInfoSections(req.InfoSections)
	if err != nil {
		return nil, err
	}
	countryIDsJSON, err := encodeJSONB(countryIDs)
	if err != nil {
		return nil, fmt.Errorf("quotation: marshal country ids: %w", err)
	}
	madridCountryIDsJSON, err := encodeJSONB(selection.madrid)
	if err != nil {
		return nil, fmt.Errorf("quotation: marshal madrid country ids: %w", err)
	}
	singleCountryIDsJSON, err := encodeJSONB(selection.single)
	if err != nil {
		return nil, fmt.Errorf("quotation: marshal single country ids: %w", err)
	}
	niceCodesJSON, err := encodeJSONB(niceCodes)
	if err != nil {
		return nil, fmt.Errorf("quotation: marshal nice category codes: %w", err)
	}
	registrationMethodsJSON, err := encodeJSONB(registrationMethods)
	if err != nil {
		return nil, fmt.Errorf("quotation: marshal registration methods: %w", err)
	}
	infoSectionsJSON, err := encodeJSONB(infoSections)
	if err != nil {
		return nil, fmt.Errorf("quotation: marshal info sections: %w", err)
	}
	q := &Quotation{
		ID:                  uuid.New(),
		CustomerID:          req.CustomerID,
		CountryID:           countryIDs[0],
		CountryIDs:          countryIDsJSON,
		MadridCountryIDs:    madridCountryIDsJSON,
		SingleCountryIDs:    singleCountryIDsJSON,
		NiceCategoryCodes:   niceCodesJSON,
		RegistrationMethods: registrationMethodsJSON,
		AgentLevel:          agentLevel,
		ServiceTier:         req.ServiceTier,
		Status:              StatusDraft,
		InfoSections:        infoSectionsJSON,
		Notes:               req.Notes,
		CreatedBy:           ownerID,
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
	selectionChanged :=
		req.CountryID != nil ||
			req.CountryIDs != nil ||
			req.RegistrationMethods != nil ||
			req.MadridCountryIDs != nil ||
			req.SingleCountryIDs != nil
	if selectionChanged {
		selection := quotationMethodCountrySelection(q)
		if req.CountryID != nil || req.CountryIDs != nil || req.RegistrationMethods != nil {
			primary := q.CountryID
			if req.CountryID != nil {
				primary = *req.CountryID
			}
			legacyCountryIDs := quotationCountryIDs(q)
			if req.CountryID != nil {
				legacyCountryIDs = []uuid.UUID{*req.CountryID}
			}
			if req.CountryIDs != nil {
				legacyCountryIDs = *req.CountryIDs
			}
			methods := quotationRegistrationMethods(q)
			if req.RegistrationMethods != nil {
				methods = *req.RegistrationMethods
			}
			var err error
			selection, err = normalizeMethodCountrySelection(primary, legacyCountryIDs, methods, nil, nil)
			if err != nil {
				return err
			}
		}
		if req.MadridCountryIDs != nil {
			madridIDs, err := normalizeUUIDList(*req.MadridCountryIDs)
			if err != nil {
				return err
			}
			selection.madrid = madridIDs
		}
		if req.SingleCountryIDs != nil {
			singleIDs, err := normalizeUUIDList(*req.SingleCountryIDs)
			if err != nil {
				return err
			}
			selection.single = singleIDs
		}

		countryIDs := selection.countryIDs()
		if len(countryIDs) == 0 {
			return ErrInvalidFormInput
		}
		countryIDsJSON, err := encodeJSONB(countryIDs)
		if err != nil {
			return fmt.Errorf("quotation: marshal country ids: %w", err)
		}
		madridCountryIDsJSON, err := encodeJSONB(selection.madrid)
		if err != nil {
			return fmt.Errorf("quotation: marshal madrid country ids: %w", err)
		}
		singleCountryIDsJSON, err := encodeJSONB(selection.single)
		if err != nil {
			return fmt.Errorf("quotation: marshal single country ids: %w", err)
		}
		registrationMethodsJSON, err := encodeJSONB(selection.registrationMethods())
		if err != nil {
			return fmt.Errorf("quotation: marshal registration methods: %w", err)
		}
		patch["country_id"] = countryIDs[0]
		patch["country_ids"] = countryIDsJSON
		patch["madrid_country_ids"] = madridCountryIDsJSON
		patch["single_country_ids"] = singleCountryIDsJSON
		patch["registration_methods"] = registrationMethodsJSON
	}
	if req.ServiceTier != nil {
		if !pricing.IsValidServiceTier(*req.ServiceTier) {
			return ErrInvalidTier
		}
		patch["service_tier"] = *req.ServiceTier
	}
	if req.NiceCategoryCodes != nil {
		codes, err := normalizeNiceCategoryCodes(*req.NiceCategoryCodes)
		if err != nil {
			return err
		}
		raw, err := encodeJSONB(codes)
		if err != nil {
			return fmt.Errorf("quotation: marshal nice category codes: %w", err)
		}
		patch["nice_category_codes"] = raw
	}
	if req.AgentLevel != nil {
		agentLevel, err := normalizeAgentLevel(*req.AgentLevel)
		if err != nil {
			return err
		}
		patch["agent_level"] = agentLevel
	}
	if req.InfoSections != nil {
		sections, err := normalizeInfoSections(*req.InfoSections)
		if err != nil {
			return err
		}
		raw, err := encodeJSONB(sections)
		if err != nil {
			return fmt.Errorf("quotation: marshal info sections: %w", err)
		}
		patch["info_sections"] = raw
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

	snap, err := s.calculateSnapshot(
		ctx,
		quotationMethodCountrySelection(q),
		q.ServiceTier,
		len(quotationNiceCategoryCodes(q)),
	)
	if err != nil {
		if errors.Is(err, pricing.ErrNoMatchingEntries) {
			return nil, ErrMissingPricing
		}
		return nil, err
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return nil, fmt.Errorf("quotation: marshal snapshot: %w", err)
	}
	now := time.Now()
	q.SnapshotJSON = audit.JSONB(raw)
	q.TotalCNYCents = &snap.TotalCNYCents
	sig := snap.Signature
	q.Signature = &sig
	q.SubmittedAt = &now

	if err := s.repo.SubmitWithSerial(ctx, q, actorID, now); err != nil {
		return nil, err
	}
	q.Status = StatusSubmitted
	return q, nil
}

func (s *Service) calculateSnapshot(
	ctx context.Context,
	selection methodCountrySelection,
	serviceTier string,
	niceCategoryCount int,
) (Snapshot, error) {
	var snap Snapshot
	countryIDs := selection.countryIDs()
	if len(countryIDs) == 0 {
		return snap, ErrInvalidFormInput
	}
	var madridSet pricing.MethodPricingSet
	var madridMethodCountries []uuid.UUID
	var madridLegacyCountries []uuid.UUID

	if len(selection.madrid) > 0 {
		var err error
		madridSet, err = s.loadMethodPricing(ctx, selection.madrid, []string{pricing.RegistrationMethodMadrid})
		if err != nil {
			return snap, err
		}
		madridMethodCountries, madridLegacyCountries = splitMadridCoverage(madridSet.Madrid, selection.madrid)
	}

	var singleSet pricing.MethodPricingSet
	var singleMethodCountries []uuid.UUID
	var singleLegacyCountries []uuid.UUID
	if len(selection.single) > 0 {
		var err error
		singleSet, err = s.loadMethodPricing(ctx, selection.single, []string{pricing.RegistrationMethodSingle})
		if err != nil {
			return snap, err
		}
		singleMethodCountries, singleLegacyCountries = splitSingleClassCoverage(singleSet.SingleClass, selection.single)
	}

	if len(madridMethodCountries) > 0 {
		calc, err := pricing.CalculateMethodPricing(madridSet, pricing.MethodCalcInput{
			CountryIDs:          madridMethodCountries,
			RegistrationMethods: []string{pricing.RegistrationMethodMadrid},
			NiceCategoryCount:   niceCategoryCount,
		})
		if err != nil {
			if errors.Is(err, pricing.ErrNoMatchingEntries) {
				return snap, ErrMissingPricing
			}
			return snap, fmt.Errorf("quotation: madrid pricing calculate: %w", err)
		}
		for _, line := range calc.Lines {
			snap.Lines = append(snap.Lines, lineToSnapshotLine(line))
		}
		snap.TotalCNYCents += calc.TotalCNYCents
	}
	if len(singleMethodCountries) > 0 {
		calc, err := pricing.CalculateMethodPricing(singleSet, pricing.MethodCalcInput{
			CountryIDs:          singleMethodCountries,
			RegistrationMethods: []string{pricing.RegistrationMethodSingle},
			NiceCategoryCount:   niceCategoryCount,
		})
		if err != nil {
			if errors.Is(err, pricing.ErrNoMatchingEntries) {
				return snap, ErrMissingPricing
			}
			return snap, fmt.Errorf("quotation: single pricing calculate: %w", err)
		}
		for _, line := range calc.Lines {
			snap.Lines = append(snap.Lines, lineToSnapshotLine(line))
		}
		snap.TotalCNYCents += calc.TotalCNYCents
	}

	sharedLegacyCountries := intersectCountryIDs(madridLegacyCountries, singleLegacyCountries)
	if err := s.appendLegacySnapshotLines(ctx, &snap, sharedLegacyCountries, serviceTier, ""); err != nil {
		return snap, err
	}
	if err := s.appendLegacySnapshotLines(
		ctx,
		&snap,
		excludeCountryIDs(madridLegacyCountries, sharedLegacyCountries),
		serviceTier,
		pricing.RegistrationMethodMadrid,
	); err != nil {
		return snap, err
	}
	if err := s.appendLegacySnapshotLines(
		ctx,
		&snap,
		excludeCountryIDs(singleLegacyCountries, sharedLegacyCountries),
		serviceTier,
		pricing.RegistrationMethodSingle,
	); err != nil {
		return snap, err
	}

	if len(snap.Lines) == 0 {
		return snap, ErrMissingPricing
	}
	snap.Signature = computeMethodSnapshotSignature(niceCategoryCount, snap.Lines, snap.TotalCNYCents)
	return snap, nil
}

func splitSingleClassCoverage(entries []pricing.SingleClassPricingEntry, countryIDs []uuid.UUID) ([]uuid.UUID, []uuid.UUID) {
	activeByCountry := make(map[uuid.UUID]struct{}, len(entries))
	for _, entry := range entries {
		if entry.EffectiveTo != nil {
			continue
		}
		activeByCountry[entry.CountryID] = struct{}{}
	}
	return splitCountriesByCoverage(countryIDs, func(countryID uuid.UUID) bool {
		_, ok := activeByCountry[countryID]
		return ok
	})
}

func splitMadridCoverage(entries []pricing.MadridPricingEntry, countryIDs []uuid.UUID) ([]uuid.UUID, []uuid.UUID) {
	hasBase := false
	activeByCountry := make(map[uuid.UUID]struct{}, len(entries))
	for _, entry := range entries {
		if entry.EffectiveTo != nil {
			continue
		}
		if entry.IsBaseFee {
			hasBase = true
			continue
		}
		if entry.CountryID != nil {
			activeByCountry[*entry.CountryID] = struct{}{}
		}
	}
	return splitCountriesByCoverage(countryIDs, func(countryID uuid.UUID) bool {
		if !hasBase {
			return false
		}
		_, ok := activeByCountry[countryID]
		return ok
	})
}

func splitCountriesByCoverage(
	countryIDs []uuid.UUID,
	hasCoverage func(uuid.UUID) bool,
) ([]uuid.UUID, []uuid.UUID) {
	methodCountries := make([]uuid.UUID, 0, len(countryIDs))
	legacyCountries := make([]uuid.UUID, 0, len(countryIDs))
	for _, countryID := range countryIDs {
		if hasCoverage(countryID) {
			methodCountries = append(methodCountries, countryID)
			continue
		}
		legacyCountries = append(legacyCountries, countryID)
	}
	return methodCountries, legacyCountries
}

func intersectCountryIDs(left, right []uuid.UUID) []uuid.UUID {
	rightSet := make(map[uuid.UUID]struct{}, len(right))
	for _, countryID := range right {
		rightSet[countryID] = struct{}{}
	}
	out := make([]uuid.UUID, 0)
	seen := make(map[uuid.UUID]struct{}, len(left))
	for _, countryID := range left {
		if _, ok := rightSet[countryID]; !ok {
			continue
		}
		if _, ok := seen[countryID]; ok {
			continue
		}
		seen[countryID] = struct{}{}
		out = append(out, countryID)
	}
	return out
}

func excludeCountryIDs(countryIDs, excluded []uuid.UUID) []uuid.UUID {
	excludedSet := make(map[uuid.UUID]struct{}, len(excluded))
	for _, countryID := range excluded {
		excludedSet[countryID] = struct{}{}
	}
	out := make([]uuid.UUID, 0, len(countryIDs))
	for _, countryID := range countryIDs {
		if _, ok := excludedSet[countryID]; ok {
			continue
		}
		out = append(out, countryID)
	}
	return out
}

func (s *Service) appendLegacySnapshotLines(
	ctx context.Context,
	snap *Snapshot,
	countryIDs []uuid.UUID,
	serviceTier string,
	registrationMethod string,
) error {
	for _, countryID := range countryIDs {
		entries, err := s.pricingRepo.ListActive(ctx, &countryID)
		if err != nil {
			return err
		}
		calc, err := pricing.Calculate(entries, pricing.CalcInput{
			CountryID:   countryID,
			ServiceTier: serviceTier,
		})
		if err != nil {
			if errors.Is(err, pricing.ErrNoMatchingEntries) {
				return ErrMissingPricing
			}
			return fmt.Errorf("quotation: pricing calculate: %w", err)
		}
		for _, line := range calc.Lines {
			snap.Lines = append(snap.Lines, legacyLineToSnapshotLine(line, countryID, registrationMethod))
		}
		snap.TotalCNYCents += calc.TotalCNYCents
	}
	return nil
}

func (s *Service) loadMethodPricing(ctx context.Context, countryIDs []uuid.UUID, registrationMethods []string) (pricing.MethodPricingSet, error) {
	var set pricing.MethodPricingSet
	methods, err := normalizeRegistrationMethods(registrationMethods)
	if err != nil {
		return set, err
	}
	for _, method := range methods {
		switch method {
		case pricing.RegistrationMethodMadrid:
			for _, countryID := range countryIDs {
				rows, err := s.pricingRepo.ListActiveMadrid(ctx, pricing.MadridActiveFilter{
					CountryID:   &countryID,
					IncludeBase: true,
				})
				if err != nil {
					return set, err
				}
				set.Madrid = append(set.Madrid, rows...)
			}
		case pricing.RegistrationMethodSingle:
			for _, countryID := range countryIDs {
				rows, err := s.pricingRepo.ListActiveSingleClass(ctx, pricing.SingleClassActiveFilter{
					CountryID: &countryID,
				})
				if err != nil {
					return set, err
				}
				set.SingleClass = append(set.SingleClass, rows...)
			}
		}
	}
	return set, nil
}

func lineToSnapshotLine(line pricing.CalcLine) SnapshotLine {
	return SnapshotLine{
		FeeItem:             line.FeeItem,
		AmountCNYCents:      line.AmountCNYCents,
		SourcePricingTable:  line.SourcePricingTable,
		SourcePricingID:     line.SourcePricingID,
		RegistrationMethod:  line.RegistrationMethod,
		CountryID:           line.CountryID,
		CountryArea:         line.CountryArea,
		Quantity:            line.Quantity,
		UnitAmountCNYCents:  line.UnitAmountCNYCents,
		OfficialFeeCHFCents: line.OfficialFeeCHFCents,
	}
}

func legacyLineToSnapshotLine(line pricing.CalcLine, countryID uuid.UUID, registrationMethod string) SnapshotLine {
	sourceID := line.SourcePricingEntryID
	cid := countryID
	return SnapshotLine{
		FeeItem:              line.FeeItem,
		AmountCNYCents:       line.AmountCNYCents,
		SourcePricingEntryID: &sourceID,
		RegistrationMethod:   registrationMethod,
		CountryID:            &cid,
	}
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
		CustomerID:          src.CustomerID,
		CountryID:           src.CountryID,
		CountryIDs:          src.CountryIDs,
		MadridCountryIDs:    src.MadridCountryIDs,
		SingleCountryIDs:    src.SingleCountryIDs,
		NiceCategoryCodes:   src.NiceCategoryCodes,
		RegistrationMethods: src.RegistrationMethods,
		AgentLevel:          src.AgentLevel,
		ServiceTier:         src.ServiceTier,
		Status:              StatusDraft,
		InfoSections:        src.InfoSections,
		Notes:               src.Notes,
		CreatedBy:           actorID,
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
	niceCodes, err := normalizeNiceCategoryCodes(req.NiceCategoryCodes)
	if err != nil {
		return nil, err
	}
	selection, err := normalizeMethodCountrySelection(
		req.CountryID,
		req.CountryIDs,
		req.RegistrationMethods,
		req.MadridCountryIDs,
		req.SingleCountryIDs,
	)
	if err != nil {
		return nil, err
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

	snap, err := s.calculateSnapshot(ctx, selection, req.ServiceTier, len(niceCodes))
	if err != nil {
		if errors.Is(err, ErrMissingPricing) {
			return nil, ErrMissingPricing
		}
		return nil, err
	}

	return &PreviewResponse{
		Lines:         snap.Lines,
		TotalCNYCents: snap.TotalCNYCents,
		Signature:     snap.Signature,
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
		CountryIDs:          quotationCountryIDs(q),
		MadridCountryIDs:    quotationMadridCountryIDs(q),
		SingleCountryIDs:    quotationSingleCountryIDs(q),
		NiceCategoryCodes:   decodeJSONB[int](q.NiceCategoryCodes),
		RegistrationMethods: quotationRegistrationMethods(q),
		AgentLevel:          q.AgentLevel,
		ServiceTier:         q.ServiceTier, Status: q.Status,
		TotalCNYCents: q.TotalCNYCents, Signature: q.Signature,
		SerialNo:    q.SerialNo,
		SubmittedAt: q.SubmittedAt, ReviewedAt: q.ReviewedAt,
		ReviewedBy: q.ReviewedBy, ReviewComment: q.ReviewComment,
		InfoSections: decodeJSONB[string](q.InfoSections),
		Notes:        q.Notes, CreatedBy: q.CreatedBy,
		CreatedAt: q.CreatedAt, UpdatedAt: q.UpdatedAt,
	}
	if len(out.CountryIDs) == 0 && q.CountryID != uuid.Nil {
		out.CountryIDs = []uuid.UUID{q.CountryID}
	}
	if out.AgentLevel == "" {
		out.AgentLevel = agentLevelFromServiceTier(q.ServiceTier)
	}
	if len(q.SnapshotJSON) > 0 {
		var snap Snapshot
		if err := json.Unmarshal(q.SnapshotJSON, &snap); err == nil {
			out.Snapshot = &snap
		}
	}
	return out
}
