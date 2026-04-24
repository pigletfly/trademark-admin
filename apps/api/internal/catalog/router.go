package catalog

import "github.com/gin-gonic/gin"

// RegisterReadRoutes mounts read endpoints on an authenticated group.
// Any role is allowed — the frontend decides who sees the menu.
func RegisterReadRoutes(authed *gin.RouterGroup, h *Handler) {
	g := authed.Group("/catalog")
	g.GET("/countries", h.GetCountries)
	g.GET("/nice-categories", h.GetNiceCategories)
}

// RegisterAdminRoutes mounts write endpoints on an admin-only group.
func RegisterAdminRoutes(admin *gin.RouterGroup, h *Handler) {
	g := admin.Group("/catalog")
	g.PATCH("/countries/:id", h.PatchCountry)
}
