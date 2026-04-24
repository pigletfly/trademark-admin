package catalog

import "github.com/google/uuid"

// CountryDTO is the wire representation of a country row.
type CountryDTO struct {
	ID                        uuid.UUID `json:"id"`
	Code                      string    `json:"code"`
	NameZh                    string    `json:"name_zh"`
	NameEn                    string    `json:"name_en"`
	IsMadridMember            bool      `json:"is_madrid_member"`
	DefaultAcceptanceDays     *int      `json:"default_acceptance_days,omitempty"`
	DefaultRegistrationMonths *int      `json:"default_registration_months,omitempty"`
	RequiresNotarization      bool      `json:"requires_notarization"`
	NotesZh                   *string   `json:"notes_zh,omitempty"`
	NotesEn                   *string   `json:"notes_en,omitempty"`
	SortOrder                 int       `json:"sort_order"`
	Enabled                   bool      `json:"enabled"`
}

// NiceCategoryDTO is the wire representation of a nice category.
type NiceCategoryDTO struct {
	Code          int     `json:"code"`
	NameZh        string  `json:"name_zh"`
	NameEn        string  `json:"name_en"`
	DescriptionZh *string `json:"description_zh,omitempty"`
	DescriptionEn *string `json:"description_en,omitempty"`
}

// UpdateCountryRequest — all fields optional; only present (non-nil) fields are applied.
type UpdateCountryRequest struct {
	NameZh                    *string `json:"name_zh,omitempty"`
	NameEn                    *string `json:"name_en,omitempty"`
	IsMadridMember            *bool   `json:"is_madrid_member,omitempty"`
	DefaultAcceptanceDays     *int    `json:"default_acceptance_days,omitempty"`
	DefaultRegistrationMonths *int    `json:"default_registration_months,omitempty"`
	RequiresNotarization      *bool   `json:"requires_notarization,omitempty"`
	NotesZh                   *string `json:"notes_zh,omitempty"`
	NotesEn                   *string `json:"notes_en,omitempty"`
	SortOrder                 *int    `json:"sort_order,omitempty"`
	Enabled                   *bool   `json:"enabled,omitempty"`
}

func toCountryDTO(c Country) CountryDTO {
	return CountryDTO{
		ID:                        c.ID,
		Code:                      c.Code,
		NameZh:                    c.NameZh,
		NameEn:                    c.NameEn,
		IsMadridMember:            c.IsMadridMember,
		DefaultAcceptanceDays:     c.DefaultAcceptanceDays,
		DefaultRegistrationMonths: c.DefaultRegistrationMonths,
		RequiresNotarization:      c.RequiresNotarization,
		NotesZh:                   c.NotesZh,
		NotesEn:                   c.NotesEn,
		SortOrder:                 c.SortOrder,
		Enabled:                   c.Enabled,
	}
}

func toNiceCategoryDTO(n NiceCategory) NiceCategoryDTO {
	return NiceCategoryDTO{
		Code:          n.Code,
		NameZh:        n.NameZh,
		NameEn:        n.NameEn,
		DescriptionZh: n.DescriptionZh,
		DescriptionEn: n.DescriptionEn,
	}
}
