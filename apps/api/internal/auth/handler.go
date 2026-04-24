package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler holds auth-related HTTP handlers.
type Handler struct {
	svc          *Service
	cookieSecure bool
	cookieDomain string // empty = host-only
	accessTTL    time.Duration
	refreshTTL   time.Duration
}

// HandlerConfig bundles non-service dependencies.
type HandlerConfig struct {
	Service      *Service
	CookieSecure bool
	CookieDomain string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
}

// NewHandler constructs a Handler.
func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		svc:          cfg.Service,
		cookieSecure: cfg.CookieSecure,
		cookieDomain: cfg.CookieDomain,
		accessTTL:    cfg.AccessTTL,
		refreshTTL:   cfg.RefreshTTL,
	}
}

// Login handles POST /auth/login.
func (h *Handler) Login(c *gin.Context) {
	var body LoginRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": err.Error()})
		return
	}
	result, err := h.svc.Login(c.Request.Context(), body.Email, body.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "ERR_INVALID_CREDENTIALS", "message": "email or password incorrect"})
			return
		}
		if errors.Is(err, ErrUserDisabled) {
			c.JSON(http.StatusForbidden, gin.H{"code": "ERR_USER_DISABLED", "message": "account disabled"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}

	h.setAuthCookies(c, result.AccessToken, result.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"user": ToResponse(result.User)})
}

// Refresh handles POST /auth/refresh.
func (h *Handler) Refresh(c *gin.Context) {
	rt, err := c.Cookie(CookieRefreshToken)
	if err != nil || rt == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "ERR_UNAUTHORIZED", "message": "refresh token missing"})
		return
	}
	result, err := h.svc.Refresh(c.Request.Context(), rt)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "ERR_UNAUTHORIZED", "message": err.Error()})
		return
	}
	h.setAuthCookies(c, result.AccessToken, result.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"user": ToResponse(result.User)})
}

// Logout handles POST /auth/logout.
func (h *Handler) Logout(c *gin.Context) {
	h.clearAuthCookies(c)
	c.Status(http.StatusNoContent)
}

// Me handles GET /auth/me.
func (h *Handler) Me(c *gin.Context) {
	u := CurrentUser(c)
	user, err := h.svc.Me(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": ToResponse(user)})
}

func (h *Handler) setAuthCookies(c *gin.Context, access, refresh string) {
	h.setCookie(c, CookieAccessToken, access, int(h.accessTTL.Seconds()), true)
	h.setCookie(c, CookieRefreshToken, refresh, int(h.refreshTTL.Seconds()), true)
	// CSRF token is NOT httpOnly: the JS client must read it and echo via header.
	csrf, _ := randomToken(24)
	h.setCookie(c, CookieCSRFToken, csrf, int(h.refreshTTL.Seconds()), false)
}

func (h *Handler) clearAuthCookies(c *gin.Context) {
	h.setCookie(c, CookieAccessToken, "", -1, true)
	h.setCookie(c, CookieRefreshToken, "", -1, true)
	h.setCookie(c, CookieCSRFToken, "", -1, false)
}

func (h *Handler) setCookie(c *gin.Context, name, value string, maxAge int, httpOnly bool) {
	// SameSite=Lax so same-site navigations send the cookie but cross-site POSTs do not.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, value, maxAge, "/", h.cookieDomain, h.cookieSecure, httpOnly)
}

// randomToken returns a URL-safe random string of approx `n` random bytes.
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
