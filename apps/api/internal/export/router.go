package export

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the export endpoints on an authed group.
// The handler enforces role + approved-only internally.
func RegisterRoutes(g *gin.RouterGroup, h *Handler) {
	g.GET("/quotations/:id/export.docx", h.ExportDOCX)
}
