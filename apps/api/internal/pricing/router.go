package pricing

import "github.com/gin-gonic/gin"

// RegisterReadRoutes mounts read endpoints on a group restricted to
// reviewer+admin (caller wires the middleware).
func RegisterReadRoutes(group *gin.RouterGroup, h *Handler) {
	g := group.Group("/pricing-entries")
	g.GET("", h.GetActive)
	g.GET("/history", h.GetHistory)
}

// RegisterAdminRoutes mounts write endpoints on an admin-only group.
func RegisterAdminRoutes(admin *gin.RouterGroup, h *Handler) {
	g := admin.Group("/pricing-entries")
	g.POST("", h.PostCreateOrReplace)
	g.POST("/:id/deprecate", h.PostDeprecate)
}
