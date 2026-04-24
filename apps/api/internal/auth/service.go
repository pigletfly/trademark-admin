package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// LoginResult bundles tokens and the user profile returned to clients.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	User         *User
}

// RefreshResult is returned by Service.Refresh.
type RefreshResult struct {
	AccessToken  string
	RefreshToken string // may be rotated; MVP reuses same for simplicity
	User         *User
}

// ServiceConfig bundles the dependencies Service needs.
type ServiceConfig struct {
	Repo          *Repository
	AccessSecret  []byte
	RefreshSecret []byte
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

// Service offers login, refresh, me, and bootstrap use-cases.
type Service struct {
	cfg ServiceConfig
}

// NewService constructs a Service.
func NewService(cfg ServiceConfig) *Service { return &Service{cfg: cfg} }

// ErrInvalidCredentials is returned when email or password does not match.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// ErrUserDisabled is returned when the account exists but is not active.
var ErrUserDisabled = errors.New("auth: user disabled")

// Bootstrap creates the very first admin user if the users table is empty.
// It is idempotent: on a non-empty table it returns nil.
func (s *Service) Bootstrap(ctx context.Context, email, password, displayName string) error {
	n, err := s.cfg.Repo.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if n > 0 {
		return nil
	}
	role, err := s.cfg.Repo.FindRoleByCode(ctx, "admin")
	if err != nil {
		return fmt.Errorf("find admin role: %w", err)
	}
	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	now := time.Now()
	u := &User{
		ID:                uuid.New(),
		Name:              displayName,
		Email:             email,
		PasswordHash:      hash,
		PasswordUpdatedAt: now,
		RoleID:            role.ID,
		Status:            "active",
	}
	return s.cfg.Repo.CreateUser(ctx, u)
}

// Login validates credentials and issues tokens.
func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	user, err := s.cfg.Repo.FindUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if user.Status != "active" {
		return nil, ErrUserDisabled
	}
	ok, err := VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}
	if !ok {
		return nil, ErrInvalidCredentials
	}
	access, err := IssueAccessToken(s.cfg.AccessSecret, user.ID, user.Role.Code, s.cfg.AccessTTL)
	if err != nil {
		return nil, fmt.Errorf("issue access: %w", err)
	}
	refresh, err := IssueRefreshToken(s.cfg.RefreshSecret, user.ID, s.cfg.RefreshTTL)
	if err != nil {
		return nil, fmt.Errorf("issue refresh: %w", err)
	}
	return &LoginResult{AccessToken: access, RefreshToken: refresh, User: user}, nil
}

// Refresh validates the refresh token and issues a new access token.
// MVP does not rotate refresh tokens (returns the same one) — acceptable because
// refresh TTL is short-ish (7 days) and the cookie is httpOnly + SameSite=Lax.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*RefreshResult, error) {
	claims, err := ParseRefreshToken(s.cfg.RefreshSecret, refreshToken)
	if err != nil {
		return nil, err
	}
	user, err := s.cfg.Repo.FindUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, ErrUserDisabled
	}
	access, err := IssueAccessToken(s.cfg.AccessSecret, user.ID, user.Role.Code, s.cfg.AccessTTL)
	if err != nil {
		return nil, err
	}
	return &RefreshResult{AccessToken: access, RefreshToken: refreshToken, User: user}, nil
}

// Me looks up a user by ID (used by /auth/me handler).
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (*User, error) {
	return s.cfg.Repo.FindUserByID(ctx, userID)
}
