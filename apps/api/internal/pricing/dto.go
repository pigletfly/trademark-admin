package pricing

import (
	"time"

	"github.com/google/uuid"
)

// PricingEntryDTO is the wire shape of a pricing row.
type PricingEntryDTO struct {
	ID             uuid.UUID `json:"id"`
	CountryID      uuid.UUID `json:"country_id"`
	ServiceTier    string    `json:"service_tier"`
	FeeItem        string    `json:"fee_item"`
	AmountCNYCents int64     `json:"amount_cny_cents"`
	Notes          *string   `json:"notes,omitempty"`
	EffectiveFrom  string    `json:"effective_from"`         // YYYY-MM-DD
	EffectiveTo    *string   `json:"effective_to,omitempty"` // YYYY-MM-DD or null
	CreatedBy      uuid.UUID `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreateOrReplaceRequest creates a new active entry for the given
// dimension. If an active entry already exists for (country, tier, item),
// the service layer deprecates it (effective_to = new.effective_from)
// and inserts the new row atomically.
type CreateOrReplaceRequest struct {
	CountryID      uuid.UUID `json:"country_id" binding:"required"`
	ServiceTier    string    `json:"service_tier" binding:"required"`
	FeeItem        string    `json:"fee_item" binding:"required"`
	AmountCNYCents int64     `json:"amount_cny_cents" binding:"gte=0"`
	Notes          *string   `json:"notes,omitempty"`
	EffectiveFrom  string    `json:"effective_from" binding:"required"` // YYYY-MM-DD
}

// DeprecateRequest retires the active entry at :id. effective_to
// defaults to tomorrow if omitted.
type DeprecateRequest struct {
	EffectiveTo *string `json:"effective_to,omitempty"` // YYYY-MM-DD
}

func toDTO(e PricingEntry) PricingEntryDTO {
	dto := PricingEntryDTO{
		ID:             e.ID,
		CountryID:      e.CountryID,
		ServiceTier:    e.ServiceTier,
		FeeItem:        e.FeeItem,
		AmountCNYCents: e.AmountCNYCents,
		Notes:          e.Notes,
		EffectiveFrom:  e.EffectiveFrom.Format("2006-01-02"),
		CreatedBy:      e.CreatedBy,
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
	}
	if e.EffectiveTo != nil {
		s := e.EffectiveTo.Format("2006-01-02")
		dto.EffectiveTo = &s
	}
	return dto
}
