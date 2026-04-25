package dashboard_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
	"github.com/pigletfly/trademark-admin/apps/api/internal/dashboard"
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// bootPg spins up a fresh Postgres 16 container + applies migrations.
func bootPg(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("dash_test"),
		tcpostgres.WithUsername("dash"),
		tcpostgres.WithPassword("dash"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("container: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	dsn, _ := container.ConnectionString(ctx, "sslmode=disable")
	mig, _ := migrator.New(api.Migrations, "migrations", dsn)
	if err := mig.Up(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = mig.Close()
	db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	return db
}

// seed plants 2 salespeople, some customers, and several quotations
// across different statuses. Returns the two user IDs.
func seed(t *testing.T, db *gorm.DB) (alice, bob uuid.UUID) {
	t.Helper()
	var roleIDStr string
	_ = db.Raw(`SELECT id FROM roles WHERE code = 'salesperson'`).Scan(&roleIDStr).Error
	roleID, _ := uuid.Parse(roleIDStr)

	alice = uuid.New()
	bob = uuid.New()
	if err := db.Exec(`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?), (?, ?, ?, ?, ?)`,
		alice, "Alice", "alice@ex.com", "x", roleID,
		bob, "Bob", "bob@ex.com", "x", roleID).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}

	countryID := uuid.New()
	_ = db.Exec(`INSERT INTO countries (id, code, name_zh, name_en) VALUES (?, ?, ?, ?)`,
		countryID, "CN", "中国", "China").Error

	// Alice has 2 customers (1 from this week, 1 from 60d ago).
	cust1, cust2 := uuid.New(), uuid.New()
	sixtyDaysAgo := time.Now().Add(-60 * 24 * time.Hour)
	_ = db.Exec(`INSERT INTO customers (id, name, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?), (?, ?, ?, NOW(), NOW())`,
		cust1, "Old Acme", alice, sixtyDaysAgo, sixtyDaysAgo,
		cust2, "New Acme", alice).Error

	// Bob has 1 recent customer.
	cust3 := uuid.New()
	_ = db.Exec(`INSERT INTO customers (id, name, created_by) VALUES (?, ?, ?)`,
		cust3, "Bob Customer", bob).Error

	// Alice has: 1 draft, 1 approved (¥100.00), 1 rejected.
	// Bob has: 1 submitted.
	// Non-draft rows need serial_no to satisfy
	// chk_quotations_serial_no_when_nondraft (M2 migration 000006);
	// serial_no uniqueness is enforced so give each a distinct value.
	approvedSnap := `{"lines":[{"fee_item":"application","amount_cny_cents":10000}],"total_cny_cents":10000,"signature":"sig-a"}`
	totalA := int64(10000)
	now := time.Now()
	aDraft, aApproved, aRejected, bSub := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := db.Exec(`INSERT INTO quotations
		(id, customer_id, country_id, service_tier, status, snapshot_json, total_cny_cents, signature, serial_no, submitted_at, reviewed_at, reviewed_by, created_by)
		VALUES
		(?, ?, ?, 'basic', 'draft', NULL, NULL, NULL, NULL, NULL, NULL, NULL, ?),
		(?, ?, ?, 'basic', 'approved', ?, ?, 'sig-a', ?, ?, ?, ?, ?),
		(?, ?, ?, 'basic', 'rejected', ?, ?, 'sig-a', ?, ?, ?, ?, ?),
		(?, ?, ?, 'basic', 'submitted', ?, ?, 'sig-b', ?, ?, NULL, NULL, ?)`,
		aDraft, cust1, countryID, alice,
		aApproved, cust1, countryID, approvedSnap, totalA, "Q202604260001", now, now, alice, alice,
		aRejected, cust1, countryID, approvedSnap, totalA, "Q202604260002", now, now, alice, alice,
		bSub, cust3, countryID, approvedSnap, totalA, "Q202604260003", now, bob,
	).Error; err != nil {
		t.Fatalf("seed quotations: %v", err)
	}
	return alice, bob
}

func TestSummary_SalespersonSeesOwnOnly(t *testing.T) {
	db := bootPg(t)
	alice, _ := seed(t, db)
	svc := dashboard.NewService(quotation.NewRepository(db), customer.NewRepository(db))

	out, err := svc.Summary(context.Background(), alice, "salesperson")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if out.Scope != "self" {
		t.Fatalf("scope = %q, want self", out.Scope)
	}
	// Alice has 3 quotations: draft, approved, rejected.
	totalByStatus := map[quotation.Status]int64{}
	for _, c := range out.QuotationsByStatus {
		totalByStatus[c.Status] = c.Count
	}
	if totalByStatus[quotation.StatusDraft] != 1 || totalByStatus[quotation.StatusApproved] != 1 || totalByStatus[quotation.StatusRejected] != 1 {
		t.Fatalf("alice counts = %+v, want 1 draft/1 approved/1 rejected", totalByStatus)
	}
	if totalByStatus[quotation.StatusSubmitted] != 0 {
		t.Fatalf("alice should have 0 submitted (bob's), got %d", totalByStatus[quotation.StatusSubmitted])
	}
	if out.ApprovedTotalCNYCents != 10000 {
		t.Fatalf("alice approved total = %d, want 10000", out.ApprovedTotalCNYCents)
	}
	// Only the recent (non-60-day-old) customer should count.
	if out.NewCustomersLast30Days != 1 {
		t.Fatalf("alice new customers 30d = %d, want 1", out.NewCustomersLast30Days)
	}
}

func TestSummary_ReviewerSeesFirm(t *testing.T) {
	db := bootPg(t)
	alice, _ := seed(t, db)
	svc := dashboard.NewService(quotation.NewRepository(db), customer.NewRepository(db))

	out, err := svc.Summary(context.Background(), alice, "reviewer")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if out.Scope != "firm" {
		t.Fatalf("scope = %q, want firm", out.Scope)
	}
	totalByStatus := map[quotation.Status]int64{}
	for _, c := range out.QuotationsByStatus {
		totalByStatus[c.Status] = c.Count
	}
	// Firm: 1 draft + 1 approved + 1 rejected + 1 submitted = 4 total.
	if totalByStatus[quotation.StatusDraft] != 1 ||
		totalByStatus[quotation.StatusApproved] != 1 ||
		totalByStatus[quotation.StatusRejected] != 1 ||
		totalByStatus[quotation.StatusSubmitted] != 1 {
		t.Fatalf("firm counts = %+v", totalByStatus)
	}
	if out.NewCustomersLast30Days != 2 {
		t.Fatalf("firm new customers 30d = %d, want 2 (alice's recent + bob's)", out.NewCustomersLast30Days)
	}
}

// An unknown/empty role must default to self scope — never firm. This
// protects against future role codes that are added without also
// updating the allowlist here.
func TestSummary_UnknownRoleDefaultsToSelf(t *testing.T) {
	db := bootPg(t)
	alice, _ := seed(t, db)
	svc := dashboard.NewService(quotation.NewRepository(db), customer.NewRepository(db))

	for _, role := range []string{"", "superadmin", "future-role"} {
		out, err := svc.Summary(context.Background(), alice, role)
		if err != nil {
			t.Fatalf("role=%q summary: %v", role, err)
		}
		if out.Scope != "self" {
			t.Fatalf("role=%q scope = %q, want self", role, out.Scope)
		}
		// Must NOT include bob's submitted quotation.
		for _, c := range out.QuotationsByStatus {
			if c.Status == quotation.StatusSubmitted && c.Count > 0 {
				t.Fatalf("role=%q leaked bob's submitted count: %d", role, c.Count)
			}
		}
	}
}
