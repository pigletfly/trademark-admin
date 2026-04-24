package audit

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes registers GET /admin/audit-logs on the provided group.
// The caller is responsible for attaching RequireAuth + RequireRole("admin").
func RegisterAdminRoutes(g *gin.RouterGroup, h *AdminHandler) {
	g.GET("/admin/audit-logs", h.List)
}
