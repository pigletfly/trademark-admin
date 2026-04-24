package audit

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserProvider returns the current user's ID from a Gin context. auth package
// injects CurrentUser — this interface lets audit avoid importing auth and
// creating a dependency cycle.
type UserProvider func(c *gin.Context) (uuid.UUID, bool)

// Middleware returns a Gin middleware that writes a row to audit_logs for every
// non-safe request. Writes are best-effort: if the write fails, it logs and
// continues (never blocks the client response).
func Middleware(repo *Repository, getUserID UserProvider, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only log state-changing methods.
		if isSafe(c.Request.Method) {
			c.Next()
			return
		}

		// Capture request body for audit. Limited to 32 KiB to avoid runaway memory.
		var body []byte
		if c.Request.Body != nil {
			body, _ = io.ReadAll(io.LimitReader(c.Request.Body, 32*1024))
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}
		body = scrubSensitive(body)

		c.Next()

		entry := &Log{
			Action:       c.Request.Method + " " + c.FullPath(),
			ResourceType: c.FullPath(),
			ResourceID:   c.Param("id"),
			ChangesJSON:  body,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			CreatedAt:    time.Now(),
		}
		if uid, ok := getUserID(c); ok {
			entry.UserID = &uid
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := repo.Insert(ctx, entry); err != nil {
			log.Warn("audit insert failed", "error", err, "path", c.FullPath())
		}
	}
}

func isSafe(method string) bool {
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return true
	}
	return false
}

// scrubSensitive redacts password-like fields in a JSON body. For the MVP we
// only look for the literal `"password":"..."` and replace with `"password":"[REDACTED]"`.
// This is intentionally simple; a proper parser should arrive with P6+.
func scrubSensitive(body []byte) []byte {
	return redactJSONField(body, `"password":`)
}

func redactJSONField(body []byte, needle string) []byte {
	idx := bytes.Index(body, []byte(needle))
	if idx < 0 {
		return body
	}
	start := idx + len(needle)
	// Skip whitespace and opening quote.
	for start < len(body) && (body[start] == ' ' || body[start] == '\t') {
		start++
	}
	if start >= len(body) || body[start] != '"' {
		return body
	}
	end := start + 1
	for end < len(body) && body[end] != '"' {
		if body[end] == '\\' && end+1 < len(body) {
			end += 2
			continue
		}
		end++
	}
	if end >= len(body) {
		return body
	}
	out := make([]byte, 0, len(body))
	out = append(out, body[:start+1]...)
	out = append(out, []byte("[REDACTED]")...)
	out = append(out, body[end:]...)
	return out
}

// RedactForTest exposes scrubSensitive to tests in the same package family.
// Use only from _test.go files.
func RedactForTest(b []byte) []byte { return scrubSensitive(b) }
