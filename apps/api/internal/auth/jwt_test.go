package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

func TestIssueAndParseAccessToken(t *testing.T) {
	secret := []byte("access-secret")
	userID := uuid.New()
	role := "admin"

	token, err := auth.IssueAccessToken(secret, userID, role, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	claims, err := auth.ParseAccessToken(secret, token)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID)
	}
	if claims.Role != role {
		t.Errorf("Role = %q, want %q", claims.Role, role)
	}
	if claims.TokenType != "access" {
		t.Errorf("TokenType = %q, want access", claims.TokenType)
	}
}

func TestAccessTokenExpires(t *testing.T) {
	secret := []byte("access-secret")
	userID := uuid.New()

	token, err := auth.IssueAccessToken(secret, userID, "admin", -1*time.Second)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	if _, err := auth.ParseAccessToken(secret, token); err == nil {
		t.Fatalf("expected expiration error")
	}
}

func TestRefreshTokenSeparateSecret(t *testing.T) {
	access := []byte("access-secret")
	refresh := []byte("refresh-secret")
	userID := uuid.New()

	token, err := auth.IssueRefreshToken(refresh, userID, time.Hour)
	if err != nil {
		t.Fatalf("IssueRefreshToken: %v", err)
	}

	// Wrong secret (access secret) must not parse a refresh token.
	if _, err := auth.ParseRefreshToken(access, token); err == nil {
		t.Fatalf("expected signature error")
	}
	claims, err := auth.ParseRefreshToken(refresh, token)
	if err != nil {
		t.Fatalf("ParseRefreshToken: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID)
	}
	if claims.TokenType != "refresh" {
		t.Errorf("TokenType = %q, want refresh", claims.TokenType)
	}
}

func TestAccessTokenTypeNotAcceptedAsRefresh(t *testing.T) {
	secret := []byte("same-secret-wrong-usage")
	userID := uuid.New()
	accessToken, _ := auth.IssueAccessToken(secret, userID, "admin", time.Hour)
	if _, err := auth.ParseRefreshToken(secret, accessToken); err == nil {
		t.Fatalf("expected type mismatch error")
	}
}
