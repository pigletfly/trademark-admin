package auth_test

import (
	"strings"
	"testing"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

func TestHashPassword_producesPHCEncodedHash(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("unexpected prefix: %q", hash)
	}
}

func TestVerifyPassword_matchAndMismatch(t *testing.T) {
	hash, err := auth.HashPassword("super-secret-1")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := auth.VerifyPassword("super-secret-1", hash)
	if err != nil || !ok {
		t.Fatalf("expected match, got ok=%v err=%v", ok, err)
	}
	ok, err = auth.VerifyPassword("super-secret-2", hash)
	if err != nil {
		t.Fatalf("VerifyPassword returned unexpected error on mismatch: %v", err)
	}
	if ok {
		t.Fatalf("expected mismatch, got ok=true")
	}
}

func TestHashPassword_uniqueSaltPerCall(t *testing.T) {
	h1, _ := auth.HashPassword("pw")
	h2, _ := auth.HashPassword("pw")
	if h1 == h2 {
		t.Fatalf("two hashes of same password collided; salt not random? h=%q", h1)
	}
}
