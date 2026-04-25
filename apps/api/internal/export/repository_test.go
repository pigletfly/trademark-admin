package export_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
)

// seedApprovedQuotation inserts the FK chain (admin role → user,
// customer, country, approved quotation with a minimal snapshot that
// satisfies chk_quotations_snapshot_when_nondraft) and returns
// (quotationID, userID). Uses the shared bootPg from handler_test.go.
func seedApprovedQuotation(t *testing.T, db *gorm.DB) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	// Look up admin role id (text → uuid.Parse, same dance as other repos).
	var roleIDStr string
	if err := db.WithContext(ctx).Raw(
		`SELECT id FROM roles WHERE code = ?`, "admin",
	).Scan(&roleIDStr).Error; err != nil {
		t.Fatalf("select role: %v", err)
	}
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		t.Fatalf("parse role id: %v", err)
	}

	userID := uuid.New()
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		userID, "Exporter", "exp-"+userID.String()+"@example.com", "x", roleID,
	).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	custID := uuid.New()
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO customers (id, name, created_by) VALUES (?, ?, ?)`,
		custID, "ExportCo-"+custID.String()[:8], userID,
	).Error; err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	countryID := uuid.New()
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO countries (id, code, name_zh, name_en) VALUES (?, ?, ?, ?)`,
		countryID, "EX", "导出国", "Exportland",
	).Error; err != nil {
		t.Fatalf("seed country: %v", err)
	}

	// approved quotation — must have snapshot_json, total_cny_cents,
	// signature, submitted_at, reviewed_at, reviewed_by to pass
	// chk_quotations_snapshot_when_nondraft, AND serial_no to pass
	// chk_quotations_serial_no_when_nondraft (M2 migration 000006).
	qid := uuid.New()
	if err := db.WithContext(ctx).Exec(`
		INSERT INTO quotations
			(id, customer_id, country_id, service_tier, status,
			 snapshot_json, total_cny_cents, signature, serial_no,
			 submitted_at, reviewed_at, reviewed_by, created_by)
		VALUES (?, ?, ?, 'standard', 'approved',
		        '{"lines":[],"total_cny_cents":0,"signature":"t"}'::jsonb,
		        0, 't', ?, NOW(), NOW(), ?, ?)`,
		qid, custID, countryID, "Q202604260001", userID, userID,
	).Error; err != nil {
		t.Fatalf("seed quotation: %v", err)
	}
	return qid, userID
}

func TestRepository_CreateAndGet(t *testing.T) {
	db := bootPg(t)
	qid, userID := seedApprovedQuotation(t, db)
	r := export.NewRepository(db)

	ctx := context.Background()
	f := &export.ExportFile{
		QuotationID: qid,
		Format:      export.FormatDOCX,
		Language:    export.LanguageZH,
		FilePath:    "/tmp/exports/foo.docx",
		FileSize:    4096,
		SHA256:      "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		CreatedBy:   userID,
	}
	if err := r.Create(ctx, f); err != nil {
		t.Fatalf("create: %v", err)
	}
	if f.ID == uuid.Nil {
		t.Fatal("Create should assign an ID")
	}
	if f.CreatedAt.IsZero() {
		t.Fatal("Create should assign CreatedAt")
	}

	got, err := r.Get(ctx, f.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("get returned nil")
	}
	if got.ID != f.ID {
		t.Fatalf("ID mismatch: got %s want %s", got.ID, f.ID)
	}
	if got.QuotationID != qid {
		t.Fatalf("QuotationID mismatch: got %s want %s", got.QuotationID, qid)
	}
	if got.Format != export.FormatDOCX {
		t.Fatalf("Format: got %q want %q", got.Format, export.FormatDOCX)
	}
	if got.Language != export.LanguageZH {
		t.Fatalf("Language: got %q want %q", got.Language, export.LanguageZH)
	}
	if got.FilePath != "/tmp/exports/foo.docx" {
		t.Fatalf("FilePath: got %q", got.FilePath)
	}
	if got.FileSize != 4096 {
		t.Fatalf("FileSize: got %d", got.FileSize)
	}
	if got.SHA256 != f.SHA256 {
		t.Fatalf("SHA256 round-trip failed: got %q want %q", got.SHA256, f.SHA256)
	}
	if got.CreatedBy != userID {
		t.Fatalf("CreatedBy: got %s want %s", got.CreatedBy, userID)
	}
	// ExpiresAt may lose nanosecond precision after Postgres round-trip,
	// but must remain in the future and within a few seconds of original.
	if !got.ExpiresAt.After(time.Now()) {
		t.Fatalf("ExpiresAt should be in the future, got %s", got.ExpiresAt)
	}
}

