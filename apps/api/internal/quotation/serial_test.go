package quotation_test

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
)

// runInTx wraps GenerateSerialAt in a fresh transaction so the advisory
// lock has a transaction to attach to. The tx is committed on success
// (a no-op commit — no INSERT is made here; prior rows seeded by
// insertQuotationWithSerial are already committed and visible to the
// MAX query inside the tx).
func runInTx(t *testing.T, db *gorm.DB, day time.Time) string {
	t.Helper()
	var serial string
	err := db.Transaction(func(tx *gorm.DB) error {
		s, err := quotation.GenerateSerialAt(context.Background(), tx, day)
		if err != nil {
			return err
		}
		serial = s
		return nil
	})
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
	return serial
}

// insertQuotationWithSerial persists a draft quotation with the given
// serial so MAX(serial_no) sees it in subsequent calls. The CHECK
// constraint allows draft + non-null serial_no (it only requires
// non-null when status != 'draft').
func insertQuotationWithSerial(t *testing.T, db *gorm.DB, custID, countryID, userID interface{}, serial string) {
	t.Helper()
	err := db.Exec(`
		INSERT INTO quotations (id, customer_id, country_id, service_tier, status, created_by, serial_no)
		VALUES (gen_random_uuid(), ?, ?, 'standard', 'draft', ?, ?)
	`, custID, countryID, userID, serial).Error
	if err != nil {
		t.Fatalf("insert quotation: %v", err)
	}
}

func TestGenerateSerial_FirstOfDay(t *testing.T) {
	db, _ := bootPg(t)
	day := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	got := runInTx(t, db, day)
	if got != "Q202605010001" {
		t.Fatalf("got %q, want Q202605010001", got)
	}
}

func TestGenerateSerial_IncrementsSameDay(t *testing.T) {
	db, _ := bootPg(t)
	day := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	custID, countryID, userID := seedCustomerCountryUser(t, db)

	// Insert a draft with serial_no pre-populated to simulate a
	// previously-generated serial being persisted.
	insertQuotationWithSerial(t, db, custID, countryID, userID, "Q202605020001")

	got := runInTx(t, db, day)
	if got != "Q202605020002" {
		t.Fatalf("got %q, want Q202605020002", got)
	}
}

func TestGenerateSerial_ResetsNextDay(t *testing.T) {
	db, _ := bootPg(t)
	day1 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	custID, countryID, userID := seedCustomerCountryUser(t, db)

	insertQuotationWithSerial(t, db, custID, countryID, userID, "Q202605030001")
	insertQuotationWithSerial(t, db, custID, countryID, userID, "Q202605030002")

	got := runInTx(t, db, day2)
	if got != "Q202605040001" {
		t.Fatalf("got %q, want Q202605040001 (reset on new day)", got)
	}
	_ = day1 // referenced for clarity
}
