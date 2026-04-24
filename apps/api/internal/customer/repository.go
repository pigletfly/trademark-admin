package customer

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrNotFound indicates no live row matches the query.
var ErrNotFound = errors.New("customer: not found")

// ErrDuplicateName indicates an attempted insert/update violates the name uniqueness constraint.
var ErrDuplicateName = errors.New("customer: duplicate name")

// Repository wraps DB access for customer rows.
type Repository struct{ db *gorm.DB }

// NewRepository builds a Repository.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// ListFilter groups list parameters (pagination + search + owner scope).
type ListFilter struct {
	Query    string     // ILIKE on name + industry
	OwnerID  *uuid.UUID // nil = no scope filter (admin/reviewer)
	Page     int        // 1-based
	PageSize int
}

// ListResult is the paginated list envelope.
type ListResult struct {
	Items    []Customer
	Page     int
	PageSize int
	Total    int64
}

// List returns customers matching filter with pagination.
func (r *Repository) List(ctx context.Context, f ListFilter) (ListResult, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 200 {
		f.PageSize = 20
	}

	q := r.db.WithContext(ctx).Model(&Customer{})
	if f.OwnerID != nil {
		q = q.Where("created_by = ?", *f.OwnerID)
	}
	if trimmed := strings.TrimSpace(f.Query); trimmed != "" {
		// Use parameterized ILIKE. Escape %/_ to prevent wildcard injection.
		esc := escapeLike(trimmed)
		like := "%" + esc + "%"
		q = q.Where("name ILIKE ? OR coalesce(industry,'') ILIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return ListResult{}, err
	}

	var rows []Customer
	err := q.Order("created_at DESC").
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		Find(&rows).Error
	if err != nil {
		return ListResult{}, err
	}

	return ListResult{Items: rows, Page: f.Page, PageSize: f.PageSize, Total: total}, nil
}

// escapeLike turns user input into a SQL ILIKE-safe pattern fragment.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// Get fetches a customer by id. If ownerID is non-nil, the row must belong to that owner.
func (r *Repository) Get(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*Customer, error) {
	q := r.db.WithContext(ctx).Where("id = ?", id)
	if ownerID != nil {
		q = q.Where("created_by = ?", *ownerID)
	}
	var row Customer
	err := q.Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Create inserts a new customer. Returns ErrDuplicateName on unique violation.
func (r *Repository) Create(ctx context.Context, in *Customer) error {
	if in.ID == uuid.Nil {
		in.ID = uuid.New()
	}
	err := r.db.WithContext(ctx).Create(in).Error
	if isUniqueViolation(err) {
		return ErrDuplicateName
	}
	return err
}

// Patch describes the optional field updates. Only non-nil fields are applied.
type Patch struct {
	Name           *string
	Industry       *string
	IsReturning    *bool
	PriceSensitive *bool
	ContactName    *string
	ContactPhone   *string
	ContactEmail   *string
	Notes          *string
}

// Update applies the patch to the row. If ownerID is non-nil, the update is
// scoped to that owner (rows owned by others remain untouched and ErrNotFound
// is returned).
func (r *Repository) Update(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID, p Patch) (*Customer, error) {
	updates := map[string]any{}
	if p.Name != nil {
		updates["name"] = *p.Name
	}
	if p.Industry != nil {
		updates["industry"] = *p.Industry
	}
	if p.IsReturning != nil {
		updates["is_returning"] = *p.IsReturning
	}
	if p.PriceSensitive != nil {
		updates["price_sensitive"] = *p.PriceSensitive
	}
	if p.ContactName != nil {
		updates["contact_name"] = *p.ContactName
	}
	if p.ContactPhone != nil {
		updates["contact_phone"] = *p.ContactPhone
	}
	if p.ContactEmail != nil {
		updates["contact_email"] = *p.ContactEmail
	}
	if p.Notes != nil {
		updates["notes"] = *p.Notes
	}
	if len(updates) == 0 {
		return r.Get(ctx, id, ownerID)
	}
	updates["updated_at"] = gorm.Expr("NOW()")

	q := r.db.WithContext(ctx).Model(&Customer{}).Where("id = ?", id)
	if ownerID != nil {
		q = q.Where("created_by = ?", *ownerID)
	}
	res := q.Updates(updates)
	if isUniqueViolation(res.Error) {
		return nil, ErrDuplicateName
	}
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, id, ownerID)
}

// isUniqueViolation checks for Postgres 23505 SQLSTATE in the error chain.
// GORM wraps pgconn errors; stringify + substring is pragmatic and the only
// thing that works across drivers without pulling in a hard pgx dependency.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key")
}
