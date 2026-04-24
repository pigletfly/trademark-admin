package audit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AdminHandler exposes admin audit-log queries.
type AdminHandler struct {
	repo *Repository
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(repo *Repository) *AdminHandler { return &AdminHandler{repo: repo} }

// List handles GET /admin/audit-logs.
func (h *AdminHandler) List(c *gin.Context) {
	var f ListFilter
	f.ResourceType = c.Query("resource_type")
	f.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	f.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if uidStr := c.Query("user_id"); uidStr != "" {
		uid, err := uuid.Parse(uidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": "invalid user_id"})
			return
		}
		f.UserID = &uid
	}
	if fromStr := c.Query("from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": "invalid from (expect RFC3339)"})
			return
		}
		f.From = &t
	}
	if toStr := c.Query("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": "invalid to (expect RFC3339)"})
			return
		}
		f.To = &t
	}

	items, total, err := h.repo.List(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"page":      f.Page,
		"page_size": f.PageSize,
		"total":     total,
	})
}
