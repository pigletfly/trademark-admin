package pricing

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

// Handler exposes pricing HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler wires a Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// GetActive GET /pricing-entries?country_id=...&service_tier=...
func (h *Handler) GetActive(c *gin.Context) {
	var f ActiveFilter
	if s := c.Query("country_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_QUERY", "message": "invalid country_id"})
			return
		}
		f.CountryID = &id
	}
	if s := c.Query("service_tier"); s != "" {
		if !IsValidServiceTier(s) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_QUERY", "message": "invalid service_tier"})
			return
		}
		f.ServiceTier = &s
	}
	rows, err := h.svc.ListActive(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": "failed to list pricing entries"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// GetHistory GET /pricing-entries/history?country_id=...&service_tier=...&fee_item=...
func (h *Handler) GetHistory(c *gin.Context) {
	cID, err := uuid.Parse(c.Query("country_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_QUERY", "message": "invalid country_id"})
		return
	}
	tier := c.Query("service_tier")
	if !IsValidServiceTier(tier) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_QUERY", "message": "invalid service_tier"})
		return
	}
	item := c.Query("fee_item")
	if item == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_QUERY", "message": "fee_item required"})
		return
	}
	rows, err := h.svc.ListHistory(c.Request.Context(), HistoryFilter{
		CountryID:   cID,
		ServiceTier: tier,
		FeeItem:     item,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": "failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// GetByID GET /pricing-entries/:id — returns one entry (active or
// deprecated). Used by M4 traceability: snapshot lines carry a
// source_pricing_entry_id; this endpoint lets the client expand that
// id back into full pricing context (effective window, amount, etc.).
func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID", "message": "invalid id"})
		return
	}
	dto, err := h.svc.GetByID(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": "failed to fetch pricing entry"})
		return
	}
	c.JSON(http.StatusOK, dto)
}

// PostCreateOrReplace POST /pricing-entries (admin).
func (h *Handler) PostCreateOrReplace(c *gin.Context) {
	var req CreateOrReplaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	u := auth.CurrentUser(c)
	if u.ID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "ERR_UNAUTHORIZED"})
		return
	}
	dto, err := h.svc.CreateOrReplace(c.Request.Context(), u.ID, req)
	if errors.Is(err, ErrInvalidTier) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_TIER", "message": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, dto)
}

// PostDeprecate POST /pricing-entries/:id/deprecate (admin).
func (h *Handler) PostDeprecate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID"})
		return
	}
	var req DeprecateRequest
	// Body is optional; ignore binding errors when body is empty.
	_ = c.ShouldBindJSON(&req)
	dto, err := h.svc.Deprecate(c.Request.Context(), id, req)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND"})
		return
	}
	if errors.Is(err, ErrNoActive) {
		c.JSON(http.StatusConflict, gin.H{"code": "ERR_ALREADY_DEPRECATED", "message": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto)
}
