package dashboard

import (
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
)

// QuotationStatusCount is one row per status.
type QuotationStatusCount struct {
	Status quotation.Status `json:"status"`
	Count  int64            `json:"count"`
}

// RecentQuotation shape is a trimmed version of the full quotation
// DTO suitable for an activity feed.
type RecentQuotation struct {
	ID            uuid.UUID        `json:"id"`
	Status        quotation.Status `json:"status"`
	ServiceTier   string           `json:"service_tier"`
	TotalCNYCents *int64           `json:"total_cny_cents,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// Summary is the single response body returned by GET /dashboard/summary.
type Summary struct {
	QuotationsByStatus     []QuotationStatusCount `json:"quotations_by_status"`
	ApprovedTotalCNYCents  int64                  `json:"approved_total_cny_cents"`
	NewCustomersLast30Days int64                  `json:"new_customers_last_30_days"`
	RecentQuotations       []RecentQuotation      `json:"recent_quotations"`
	// Scope is either "self" (salesperson) or "firm" (reviewer/admin).
	// Frontend uses it to label the KPI cards appropriately.
	Scope string `json:"scope"`
}
