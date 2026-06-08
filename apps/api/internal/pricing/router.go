package pricing

import "github.com/gin-gonic/gin"

// RegisterReadRoutes mounts read endpoints on a group restricted to
// reviewer+admin (caller wires the middleware).
func RegisterReadRoutes(group *gin.RouterGroup, h *Handler) {
	g := group.Group("/pricing-entries")
	g.GET("", h.GetActive)
	g.GET("/history", h.GetHistory)
	g.GET("/:id", h.GetByID)

	group.GET("/madrid-pricing-entries", h.GetActiveMadrid)
	group.GET("/single-class-pricing-entries", h.GetActiveSingleClass)
}

// RegisterAdminRoutes mounts write endpoints on an admin-only group.
func RegisterAdminRoutes(admin *gin.RouterGroup, h *Handler) {
	g := admin.Group("/pricing-entries")
	g.POST("", h.PostCreateOrReplace)
	g.POST("/:id/deprecate", h.PostDeprecate)

	madrid := admin.Group("/madrid-pricing-entries")
	madrid.POST("", h.PostCreateOrReplaceMadrid)
	madrid.POST("/:id/deprecate", h.PostDeprecateMadrid)

	single := admin.Group("/single-class-pricing-entries")
	single.POST("", h.PostCreateOrReplaceSingleClass)
	single.POST("/:id/deprecate", h.PostDeprecateSingleClass)
}
