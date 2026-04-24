package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	// ErrNotFound indicates the queried entity does not exist.
	ErrNotFound = errors.New("auth: not found")
)

// Repository encapsulates user/role persistence.
type Repository struct {
	db *gorm.DB
}

// NewRepository constructs a Repository bound to the given GORM handle.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// FindRoleByCode returns the role with the given code or ErrNotFound.
func (r *Repository) FindRoleByCode(ctx context.Context, code string) (*Role, error) {
	var role Role
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// FindUserByEmail returns the user with the given email (with Role preloaded).
func (r *Repository) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).
		Preload("Role").
		Where("email = ?", email).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindUserByID returns the user with the given id (with Role preloaded).
func (r *Repository) FindUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).
		Preload("Role").
		First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateUser persists a new user. Fills ID if zero.
func (r *Repository) CreateUser(ctx context.Context, u *User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(u).Error
}

// CountUsers returns the total number of users (used for bootstrap decisions).
func (r *Repository) CountUsers(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&User{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// UpdateUser persists changes to an existing user. Uses full-struct update
// (columns listed in fields). Empty fields slice updates all non-zero columns.
func (r *Repository) UpdateUser(ctx context.Context, u *User, fields ...string) error {
	q := r.db.WithContext(ctx).Model(u)
	if len(fields) > 0 {
		q = q.Select(fields)
	}
	return q.Updates(u).Error
}

// ListUsers returns users filtered by optional email prefix and role code.
// page is 1-based.
func (r *Repository) ListUsers(ctx context.Context, q string, roleCode string, page, pageSize int) ([]User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	query := r.db.WithContext(ctx).Model(&User{}).Preload("Role")
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("email ILIKE ? OR name ILIKE ?", like, like)
	}
	if roleCode != "" {
		query = query.Joins("JOIN roles ON roles.id = users.role_id").
			Where("roles.code = ?", roleCode)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var out []User
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).
		Order("users.created_at DESC").Find(&out).Error
	return out, total, err
}
