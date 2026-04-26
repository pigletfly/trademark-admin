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
