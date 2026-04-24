package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AdminService handles admin user-management use cases.
type AdminService struct {
	repo *Repository
}

// NewAdminService constructs an AdminService.
func NewAdminService(repo *Repository) *AdminService { return &AdminService{repo: repo} }

// ErrEmailTaken indicates a duplicate email on user creation.
var ErrEmailTaken = errors.New("admin: email already in use")

// CreateUser persists a new user with the given role.
func (a *AdminService) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
	// Duplicate email check.
	if existing, err := a.repo.FindUserByEmail(ctx, req.Email); err == nil && existing != nil {
		return nil, ErrEmailTaken
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	role, err := a.repo.FindRoleByCode(ctx, req.RoleCode)
	if err != nil {
		return nil, fmt.Errorf("find role: %w", err)
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	u := &User{
		ID:                uuid.New(),
		Name:              req.Name,
		Email:             req.Email,
		Phone:             req.Phone,
		PasswordHash:      hash,
		PasswordUpdatedAt: time.Now(),
		RoleID:            role.ID,
		Status:            "active",
	}
	if err := a.repo.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	u.Role = *role
	return u, nil
}

// UpdateUser applies the non-nil fields from req to user id.
func (a *AdminService) UpdateUser(ctx context.Context, id uuid.UUID, req UpdateUserRequest) (*User, error) {
	u, err := a.repo.FindUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	changedFields := make([]string, 0, 4)
	if req.Name != nil {
		u.Name = *req.Name
		changedFields = append(changedFields, "name")
	}
	if req.Phone != nil {
		u.Phone = *req.Phone
		changedFields = append(changedFields, "phone")
	}
	if req.RoleCode != nil {
		role, err := a.repo.FindRoleByCode(ctx, *req.RoleCode)
		if err != nil {
			return nil, err
		}
		u.RoleID = role.ID
		u.Role = *role
		changedFields = append(changedFields, "role_id")
	}
	if req.Status != nil {
		u.Status = *req.Status
		changedFields = append(changedFields, "status")
	}
	if len(changedFields) == 0 {
		return u, nil
	}
	if err := a.repo.UpdateUser(ctx, u, changedFields...); err != nil {
		return nil, err
	}
	return u, nil
}

// ResetPassword generates a random password, stores its hash, and returns the plaintext
// to the admin (only this one time) so they can hand it to the user out of band.
func (a *AdminService) ResetPassword(ctx context.Context, id uuid.UUID) (string, error) {
	user, err := a.repo.FindUserByID(ctx, id)
	if err != nil {
		return "", err
	}
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	plain := base64.RawURLEncoding.EncodeToString(raw)
	hash, err := HashPassword(plain)
	if err != nil {
		return "", err
	}
	user.PasswordHash = hash
	user.PasswordUpdatedAt = time.Now()
	if err := a.repo.UpdateUser(ctx, user, "password_hash", "password_updated_at"); err != nil {
		return "", err
	}
	return plain, nil
}

// ListUsers delegates to the repository.
func (a *AdminService) ListUsers(ctx context.Context, q, roleCode string, page, pageSize int) ([]User, int64, error) {
	return a.repo.ListUsers(ctx, q, roleCode, page, pageSize)
}