func TestRepository_Get_Expired(t *testing.T) {
	db := bootPg(t)
	qid, userID := seedApprovedQuotation(t, db)
	r := export.NewRepository(db)
	ctx := context.Background()

	f := &export.ExportFile{
		QuotationID: qid,
		Format:      export.FormatPDF,
		Language:    export.LanguageEN,
		FilePath:    "/tmp/exports/expired.pdf",
		FileSize:    1024,
		SHA256:      "dead0000000000000000000000000000000000000000000000000000deadbeef",
		ExpiresAt:   time.Now().Add(-1 * time.Hour), // already expired
		CreatedBy:   userID,
	}
	if err := r.Create(ctx, f); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := r.Get(ctx, f.ID)
	if err == nil {
		t.Fatal("expected ErrNotFound for expired row, got nil")
	}
	if err != export.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	// Also verify a completely missing id → ErrNotFound.
	_, err = r.Get(ctx, uuid.New())
	if err != export.ErrNotFound {
		t.Fatalf("missing id: want ErrNotFound, got %v", err)
	}
}

func TestRepository_ByQuotation_Ordered(t *testing.T) {
	db := bootPg(t)
	qid, userID := seedApprovedQuotation(t, db)
	r := export.NewRepository(db)
	ctx := context.Background()

	now := time.Now()
	// Insert 3 rows with deliberately spaced CreatedAt so DESC order
	// is unambiguous even if the test runs on a fast clock.
	rows := []*export.ExportFile{
		{
			QuotationID: qid, Format: export.FormatDOCX, Language: export.LanguageZH,
			FilePath: "/tmp/a.docx", FileSize: 1, SHA256: "a",
			ExpiresAt: now.Add(24 * time.Hour), CreatedBy: userID,
			CreatedAt: now.Add(-3 * time.Minute),
		},
		{
			QuotationID: qid, Format: export.FormatDOCX, Language: export.LanguageEN,
			FilePath: "/tmp/b.docx", FileSize: 2, SHA256: "b",
			ExpiresAt: now.Add(24 * time.Hour), CreatedBy: userID,
			CreatedAt: now.Add(-2 * time.Minute),
		},
		{
			QuotationID: qid, Format: export.FormatPDF, Language: export.LanguageBilingual,
			FilePath: "/tmp/c.pdf", FileSize: 3, SHA256: "c",
			ExpiresAt: now.Add(24 * time.Hour), CreatedBy: userID,
			CreatedAt: now.Add(-1 * time.Minute),
		},
	}
	for _, f := range rows {
		if err := r.Create(ctx, f); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	got, err := r.ByQuotation(ctx, qid, 10)
	if err != nil {
		t.Fatalf("ByQuotation: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 rows, got %d", len(got))
	}
	// Newest first: c, b, a.
	if got[0].SHA256 != "c" || got[1].SHA256 != "b" || got[2].SHA256 != "a" {
		t.Fatalf("wrong DESC order: got [%s, %s, %s]",
			got[0].SHA256, got[1].SHA256, got[2].SHA256)
	}

	// limit=2 should return the two newest.
	got2, err := r.ByQuotation(ctx, qid, 2)
	if err != nil {
		t.Fatalf("ByQuotation limit=2: %v", err)
	}
	if len(got2) != 2 || got2[0].SHA256 != "c" || got2[1].SHA256 != "b" {
		t.Fatalf("limit=2 wrong: %+v", got2)
	}
}

func TestRepository_ByQuotation_ExcludesExpired(t *testing.T) {
	db := bootPg(t)
	qid, userID := seedApprovedQuotation(t, db)
	r := export.NewRepository(db)
	ctx := context.Background()

	valid := &export.ExportFile{
		QuotationID: qid, Format: export.FormatDOCX, Language: export.LanguageZH,
		FilePath: "/tmp/valid.docx", FileSize: 100, SHA256: "valid",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedBy: userID,
	}
	expired := &export.ExportFile{
		QuotationID: qid, Format: export.FormatDOCX, Language: export.LanguageZH,
		FilePath: "/tmp/expired.docx", FileSize: 100, SHA256: "expired",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		CreatedBy: userID,
	}
	if err := r.Create(ctx, valid); err != nil {
		t.Fatalf("create valid: %v", err)
	}
	if err := r.Create(ctx, expired); err != nil {
		t.Fatalf("create expired: %v", err)
	}

	got, err := r.ByQuotation(ctx, qid, 10)
	if err != nil {
		t.Fatalf("ByQuotation: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 row (expired filtered out), got %d: %+v", len(got), got)
	}
	if got[0].SHA256 != "valid" {
		t.Fatalf("want the valid row, got %q", got[0].SHA256)
	}
}
