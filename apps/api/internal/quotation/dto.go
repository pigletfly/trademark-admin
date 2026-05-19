package quotation

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CreateRequest is the POST /quotations body. Creates a new draft.
type CreateRequest struct {
	CustomerID          uuid.UUID   `json:"customer_id" binding:"required"`
	CountryID           uuid.UUID   `json:"country_id"  binding:"required"`
	CountryIDs          []uuid.UUID `json:"country_ids,omitempty"`
	NiceCategoryCodes   []int       `json:"nice_category_codes,omitempty"`
	RegistrationMethods []string    `json:"registration_methods,omitempty"`
	AgentLevel          string      `json:"agent_level,omitempty"`
	ServiceTier         string      `json:"service_tier" binding:"required"`
	InfoSections        []string    `json:"info_sections,omitempty"`
	Notes               *string     `json:"notes"`
}

// UpdateDraftRequest patches a draft's editable fields. Only applicable
// while status == draft.
type UpdateDraftRequest struct {
	CustomerID          *uuid.UUID   `json:"customer_id"`
	CountryID           *uuid.UUID   `json:"country_id"`
	CountryIDs          *[]uuid.UUID `json:"country_ids"`
	NiceCategoryCodes   *[]int       `json:"nice_category_codes"`
	RegistrationMethods *[]string    `json:"registration_methods"`
	AgentLevel          *string      `json:"agent_level"`
	ServiceTier         *string      `json:"service_tier"`
	InfoSections        *[]string    `json:"info_sections"`
	Notes               *string      `json:"notes"`
}

// ReviewRequest is the body for approve/reject. Comment is optional but
// strongly recommended when rejecting.
type ReviewRequest struct {
	Comment *string `json:"comment"`
}

// AdjustRequest is the body of POST /quotations/:id/adjust — reviewer
// hand-edits the snapshot lines. Comment is optional but recommended
// for audit. Lines gets `required,min=1,dive` so an empty payload is
// rejected before the service sees it.
type AdjustRequest struct {
	Lines   []SnapshotLine `json:"lines" binding:"required,min=1,dive"`
	Comment *string        `json:"comment,omitempty"`
}

// SnapshotLine is one priced fee item. Shape mirrors pricing.CalcLine,
// except SourcePricingEntryID is nullable — reviewer-adjusted lines
// (manual override) have no source entry, and legacy snapshots written
// before M4 decode to nil here (missing JSON key -> nil *uuid.UUID).
type SnapshotLine struct {
	FeeItem              string     `json:"fee_item"`
	AmountCNYCents       int64      `json:"amount_cny_cents"`
	SourcePricingEntryID *uuid.UUID `json:"source_pricing_entry_id,omitempty"`
}

// Snapshot is what's persisted in snapshot_json. Signature + total live
// in their own columns for indexing, but are duplicated here so the
// JSONB blob is self-contained for exports later.
type Snapshot struct {
	Lines         []SnapshotLine `json:"lines"`
	TotalCNYCents int64          `json:"total_cny_cents"`
	Signature     string         `json:"signature"`
}

// Response is the GET response. Shape is flat for easy consumption.
type Response struct {
	ID                  uuid.UUID   `json:"id"`
	CustomerID          uuid.UUID   `json:"customer_id"`
	CountryID           uuid.UUID   `json:"country_id"`
	CountryIDs          []uuid.UUID `json:"country_ids,omitempty"`
	NiceCategoryCodes   []int       `json:"nice_category_codes,omitempty"`
	RegistrationMethods []string    `json:"registration_methods,omitempty"`
	AgentLevel          string      `json:"agent_level,omitempty"`
	ServiceTier         string      `json:"service_tier"`
	Status              Status      `json:"status"`
	Snapshot            *Snapshot   `json:"snapshot,omitempty"`
	TotalCNYCents       *int64      `json:"total_cny_cents,omitempty"`
	Signature           *string     `json:"signature,omitempty"`
	SerialNo            *string     `json:"serial_no,omitempty"`
	SubmittedAt         *time.Time  `json:"submitted_at,omitempty"`
	ReviewedAt          *time.Time  `json:"reviewed_at,omitempty"`
	ReviewedBy          *uuid.UUID  `json:"reviewed_by,omitempty"`
	ReviewComment       *string     `json:"review_comment,omitempty"`
	InfoSections        []string    `json:"info_sections,omitempty"`
	Notes               *string     `json:"notes,omitempty"`
	CreatedBy           uuid.UUID   `json:"created_by"`
	CreatedAt           time.Time   `json:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at"`
}

// HistoryEntry is one row in the transition log, returned by the
// history endpoint.
type HistoryEntry struct {
	FromStatus Status          `json:"from_status"`
	ToStatus   Status          `json:"to_status"`
	ActorID    *uuid.UUID      `json:"actor_id,omitempty"`
	Comment    *string         `json:"comment,omitempty"`
	At         time.Time       `json:"at"`
	DiffJSON   json.RawMessage `json:"diff_json,omitempty"`
}

// PreviewRequest is the body of POST /quotations/preview — a non-persistent
// pricing lookup used by the wizard before the quotation row exists.
// Validation tags mirror CreateRequest so bad bodies are rejected before
// reaching the service.
type PreviewRequest struct {
	CustomerID  uuid.UUID   `json:"customer_id"  binding:"required"`
	CountryID   uuid.UUID   `json:"country_id"   binding:"required"`
	CountryIDs  []uuid.UUID `json:"country_ids,omitempty"`
	ServiceTier string      `json:"service_tier" binding:"required"`
}

// PreviewResponse is the shape returned by POST /quotations/preview.
// Intentionally mirrors the quotation Snapshot so the frontend can reuse
// the same rendering component (QuotationSnapshotView).
type PreviewResponse struct {
	Lines         []SnapshotLine `json:"lines"`
	TotalCNYCents int64          `json:"total_cny_cents"`
	Signature     string         `json:"signature"`
}
