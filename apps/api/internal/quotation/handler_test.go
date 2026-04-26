// apps/api/internal/quotation/handler_test.go
package quotation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
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
func buildRouter(t *testing.T, quotHandler *quotation.Handler, pricingHandler *pricing.Handler) *gin.Engine {
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
	reviewer := r.Group("/api/v1", auth.RequireRole("reviewer", "admin"))
	quotation.RegisterReviewerRoutes(reviewer, quotHandler)
	// Pricing reads are reviewer+admin in main.go — mirror that so the
	// traceability endpoint can be exercised in tests under role=admin
	// or role=reviewer.
	pricing.RegisterReadRoutes(reviewer, pricingHandler)
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
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricing.NewHandler(pricing.NewService(pricingRepo)))

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
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricing.NewHandler(pricing.NewService(pricingRepo)))

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

// seedUserWithRole inserts a new user with the given role code and returns
// the user's UUID. Used in tests below that need a reviewer or a second
// salesperson alongside the primary user from seedCustomerCountryUser.
func seedUserWithRole(t *testing.T, db *gorm.DB, roleCode, displayName string) uuid.UUID {
	t.Helper()
	var roleIDStr string
	if err := db.Raw(`SELECT id FROM roles WHERE code = ?`, roleCode).Scan(&roleIDStr).Error; err != nil {
		t.Fatalf("select %s role: %v", roleCode, err)
	}
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		t.Fatalf("parse role id: %v", err)
	}
	uid := uuid.New()
	if err := db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		uid, displayName, displayName+"-"+uid.String()+"@example.com", "x", roleID,
	).Error; err != nil {
		t.Fatalf("seed %s user: %v", roleCode, err)
	}
	return uid
}

