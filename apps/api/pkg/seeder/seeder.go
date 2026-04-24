// Package seeder loads JSON seed data from an embed.FS and upserts it into
// Postgres. Upserts are idempotent: ON CONFLICT DO UPDATE, so re-running
// is safe.
package seeder

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SeedCountry mirrors apps/api/seed/countries.json rows.
type SeedCountry struct {
	Code                      string  `json:"code"`
	NameZh                    string  `json:"name_zh"`
	NameEn                    string  `json:"name_en"`
	IsMadridMember            bool    `json:"is_madrid_member"`
	DefaultAcceptanceDays     *int    `json:"default_acceptance_days,omitempty"`
	DefaultRegistrationMonths *int    `json:"default_registration_months,omitempty"`
	RequiresNotarization      bool    `json:"requires_notarization"`
	NotesZh                   *string `json:"notes_zh,omitempty"`
	NotesEn                   *string `json:"notes_en,omitempty"`
	SortOrder                 int     `json:"sort_order"`
}

// SeedNiceCategory mirrors apps/api/seed/nice_categories.json rows.
type SeedNiceCategory struct {
	Code          int     `json:"code"`
	NameZh        string  `json:"name_zh"`
	NameEn        string  `json:"name_en"`
	DescriptionZh *string `json:"description_zh,omitempty"`
	DescriptionEn *string `json:"description_en,omitempty"`
}

// Run loads both seed files from seedFS and upserts them inside a single
// transaction. countriesPath and categoriesPath are paths relative to
// seedFS root (e.g. "seed/countries.json").
func Run(ctx context.Context, db *gorm.DB, seedFS fs.FS, countriesPath, categoriesPath string) error {
	countries, err := loadCountries(seedFS, countriesPath)
	if err != nil {
		return fmt.Errorf("load countries: %w", err)
	}
	categories, err := loadNiceCategories(seedFS, categoriesPath)
	if err != nil {
		return fmt.Errorf("load nice_categories: %w", err)
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := upsertCountries(tx, countries); err != nil {
			return fmt.Errorf("upsert countries: %w", err)
		}
		if err := upsertNiceCategories(tx, categories); err != nil {
			return fmt.Errorf("upsert nice_categories: %w", err)
		}
		return nil
	})
}

func loadCountries(seedFS fs.FS, path string) ([]SeedCountry, error) {
	raw, err := fs.ReadFile(seedFS, path)
	if err != nil {
		return nil, err
	}
	var rows []SeedCountry
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func loadNiceCategories(seedFS fs.FS, path string) ([]SeedNiceCategory, error) {
	raw, err := fs.ReadFile(seedFS, path)
	if err != nil {
		return nil, err
	}
	var rows []SeedNiceCategory
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func upsertCountries(tx *gorm.DB, rows []SeedCountry) error {
	if len(rows) == 0 {
		return nil
	}
	payload := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		payload = append(payload, map[string]any{
			"code":                        r.Code,
			"name_zh":                     r.NameZh,
			"name_en":                     r.NameEn,
			"is_madrid_member":            r.IsMadridMember,
			"default_acceptance_days":     r.DefaultAcceptanceDays,
			"default_registration_months": r.DefaultRegistrationMonths,
			"requires_notarization":       r.RequiresNotarization,
			"notes_zh":                    r.NotesZh,
			"notes_en":                    r.NotesEn,
			"sort_order":                  r.SortOrder,
		})
	}
	// NOTE: use clause.Assignments (not AssignmentColumns): the JSON payload
	// does not carry updated_at, so EXCLUDED.updated_at would be NULL and
	// break the NOT NULL constraint. We set updated_at = NOW() explicitly.
	return tx.Table("countries").Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "code"}},
		DoUpdates: clause.Assignments(map[string]any{
			"name_zh":                     gorm.Expr("EXCLUDED.name_zh"),
			"name_en":                     gorm.Expr("EXCLUDED.name_en"),
			"is_madrid_member":            gorm.Expr("EXCLUDED.is_madrid_member"),
			"default_acceptance_days":     gorm.Expr("EXCLUDED.default_acceptance_days"),
			"default_registration_months": gorm.Expr("EXCLUDED.default_registration_months"),
			"requires_notarization":       gorm.Expr("EXCLUDED.requires_notarization"),
			"notes_zh":                    gorm.Expr("EXCLUDED.notes_zh"),
			"notes_en":                    gorm.Expr("EXCLUDED.notes_en"),
			"sort_order":                  gorm.Expr("EXCLUDED.sort_order"),
			"updated_at":                  gorm.Expr("NOW()"),
		}),
	}).Create(&payload).Error
}

func upsertNiceCategories(tx *gorm.DB, rows []SeedNiceCategory) error {
	if len(rows) == 0 {
		return nil
	}
	payload := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		payload = append(payload, map[string]any{
			"code":           r.Code,
			"name_zh":        r.NameZh,
			"name_en":        r.NameEn,
			"description_zh": r.DescriptionZh,
			"description_en": r.DescriptionEn,
		})
	}
	return tx.Table("nice_categories").Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "code"}},
		DoUpdates: clause.Assignments(map[string]any{
			"name_zh":        gorm.Expr("EXCLUDED.name_zh"),
			"name_en":        gorm.Expr("EXCLUDED.name_en"),
			"description_zh": gorm.Expr("EXCLUDED.description_zh"),
			"description_en": gorm.Expr("EXCLUDED.description_en"),
			"updated_at":     gorm.Expr("NOW()"),
		}),
	}).Create(&payload).Error
}
