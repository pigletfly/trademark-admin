// apps/api/internal/quotation/handler_test.go
package quotation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
)

// pricingRepoAdapter bridges pricing.Repository.ListActive(ctx, ActiveFilter)
// into the pricingRepo interface that quotation.Service expects
// (ListActive(ctx, *uuid.UUID)). Mirrors the adapter in cmd/server/main.go.
type pricingRepoAdapter struct{ *pricing.Repository }

func (a pricingRepoAdapter) ListActive(ctx context.Context, countryID *uuid.UUID) ([]pricing.PricingEntry, error) {
	return a.Repository.ListActive(ctx, pricing.ActiveFilter{CountryID: countryID})
}

// buildRouter wires up a Gin router with a synthetic auth middleware
// that injects the current user into Gin's context using the key the
// auth package already uses (`auth.currentUser`). Other handler tests
// in this repo follow the same pattern — see customer/handler_test.go.
func buildRouter(t *testing.T, quotHandler *quotation.Handler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		uid, _ := uuid.Parse(c.GetHeader("X-Test-User-ID"))
		role := c.GetHeader("X-Test-Role")
		c.Set("auth.currentUser", auth.CurrentUserSummary{ID: uid, Role: role})
		c.Next()
	})
	authed := r.Group("/api/v1")
	quotation.RegisterAuthedRoutes(authed, quotHandler)
	reviewer := r.Group("/api/v1")
	quotation.RegisterReviewerRoutes(reviewer, quotHandler)
	return r
}

func TestHandler_HappyPath_SubmitThenApprove(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, salesID := seedCustomerCountryUser(t, db)

	// Seed a single active pricing entry so submit can snapshot.
	if err := db.Exec(
		`INSERT INTO pricing_entries
		 (id, country_id, service_tier, fee_item, amount_cny_cents, effective_from, created_by)
		 VALUES (?, ?, 'basic', 'application', 10000, ?, ?)`,
		uuid.New(), countryID, time.Now(), salesID,
	).Error; err != nil {
		t.Fatalf("seed pricing entry: %v", err)
	}

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo})
	r := buildRouter(t, quotation.NewHandler(svc))

	// Salesperson creates draft.
	body, _ := json.Marshal(map[string]any{
		"customer_id":  custID,
		"country_id":   countryID,
		"service_tier": "basic",
	})
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/quotations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status %d body %s", w.Code, w.Body.String())
	}
	var created quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	// Salesperson submits.
	req, _ = http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+created.ID.String()+"/submit", nil)
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("submit: status %d body %s", w.Code, w.Body.String())
	}
	var submitted quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &submitted)
	if submitted.Status != quotation.StatusSubmitted || submitted.Snapshot == nil {
		t.Fatalf("submit result: %+v", submitted)
	}
	if submitted.TotalCNYCents == nil || *submitted.TotalCNYCents != 10000 {
		t.Fatalf("total = %v, want 10000", submitted.TotalCNYCents)
	}
	if submitted.SerialNo == nil {
		t.Fatalf("expected serial_no on submitted quotation, got nil")
	}
	re := regexp.MustCompile(`^Q\d{12}$`)
	if !re.MatchString(*submitted.SerialNo) {
		t.Fatalf("serial_no %q doesn't match Q\\d{12}", *submitted.SerialNo)
	}

	// Seed a reviewer user too.
	var roleIDStr string
	if err := db.Raw(`SELECT id FROM roles WHERE code = 'reviewer'`).Scan(&roleIDStr).Error; err != nil {
		t.Fatalf("select reviewer role: %v", err)
	}
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		t.Fatalf("parse reviewer role id: %v", err)
	}
	reviewerID := uuid.New()
	if err := db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		reviewerID, "Rev", "rev-"+reviewerID.String()+"@example.com", "x", roleID,
	).Error; err != nil {
		t.Fatalf("seed reviewer user: %v", err)
	}

	// Reviewer approves.
	req, _ = http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+created.ID.String()+"/approve", bytes.NewReader([]byte(`{"comment":"ok"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("approve: status %d body %s", w.Code, w.Body.String())
	}
	var approved quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &approved)
	if approved.Status != quotation.StatusApproved {
		t.Fatalf("status = %q, want approved", approved.Status)
	}

	// History endpoint returns 2 rows.
	req, _ = http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/quotations/"+created.ID.String()+"/history", nil)
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("history: status %d body %s", w.Code, w.Body.String())
	}
	var hist struct {
		Items []quotation.HistoryEntry `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &hist)
	if len(hist.Items) != 2 {
		t.Fatalf("history items = %d, want 2", len(hist.Items))
	}
}

func TestHandler_SalespersonCannotReadAnothersQuotation(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, aliceID := seedCustomerCountryUser(t, db)

	// Seed bob too.
	var roleIDStr string
	if err := db.Raw(`SELECT id FROM roles WHERE code = 'salesperson'`).Scan(&roleIDStr).Error; err != nil {
		t.Fatalf("select salesperson role: %v", err)
	}
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		t.Fatalf("parse salesperson role id: %v", err)
	}
	bobID := uuid.New()
	if err := db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		bobID, "Bob", "bob-"+bobID.String()+"@example.com", "x", roleID,
	).Error; err != nil {
		t.Fatalf("seed bob user: %v", err)
	}

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo})
	r := buildRouter(t, quotation.NewHandler(svc))

	// Alice creates a quotation.
	body, _ := json.Marshal(map[string]any{
		"customer_id": custID, "country_id": countryID, "service_tier": "basic",
	})
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", aliceID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status %d", w.Code)
	}
	var created quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	// Bob tries to read it → 403.
	req, _ = http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/quotations/"+created.ID.String(), nil)
	req.Header.Set("X-Test-User-ID", bobID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("bob read alice's quote: want 403, got %d", w.Code)
	}
}
