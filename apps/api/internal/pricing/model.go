package pricing

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

// PricingEntry mirrors the pricing_entries table.
// Entries are immutable except for effective_to, which is set when the
// entry is deprecated.
type PricingEntry struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	CountryID      uuid.UUID `gorm:"type:uuid;not null;index"`
	ServiceTier    string    `gorm:"not null"`
	FeeItem        string    `gorm:"not null"`
	AmountCNYCents int64     `gorm:"not null"`
	Notes          *string
	EffectiveFrom  time.Time  `gorm:"type:date;not null"`
	EffectiveTo    *time.Time `gorm:"type:date"`
	CreatedBy      uuid.UUID  `gorm:"type:uuid;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (PricingEntry) TableName() string { return "pricing_entries" }

const (
	RegistrationMethodMadrid = "madrid"
	RegistrationMethodSingle = "single"

	MadridPricingTable      = "madrid_pricing_entries"
	SingleClassPricingTable = "single_class_pricing_entries"
)

// ServiceTiers enumerates supported tiers; keep in sync with the
// CHECK constraint in migration 000003.
var ServiceTiers = []string{"basic", "standard", "premium"}

// IsValidServiceTier reports whether t is one of the allowed tiers.
func IsValidServiceTier(t string) bool {
	return slices.Contains(ServiceTiers, t)
}

// RegistrationMethods enumerates supported registration pricing paths.
var RegistrationMethods = []string{RegistrationMethodMadrid, RegistrationMethodSingle}

// IsValidRegistrationMethod reports whether m is one of the supported
// registration pricing paths.
func IsValidRegistrationMethod(m string) bool {
	return slices.Contains(RegistrationMethods, m)
}

// MadridPricingEntry mirrors madrid_pricing_entries. It stores the
// fields from the Madrid pricing screenshot: sequence, country/region,
// official fee in CHF, and agency fee in CNY. The base row has
// IsBaseFee=true and a nil CountryID.
type MadridPricingEntry struct {
	ID                  uuid.UUID  `gorm:"type:uuid;primaryKey"`
	CountryID           *uuid.UUID `gorm:"type:uuid;index"`
	SequenceNo          *int
	CountryArea         string `gorm:"not null"`
	OfficialFeeCHFCents int64  `gorm:"not null"`
	AgencyFeeCNYCents   int64  `gorm:"not null"`
	IsBaseFee           bool   `gorm:"not null;default:false"`
	Notes               *string
	EffectiveFrom       time.Time  `gorm:"type:date;not null"`
	EffectiveTo         *time.Time `gorm:"type:date"`
	CreatedBy           uuid.UUID  `gorm:"type:uuid;not null"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (MadridPricingEntry) TableName() string { return MadridPricingTable }

// SingleClassPricingEntry mirrors single_class_pricing_entries. It keeps
// the operational fields from the "full single filing" sheet.
type SingleClassPricingEntry struct {
	ID                             uuid.UUID `gorm:"type:uuid;primaryKey"`
	CountryID                      uuid.UUID `gorm:"type:uuid;not null;index"`
	Continent                      string    `gorm:"not null"`
	CountryArea                    string    `gorm:"not null"`
	FirstClassFeeCNYCents          int64     `gorm:"not null"`
	FirstClassFeeTax6CNYCents      int64     `gorm:"not null"`
	FirstClassFeeTax1CNYCents      int64     `gorm:"not null"`
	AdditionalClassFeeCNYCents     int64     `gorm:"not null"`
	AdditionalClassFeeTax6CNYCents int64     `gorm:"not null"`
	AdditionalClassFeeTax1CNYCents int64     `gorm:"not null"`
	RequiredDocuments              string    `gorm:"not null"`
	NotarizationFee                string    `gorm:"not null"`
	AcceptanceTime                 string    `gorm:"not null"`
	RegistrationMonths             string    `gorm:"not null"`
	ValidityYears                  *int
	Note1                          *string
	Note2                          *string
	EffectiveFrom                  time.Time  `gorm:"type:date;not null"`
	EffectiveTo                    *time.Time `gorm:"type:date"`
	CreatedBy                      uuid.UUID  `gorm:"type:uuid;not null"`
	CreatedAt                      time.Time
	UpdatedAt                      time.Time
}

func (SingleClassPricingEntry) TableName() string { return SingleClassPricingTable }
