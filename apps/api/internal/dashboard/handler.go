package dashboard

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// GetSummary handles GET /dashboard/summary — any authed user. Role determines scope.
func (h *Handler) GetSummary(c *gin.Context) {
	user := auth.CurrentUser(c)
	sum, err := h.svc.Summary(c.Request.Context(), user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sum)
}
