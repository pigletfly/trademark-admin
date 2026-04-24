package auth

import "github.com/gin-gonic/gin"

// RegisterRoutes wires the /auth/* routes. protect is the authenticated group.
func RegisterRoutes(public *gin.RouterGroup, authenticated *gin.RouterGroup, h *Handler) {
	public.POST("/auth/login", h.Login)
	public.POST("/auth/refresh", h.Refresh)

	authenticated.POST("/auth/logout", h.Logout)
	authenticated.GET("/auth/me", h.Me)
}
