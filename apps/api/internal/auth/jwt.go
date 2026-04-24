package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType discriminates access vs refresh tokens so a stolen access token
// cannot be used to refresh, and vice versa.
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// Claims is the JWT payload used by both token types.
type Claims struct {
	UserID    uuid.UUID `json:"-"`
	Role      string    `json:"role,omitempty"`
	TokenType TokenType `json:"typ"`
	jwt.RegisteredClaims
}

// IssueAccessToken signs a short-lived access token carrying the user's role.
func IssueAccessToken(secret []byte, userID uuid.UUID, role string, ttl time.Duration) (string, error) {
	return issue(secret, userID, role, TokenTypeAccess, ttl)
}

// IssueRefreshToken signs a long-lived refresh token without role info.
func IssueRefreshToken(secret []byte, userID uuid.UUID, ttl time.Duration) (string, error) {
	return issue(secret, userID, "", TokenTypeRefresh, ttl)
}

func issue(secret []byte, userID uuid.UUID, role string, typ TokenType, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userID,
		Role:      role,
		TokenType: typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(secret)
}

// ParseAccessToken verifies an access token's signature and type.
func ParseAccessToken(secret []byte, tokenString string) (*Claims, error) {
	return parse(secret, tokenString, TokenTypeAccess)
}

// ParseRefreshToken verifies a refresh token's signature and type.
func ParseRefreshToken(secret []byte, tokenString string) (*Claims, error) {
	return parse(secret, tokenString, TokenTypeRefresh)
}

func parse(secret []byte, tokenString string, expected TokenType) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.TokenType != expected {
		return nil, fmt.Errorf("token type %q, want %q", claims.TokenType, expected)
	}
	// Reconstruct UserID from Subject so the uuid.UUID field is populated.
	if id, err := uuid.Parse(claims.Subject); err == nil {
		claims.UserID = id
	}
	return claims, nil
}
