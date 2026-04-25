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
