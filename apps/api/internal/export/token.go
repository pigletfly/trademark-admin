package export

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidToken indicates a download token is malformed, tampered with,
// or past its embedded expiry.
var ErrInvalidToken = errors.New("export: invalid download token")

// Signer HMACs export-file IDs so the download link is bearer-capable
// without requiring login. We still check expires_at server-side when
// serving the download so a token alone can't extend persistence.
//
// Tokens are bearer credentials with no replay protection; anyone with
// the URL can download until expiry. Keep TTLs short and treat access
// logs as sensitive.
type Signer struct{ secret []byte }

// NewSigner constructs a Signer. Panics if secret is shorter than 32
// bytes — HMAC-SHA256 is only as strong as its key, and accepting a
// short secret would mask configuration bugs (e.g. an unset env var
// falling through as an empty string).
func NewSigner(secret []byte) *Signer {
	if len(secret) < 32 {
		panic(fmt.Sprintf("export.NewSigner: secret must be >= 32 bytes, got %d", len(secret)))
	}
	return &Signer{secret: secret}
}

// Sign returns a base64url token encoding: "<id>.<expiresUnix>.<sig>".
// `expires` is the ABSOLUTE UTC expiry — the first second at which the
// token will be rejected. Verify compares both the embedded expiry AND
// the signature so a tampered expiry breaks the MAC.
func (s *Signer) Sign(id uuid.UUID, expires time.Time) string {
	payload := fmt.Sprintf("%s.%d", id, expires.Unix())
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// Verify returns the export-file ID if the token is intact and unexpired.
func (s *Signer) Verify(token string) (uuid.UUID, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return uuid.Nil, ErrInvalidToken
	}
	idStr, expStr, sig := parts[0], parts[1], parts[2]
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	if time.Now().Unix() >= exp {
		return uuid.Nil, ErrInvalidToken
	}
	payload := idStr + "." + expStr
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(sig)) {
		return uuid.Nil, ErrInvalidToken
	}
	return id, nil
}
