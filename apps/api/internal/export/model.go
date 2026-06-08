package export

import (
	"time"

	"github.com/google/uuid"
)

// Format enumerates supported export formats. Keep in sync with the
// CHECK constraint in migration 000005.
type Format string

const (
	FormatPDF  Format = "pdf"
	FormatDOCX Format = "docx"
	FormatXLSX Format = "xlsx"
)

// Language enumerates the UI language of the rendered document.
type Language string

const (
	LanguageZH        Language = "zh"
	LanguageEN        Language = "en"
	LanguageBilingual Language = "bilingual"
)

// ExportFile mirrors the export_files table. One row per generated file.
type ExportFile struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	QuotationID uuid.UUID `gorm:"type:uuid;not null;index"`
	Format      Format    `gorm:"not null"`
	Language    Language  `gorm:"not null"`
	FilePath    string    `gorm:"not null"`
	FileSize    int64     `gorm:"not null"`
	SHA256      string    `gorm:"not null"`
	ExpiresAt   time.Time `gorm:"not null"`
	CreatedBy   uuid.UUID `gorm:"type:uuid;not null"`
	CreatedAt   time.Time
}

func (ExportFile) TableName() string { return "export_files" }
