package quotation

import (
	"github.com/gin-gonic/gin"
)

// RegisterAuthedRoutes registers endpoints available to any authed user.
// Inside the handlers we apply finer-grained role/ownership checks.
func RegisterAuthedRoutes(g *gin.RouterGroup, h *Handler) {
	g.POST("/quotations", h.Create)
	g.GET("/quotations", h.List)
	g.GET("/quotations/:id", h.Get)
	g.GET("/quotations/:id/history", h.History)
	g.PATCH("/quotations/:id", h.Update)
	g.POST("/quotations/:id/submit", h.Submit)
	g.POST("/quotations/:id/cancel", h.Cancel)
}

// RegisterReviewerRoutes registers reviewer-only transitions. The group
// is expected to already chain RequireRole("reviewer","admin").
func RegisterReviewerRoutes(g *gin.RouterGroup, h *Handler) {
	g.POST("/quotations/:id/approve", h.Review(true))
	g.POST("/quotations/:id/reject", h.Review(false))
}
