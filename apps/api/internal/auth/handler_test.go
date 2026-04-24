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

func buildRouter(t *testing.T) (*gin.Engine, *auth.Service) {
	t.Helper()
	db := freshDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(auth.ServiceConfig{
		Repo:          repo,
		AccessSecret:  []byte("a"),
		RefreshSecret: []byte("r"),
		AccessTTL:     5 * time.Minute,
		RefreshTTL:    time.Hour,
	})
	h := auth.NewHandler(auth.HandlerConfig{
		Service:      svc,
		CookieSecure: false,
		AccessTTL:    5 * time.Minute,
		RefreshTTL:   time.Hour,
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	public := r.Group("/api/v1")
	authed := r.Group("/api/v1")
	authed.Use(auth.RequireAuth([]byte("a")))
	auth.RegisterRoutes(public, authed, h)
	return r, svc
}

func TestLoginHandler_setsCookiesAndMe(t *testing.T) {
	r, svc := buildRouter(t)

	if err := svc.Bootstrap(t.Context(), "admin@example.com", "pw-abcdefg-1234", "Admin"); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	body, _ := json.Marshal(auth.LoginRequest{Email: "admin@example.com", Password: "pw-abcdefg-1234"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login code = %d, body=%s", w.Code, w.Body.String())
	}
	resp := w.Result()
	var accessCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == auth.CookieAccessToken {
			accessCookie = c
		}
	}
	if accessCookie == nil || accessCookie.Value == "" {
		t.Fatalf("access cookie missing; got=%v", resp.Cookies())
	}

	// Hit /me with the access cookie.
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req2.AddCookie(accessCookie)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("me code = %d, body = %s", w2.Code, w2.Body.String())
	}
	var meBody struct {
		User auth.UserResponse `json:"user"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if meBody.User.Email != "admin@example.com" {
		t.Fatalf("email = %q", meBody.User.Email)
	}
}
