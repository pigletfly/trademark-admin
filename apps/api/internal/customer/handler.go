package customer

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

// Handler exposes customer HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler wires a Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// List GET /customers[?q=&page=&page_size=]
func (h *Handler) List(c *gin.Context) {
	caller := auth.CurrentUser(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.svc.List(c.Request.Context(), caller.ID, caller.Role, c.Query("q"), page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Get GET /customers/:id
func (h *Handler) Get(c *gin.Context) {
	caller := auth.CurrentUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID", "message": "invalid uuid"})
		return
	}
	dto, err := h.svc.Get(c.Request.Context(), caller.ID, caller.Role, id)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND", "message": "customer not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto)
}

// Create POST /customers
func (h *Handler) Create(c *gin.Context) {
	caller := auth.CurrentUser(c)
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	dto, err := h.svc.Create(c.Request.Context(), caller.ID, req)
	if errors.Is(err, ErrDuplicateName) {
		c.JSON(http.StatusConflict, gin.H{"code": "ERR_DUPLICATE_NAME", "message": "a customer with this name already exists"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, dto)
}

// Patch PATCH /customers/:id
func (h *Handler) Patch(c *gin.Context) {
	caller := auth.CurrentUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID", "message": "invalid uuid"})
		return
	}
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	dto, err := h.svc.Update(c.Request.Context(), caller.ID, caller.Role, id, req)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND", "message": "customer not found"})
		return
	}
	if errors.Is(err, ErrDuplicateName) {
		c.JSON(http.StatusConflict, gin.H{"code": "ERR_DUPLICATE_NAME", "message": "name already exists"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto)
}
