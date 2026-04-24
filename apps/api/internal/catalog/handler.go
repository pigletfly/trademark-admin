package catalog

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler exposes the catalog HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler wires a Handler with its Service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// GetCountries GET /catalog/countries[?include_disabled=true]
func (h *Handler) GetCountries(c *gin.Context) {
	includeDisabled := c.Query("include_disabled") == "true"
	rows, err := h.svc.ListCountries(c.Request.Context(), includeDisabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": "failed to list countries"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// GetNiceCategories GET /catalog/nice-categories
func (h *Handler) GetNiceCategories(c *gin.Context) {
	rows, err := h.svc.ListNiceCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": "failed to list nice categories"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// PatchCountry PATCH /catalog/countries/:id (admin).
func (h *Handler) PatchCountry(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID", "message": "invalid uuid"})
		return
	}
	var req UpdateCountryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	dto, err := h.svc.UpdateCountry(c.Request.Context(), id, req)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND", "message": "country not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": "failed to update country"})
		return
	}
	c.JSON(http.StatusOK, dto)
}
