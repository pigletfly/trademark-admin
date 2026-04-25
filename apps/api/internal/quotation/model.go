package quotation

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"
)

// Status enumerates the finite set of quotation states. Keep in sync with
// the CHECK constraint in migration 000004.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusSubmitted Status = "submitted"
	StatusApproved  Status = "approved"
	StatusRejected  Status = "rejected"
	StatusCancelled Status = "cancelled"
)

// Quotation mirrors the quotations table.
type Quotation struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	CustomerID      uuid.UUID  `gorm:"type:uuid;not null;index"`
	CountryID       uuid.UUID  `gorm:"type:uuid;not null"`
	ServiceTier     string     `gorm:"not null"`
	Status          Status     `gorm:"not null;default:draft"`
	SnapshotJSON    audit.JSONB `gorm:"type:jsonb"`
	TotalCNYCents   *int64
	Signature       *string
	SerialNo        *string `gorm:"column:serial_no"`
	SubmittedAt     *time.Time
	ReviewedAt      *time.Time
	ReviewedBy      *uuid.UUID `gorm:"type:uuid"`
	ReviewComment   *string
	Notes           *string
	CreatedBy       uuid.UUID `gorm:"type:uuid;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

func (Quotation) TableName() string { return "quotations" }

// StatusHistory mirrors quotation_status_history. Rows are append-only.
type StatusHistory struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	QuotationID uuid.UUID  `gorm:"type:uuid;not null;index"`
	FromStatus  Status     `gorm:"not null"`
	ToStatus    Status     `gorm:"not null"`
	ActorID     *uuid.UUID `gorm:"type:uuid"`
	Comment     *string
	DiffJSON    audit.JSONB `gorm:"column:diff_json;type:jsonb"`
	At          time.Time
}

func (StatusHistory) TableName() string { return "quotation_status_history" }

// DecodeSnapshot parses the stored JSONB blob into a typed Snapshot.
// Returns an empty Snapshot with no error when SnapshotJSON is nil/empty.
func (q *Quotation) DecodeSnapshot() (Snapshot, error) {
	var s Snapshot
	if len(q.SnapshotJSON) == 0 {
		return s, nil
	}
	return s, json.Unmarshal(q.SnapshotJSON, &s)
}
