package dashboard

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
)

// Service aggregates numbers for the dashboard. It has no DB of its own;
// it fans out to the quotation + customer repos.
type Service struct {
	quotRepo *quotation.Repository
	custRepo *customer.Repository
}

func NewService(q *quotation.Repository, c *customer.Repository) *Service {
	return &Service{quotRepo: q, custRepo: c}
}

// Summary builds the full dashboard payload. When role == salesperson
// the scope is narrowed to userID; otherwise the firm-wide numbers are
// returned.
func (s *Service) Summary(ctx context.Context, userID uuid.UUID, role string) (Summary, error) {
	var owner *uuid.UUID
	scope := "firm"
	if role == "salesperson" {
		owner = &userID
		scope = "self"
	}

	counts, err := s.quotRepo.CountByStatus(ctx, owner)
	if err != nil {
		return Summary{}, err
	}
	sum, err := s.quotRepo.SumApprovedCNYCents(ctx, owner)
	if err != nil {
		return Summary{}, err
	}
	since := time.Now().Add(-30 * 24 * time.Hour)
	newCusts, err := s.custRepo.CountCreatedAfter(ctx, since, owner)
	if err != nil {
		return Summary{}, err
	}
	recent, err := s.quotRepo.Recent(ctx, owner, 5)
	if err != nil {
		return Summary{}, err
	}

	out := Summary{
		ApprovedTotalCNYCents:  sum,
		NewCustomersLast30Days: newCusts,
		Scope:                  scope,
	}
	for _, c := range counts {
		out.QuotationsByStatus = append(out.QuotationsByStatus, QuotationStatusCount{
			Status: c.Status, Count: c.Count,
		})
	}
	for _, r := range recent {
		out.RecentQuotations = append(out.RecentQuotations, RecentQuotation{
			ID: r.ID, Status: r.Status, ServiceTier: r.ServiceTier,
			TotalCNYCents: r.TotalCNYCents,
			CreatedAt:     r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}
