package quotation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Advisory lock key for serial generation. Fixed (1, 1) — no other code
// in this repo uses pg_advisory_xact_lock today (grep confirmed). Both
// ints are arbitrary; callers do not need to know them.
const (
	advisoryLockDomainQuotation int32 = 1
	advisoryLockResourceSerial  int32 = 1
)

// ErrSerialExhausted is returned when a day already has 9999 serials.
var ErrSerialExhausted = errors.New("quotation: daily serial exhausted (max 9999)")

// GenerateSerialAt returns the next serial number for the given day.
// Format: "Q" + YYYYMMDD + 4-digit daily sequence (zero-padded).
//
// IMPORTANT: must be called inside a transaction (tx MUST be a
// transactional *gorm.DB, e.g. from db.Transaction(...) or db.Begin()).
// The advisory lock is transaction-scoped: calling this outside a tx
// silently releases the lock immediately and defeats the purpose.
//
// Concurrent callers for the same day queue on the advisory lock;
// the MAX(serial_no) subquery is day-scoped via LIKE prefix, so
// different-day calls contend briefly but produce correct sequences.
//
// Returns ErrSerialExhausted if the day has already used >= 9999
// serials (rare but must be surfaced rather than wrapping to 0000).
func GenerateSerialAt(ctx context.Context, tx *gorm.DB, day time.Time) (string, error) {
	if err := tx.WithContext(ctx).Exec(
		"SELECT pg_advisory_xact_lock(?, ?)",
		advisoryLockDomainQuotation, advisoryLockResourceSerial,
	).Error; err != nil {
		return "", fmt.Errorf("quotation: advisory lock: %w", err)
	}

	prefix := "Q" + day.UTC().Format("20060102")
	var maxSerial sql.NullString
	err := tx.WithContext(ctx).Raw(`
		SELECT MAX(serial_no)
		FROM quotations
		WHERE serial_no LIKE ?
	`, prefix+"%").Scan(&maxSerial).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("quotation: max serial: %w", err)
	}

	next := 1
	if maxSerial.Valid && len(maxSerial.String) == len(prefix)+4 {
		var seq int
		if _, scanErr := fmt.Sscanf(maxSerial.String[len(prefix):], "%04d", &seq); scanErr == nil {
			next = seq + 1
		}
	}
	if next > 9999 {
		return "", ErrSerialExhausted
	}

	return fmt.Sprintf("%s%04d", prefix, next), nil
}
