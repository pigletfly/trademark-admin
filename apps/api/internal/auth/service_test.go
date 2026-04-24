//go:build integration

package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

func TestService_BootstrapAdmin(t *testing.T) {
	db := freshDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(auth.ServiceConfig{
		Repo:          repo,
		AccessSecret:  []byte("a"),
		RefreshSecret: []byte("r"),
		AccessTTL:     5 * time.Minute,
		RefreshTTL:    time.Hour,
	})

	if err := svc.Bootstrap(context.Background(), "root@example.com", "initial-pass-123", "Root Admin"); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	n, _ := repo.CountUsers(context.Background())
	if n != 1 {
		t.Fatalf("expected 1 user, got %d", n)
	}
	// Running Bootstrap again is a no-op when a user already exists.
	if err := svc.Bootstrap(context.Background(), "root@example.com", "initial-pass-123", "Root Admin"); err != nil {
		t.Fatalf("second Bootstrap: %v", err)
	}
	n, _ = repo.CountUsers(context.Background())
	if n != 1 {
		t.Fatalf("Bootstrap should be idempotent, got %d users", n)
	}
}

func TestService_LoginSuccess(t *testing.T) {
	db := freshDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(auth.ServiceConfig{
		Repo:          repo,
		AccessSecret:  []byte("a"),
		RefreshSecret: []byte("r"),
		AccessTTL:     5 * time.Minute,
		RefreshTTL:    time.Hour,
	})
	ctx := context.Background()

	if err := svc.Bootstrap(ctx, "root@example.com", "pw-abcdefg-1234", "Root"); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	result, err := svc.Login(ctx, "root@example.com", "pw-abcdefg-1234")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		t.Fatalf("tokens empty")
	}
	if result.User.Email != "root@example.com" {
		t.Fatalf("user email mismatch")
	}
	if result.User.Role.Code != "admin" {
		t.Fatalf("bootstrapped user should have admin role, got %q", result.User.Role.Code)
	}
}

func TestService_LoginWrongPassword(t *testing.T) {
	db := freshDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(auth.ServiceConfig{
		Repo:          repo,
		AccessSecret:  []byte("a"),
		RefreshSecret: []byte("r"),
		AccessTTL:     5 * time.Minute,
		RefreshTTL:    time.Hour,
	})
	ctx := context.Background()
	_ = svc.Bootstrap(ctx, "root@example.com", "pw-abcdefg-1234", "Root")

	_, err := svc.Login(ctx, "root@example.com", "wrong-password")
	if err == nil {
		t.Fatalf("expected login error")
	}
}

func TestService_Refresh(t *testing.T) {
	db := freshDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(auth.ServiceConfig{
		Repo:          repo,
		AccessSecret:  []byte("a"),
		RefreshSecret: []byte("r"),
		AccessTTL:     5 * time.Minute,
		RefreshTTL:    time.Hour,
	})
	ctx := context.Background()
	_ = svc.Bootstrap(ctx, "root@example.com", "pw-abcdefg-1234", "Root")

	login, err := svc.Login(ctx, "root@example.com", "pw-abcdefg-1234")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	refreshed, err := svc.Refresh(ctx, login.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if refreshed.AccessToken == "" {
		t.Fatalf("Refresh must return a new access token")
	}
}
