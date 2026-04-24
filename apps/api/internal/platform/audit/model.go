package audit

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// JSONB is a []byte that satisfies driver.Valuer so the Postgres driver sends
// it as a JSON string (with implicit cast to jsonb) rather than as bytea.
type JSONB []byte

// Value implements driver.Valuer. Returns the raw JSON as a string so that the
// pgx driver can cast it to jsonb without error.
func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

// Scan implements sql.Scanner so that GORM can read jsonb rows back into JSONB.
func (j *JSONB) Scan(src any) error {
	if src == nil {
		*j = nil
		return nil
	}
	switch v := src.(type) {
	case []byte:
		*j = append((*j)[:0], v...)
		return nil
	case string:
		*j = JSONB(v)
		return nil
	}
	return errors.New("audit: cannot scan into JSONB")
}

// MarshalJSON forwards the raw bytes so the JSON encoder doesn't base64-encode them.
func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

// UnmarshalJSON stores the raw JSON bytes.
func (j *JSONB) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*j = nil
		return nil
	}
	*j = append((*j)[:0], data...)
	return nil
}

// Ensure JSONB satisfies the json.Marshaler / json.Unmarshaler interfaces at compile time.
var _ json.Marshaler = JSONB(nil)
var _ json.Unmarshaler = (*JSONB)(nil)

// Log mirrors the audit_logs table.
type Log struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       *uuid.UUID `gorm:"type:uuid" json:"user_id,omitempty"`
	Action       string     `gorm:"not null" json:"action"`
	ResourceType string     `gorm:"not null" json:"resource_type"`
	ResourceID   string     `gorm:"not null" json:"resource_id"`
	ChangesJSON  JSONB      `gorm:"type:jsonb" json:"changes_json,omitempty"`
	IP           string     `gorm:"type:inet" json:"ip,omitempty"`
	UserAgent    string     `json:"user_agent,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (Log) TableName() string { return "audit_logs" }
