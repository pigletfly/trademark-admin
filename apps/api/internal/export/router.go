package export

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the legacy GET /quotations/:id/export.docx
// endpoint on an authed group. Kept for backward compatibility while
// the frontend migrates to POST /export + public download.
// The handler enforces role + approved-only internally.
func RegisterRoutes(g *gin.RouterGroup, h *Handler) {
	g.GET("/quotations/:id/export.docx", h.ExportDOCX)
}

// RegisterAuthedRoutes mounts the new POST /quotations/:id/export on
// an authed group. Visibility is enforced by the handler (salesperson
// → owner only; reviewer/admin → any). Role-level access is assumed
// to be gated by middleware at the group level.
func RegisterAuthedRoutes(g *gin.RouterGroup, h *Handler) {
	g.POST("/quotations/:id/export", h.Export)
}

// RegisterPublicRoutes mounts GET /exports/:id/download on a PUBLIC
// group — the signed HMAC token in ?token= authorizes the download,
// not a cookie. Mount this alongside the authed routes under /api/v1
// but WITHOUT RequireAuth or CSRF middleware.
func RegisterPublicRoutes(g *gin.RouterGroup, h *Handler) {
	g.GET("/exports/:id/download", h.Download)
}
