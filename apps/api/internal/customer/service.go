package customer

import (
	"context"

	"github.com/google/uuid"
)

// Role codes used to decide owner scoping. Kept as constants rather than
// imported from internal/auth to avoid a package-level circular dependency
// (auth → customer handler later would re-enter here).
const (
	RoleSalesperson = "salesperson"
	RoleReviewer    = "reviewer"
	RoleAdmin       = "admin"
)

// Service orchestrates owner scoping on top of Repository.
type Service struct{ repo *Repository }

// NewService wires a Service.
func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// ownerScope decides the visibility filter for the caller:
//   - reviewer / admin: see everything (nil scope).
//   - salesperson OR any other / unknown role: scoped to own rows.
//
// The fail-closed default means a JWT with a malformed or stale role
// claim never silently grants admin-level visibility.
func ownerScope(callerID uuid.UUID, role string) *uuid.UUID {
	if role == RoleReviewer || role == RoleAdmin {
		return nil
	}
	c := callerID
	return &c
}

// List returns a paginated list, scoped to the caller's role.
func (s *Service) List(ctx context.Context, callerID uuid.UUID, role string, q string, page, pageSize int) (ListResponse, error) {
	res, err := s.repo.List(ctx, ListFilter{
		Query:    q,
		OwnerID:  ownerScope(callerID, role),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return ListResponse{}, err
	}
	items := make([]CustomerDTO, len(res.Items))
	for i, r := range res.Items {
		items[i] = toDTO(r)
	}
	return ListResponse{Items: items, Page: res.Page, PageSize: res.PageSize, Total: res.Total}, nil
}

// Get fetches a single customer respecting the caller's owner scope.
func (s *Service) Get(ctx context.Context, callerID uuid.UUID, role string, id uuid.UUID) (*CustomerDTO, error) {
	row, err := s.repo.Get(ctx, id, ownerScope(callerID, role))
	if err != nil {
		return nil, err
	}
	d := toDTO(*row)
	return &d, nil
}

// Create inserts a new customer owned by the caller.
func (s *Service) Create(ctx context.Context, callerID uuid.UUID, req CreateRequest) (*CustomerDTO, error) {
	row := &Customer{
		Name:           req.Name,
		Industry:       req.Industry,
		IsReturning:    req.IsReturning,
		PriceSensitive: req.PriceSensitive,
		ContactName:    req.ContactName,
		ContactPhone:   req.ContactPhone,
		ContactEmail:   req.ContactEmail,
		Notes:          req.Notes,
		CreatedBy:      callerID,
	}
	if err := s.repo.Create(ctx, row); err != nil {
		return nil, err
	}
	d := toDTO(*row)
	return &d, nil
}

// Update applies a patch; owner scope applies.
func (s *Service) Update(ctx context.Context, callerID uuid.UUID, role string, id uuid.UUID, req UpdateRequest) (*CustomerDTO, error) {
	row, err := s.repo.Update(ctx, id, ownerScope(callerID, role), Patch{
		Name:           req.Name,
		Industry:       req.Industry,
		IsReturning:    req.IsReturning,
		PriceSensitive: req.PriceSensitive,
		ContactName:    req.ContactName,
		ContactPhone:   req.ContactPhone,
		ContactEmail:   req.ContactEmail,
		Notes:          req.Notes,
	})
	if err != nil {
		return nil, err
	}
	d := toDTO(*row)
	return &d, nil
}
