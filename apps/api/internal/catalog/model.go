package catalog

import (
	"time"

	"github.com/google/uuid"
)

// Country mirrors the countries table.
type Country struct {
	ID                        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code                      string    `gorm:"uniqueIndex;not null"`
	NameZh                    string    `gorm:"not null"`
	NameEn                    string    `gorm:"not null"`
	IsMadridMember            bool      `gorm:"not null;default:false"`
	DefaultAcceptanceDays     *int
	DefaultRegistrationMonths *int
	RequiresNotarization      bool    `gorm:"not null;default:false"`
	NotesZh                   *string
	NotesEn                   *string
	SortOrder                 int  `gorm:"not null;default:0"`
	Enabled                   bool `gorm:"not null;default:true"`
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

// TableName pins the GORM mapping.
func (Country) TableName() string { return "countries" }

// NiceCategory mirrors the nice_categories table.
type NiceCategory struct {
	Code          int    `gorm:"primaryKey"`
	NameZh        string `gorm:"not null"`
	NameEn        string `gorm:"not null"`
	DescriptionZh *string
	DescriptionEn *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (NiceCategory) TableName() string { return "nice_categories" }
