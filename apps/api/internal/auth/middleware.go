package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Cookie names. Exported so handlers and frontend share them.
const (
	CookieAccessToken  = "tm_access_token"
	CookieRefreshToken = "tm_refresh_token"
	CookieCSRFToken    = "tm_csrf_token"
)

// currentUserKey is the Gin context key storing the authenticated user summary.
const currentUserKey = "auth.currentUser"

// CurrentUserSummary is what RequireAuth puts into the Gin context. Handlers
// should use CurrentUser to fetch it.
type CurrentUserSummary struct {
	ID   uuid.UUID
	Role string // role code
}

// CurrentUser returns the authenticated user summary from the Gin context,
// or a zero value if no RequireAuth middleware ran.
func CurrentUser(c *gin.Context) CurrentUserSummary {
	v, ok := c.Get(currentUserKey)
	if !ok {
		return CurrentUserSummary{}
	}
	return v.(CurrentUserSummary)
}

// RequireAuth verifies the access token cookie and injects the user summary.
func RequireAuth(accessSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(CookieAccessToken)
		if err != nil || cookie == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "ERR_UNAUTHORIZED",
				"message": "authentication required",
			})
			return
		}
		claims, err := ParseAccessToken(accessSecret, cookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "ERR_UNAUTHORIZED",
				"message": "invalid or expired token",
			})
			return
		}
		c.Set(currentUserKey, CurrentUserSummary{ID: claims.UserID, Role: claims.Role})
		c.Next()
	}
}

// RequireRole checks that the authenticated user belongs to one of the
// allowed roles. RequireAuth must precede it.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		user := CurrentUser(c)
		if _, ok := allowed[user.Role]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "ERR_FORBIDDEN",
				"message": "role not permitted",
			})
			return
		}
		c.Next()
	}
}

// CSRF enforces double-submit token validation for non-safe HTTP methods.
// Cookie `tm_csrf_token` (not httpOnly) must equal header `X-CSRF-Token`.
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isSafeMethod(c.Request.Method) {
			c.Next()
			return
		}
		cookie, err := c.Cookie(CookieCSRFToken)
		header := c.GetHeader("X-CSRF-Token")
		if err != nil || cookie == "" || header == "" || cookie != header {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "ERR_CSRF",
				"message": "CSRF token missing or mismatched",
			})
			return
		}
		c.Next()
	}
}

func isSafeMethod(m string) bool {
	switch strings.ToUpper(m) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}
