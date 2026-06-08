package pricing

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrNotFound is returned when the repo can't find a row.
var ErrNotFound = errors.New("pricing: not found")

// ErrNoActive is returned by Deprecate when the entry at :id is already
// deprecated (effective_to already set).
var ErrNoActive = errors.New("pricing: entry already deprecated")

// Repository wraps DB access for pricing entries.
type Repository struct{ db *gorm.DB }

// NewRepository wires a Repository.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// ActiveFilter narrows ListActive.
type ActiveFilter struct {
	CountryID   *uuid.UUID
	ServiceTier *string
}

// ListActive returns all entries where effective_to IS NULL, filtered
// optionally by country and/or tier. Ordered by country_id, tier, fee_item
// so the frontend can render a 2-D table deterministically.
func (r *Repository) ListActive(ctx context.Context, f ActiveFilter) ([]PricingEntry, error) {
	q := r.db.WithContext(ctx).
		Where("effective_to IS NULL").
		Order("country_id, service_tier, fee_item")
	if f.CountryID != nil {
		q = q.Where("country_id = ?", *f.CountryID)
	}
	if f.ServiceTier != nil {
		q = q.Where("service_tier = ?", *f.ServiceTier)
	}
	var rows []PricingEntry
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// MadridActiveFilter narrows ListActiveMadrid.
type MadridActiveFilter struct {
	CountryID   *uuid.UUID
	IncludeBase bool
}

// ListActiveMadrid returns active Madrid pricing rows. When CountryID is
// set and IncludeBase is true, the result includes both the base row and
// the selected country's row.
func (r *Repository) ListActiveMadrid(ctx context.Context, f MadridActiveFilter) ([]MadridPricingEntry, error) {
	q := r.db.WithContext(ctx).
		Where("effective_to IS NULL").
		Order("is_base_fee DESC, sequence_no ASC NULLS LAST, country_area ASC")
	if f.CountryID != nil {
		if f.IncludeBase {
			q = q.Where("(country_id = ? OR is_base_fee = TRUE)", *f.CountryID)
		} else {
			q = q.Where("country_id = ? AND is_base_fee = FALSE", *f.CountryID)
		}
	}
	var rows []MadridPricingEntry
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// SingleClassActiveFilter narrows ListActiveSingleClass.
type SingleClassActiveFilter struct {
	CountryID *uuid.UUID
}

// ListActiveSingleClass returns active single-filing pricing rows.
func (r *Repository) ListActiveSingleClass(ctx context.Context, f SingleClassActiveFilter) ([]SingleClassPricingEntry, error) {
	q := r.db.WithContext(ctx).
		Where("effective_to IS NULL").
		Order("continent ASC, country_area ASC")
	if f.CountryID != nil {
		q = q.Where("country_id = ?", *f.CountryID)
	}
	var rows []SingleClassPricingEntry
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// HistoryFilter queries every version of a single dimension.
type HistoryFilter struct {
	CountryID   uuid.UUID
	ServiceTier string
	FeeItem     string
}

// ListHistory returns every version of the (country, tier, item) tuple
// newest first.
func (r *Repository) ListHistory(ctx context.Context, f HistoryFilter) ([]PricingEntry, error) {
	var rows []PricingEntry
	err := r.db.WithContext(ctx).
		Where("country_id = ? AND service_tier = ? AND fee_item = ?",
			f.CountryID, f.ServiceTier, f.FeeItem).
		Order("effective_from DESC, created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetByID fetches a single row by id; used by Deprecate.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*PricingEntry, error) {
	var row PricingEntry
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// GetMadridByID fetches a Madrid pricing row by id.
func (r *Repository) GetMadridByID(ctx context.Context, id uuid.UUID) (*MadridPricingEntry, error) {
	var row MadridPricingEntry
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// GetSingleClassByID fetches a single-filing pricing row by id.
func (r *Repository) GetSingleClassByID(ctx context.Context, id uuid.UUID) (*SingleClassPricingEntry, error) {
	var row SingleClassPricingEntry
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// NewEntry carries the fields required to insert an entry.
type NewEntry struct {
	CountryID      uuid.UUID
	ServiceTier    string
	FeeItem        string
	AmountCNYCents int64
	Notes          *string
	EffectiveFrom  time.Time
	CreatedBy      uuid.UUID
}

// NewMadridEntry carries the fields required to insert a Madrid pricing row.
type NewMadridEntry struct {
	CountryID           *uuid.UUID
	SequenceNo          *int
	CountryArea         string
	OfficialFeeCHFCents int64
	AgencyFeeCNYCents   int64
	IsBaseFee           bool
	Notes               *string
	EffectiveFrom       time.Time
	CreatedBy           uuid.UUID
}

// NewSingleClassEntry carries the fields required to insert a
// single-filing pricing row.
type NewSingleClassEntry struct {
	CountryID                      uuid.UUID
	Continent                      string
	CountryArea                    string
	FirstClassFeeCNYCents          int64
	FirstClassFeeTax6CNYCents      int64
	FirstClassFeeTax1CNYCents      int64
	AdditionalClassFeeCNYCents     int64
	AdditionalClassFeeTax6CNYCents int64
	AdditionalClassFeeTax1CNYCents int64
	RequiredDocuments              string
	NotarizationFee                string
	AcceptanceTime                 string
	RegistrationMonths             string
	ValidityYears                  *int
	Note1                          *string
	Note2                          *string
	EffectiveFrom                  time.Time
	CreatedBy                      uuid.UUID
}

// ReplaceActive inserts a new active entry for (country, tier, item),
// deprecating the existing active row (if any) by setting
// effective_to = newEntry.EffectiveFrom. Runs in a single transaction so
// readers never observe two active rows at once.
//
// Returns the inserted entry.
func (r *Repository) ReplaceActive(ctx context.Context, n NewEntry) (*PricingEntry, error) {
	var inserted *PricingEntry
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Deprecate any existing active row for this dimension.
		if err := tx.Model(&PricingEntry{}).
			Where("country_id = ? AND service_tier = ? AND fee_item = ? AND effective_to IS NULL",
				n.CountryID, n.ServiceTier, n.FeeItem).
			Updates(map[string]any{
				"effective_to": n.EffectiveFrom,
				"updated_at":   gorm.Expr("NOW()"),
			}).Error; err != nil {
			return err
		}
		row := PricingEntry{
			ID:             uuid.New(),
			CountryID:      n.CountryID,
			ServiceTier:    n.ServiceTier,
			FeeItem:        n.FeeItem,
			AmountCNYCents: n.AmountCNYCents,
			Notes:          n.Notes,
			EffectiveFrom:  n.EffectiveFrom,
			CreatedBy:      n.CreatedBy,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		inserted = &row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return inserted, nil
}

// ReplaceActiveMadrid inserts a new active Madrid row, deprecating the
// currently active base row or country row in the same transaction.
func (r *Repository) ReplaceActiveMadrid(ctx context.Context, n NewMadridEntry) (*MadridPricingEntry, error) {
	if n.IsBaseFee && n.CountryID != nil {
		return nil, errors.New("pricing: madrid base row must not have country_id")
	}
	if !n.IsBaseFee && n.CountryID == nil {
		return nil, errors.New("pricing: madrid country row requires country_id")
	}
	var inserted *MadridPricingEntry
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&MadridPricingEntry{}).Where("effective_to IS NULL")
		if n.IsBaseFee {
			q = q.Where("is_base_fee = TRUE")
		} else {
			q = q.Where("is_base_fee = FALSE AND country_id = ?", *n.CountryID)
		}
		if err := q.Updates(map[string]any{
			"effective_to": n.EffectiveFrom,
			"updated_at":   gorm.Expr("NOW()"),
		}).Error; err != nil {
			return err
		}
		row := MadridPricingEntry{
			ID:                  uuid.New(),
			CountryID:           n.CountryID,
			SequenceNo:          n.SequenceNo,
			CountryArea:         n.CountryArea,
			OfficialFeeCHFCents: n.OfficialFeeCHFCents,
			AgencyFeeCNYCents:   n.AgencyFeeCNYCents,
			IsBaseFee:           n.IsBaseFee,
			Notes:               n.Notes,
			EffectiveFrom:       n.EffectiveFrom,
			CreatedBy:           n.CreatedBy,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		inserted = &row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return inserted, nil
}

// ReplaceActiveSingleClass inserts a new active single-filing row for a
// country and deprecates the existing active version in one transaction.
func (r *Repository) ReplaceActiveSingleClass(ctx context.Context, n NewSingleClassEntry) (*SingleClassPricingEntry, error) {
	var inserted *SingleClassPricingEntry
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&SingleClassPricingEntry{}).
			Where("country_id = ? AND effective_to IS NULL", n.CountryID).
			Updates(map[string]any{
				"effective_to": n.EffectiveFrom,
				"updated_at":   gorm.Expr("NOW()"),
			}).Error; err != nil {
			return err
		}
		row := SingleClassPricingEntry{
			ID:                             uuid.New(),
			CountryID:                      n.CountryID,
			Continent:                      n.Continent,
			CountryArea:                    n.CountryArea,
			FirstClassFeeCNYCents:          n.FirstClassFeeCNYCents,
			FirstClassFeeTax6CNYCents:      n.FirstClassFeeTax6CNYCents,
			FirstClassFeeTax1CNYCents:      n.FirstClassFeeTax1CNYCents,
			AdditionalClassFeeCNYCents:     n.AdditionalClassFeeCNYCents,
			AdditionalClassFeeTax6CNYCents: n.AdditionalClassFeeTax6CNYCents,
			AdditionalClassFeeTax1CNYCents: n.AdditionalClassFeeTax1CNYCents,
			RequiredDocuments:              n.RequiredDocuments,
			NotarizationFee:                n.NotarizationFee,
			AcceptanceTime:                 n.AcceptanceTime,
			RegistrationMonths:             n.RegistrationMonths,
			ValidityYears:                  n.ValidityYears,
			Note1:                          n.Note1,
			Note2:                          n.Note2,
			EffectiveFrom:                  n.EffectiveFrom,
			CreatedBy:                      n.CreatedBy,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		inserted = &row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return inserted, nil
}

// Deprecate sets effective_to on the entry at :id. Returns ErrNoActive if
// the row is already deprecated. Returns ErrNotFound if no such row.
func (r *Repository) Deprecate(ctx context.Context, id uuid.UUID, effectiveTo time.Time) (*PricingEntry, error) {
	row, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.EffectiveTo != nil {
		return nil, ErrNoActive
	}
	if !effectiveTo.After(row.EffectiveFrom) {
		return nil, errors.New("pricing: effective_to must be after effective_from")
	}
	if err := r.db.WithContext(ctx).
		Model(&PricingEntry{}).Where("id = ?", id).
		Updates(map[string]any{
			"effective_to": effectiveTo,
			"updated_at":   gorm.Expr("NOW()"),
		}).Error; err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

// DeprecateMadrid sets effective_to on a Madrid pricing row.
func (r *Repository) DeprecateMadrid(ctx context.Context, id uuid.UUID, effectiveTo time.Time) (*MadridPricingEntry, error) {
	row, err := r.GetMadridByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.EffectiveTo != nil {
		return nil, ErrNoActive
	}
	if !effectiveTo.After(row.EffectiveFrom) {
		return nil, errors.New("pricing: effective_to must be after effective_from")
	}
	if err := r.db.WithContext(ctx).
		Model(&MadridPricingEntry{}).Where("id = ?", id).
		Updates(map[string]any{
			"effective_to": effectiveTo,
			"updated_at":   gorm.Expr("NOW()"),
		}).Error; err != nil {
		return nil, err
	}
	return r.GetMadridByID(ctx, id)
}

// DeprecateSingleClass sets effective_to on a single-filing pricing row.
func (r *Repository) DeprecateSingleClass(ctx context.Context, id uuid.UUID, effectiveTo time.Time) (*SingleClassPricingEntry, error) {
	row, err := r.GetSingleClassByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.EffectiveTo != nil {
		return nil, ErrNoActive
	}
	if !effectiveTo.After(row.EffectiveFrom) {
		return nil, errors.New("pricing: effective_to must be after effective_from")
	}
	if err := r.db.WithContext(ctx).
		Model(&SingleClassPricingEntry{}).Where("id = ?", id).
		Updates(map[string]any{
			"effective_to": effectiveTo,
			"updated_at":   gorm.Expr("NOW()"),
		}).Error; err != nil {
		return nil, err
	}
	return r.GetSingleClassByID(ctx, id)
}