// seedBasicPricing inserts one pricing_entries row for (countryID, basic,
// application) at the given amount. Returns nothing — caller just needs
// the side effect so Submit can snapshot.
func seedBasicPricing(t *testing.T, db *gorm.DB, countryID, createdBy uuid.UUID, amountCents int64) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO pricing_entries
		 (id, country_id, service_tier, fee_item, amount_cny_cents, effective_from, created_by)
		 VALUES (?, ?, 'basic', 'application', ?, ?, ?)`,
		uuid.New(), countryID, amountCents, time.Now(), createdBy,
	).Error; err != nil {
		t.Fatalf("seed pricing entry: %v", err)
	}
}

// createAndSubmit is a tiny helper that drives POST /quotations then
// POST /submit as the given salesperson and returns the submitted
// Response. Tests below use it to get to the submitted state cheaply.
func createAndSubmit(t *testing.T, r *gin.Engine, custID, countryID, salesID uuid.UUID) quotation.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"customer_id": custID, "country_id": countryID, "service_tier": "basic",
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
	return submitted
}

func TestHandler_Withdraw_OwnerOK(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, aliceID := seedCustomerCountryUser(t, db)
	seedBasicPricing(t, db, countryID, aliceID, 10000)

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricing.NewHandler(pricing.NewService(pricingRepo)))

	submitted := createAndSubmit(t, r, custID, countryID, aliceID)
	if submitted.SerialNo == nil {
		t.Fatalf("expected submitted quote to have serial_no")
	}
	serial := *submitted.SerialNo

	// Alice withdraws her own submission → 200, back to draft.
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+submitted.ID.String()+"/withdraw", nil)
	req.Header.Set("X-Test-User-ID", aliceID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("withdraw: status %d body %s", w.Code, w.Body.String())
	}
	var withdrawn quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &withdrawn)
	if withdrawn.Status != quotation.StatusDraft {
		t.Fatalf("status = %q, want draft", withdrawn.Status)
	}
	if withdrawn.Snapshot != nil {
		t.Fatalf("snapshot should be cleared, got %+v", withdrawn.Snapshot)
	}
	if withdrawn.TotalCNYCents != nil {
		t.Fatalf("total should be cleared, got %v", withdrawn.TotalCNYCents)
	}
	if withdrawn.Signature != nil {
		t.Fatalf("signature should be cleared, got %v", withdrawn.Signature)
	}
	if withdrawn.SerialNo == nil || *withdrawn.SerialNo != serial {
		t.Fatalf("serial_no should be preserved; got %v want %q", withdrawn.SerialNo, serial)
	}

	// History → 2 rows (submit + withdraw). Withdraw row lands in draft.
	req, _ = http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/quotations/"+submitted.ID.String()+"/history", nil)
	req.Header.Set("X-Test-User-ID", aliceID.String())
	req.Header.Set("X-Test-Role", "salesperson")
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
	if hist.Items[1].ToStatus != quotation.StatusDraft {
		t.Fatalf("history[1].to = %q, want draft", hist.Items[1].ToStatus)
	}
}

func TestHandler_Withdraw_Forbidden_NonOwner(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, aliceID := seedCustomerCountryUser(t, db)
	seedBasicPricing(t, db, countryID, aliceID, 10000)
	bobID := seedUserWithRole(t, db, "salesperson", "Bob")

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricing.NewHandler(pricing.NewService(pricingRepo)))

	submitted := createAndSubmit(t, r, custID, countryID, aliceID)

	// Bob (different salesperson) calls withdraw → 403 ERR_NOT_OWNER.
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+submitted.ID.String()+"/withdraw", nil)
	req.Header.Set("X-Test-User-ID", bobID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("bob withdraw: status %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ERR_NOT_OWNER") {
		t.Fatalf("want body containing ERR_NOT_OWNER, got %s", w.Body.String())
	}
}

func TestHandler_Withdraw_InvalidFromApproved(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, aliceID := seedCustomerCountryUser(t, db)
	seedBasicPricing(t, db, countryID, aliceID, 10000)
	reviewerID := seedUserWithRole(t, db, "reviewer", "Rev")

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricing.NewHandler(pricing.NewService(pricingRepo)))

	submitted := createAndSubmit(t, r, custID, countryID, aliceID)

	// Reviewer approves.
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+submitted.ID.String()+"/approve", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("approve: status %d body %s", w.Code, w.Body.String())
	}

	// Alice tries to withdraw an approved quote → 409 ERR_INVALID_TRANSITION.
	req, _ = http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+submitted.ID.String()+"/withdraw", nil)
	req.Header.Set("X-Test-User-ID", aliceID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("withdraw after approve: status %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ERR_INVALID_TRANSITION") {
		t.Fatalf("want body containing ERR_INVALID_TRANSITION, got %s", w.Body.String())
	}
}

func TestHandler_Copy_ReturnsFreshDraft(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, aliceID := seedCustomerCountryUser(t, db)
	seedBasicPricing(t, db, countryID, aliceID, 10000)

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricing.NewHandler(pricing.NewService(pricingRepo)))

	source := createAndSubmit(t, r, custID, countryID, aliceID)

	// Alice copies her own submitted quote → 201 with fresh draft.
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+source.ID.String()+"/copy", nil)
	req.Header.Set("X-Test-User-ID", aliceID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("copy: status %d body %s", w.Code, w.Body.String())
	}
	var copied quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &copied)
	if copied.Status != quotation.StatusDraft {
		t.Fatalf("status = %q, want draft", copied.Status)
	}
	if copied.ID == source.ID {
		t.Fatalf("copy should have a new id; got source id %s", copied.ID)
	}
	if copied.CustomerID != source.CustomerID {
		t.Fatalf("customer_id = %s, want %s", copied.CustomerID, source.CustomerID)
	}
	if copied.CountryID != source.CountryID {
		t.Fatalf("country_id = %s, want %s", copied.CountryID, source.CountryID)
	}
	if copied.ServiceTier != source.ServiceTier {
		t.Fatalf("service_tier = %s, want %s", copied.ServiceTier, source.ServiceTier)
	}
	if copied.SerialNo != nil {
		t.Fatalf("copy should have no serial_no, got %v", copied.SerialNo)
	}
	if copied.Snapshot != nil {
		t.Fatalf("copy should have no snapshot, got %+v", copied.Snapshot)
	}
	if copied.TotalCNYCents != nil {
		t.Fatalf("copy should have no total, got %v", copied.TotalCNYCents)
	}
	if copied.Signature != nil {
		t.Fatalf("copy should have no signature, got %v", copied.Signature)
	}
	if copied.CreatedBy != aliceID {
		t.Fatalf("created_by = %s, want %s", copied.CreatedBy, aliceID)
	}
}

func TestHandler_Adjust_RecordsDiff(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, aliceID := seedCustomerCountryUser(t, db)
	seedBasicPricing(t, db, countryID, aliceID, 10000)
	reviewerID := seedUserWithRole(t, db, "reviewer", "Rev")

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricing.NewHandler(pricing.NewService(pricingRepo)))

	submitted := createAndSubmit(t, r, custID, countryID, aliceID)
	if submitted.TotalCNYCents == nil || *submitted.TotalCNYCents != 10000 {
		t.Fatalf("pre-adjust total = %v, want 10000", submitted.TotalCNYCents)
	}
	if submitted.Signature == nil {
		t.Fatalf("pre-adjust signature should be set")
	}
	preSig := *submitted.Signature

	// Reviewer adjusts the submitted snapshot: application from 10000 → 15000.
	adjustBody := []byte(`{"lines":[{"fee_item":"application","amount_cny_cents":15000}]}`)
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+submitted.ID.String()+"/adjust", bytes.NewReader(adjustBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("adjust: status %d body %s", w.Code, w.Body.String())
	}
	var adjusted quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &adjusted)
	if adjusted.Status != quotation.StatusSubmitted {
		t.Fatalf("post-adjust status = %q, want submitted", adjusted.Status)
	}
	if adjusted.TotalCNYCents == nil || *adjusted.TotalCNYCents != 15000 {
		t.Fatalf("post-adjust total = %v, want 15000", adjusted.TotalCNYCents)
	}
	if adjusted.Signature == nil || *adjusted.Signature == preSig {
		t.Fatalf("signature should differ from pre-adjust; got %v", adjusted.Signature)
	}

	// History → 2 rows; second is submitted→submitted w/ non-empty diff.
	req, _ = http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/quotations/"+submitted.ID.String()+"/history", nil)
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
	adjustRow := hist.Items[1]
	if adjustRow.FromStatus != quotation.StatusSubmitted || adjustRow.ToStatus != quotation.StatusSubmitted {
		t.Fatalf("adjust history row: from=%q to=%q, want submitted→submitted",
			adjustRow.FromStatus, adjustRow.ToStatus)
	}
	if len(adjustRow.DiffJSON) == 0 {
		t.Fatalf("adjust history row: expected non-empty diff_json")
	}
	var diff struct {
		TotalBefore int64 `json:"total_before"`
		TotalAfter  int64 `json:"total_after"`
	}
	if err := json.Unmarshal(adjustRow.DiffJSON, &diff); err != nil {
		t.Fatalf("unmarshal diff_json: %v", err)
	}
	if diff.TotalBefore != 10000 || diff.TotalAfter != 15000 {
		t.Fatalf("diff totals: before=%d after=%d, want 10000/15000",
			diff.TotalBefore, diff.TotalAfter)
	}
}

func TestHandler_Adjust_Forbidden_Salesperson(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, aliceID := seedCustomerCountryUser(t, db)
	seedBasicPricing(t, db, countryID, aliceID, 10000)

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricing.NewHandler(pricing.NewService(pricingRepo)))

	submitted := createAndSubmit(t, r, custID, countryID, aliceID)

	// Alice (salesperson role) attempts adjust → 403 from RequireRole middleware.
	adjustBody := []byte(`{"lines":[{"fee_item":"application","amount_cny_cents":15000}]}`)
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+submitted.ID.String()+"/adjust", bytes.NewReader(adjustBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", aliceID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("adjust as salesperson: status %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ERR_FORBIDDEN") {
		t.Fatalf("want body containing ERR_FORBIDDEN, got %s", w.Body.String())
	}
}

func TestHandler_Adjust_InvalidOnDraft(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, aliceID := seedCustomerCountryUser(t, db)
	reviewerID := seedUserWithRole(t, db, "reviewer", "Rev")

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricing.NewHandler(pricing.NewService(pricingRepo)))

	// Alice creates a draft but does NOT submit.
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
		t.Fatalf("create: status %d body %s", w.Code, w.Body.String())
	}
	var draft quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &draft)

	// Reviewer tries to adjust the draft → 409 ERR_INVALID_TRANSITION.
	adjustBody := []byte(`{"lines":[{"fee_item":"application","amount_cny_cents":15000}]}`)
	req, _ = http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+draft.ID.String()+"/adjust", bytes.NewReader(adjustBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("adjust on draft: status %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ERR_INVALID_TRANSITION") {
		t.Fatalf("want body containing ERR_INVALID_TRANSITION, got %s", w.Body.String())
	}
}

func TestHandler_Preview_OK(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, salesID := seedCustomerCountryUser(t, db)

	// Seed one active pricing entry so the preview can produce lines.
	if err := db.Exec(
		`INSERT INTO pricing_entries
		 (id, country_id, service_tier, fee_item, amount_cny_cents, effective_from, created_by)
		 VALUES (?, ?, 'basic', 'application', 50000, ?, ?)`,
		uuid.New(), countryID, time.Now(), salesID,
	).Error; err != nil {
		t.Fatalf("seed pricing: %v", err)
	}

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricing.NewHandler(pricing.NewService(pricingRepo)))

	body, _ := json.Marshal(map[string]any{
		"customer_id":  custID,
		"country_id":   countryID,
		"service_tier": "basic",
	})
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("preview: status %d body %s", w.Code, w.Body.String())
	}
	var resp quotation.PreviewResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.TotalCNYCents != 50000 || len(resp.Lines) != 1 {
		t.Fatalf("resp = %+v, want 1 line totalling 50000", resp)
	}
	if resp.Signature == "" {
		t.Fatalf("empty signature")
	}
}

func TestHandler_Preview_BadBody(t *testing.T) {
	db, _ := bootPg(t)
	_, _, salesID := seedCustomerCountryUser(t, db)

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricing.NewHandler(pricing.NewService(pricingRepo)))

	// Missing country_id.
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/preview", strings.NewReader(`{"customer_id":"00000000-0000-0000-0000-000000000001","service_tier":"basic"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ERR_INVALID_BODY") {
		t.Fatalf("body = %s, want ERR_INVALID_BODY", w.Body.String())
	}
}

