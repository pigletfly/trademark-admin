//go:build integration

package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

func buildAdminRouter(t *testing.T) (*gin.Engine, *auth.Service) {
	t.Helper()
	db := freshDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(auth.ServiceConfig{
		Repo: repo, AccessSecret: []byte("a"), RefreshSecret: []byte("r"),
		AccessTTL: 5 * time.Minute, RefreshTTL: time.Hour,
	})
	adminSvc := auth.NewAdminService(repo)
	h := auth.NewHandler(auth.HandlerConfig{Service: svc, AccessTTL: 5 * time.Minute, RefreshTTL: time.Hour})
	ah := auth.NewAdminHandler(adminSvc)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	public := r.Group("/api/v1")
	authed := r.Group("/api/v1")
	authed.Use(auth.RequireAuth([]byte("a")))
	adminOnly := r.Group("/api/v1")
	adminOnly.Use(auth.RequireAuth([]byte("a")), auth.RequireRole("admin"))

	auth.RegisterRoutes(public, authed, h)
	auth.RegisterAdminRoutes(adminOnly, ah)
	return r, svc
}

func adminCookie(t *testing.T, r *gin.Engine) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(auth.LoginRequest{Email: "admin@example.com", Password: "pw-abcdefg-1234"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d / %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.CookieAccessToken {
			return c
		}
	}
	t.Fatalf("access cookie missing")
	return nil
}

func TestAdmin_CreateAndListUser(t *testing.T) {
	r, svc := buildAdminRouter(t)
	_ = svc.Bootstrap(t.Context(), "admin@example.com", "pw-abcdefg-1234", "Admin")
	cookie := adminCookie(t, r)

	body := `{"name":"Bob","email":"bob@example.com","role":"salesperson","password":"another-pw-12"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users",
		bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create code = %d, body = %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("list code = %d, body = %s", w2.Code, w2.Body.String())
	}
	var listBody struct {
		Items []auth.UserResponse `json:"items"`
		Total int64               `json:"total"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &listBody)
	if listBody.Total != 2 {
		t.Fatalf("expected 2 users (admin + bob), got %d", listBody.Total)
	}
}

func TestAdmin_NonAdminForbidden(t *testing.T) {
	r, svc := buildAdminRouter(t)
	_ = svc.Bootstrap(t.Context(), "admin@example.com", "pw-abcdefg-1234", "Admin")
	cookie := adminCookie(t, r)

	// Create a salesperson user
	body := `{"name":"Sam","email":"sam@example.com","role":"salesperson","password":"another-pw-12"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users",
		bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Log in as sam
	samBody, _ := json.Marshal(auth.LoginRequest{Email: "sam@example.com", Password: "another-pw-12"})
	samReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(samBody))
	samReq.Header.Set("Content-Type", "application/json")
	samResp := httptest.NewRecorder()
	r.ServeHTTP(samResp, samReq)
	if samResp.Code != http.StatusOK {
		t.Fatalf("sam login failed: %d", samResp.Code)
	}
	var samCookie *http.Cookie
	for _, c := range samResp.Result().Cookies() {
		if c.Name == auth.CookieAccessToken {
			samCookie = c
		}
	}

	// Sam attempts /admin/users → 403
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	listReq.AddCookie(samCookie)
	listResp := httptest.NewRecorder()
	r.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", listResp.Code)
	}
}
