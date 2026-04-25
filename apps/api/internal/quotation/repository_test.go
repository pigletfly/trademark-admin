package quotation_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"
	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// bootPg spins up a fresh Postgres container, applies migrations, and
// returns a GORM handle + the raw DSN.
func bootPg(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("quot_test"),
		tcpostgres.WithUsername("quot"),
		tcpostgres.WithPassword("quot"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("postgres container: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	mig, err := migrator.New(api.Migrations, "migrations", dsn)
	if err != nil {
		t.Fatalf("migrator: %v", err)
	}
	if err := mig.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	_ = mig.Close()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	return db, dsn
}

// seedCustomerCountryUser inserts the minimum FK targets so a quotation
// row can exist. Returns (customerID, countryID, userID).
func seedCustomerCountryUser(t *testing.T, db *gorm.DB) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	custID := uuid.New()
	countryID := uuid.New()
	userID := uuid.New()
	// users table has role_id FK — look up salesperson role id.
	// GORM+pgx returns UUID columns as text strings, so scan via string first.
	var roleIDStr string
	if err := db.WithContext(ctx).Raw(
		`SELECT id FROM roles WHERE code = ?`, "salesperson",
	).Scan(&roleIDStr).Error; err != nil {
		t.Fatalf("select role: %v", err)
	}
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		t.Fatalf("parse role id: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		userID, "Tester", "tester-"+userID.String()+"@example.com", "x", roleID,
	).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO customers (id, name, created_by) VALUES (?, ?, ?)`,
		custID, "Test Customer", userID,
	).Error; err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO countries (id, code, name_zh, name_en) VALUES (?, ?, ?, ?)`,
		countryID, "XX", "测试国", "Testland",
	).Error; err != nil {
		t.Fatalf("seed country: %v", err)
	}
	return custID, countryID, userID
}

