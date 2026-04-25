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
		(id, customer_id, country_id, service_tier, status, snapshot_json, total_cny_cents, signature, submitted_at, reviewed_at, reviewed_by, created_by)
		VALUES (?, ?, ?, 'basic', 'approved', ?, ?, ?, ?, ?, ?, ?)`,
		qID, custID, countryID, string(snapJSON), total, sig, submitted, reviewed, userID, userID).Error

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
// middleware injecting the given user.
func buildRouter(t *testing.T, db *gorm.DB, userID uuid.UUID, role string) *gin.Engine {
	t.Helper()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("auth.currentUser", auth.CurrentUserSummary{ID: userID, Role: role})
		c.Next()
	})

	quotRepo := quotation.NewRepository(db)
	pRepo := pricing.NewRepository(db)
	quotSvc := quotation.NewService(quotRepo, pricingRepoAdapter{pRepo})

	custRepo := customer.NewRepository(db)
	custSvc := customer.NewService(custRepo)

	catRepo := catalog.NewRepository(db)

	h := export.NewHandler(quotSvc, custSvc, catRepo)
	grp := r.Group("/api/v1")
	export.RegisterRoutes(grp, h)
	return r
}

// unused import silencer
var _ = audit.JSONB(nil)
