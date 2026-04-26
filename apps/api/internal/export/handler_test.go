// apps/api/internal/export/handler_test.go
package export_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/catalog"
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"
	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func bootPg(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("exp_test"),
		tcpostgres.WithUsername("exp"),
		tcpostgres.WithPassword("exp"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("container: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	dsn, _ := container.ConnectionString(ctx, "sslmode=disable")
	mig, err := migrator.New(api.Migrations, "migrations", dsn)
	if err != nil {
		t.Fatalf("migrator: %v", err)
	}
	if err := mig.Up(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = mig.Close()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	return db
}

// pricingRepoAdapter matches the signature quotation.Service expects.
type pricingRepoAdapter struct{ *pricing.Repository }

func (a pricingRepoAdapter) ListActive(ctx context.Context, countryID *uuid.UUID) ([]pricing.PricingEntry, error) {
	return a.Repository.ListActive(ctx, pricing.ActiveFilter{CountryID: countryID})
}

// wireDomainServices builds the quotation / customer / catalog
// dependencies the export.Handler needs. Extracted so both the legacy
// and new tests share one code path for assembling these.
func wireDomainServices(t *testing.T, db *gorm.DB) (
	*customer.Service,
	*quotation.Service,
	*catalog.Repository,
) {
	t.Helper()
	quotRepo := quotation.NewRepository(db)
	pRepo := pricing.NewRepository(db)
	custRepo := customer.NewRepository(db)
	quotSvc := quotation.NewService(quotRepo, pricingRepoAdapter{pRepo}, custRepo)
	custSvc := customer.NewService(custRepo)
	catRepo := catalog.NewRepository(db)
	return custSvc, quotSvc, catRepo
}

func TestExportDOCX_RejectsDraftWith422(t *testing.T) {
	db := bootPg(t)
	gin.SetMode(gin.TestMode)

	// Seed minimal data: role, user, customer, country, quotation (draft).
	var roleIDStr string
	if err := db.Raw(`SELECT id FROM roles WHERE code = 'admin'`).Scan(&roleIDStr).Error; err != nil {
		t.Fatalf("role: %v", err)
	}
	roleID, _ := uuid.Parse(roleIDStr)
	userID := uuid.New()
	_ = db.Exec(`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		userID, "Admin", "admin@ex.com", "x", roleID).Error
	custID := uuid.New()
	_ = db.Exec(`INSERT INTO customers (id, name, created_by) VALUES (?, ?, ?)`,
		custID, "Acme", userID).Error
	countryID := uuid.New()
	_ = db.Exec(`INSERT INTO countries (id, code, name_zh, name_en) VALUES (?, ?, ?, ?)`,
		countryID, "CN", "中国", "China").Error
	qID := uuid.New()
	_ = db.Exec(`INSERT INTO quotations (id, customer_id, country_id, service_tier, status, created_by)
		VALUES (?, ?, ?, 'basic', 'draft', ?)`, qID, custID, countryID, userID).Error

	r := buildRouter(t, db, userID, "admin")
	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/quotations/"+qID.String()+"/export.docx", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ERR_NOT_APPROVED") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestExportDOCX_HappyPath_ReturnsZipWithChineseCustomerName(t *testing.T) {
	db := bootPg(t)
	gin.SetMode(gin.TestMode)

	var roleIDStr string
	_ = db.Raw(`SELECT id FROM roles WHERE code = 'admin'`).Scan(&roleIDStr).Error
	roleID, _ := uuid.Parse(roleIDStr)
	userID := uuid.New()
	_ = db.Exec(`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		userID, "Admin", "admin@ex.com", "x", roleID).Error
	custID := uuid.New()
	_ = db.Exec(`INSERT INTO customers (id, name, created_by) VALUES (?, ?, ?)`,
		custID, "Acme 有限公司", userID).Error
	countryID := uuid.New()
	_ = db.Exec(`INSERT INTO countries (id, code, name_zh, name_en) VALUES (?, ?, ?, ?)`,
		countryID, "CN", "中国", "China").Error

	// Seed pricing + create approved quotation with a snapshot JSON.
	snap := map[string]any{
		"lines":           []map[string]any{{"fee_item": "application", "amount_cny_cents": 10000}},
		"total_cny_cents": 10000,
		"signature":       "sig-abc",
	}
	snapJSON, _ := json.Marshal(snap)
	sig := "sig-abc"
	total := int64(10000)
	submitted := time.Now().Add(-2 * time.Hour)
	reviewed := time.Now().Add(-1 * time.Hour)

	qID := uuid.New()
	_ = db.Exec(`INSERT INTO quotations
		(id, customer_id, country_id, service_tier, status, snapshot_json, total_cny_cents, signature, serial_no, submitted_at, reviewed_at, reviewed_by, created_by)
		VALUES (?, ?, ?, 'basic', 'approved', ?, ?, ?, ?, ?, ?, ?, ?)`,
		qID, custID, countryID, string(snapJSON), total, sig, "Q202604260001", submitted, reviewed, userID, userID).Error

	r := buildRouter(t, db, userID, "admin")
	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/quotations/"+qID.String()+"/export.docx", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body %s", w.Code, w.Body.String())
	}
	// Validate it's a valid zip with a word/document.xml that contains the customer name in Chinese.
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	var doc string
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			doc = string(b)
			break
		}
	}
	if doc == "" {
		t.Fatal("word/document.xml missing")
	}
	if !strings.Contains(doc, "Acme 有限公司") {
		t.Fatalf("Chinese customer name missing from doc:\n%s", doc[:500])
	}
	if !strings.Contains(doc, "中国") {
		t.Fatalf("Chinese country name missing")
	}
	if !strings.Contains(doc, "¥ 100.00") {
		t.Fatalf("formatted total missing")
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("missing attachment header: %s", w.Header().Get("Content-Disposition"))
	}
}

