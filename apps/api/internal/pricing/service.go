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

// ListActiveMadrid delegates to the repository.
func (s *Service) ListActiveMadrid(ctx context.Context, f MadridActiveFilter) ([]MadridPricingEntryDTO, error) {
	rows, err := s.repo.ListActiveMadrid(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]MadridPricingEntryDTO, len(rows))
	for i, r := range rows {
		out[i] = toMadridDTO(r)
	}
	return out, nil
}

// ListActiveSingleClass delegates to the repository.
func (s *Service) ListActiveSingleClass(ctx context.Context, f SingleClassActiveFilter) ([]SingleClassPricingEntryDTO, error) {
	rows, err := s.repo.ListActiveSingleClass(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]SingleClassPricingEntryDTO, len(rows))
	for i, r := range rows {
		out[i] = toSingleClassDTO(r)
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

// CreateOrReplaceMadrid validates the request and delegates to repo.
func (s *Service) CreateOrReplaceMadrid(ctx context.Context, callerID uuid.UUID, req CreateOrReplaceMadridRequest) (*MadridPricingEntryDTO, error) {
	if req.CountryArea == "" {
		return nil, errors.New("pricing: country_area required")
	}
	if req.OfficialFeeCHFCents < 0 || req.AgencyFeeCNYCents < 0 {
		return nil, errors.New("pricing: amount fields must be >= 0")
	}
	eff, err := parseEffectiveFrom(req.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.ReplaceActiveMadrid(ctx, NewMadridEntry{
		CountryID:           req.CountryID,
		SequenceNo:          req.SequenceNo,
		CountryArea:         req.CountryArea,
		OfficialFeeCHFCents: req.OfficialFeeCHFCents,
		AgencyFeeCNYCents:   req.AgencyFeeCNYCents,
		IsBaseFee:           req.IsBaseFee,
		Notes:               req.Notes,
		EffectiveFrom:       eff,
		CreatedBy:           callerID,
	})
	if err != nil {
		return nil, err
	}
	dto := toMadridDTO(*row)
	return &dto, nil
}

// CreateOrReplaceSingleClass validates the request and delegates to repo.
func (s *Service) CreateOrReplaceSingleClass(ctx context.Context, callerID uuid.UUID, req CreateOrReplaceSingleClassRequest) (*SingleClassPricingEntryDTO, error) {
	if req.CountryID == uuid.Nil {
		return nil, errors.New("pricing: country_id required")
	}
	if req.Continent == "" || req.CountryArea == "" {
		return nil, errors.New("pricing: continent and country_area required")
	}
	if req.FirstClassFeeCNYCents < 0 || req.AdditionalClassFeeCNYCents < 0 {
		return nil, errors.New("pricing: amount fields must be >= 0")
	}
	eff, err := parseEffectiveFrom(req.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	firstClassTax6, firstClassTax1 := deriveSingleClassTaxedCents(req.FirstClassFeeCNYCents)
	additionalClassTax6, additionalClassTax1 := deriveSingleClassTaxedCents(req.AdditionalClassFeeCNYCents)
	row, err := s.repo.ReplaceActiveSingleClass(ctx, NewSingleClassEntry{
		CountryID:                      req.CountryID,
		Continent:                      req.Continent,
		CountryArea:                    req.CountryArea,
		FirstClassFeeCNYCents:          req.FirstClassFeeCNYCents,
		FirstClassFeeTax6CNYCents:      firstClassTax6,
		FirstClassFeeTax1CNYCents:      firstClassTax1,
		AdditionalClassFeeCNYCents:     req.AdditionalClassFeeCNYCents,
		AdditionalClassFeeTax6CNYCents: additionalClassTax6,
		AdditionalClassFeeTax1CNYCents: additionalClassTax1,
		RequiredDocuments:              req.RequiredDocuments,
		NotarizationFee:                req.NotarizationFee,
		AcceptanceTime:                 req.AcceptanceTime,
		RegistrationMonths:             req.RegistrationMonths,
		ValidityYears:                  req.ValidityYears,
		Note1:                          req.Note1,
		Note2:                          req.Note2,
		EffectiveFrom:                  eff,
		CreatedBy:                      callerID,
	})
	if err != nil {
		return nil, err
	}
	dto := toSingleClassDTO(*row)
	return &dto, nil
}

// Deprecate retires a single entry.
func (s *Service) Deprecate(ctx context.Context, id uuid.UUID, req DeprecateRequest) (*PricingEntryDTO, error) {
	effTo, err := parseEffectiveTo(req.EffectiveTo)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.Deprecate(ctx, id, effTo)
	if err != nil {
		return nil, err
	}
	dto := toDTO(*row)
	return &dto, nil
}

// DeprecateMadrid retires a single Madrid pricing row.
func (s *Service) DeprecateMadrid(ctx context.Context, id uuid.UUID, req DeprecateRequest) (*MadridPricingEntryDTO, error) {
	effTo, err := parseEffectiveTo(req.EffectiveTo)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.DeprecateMadrid(ctx, id, effTo)
	if err != nil {
		return nil, err
	}
	dto := toMadridDTO(*row)
	return &dto, nil
}

// DeprecateSingleClass retires a single-filing pricing row.
func (s *Service) DeprecateSingleClass(ctx context.Context, id uuid.UUID, req DeprecateRequest) (*SingleClassPricingEntryDTO, error) {
	effTo, err := parseEffectiveTo(req.EffectiveTo)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.DeprecateSingleClass(ctx, id, effTo)
	if err != nil {
		return nil, err
	}
	dto := toSingleClassDTO(*row)
	return &dto, nil
}

// GetByID returns one pricing entry by id, regardless of whether
// it's been deprecated. Used by the traceability endpoint so snapshot
// lines can be resolved back to their source pricing row (including
// historical versions).
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*PricingEntryDTO, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrNotFound
	}
	dto := toDTO(*row)
	return &dto, nil
}

func parseEffectiveFrom(value string) (time.Time, error) {
	eff, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("pricing: invalid effective_from: %w", err)
	}
	return eff, nil
}

func deriveSingleClassTaxedCents(cents int64) (tax6 int64, tax1 int64) {
	return cents * 106 / 100, cents * 101 / 100
}

func parseEffectiveTo(value *string) (time.Time, error) {
	if value == nil {
		return time.Now().UTC().Add(24 * time.Hour).Truncate(24 * time.Hour), nil
	}
	eff, err := time.Parse("2006-01-02", *value)
	if err != nil {
		return time.Time{}, fmt.Errorf("pricing: invalid effective_to: %w", err)
	}
	return eff, nil
}
