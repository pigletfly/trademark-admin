package pricing

import (
	"time"

	"github.com/google/uuid"
)

// PricingEntry mirrors the pricing_entries table.
// Entries are immutable except for effective_to, which is set when the
// entry is deprecated.
type PricingEntry struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	CountryID      uuid.UUID  `gorm:"type:uuid;not null;index"`
	ServiceTier    string     `gorm:"not null"`
	FeeItem        string     `gorm:"not null"`
	AmountCNYCents int64      `gorm:"not null"`
	Notes          *string
	EffectiveFrom  time.Time  `gorm:"type:date;not null"`
	EffectiveTo    *time.Time `gorm:"type:date"`
	CreatedBy      uuid.UUID  `gorm:"type:uuid;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (PricingEntry) TableName() string { return "pricing_entries" }

// ServiceTiers enumerates supported tiers; keep in sync with the
// CHECK constraint in migration 000003.
var ServiceTiers = []string{"basic", "standard", "premium"}

// IsValidServiceTier reports whether t is one of the allowed tiers.
func IsValidServiceTier(t string) bool {
	for _, v := range ServiceTiers {
		if v == t {
			return true
		}
	}
	return false
}