// buildRouter builds a Gin router with quotation + export + auth
// middleware injecting the given user. Legacy helper — exercises only
// the GET /export.docx path, so svc+signer are nil.
func buildRouter(t *testing.T, db *gorm.DB, userID uuid.UUID, role string) *gin.Engine {
	t.Helper()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("auth.currentUser", auth.CurrentUserSummary{ID: userID, Role: role})
		c.Next()
	})
	custSvc, quotSvc, catRepo := wireDomainServices(t, db)
	h := export.NewHandler(quotSvc, custSvc, catRepo, nil, nil)
	grp := r.Group("/api/v1")
	export.RegisterRoutes(grp, h)
	return r
}

// --- New route tests: POST /quotations/:id/export + GET /exports/:id/download ---

// testSigningSecret is 32 bytes exactly — the minimum Signer accepts.
var testSigningSecret = []byte("test-secret-32-bytes-min-length!")

// buildNewRouter wires a full Handler with Service + Signer and mounts
// BOTH authed and public groups so a single test can exercise end-to-end
// export + download flows. The authed group is synthetic: we skip
// RequireAuth and inject a current user directly.
func buildNewRouter(
	t *testing.T,
	db *gorm.DB,
	pdfRenderer export.PDFRenderer,
	userID uuid.UUID,
	role string,
) (*gin.Engine, *export.Service, *export.Signer, *export.Handler) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := export.NewRepository(db)
	storage := export.NewStorage(t.TempDir())
	svc := export.NewService(repo, storage, pdfRenderer, time.Hour)
	signer := export.NewSigner(testSigningSecret)

	custSvc, quotSvc, catRepo := wireDomainServices(t, db)
	h := export.NewHandler(quotSvc, custSvc, catRepo, svc, signer)

	r := gin.New()
	// Public group: NO auth middleware.
	public := r.Group("/api/v1")
	export.RegisterPublicRoutes(public, h)

	// Authed group: inject a user so handler's role/ownership check passes.
	authed := r.Group("/api/v1")
	authed.Use(func(c *gin.Context) {
		c.Set("auth.currentUser", auth.CurrentUserSummary{ID: userID, Role: role})
		c.Next()
	})
	export.RegisterAuthedRoutes(authed, h)
	return r, svc, signer, h
}

