package dashboard

import "github.com/gin-gonic/gin"

func RegisterRoutes(g *gin.RouterGroup, h *Handler) {
	g.GET("/dashboard/summary", h.GetSummary)
}
