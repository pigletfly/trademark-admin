package catalog

import (
	"context"

	"github.com/google/uuid"
)

// Service is a thin orchestration layer around Repository.
type Service struct{ repo *Repository }

// NewService wires a Service with its Repository.
func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// ListCountries returns countries for API consumers.
func (s *Service) ListCountries(ctx context.Context, includeDisabled bool) ([]CountryDTO, error) {
	rows, err := s.repo.ListCountries(ctx, !includeDisabled)
	if err != nil {
		return nil, err
	}
	out := make([]CountryDTO, len(rows))
	for i, r := range rows {
		out[i] = toCountryDTO(r)
	}
	return out, nil
}

// UpdateCountry applies admin-provided patch and returns the new state.
func (s *Service) UpdateCountry(ctx context.Context, id uuid.UUID, req UpdateCountryRequest) (*CountryDTO, error) {
	patch := CountryPatch{
		NameZh:                    req.NameZh,
		NameEn:                    req.NameEn,
		IsMadridMember:            req.IsMadridMember,
		DefaultAcceptanceDays:     req.DefaultAcceptanceDays,
		DefaultRegistrationMonths: req.DefaultRegistrationMonths,
		RequiresNotarization:      req.RequiresNotarization,
		NotesZh:                   req.NotesZh,
		NotesEn:                   req.NotesEn,
		SortOrder:                 req.SortOrder,
		Enabled:                   req.Enabled,
	}
	row, err := s.repo.UpdateCountry(ctx, id, patch)
	if err != nil {
		return nil, err
	}
	dto := toCountryDTO(*row)
	return &dto, nil
}

// ListNiceCategories returns all 45 nice categories.
func (s *Service) ListNiceCategories(ctx context.Context) ([]NiceCategoryDTO, error) {
	rows, err := s.repo.ListNiceCategories(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]NiceCategoryDTO, len(rows))
	for i, r := range rows {
		out[i] = toNiceCategoryDTO(r)
	}
	return out, nil
}