func TestHandler_Export_PDF_ReturnsSignedURL(t *testing.T) {
	db := bootPg(t)
	qid, actorID := seedApprovedQuotation(t, db)

	fakePDF := &fakePDFRenderer{out: []byte("%PDF-fake")}
	r, _, _, _ := buildNewRouter(t, db, fakePDF, actorID, "admin")

	body := `{"format":"pdf","language":"bilingual"}`
	req := httptest.NewRequest("POST",
		"/api/v1/quotations/"+qid.String()+"/export",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var out export.ExportFileDTO
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Format != export.FormatPDF {
		t.Errorf("format: got %q want pdf", out.Format)
	}
	if out.Language != export.LanguageBilingual {
		t.Errorf("language: got %q want bilingual", out.Language)
	}
	if out.QuotationID != qid {
		t.Errorf("quotation id: got %s want %s", out.QuotationID, qid)
	}
	if out.DownloadURL == "" {
		t.Errorf("missing download_url")
	}
	if !strings.HasPrefix(out.DownloadURL, "/api/v1/exports/"+out.ID.String()+"/download?token=") {
		t.Errorf("unexpected download_url: %s", out.DownloadURL)
	}
	if out.SHA256 == "" {
		t.Errorf("missing sha256")
	}
	if out.FileSize <= 0 {
		t.Errorf("unexpected file size %d", out.FileSize)
	}
}

func TestHandler_Download_WithValidToken(t *testing.T) {
	db := bootPg(t)
	qid, actorID := seedApprovedQuotation(t, db)

	fakePDF := &fakePDFRenderer{out: []byte("%PDF-fake-body")}
	r, svc, signer, _ := buildNewRouter(t, db, fakePDF, actorID, "admin")

	// Generate directly (no HTTP) so the download test isolates its concerns.
	view := baseView()
	view.QuotationID = qid.String()
	f, err := svc.GeneratePDF(context.Background(), view, export.LanguageBilingual, qid, actorID)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	tok := signer.Sign(f.ID, f.ExpiresAt)

	req := httptest.NewRequest("GET",
		"/api/v1/exports/"+f.ID.String()+"/download?token="+tok, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/pdf" {
		t.Errorf("content-type: got %q want application/pdf", got)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Errorf("content-disposition: got %q want attachment", cd)
	}
	if got := w.Header().Get("X-Content-SHA256"); got != f.SHA256 {
		t.Errorf("sha header: got %q want %q", got, f.SHA256)
	}
	if w.Body.String() != "%PDF-fake-body" {
		t.Errorf("body mismatch: got %q", w.Body.String())
	}
}

func TestHandler_Download_BadToken(t *testing.T) {
	db := bootPg(t)
	_, actorID := seedApprovedQuotation(t, db)

	r, _, _, _ := buildNewRouter(t, db, &fakePDFRenderer{}, actorID, "admin")

	// Invalid token + random export id → 403.
	req := httptest.NewRequest("GET",
		"/api/v1/exports/"+uuid.New().String()+"/download?token=bogus", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ERR_INVALID_TOKEN") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestHandler_Export_InvalidOpts(t *testing.T) {
	db := bootPg(t)
	qid, actorID := seedApprovedQuotation(t, db)

	r, _, _, _ := buildNewRouter(t, db, &fakePDFRenderer{}, actorID, "admin")

	// Unknown format + unknown language — both rejected.
	body := `{"format":"jpeg","language":"fr"}`
	req := httptest.NewRequest("POST",
		"/api/v1/quotations/"+qid.String()+"/export",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ERR_INVALID_EXPORT_OPTS") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

// unused import silencer
var _ = audit.JSONB(nil)