func TestRepository_CreateGet(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, userID := seedCustomerCountryUser(t, db)
	r := quotation.NewRepository(db)

	q := &quotation.Quotation{
		CustomerID: custID, CountryID: countryID,
		ServiceTier: "basic", Status: quotation.StatusDraft,
		CreatedBy: userID,
	}
	if err := r.Create(context.Background(), q); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := r.Get(context.Background(), q.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.Status != quotation.StatusDraft {
		t.Fatalf("got %+v", got)
	}
}

func TestRepository_TransitionRecordsHistory(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, userID := seedCustomerCountryUser(t, db)
	r := quotation.NewRepository(db)

	// Create draft.
	q := &quotation.Quotation{
		CustomerID: custID, CountryID: countryID,
		ServiceTier: "basic", Status: quotation.StatusDraft,
		CreatedBy: userID,
	}
	if err := r.Create(context.Background(), q); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Transition to submitted with a fake snapshot payload.
	snap, _ := json.Marshal(quotation.Snapshot{
		Lines:         []quotation.SnapshotLine{{FeeItem: "f", AmountCNYCents: 1000}},
		TotalCNYCents: 1000, Signature: "sig",
	})
	q.SnapshotJSON = audit.JSONB(snap)
	total := int64(1000)
	sig := "sig"
	now := time.Now()
	serial := "Q202604260001"
	q.TotalCNYCents = &total
	q.Signature = &sig
	q.SubmittedAt = &now
	q.SerialNo = &serial
	if err := r.Transition(context.Background(), q, quotation.StatusSubmitted, userID, nil); err != nil {
		t.Fatalf("transition: %v", err)
	}
	got, _ := r.Get(context.Background(), q.ID)
	if got.Status != quotation.StatusSubmitted || got.Signature == nil || *got.Signature != "sig" {
		t.Fatalf("after submit: %+v", got)
	}

	hist, err := r.History(context.Background(), q.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(hist) != 1 || hist[0].FromStatus != quotation.StatusDraft || hist[0].ToStatus != quotation.StatusSubmitted {
		t.Fatalf("history: %+v", hist)
	}
}

func TestRepository_CheckConstraintBlocksSubmittedWithoutSnapshot(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, userID := seedCustomerCountryUser(t, db)
	r := quotation.NewRepository(db)

	// Directly attempt to insert a submitted quotation with no snapshot.
	q := &quotation.Quotation{
		CustomerID: custID, CountryID: countryID,
		ServiceTier: "basic", Status: quotation.StatusSubmitted,
		CreatedBy: userID,
	}
	if err := r.Create(context.Background(), q); err == nil {
		t.Fatal("expected CHECK violation on submitted without snapshot")
	}
}

func TestRepository_ListFilters(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, userID := seedCustomerCountryUser(t, db)
	r := quotation.NewRepository(db)

	// Insert 3 quotations — 2 owned by userID as draft, 1 by a different
	// user with status cancelled.
	for i := 0; i < 2; i++ {
		_ = r.Create(context.Background(), &quotation.Quotation{
			CustomerID: custID, CountryID: countryID,
			ServiceTier: "basic", Status: quotation.StatusDraft,
			CreatedBy: userID,
		})
	}
	// Seed a second user to own the third row.
	var roleIDStr string
	_ = db.Raw(`SELECT id FROM roles WHERE code = 'salesperson'`).Scan(&roleIDStr).Error
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		t.Fatalf("parse role id: %v", err)
	}
	other := uuid.New()
	_ = db.Exec(`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		other, "Other", "other-"+other.String()+"@example.com", "x", roleID).Error
	_ = r.Create(context.Background(), &quotation.Quotation{
		CustomerID: custID, CountryID: countryID,
		ServiceTier: "basic", Status: quotation.StatusDraft,
		CreatedBy: other,
	})

	// Owner filter → 2 rows.
	ownFilter := userID
	got, total, err := r.List(context.Background(), quotation.ListFilter{OwnerID: &ownFilter})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(got) != 2 {
		t.Fatalf("owner filter: total=%d got=%d", total, len(got))
	}

	// Status filter → all 3 are drafts.
	draftStatus := quotation.StatusDraft
	_, total, _ = r.List(context.Background(), quotation.ListFilter{Status: &draftStatus})
	if total != 3 {
		t.Fatalf("draft total: want 3, got %d", total)
	}
}

// TestRepository_TransitionRejectsStaleFrom verifies that Transition
// checks RowsAffected and returns ErrInvalidTransition when the row has
// already moved — so two concurrent Submit calls cannot both append
// history rows with divergent snapshots.
func TestRepository_TransitionRejectsStaleFrom(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, userID := seedCustomerCountryUser(t, db)
	r := quotation.NewRepository(db)

	q := &quotation.Quotation{
		CustomerID: custID, CountryID: countryID,
		ServiceTier: "basic", Status: quotation.StatusDraft,
		CreatedBy: userID,
	}
	if err := r.Create(context.Background(), q); err != nil {
		t.Fatalf("create: %v", err)
	}

	// First submit wins.
	snap, _ := json.Marshal(quotation.Snapshot{
		Lines:         []quotation.SnapshotLine{{FeeItem: "f", AmountCNYCents: 1000}},
		TotalCNYCents: 1000, Signature: "sig",
	})
	total := int64(1000)
	sig := "sig"
	now := time.Now()
	serial := "Q202604260001"
	q.SnapshotJSON = audit.JSONB(snap)
	q.TotalCNYCents = &total
	q.Signature = &sig
	q.SubmittedAt = &now
	q.SerialNo = &serial
	if err := r.Transition(context.Background(), q, quotation.StatusSubmitted, userID, nil); err != nil {
		t.Fatalf("first submit: %v", err)
	}

	// Second caller simulates a racer: still holds a stale in-memory
	// snapshot claiming status=draft. The guarded UPDATE matches zero
	// rows now, so Transition must refuse.
	stale := &quotation.Quotation{
		ID: q.ID, CustomerID: custID, CountryID: countryID,
		ServiceTier: "basic", Status: quotation.StatusDraft,
		CreatedBy: userID,
	}
	stale.SnapshotJSON = audit.JSONB(snap)
	stale.TotalCNYCents = &total
	stale.Signature = &sig
	stale.SubmittedAt = &now
	err := r.Transition(context.Background(), stale, quotation.StatusSubmitted, userID, nil)
	if err == nil {
		t.Fatal("second submit: expected error, got nil — race is silent")
	}
	if err != quotation.ErrInvalidTransition {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}

	// History must NOT contain two rows.
	hist, _ := r.History(context.Background(), q.ID)
	if len(hist) != 1 {
		t.Fatalf("history rows: want 1, got %d — racer leaked a row", len(hist))
	}
}

// TestRepository_WithdrawClearsSnapshotAndKeepsSerial verifies that
// Withdraw reverts a submitted quotation to draft, NULLs out the
// snapshot/total/signature columns (required by
// chk_quotations_snapshot_when_nondraft), preserves serial_no, and
// appends a status_history row.
func TestRepository_WithdrawClearsSnapshotAndKeepsSerial(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, userID := seedCustomerCountryUser(t, db)
	r := quotation.NewRepository(db)

	// Create draft.
	q := &quotation.Quotation{
		CustomerID: custID, CountryID: countryID,
		ServiceTier: "basic", Status: quotation.StatusDraft,
		CreatedBy: userID,
	}
	if err := r.Create(context.Background(), q); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Transition to submitted with filled snapshot + serial.
	snap, _ := json.Marshal(quotation.Snapshot{
		Lines:         []quotation.SnapshotLine{{FeeItem: "f", AmountCNYCents: 1000}},
		TotalCNYCents: 1000, Signature: "sig",
	})
	total := int64(1000)
	sig := "sig"
	now := time.Now()
	serial := "Q202604260001"
	q.SnapshotJSON = audit.JSONB(snap)
	q.TotalCNYCents = &total
	q.Signature = &sig
	q.SubmittedAt = &now
	q.SerialNo = &serial
	if err := r.Transition(context.Background(), q, quotation.StatusSubmitted, userID, nil); err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Withdraw back to draft.
	if err := r.Withdraw(context.Background(), q, userID); err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	got, err := r.Get(context.Background(), q.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != quotation.StatusDraft {
		t.Fatalf("status = %q, want draft", got.Status)
	}
	if len(got.SnapshotJSON) != 0 {
		t.Fatalf("snapshot_json: want nil/empty, got %q", string(got.SnapshotJSON))
	}
	if got.TotalCNYCents != nil {
		t.Fatalf("total_cny_cents: want nil, got %v", *got.TotalCNYCents)
	}
	if got.Signature != nil {
		t.Fatalf("signature: want nil, got %q", *got.Signature)
	}
	if got.SerialNo == nil || *got.SerialNo != serial {
		t.Fatalf("serial_no: want %q preserved, got %v", serial, got.SerialNo)
	}

	hist, err := r.History(context.Background(), q.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("history: want 2 rows (submit + withdraw), got %d", len(hist))
	}
	if hist[1].FromStatus != quotation.StatusSubmitted || hist[1].ToStatus != quotation.StatusDraft {
		t.Fatalf("withdraw history row: from=%q to=%q, want submitted→draft", hist[1].FromStatus, hist[1].ToStatus)
	}
}

// This silences the unused-import warning if pricing is elided.
var _ = pricing.ServiceTiers
