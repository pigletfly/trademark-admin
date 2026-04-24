package catalog

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrNotFound is returned when a country or category does not exist.
var ErrNotFound = errors.New("catalog: not found")

// Repository wraps DB access for catalog dictionaries.
type Repository struct{ db *gorm.DB }

// NewRepository builds a Repository.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// ListCountries returns all enabled countries sorted by sort_order then code.
// onlyEnabled=false returns everything (for admin view).
func (r *Repository) ListCountries(ctx context.Context, onlyEnabled bool) ([]Country, error) {
	var rows []Country
	q := r.db.WithContext(ctx).Order("sort_order ASC, code ASC")
	if onlyEnabled {
		q = q.Where("enabled = TRUE")
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetCountry fetches a country by id.
func (r *Repository) GetCountry(ctx context.Context, id uuid.UUID) (*Country, error) {
	var row Country
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// CountryPatch carries optional updates to a country row.
type CountryPatch struct {
	NameZh                    *string
	NameEn                    *string
	IsMadridMember            *bool
	DefaultAcceptanceDays     *int
	DefaultRegistrationMonths *int
	RequiresNotarization      *bool
	NotesZh                   *string
	NotesEn                   *string
	SortOrder                 *int
	Enabled                   *bool
}

// UpdateCountry applies the admin-settable fields. Only fields present in
// patch (non-nil) are updated.
func (r *Repository) UpdateCountry(ctx context.Context, id uuid.UUID, patch CountryPatch) (*Country, error) {
	updates := map[string]any{}
	if patch.NameZh != nil {
		updates["name_zh"] = *patch.NameZh
	}
	if patch.NameEn != nil {
		updates["name_en"] = *patch.NameEn
	}
	if patch.IsMadridMember != nil {
		updates["is_madrid_member"] = *patch.IsMadridMember
	}
	if patch.DefaultAcceptanceDays != nil {
		updates["default_acceptance_days"] = *patch.DefaultAcceptanceDays
	}
	if patch.DefaultRegistrationMonths != nil {
		updates["default_registration_months"] = *patch.DefaultRegistrationMonths
	}
	if patch.RequiresNotarization != nil {
		updates["requires_notarization"] = *patch.RequiresNotarization
	}
	if patch.NotesZh != nil {
		updates["notes_zh"] = *patch.NotesZh
	}
	if patch.NotesEn != nil {
		updates["notes_en"] = *patch.NotesEn
	}
	if patch.SortOrder != nil {
		updates["sort_order"] = *patch.SortOrder
	}
	if patch.Enabled != nil {
		updates["enabled"] = *patch.Enabled
	}
	if len(updates) == 0 {
		return r.GetCountry(ctx, id)
	}
	updates["updated_at"] = gorm.Expr("NOW()")

	res := r.db.WithContext(ctx).Model(&Country{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.GetCountry(ctx, id)
}

// ListNiceCategories returns all 45 categories, ordered by code.
func (r *Repository) ListNiceCategories(ctx context.Context) ([]NiceCategory, error) {
	var rows []NiceCategory
	if err := r.db.WithContext(ctx).Order("code ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
