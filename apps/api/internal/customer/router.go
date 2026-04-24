package customer

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts /customers on an authenticated group.
// Owner scoping is handled inside the service based on role.
func RegisterRoutes(authed *gin.RouterGroup, h *Handler) {
	g := authed.Group("/customers")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.PATCH("/:id", h.Patch)
}
