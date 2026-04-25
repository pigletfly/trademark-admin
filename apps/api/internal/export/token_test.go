package export_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
)

func TestSigner_RoundTrip(t *testing.T) {
	s := export.NewSigner([]byte("test-secret-32-bytes-min-length!"))
	id := uuid.New()
	exp := time.Now().Add(24 * time.Hour)

	tok := s.Sign(id, exp)
	got, err := s.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got != id {
		t.Fatalf("id mismatch: got=%s want=%s", got, id)
	}
}

func TestSigner_Expired(t *testing.T) {
	s := export.NewSigner([]byte("test-secret-32-bytes-min-length!"))
	tok := s.Sign(uuid.New(), time.Now().Add(-1*time.Second))
	if _, err := s.Verify(tok); err != export.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestSigner_Tampered(t *testing.T) {
	s := export.NewSigner([]byte("test-secret-32-bytes-min-length!"))
	tok := s.Sign(uuid.New(), time.Now().Add(time.Hour))
	// flip one char in the signature segment
	tampered := tok[:len(tok)-1] + "X"
	if _, err := s.Verify(tampered); err != export.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestSigner_DifferentSecret(t *testing.T) {
	s1 := export.NewSigner([]byte("secret-one-32-bytes-min-length!!"))
	s2 := export.NewSigner([]byte("secret-two-32-bytes-min-length!!"))
	tok := s1.Sign(uuid.New(), time.Now().Add(time.Hour))
	if _, err := s2.Verify(tok); err != export.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestSigner_Malformed(t *testing.T) {
	s := export.NewSigner([]byte("secret-32-bytes-min-length-here!"))
	// zero-part
	if _, err := s.Verify(""); err != export.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for empty, got %v", err)
	}
	// only two parts
	if _, err := s.Verify("a.b"); err != export.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for 2-part, got %v", err)
	}
	// bad uuid
	if _, err := s.Verify("not-uuid.1700000000.sig"); err != export.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for bad uuid, got %v", err)
	}
	// bad expiry int
	if _, err := s.Verify(uuid.New().String() + ".not-int.sig"); err != export.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for bad expiry, got %v", err)
	}
}

func TestSigner_TamperedExpiry(t *testing.T) {
	s := export.NewSigner([]byte("secret-32-bytes-min-length-here!"))
	id := uuid.New()
	tok := s.Sign(id, time.Now().Add(time.Hour))
	// Swap expiry portion for a future far-later value — signature now invalid.
	parts := []byte(tok)
	// Replace the middle segment. Simpler: manually construct a tampered token.
	// Parse tok into 3 parts, then re-assemble with a different expiry.
	dotPositions := []int{}
	for i, b := range parts {
		if b == '.' {
			dotPositions = append(dotPositions, i)
		}
	}
	if len(dotPositions) != 2 {
		t.Fatalf("token shape unexpected: %q", tok)
	}
	before := tok[:dotPositions[0]+1]
	after := tok[dotPositions[1]:]
	tampered := before + "9999999999" + after
	if _, err := s.Verify(tampered); err != export.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken on tampered expiry, got %v", err)
	}
}

func TestSigner_ShortSecretPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for short secret")
		}
	}()
	_ = export.NewSigner([]byte("too-short"))
}
