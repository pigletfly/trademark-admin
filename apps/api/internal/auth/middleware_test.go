package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

func setupRouter(accessSecret []byte, requireRoles ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	protected := r.Group("/")
	protected.Use(auth.RequireAuth(accessSecret))
	if len(requireRoles) > 0 {
		protected.Use(auth.RequireRole(requireRoles...))
	}
	protected.GET("/whoami", func(c *gin.Context) {
		user := auth.CurrentUser(c)
		c.JSON(http.StatusOK, gin.H{"user_id": user.ID, "role": user.Role})
	})
	return r
}

func TestRequireAuth_missingCookie(t *testing.T) {
	r := setupRouter([]byte("s"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", w.Code)
	}
}

func TestRequireAuth_validCookie(t *testing.T) {
	secret := []byte("sekret")
	uid := uuid.New()
	token, err := auth.IssueAccessToken(secret, uid, "salesperson", time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	r := setupRouter(secret)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccessToken, Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestRequireRole_forbidden(t *testing.T) {
	secret := []byte("sekret")
	uid := uuid.New()
	token, _ := auth.IssueAccessToken(secret, uid, "salesperson", time.Minute)

	r := setupRouter(secret, "admin")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccessToken, Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403", w.Code)
	}
}

func TestRequireRole_allowed(t *testing.T) {
	secret := []byte("sekret")
	uid := uuid.New()
	token, _ := auth.IssueAccessToken(secret, uid, "admin", time.Minute)

	r := setupRouter(secret, "admin", "reviewer")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccessToken, Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestCSRF_blocksMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(auth.CSRF())
	r.POST("/do", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/do", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieCSRFToken, Value: "abc"})
	// Header missing
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403", w.Code)
	}
}

func TestCSRF_passesWhenMatches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(auth.CSRF())
	r.POST("/do", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/do", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieCSRFToken, Value: "abc"})
	req.Header.Set("X-CSRF-Token", "abc")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
}

func TestCSRF_ignoresSafeMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(auth.CSRF())
	r.GET("/read", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/read", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CSRF must pass through GET; got %d", w.Code)
	}
}