func TestHandler_Preview_CustomerNotFound(t *testing.T) {
	db, _ := bootPg(t)
	_, countryID, salesID := seedCustomerCountryUser(t, db)

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricing.NewHandler(pricing.NewService(pricingRepo)))

	body, _ := json.Marshal(map[string]any{
		"customer_id":  uuid.New(), // does NOT exist
		"country_id":   countryID,
		"service_tier": "basic",
	})
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Preview_MissingPricing(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, salesID := seedCustomerCountryUser(t, db)
	// No pricing entries seeded.

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricing.NewHandler(pricing.NewService(pricingRepo)))

	body, _ := json.Marshal(map[string]any{
		"customer_id":  custID,
		"country_id":   countryID,
		"service_tier": "basic",
	})
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ERR_MISSING_PRICING") {
		t.Fatalf("body = %s, want ERR_MISSING_PRICING", w.Body.String())
	}
}

// TestHandler_SnapshotSourceIDs_LookupPricingEntry exercises the full
// traceability chain: submit a draft → read snapshot → extract
// source_pricing_entry_id from each line → hit GET /pricing-entries/:id
// and confirm we get the underlying entry back.
func TestHandler_SnapshotSourceIDs_LookupPricingEntry(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, salesID := seedCustomerCountryUser(t, db)

	// Seed two pricing entries so we have multiple lines to trace.
	appID := uuid.New()
	agentID := uuid.New()
	if err := db.Exec(
		`INSERT INTO pricing_entries
		 (id, country_id, service_tier, fee_item, amount_cny_cents, effective_from, created_by)
		 VALUES (?, ?, 'basic', 'application', 10000, ?, ?)`,
		appID, countryID, time.Now(), salesID,
	).Error; err != nil {
		t.Fatalf("seed pricing app: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO pricing_entries
		 (id, country_id, service_tier, fee_item, amount_cny_cents, effective_from, created_by)
		 VALUES (?, ?, 'basic', 'agent', 5000, ?, ?)`,
		agentID, countryID, time.Now(), salesID,
	).Error; err != nil {
		t.Fatalf("seed pricing agent: %v", err)
	}

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	pricingSvc := pricing.NewService(pricingRepo)
	pricingHandler := pricing.NewHandler(pricingSvc)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricingHandler)

	// Create a draft.
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

	// Submit — freezes snapshot.
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
	if submitted.Snapshot == nil {
		t.Fatal("submitted quotation has nil snapshot")
	}
	if len(submitted.Snapshot.Lines) != 2 {
		t.Fatalf("snapshot lines: want 2, got %d", len(submitted.Snapshot.Lines))
	}

	// Reviewer user needed to hit GET /pricing-entries/:id (reviewer+admin only).
	reviewerID, _ := ensureReviewer(t, db)

	// Trace each snapshot line back to its source pricing entry.
	for _, line := range submitted.Snapshot.Lines {
		if line.SourcePricingEntryID == nil {
			t.Errorf("line %s: source id is nil", line.FeeItem)
			continue
		}
		req, _ = http.NewRequestWithContext(context.Background(), "GET",
			"/api/v1/pricing-entries/"+line.SourcePricingEntryID.String(), nil)
		req.Header.Set("X-Test-User-ID", reviewerID.String())
		req.Header.Set("X-Test-Role", "reviewer")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("lookup line %s: status %d body %s", line.FeeItem, w.Code, w.Body.String())
		}
		var entry map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &entry)
		if entry["fee_item"] != line.FeeItem {
			t.Errorf("lookup mismatch: snapshot says %s, pricing entry says %v",
				line.FeeItem, entry["fee_item"])
		}
		gotAmount, _ := entry["amount_cny_cents"].(float64)
		if int64(gotAmount) != line.AmountCNYCents {
			t.Errorf("amount mismatch for %s: snapshot %d, entry %d",
				line.FeeItem, line.AmountCNYCents, int64(gotAmount))
		}
	}

	// Bonus: 404 on random UUID.
	req, _ = http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/pricing-entries/"+uuid.New().String(), nil)
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("random id lookup: want 404, got %d body %s", w.Code, w.Body.String())
	}

	// 400 on invalid UUID.
	req, _ = http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/pricing-entries/not-a-uuid", nil)
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid uuid: want 400, got %d body %s", w.Code, w.Body.String())
	}
}

// TestHandler_Adjust_PreservesSourceIDs covers the HTTP JSON round-trip:
// reviewer POSTs /:id/adjust with lines whose JSON carries
// source_pricing_entry_id, and we read back GET /:id and confirm the
// field persisted through json.Unmarshal → snapshot.Lines.
func TestHandler_Adjust_PreservesSourceIDs(t *testing.T) {
	db, _ := bootPg(t)
	custID, countryID, salesID := seedCustomerCountryUser(t, db)
	reviewerID, _ := ensureReviewer(t, db)

	// Seed one pricing entry for the initial submit.
	entryID := uuid.New()
	if err := db.Exec(
		`INSERT INTO pricing_entries
		 (id, country_id, service_tier, fee_item, amount_cny_cents, effective_from, created_by)
		 VALUES (?, ?, 'basic', 'application', 10000, ?, ?)`,
		entryID, countryID, time.Now(), salesID,
	).Error; err != nil {
		t.Fatalf("seed pricing: %v", err)
	}

	quotRepo := quotation.NewRepository(db)
	pricingRepo := pricing.NewRepository(db)
	pricingSvc := pricing.NewService(pricingRepo)
	pricingHandler := pricing.NewHandler(pricingSvc)
	svc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, customer.NewRepository(db))
	r := buildRouter(t, quotation.NewHandler(svc), pricingHandler)

	// Create + submit as salesperson.
	body, _ := json.Marshal(map[string]any{
		"customer_id": custID, "country_id": countryID, "service_tier": "basic",
	})
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/quotations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var created quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	req, _ = http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+created.ID.String()+"/submit", nil)
	req.Header.Set("X-Test-User-ID", salesID.String())
	req.Header.Set("X-Test-Role", "salesperson")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("submit: %d %s", w.Code, w.Body.String())
	}

	// Reviewer adjusts — carry one line with source_id, one without.
	preserved := uuid.New()
	adjustBody, _ := json.Marshal(map[string]any{
		"lines": []map[string]any{
			{"fee_item": "preserved", "amount_cny_cents": 500, "source_pricing_entry_id": preserved.String()},
			{"fee_item": "orphan", "amount_cny_cents": 700},
		},
	})
	req, _ = http.NewRequestWithContext(context.Background(), "POST",
		"/api/v1/quotations/"+created.ID.String()+"/adjust", bytes.NewReader(adjustBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("adjust: %d %s", w.Code, w.Body.String())
	}

	// Read back via GET /:id — snapshot should contain both lines with
	// their source_id state intact.
	req, _ = http.NewRequestWithContext(context.Background(), "GET",
		"/api/v1/quotations/"+created.ID.String(), nil)
	req.Header.Set("X-Test-User-ID", reviewerID.String())
	req.Header.Set("X-Test-Role", "reviewer")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get: %d", w.Code)
	}
	var got quotation.Response
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Snapshot == nil {
		t.Fatal("snapshot nil")
	}
	byItem := map[string]*uuid.UUID{}
	for i := range got.Snapshot.Lines {
		byItem[got.Snapshot.Lines[i].FeeItem] = got.Snapshot.Lines[i].SourcePricingEntryID
	}
	if byItem["preserved"] == nil || *byItem["preserved"] != preserved {
		t.Errorf("preserved source: want %s, got %v", preserved, byItem["preserved"])
	}
	if byItem["orphan"] != nil {
		t.Errorf("orphan source: want nil, got %v", byItem["orphan"])
	}
}

// ensureReviewer inserts a reviewer user and returns (userID, roleID).
// Small helper scoped to this test file.
func ensureReviewer(t *testing.T, db *gorm.DB) (uuid.UUID, uuid.UUID) {
	t.Helper()
	var reviewerRoleID string
	require.NoError(t, db.Raw("SELECT id FROM roles WHERE code = ?", "reviewer").Scan(&reviewerRoleID).Error)
	rid, err := uuid.Parse(reviewerRoleID)
	require.NoError(t, err)
	uid := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		uid, "M4 Reviewer", "m4-reviewer-"+uid.String()+"@test.local", "hash", rid,
	).Error)
	return uid, rid
}
