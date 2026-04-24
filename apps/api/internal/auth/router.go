package auth

import "github.com/gin-gonic/gin"

// RegisterRoutes wires the /auth/* routes.
func RegisterRoutes(public *gin.RouterGroup, authenticated *gin.RouterGroup, h *Handler) {
	public.POST("/auth/login", h.Login)
	public.POST("/auth/refresh", h.Refresh)

	authenticated.POST("/auth/logout", h.Logout)
	authenticated.GET("/auth/me", h.Me)
}

// RegisterAdminRoutes wires the /admin/users endpoints. The caller is
// responsible for applying RequireAuth and RequireRole("admin") to the group.
func RegisterAdminRoutes(g *gin.RouterGroup, h *AdminHandler) {
	g.GET("/admin/users", h.List)
	g.POST("/admin/users", h.Create)
	g.PATCH("/admin/users/:id", h.Update)
	g.POST("/admin/users/:id/reset-password", h.ResetPassword)
}
