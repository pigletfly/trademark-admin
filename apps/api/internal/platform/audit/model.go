package audit

import (
	"time"

	"github.com/google/uuid"
)

// Log mirrors the audit_logs table.
type Log struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       *uuid.UUID `gorm:"type:uuid" json:"user_id,omitempty"`
	Action       string     `gorm:"not null" json:"action"`
	ResourceType string     `gorm:"not null" json:"resource_type"`
	ResourceID   string     `gorm:"not null" json:"resource_id"`
	ChangesJSON  []byte     `gorm:"type:jsonb" json:"changes_json,omitempty"`
	IP           string     `gorm:"type:inet" json:"ip,omitempty"`
	UserAgent    string     `json:"user_agent,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (Log) TableName() string { return "audit_logs" }
