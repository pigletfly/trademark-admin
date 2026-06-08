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

// MadridPricingEntryDTO is the wire shape of a Madrid pricing row.
type MadridPricingEntryDTO struct {
	ID                  uuid.UUID  `json:"id"`
	CountryID           *uuid.UUID `json:"country_id,omitempty"`
	SequenceNo          *int       `json:"sequence_no,omitempty"`
	CountryArea         string     `json:"country_area"`
	OfficialFeeCHFCents int64      `json:"official_fee_chf_cents"`
	AgencyFeeCNYCents   int64      `json:"agency_fee_cny_cents"`
	IsBaseFee           bool       `json:"is_base_fee"`
	Notes               *string    `json:"notes,omitempty"`
	EffectiveFrom       string     `json:"effective_from"`
	EffectiveTo         *string    `json:"effective_to,omitempty"`
	CreatedBy           uuid.UUID  `json:"created_by"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// CreateOrReplaceMadridRequest creates a new active Madrid pricing row.
type CreateOrReplaceMadridRequest struct {
	CountryID           *uuid.UUID `json:"country_id,omitempty"`
	SequenceNo          *int       `json:"sequence_no,omitempty"`
	CountryArea         string     `json:"country_area" binding:"required"`
	OfficialFeeCHFCents int64      `json:"official_fee_chf_cents" binding:"gte=0"`
	AgencyFeeCNYCents   int64      `json:"agency_fee_cny_cents" binding:"gte=0"`
	IsBaseFee           bool       `json:"is_base_fee"`
	Notes               *string    `json:"notes,omitempty"`
	EffectiveFrom       string     `json:"effective_from" binding:"required"`
}

// SingleClassPricingEntryDTO is the wire shape of a single-filing row.
type SingleClassPricingEntryDTO struct {
	ID                             uuid.UUID `json:"id"`
	CountryID                      uuid.UUID `json:"country_id"`
	Continent                      string    `json:"continent"`
	CountryArea                    string    `json:"country_area"`
	FirstClassFeeCNYCents          int64     `json:"first_class_fee_cny_cents"`
	FirstClassFeeTax6CNYCents      int64     `json:"first_class_fee_tax6_cny_cents"`
	FirstClassFeeTax1CNYCents      int64     `json:"first_class_fee_tax1_cny_cents"`
	AdditionalClassFeeCNYCents     int64     `json:"additional_class_fee_cny_cents"`
	AdditionalClassFeeTax6CNYCents int64     `json:"additional_class_fee_tax6_cny_cents"`
	AdditionalClassFeeTax1CNYCents int64     `json:"additional_class_fee_tax1_cny_cents"`
	RequiredDocuments              string    `json:"required_documents"`
	NotarizationFee                string    `json:"notarization_fee"`
	AcceptanceTime                 string    `json:"acceptance_time"`
	RegistrationMonths             string    `json:"registration_months"`
	ValidityYears                  *int      `json:"validity_years,omitempty"`
	Note1                          *string   `json:"note1,omitempty"`
	Note2                          *string   `json:"note2,omitempty"`
	EffectiveFrom                  string    `json:"effective_from"`
	EffectiveTo                    *string   `json:"effective_to,omitempty"`
	CreatedBy                      uuid.UUID `json:"created_by"`
	CreatedAt                      time.Time `json:"created_at"`
	UpdatedAt                      time.Time `json:"updated_at"`
}

// CreateOrReplaceSingleClassRequest creates a new active single-filing row.
type CreateOrReplaceSingleClassRequest struct {
	CountryID                      uuid.UUID `json:"country_id" binding:"required"`
	Continent                      string    `json:"continent" binding:"required"`
	CountryArea                    string    `json:"country_area" binding:"required"`
	FirstClassFeeCNYCents          int64     `json:"first_class_fee_cny_cents" binding:"gte=0"`
	FirstClassFeeTax6CNYCents      int64     `json:"first_class_fee_tax6_cny_cents" binding:"gte=0"`
	FirstClassFeeTax1CNYCents      int64     `json:"first_class_fee_tax1_cny_cents" binding:"gte=0"`
	AdditionalClassFeeCNYCents     int64     `json:"additional_class_fee_cny_cents" binding:"gte=0"`
	AdditionalClassFeeTax6CNYCents int64     `json:"additional_class_fee_tax6_cny_cents" binding:"gte=0"`
	AdditionalClassFeeTax1CNYCents int64     `json:"additional_class_fee_tax1_cny_cents" binding:"gte=0"`
	RequiredDocuments              string    `json:"required_documents"`
	NotarizationFee                string    `json:"notarization_fee"`
	AcceptanceTime                 string    `json:"acceptance_time"`
	RegistrationMonths             string    `json:"registration_months"`
	ValidityYears                  *int      `json:"validity_years,omitempty"`
	Note1                          *string   `json:"note1,omitempty"`
	Note2                          *string   `json:"note2,omitempty"`
	EffectiveFrom                  string    `json:"effective_from" binding:"required"`
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

func toMadridDTO(e MadridPricingEntry) MadridPricingEntryDTO {
	dto := MadridPricingEntryDTO{
		ID:                  e.ID,
		CountryID:           e.CountryID,
		SequenceNo:          e.SequenceNo,
		CountryArea:         e.CountryArea,
		OfficialFeeCHFCents: e.OfficialFeeCHFCents,
		AgencyFeeCNYCents:   e.AgencyFeeCNYCents,
		IsBaseFee:           e.IsBaseFee,
		Notes:               e.Notes,
		EffectiveFrom:       e.EffectiveFrom.Format("2006-01-02"),
		CreatedBy:           e.CreatedBy,
		CreatedAt:           e.CreatedAt,
		UpdatedAt:           e.UpdatedAt,
	}
	if e.EffectiveTo != nil {
		s := e.EffectiveTo.Format("2006-01-02")
		dto.EffectiveTo = &s
	}
	return dto
}

func toSingleClassDTO(e SingleClassPricingEntry) SingleClassPricingEntryDTO {
	dto := SingleClassPricingEntryDTO{
		ID:                             e.ID,
		CountryID:                      e.CountryID,
		Continent:                      e.Continent,
		CountryArea:                    e.CountryArea,
		FirstClassFeeCNYCents:          e.FirstClassFeeCNYCents,
		FirstClassFeeTax6CNYCents:      e.FirstClassFeeTax6CNYCents,
		FirstClassFeeTax1CNYCents:      e.FirstClassFeeTax1CNYCents,
		AdditionalClassFeeCNYCents:     e.AdditionalClassFeeCNYCents,
		AdditionalClassFeeTax6CNYCents: e.AdditionalClassFeeTax6CNYCents,
		AdditionalClassFeeTax1CNYCents: e.AdditionalClassFeeTax1CNYCents,
		RequiredDocuments:              e.RequiredDocuments,
		NotarizationFee:                e.NotarizationFee,
		AcceptanceTime:                 e.AcceptanceTime,
		RegistrationMonths:             e.RegistrationMonths,
		ValidityYears:                  e.ValidityYears,
		Note1:                          e.Note1,
		Note2:                          e.Note2,
		EffectiveFrom:                  e.EffectiveFrom.Format("2006-01-02"),
		CreatedBy:                      e.CreatedBy,
		CreatedAt:                      e.CreatedAt,
		UpdatedAt:                      e.UpdatedAt,
	}
	if e.EffectiveTo != nil {
		s := e.EffectiveTo.Format("2006-01-02")
		dto.EffectiveTo = &s
	}
	return dto
}
