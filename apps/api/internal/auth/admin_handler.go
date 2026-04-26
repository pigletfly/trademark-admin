package auth

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AdminHandler exposes /admin/users endpoints.
type AdminHandler struct {
	svc *AdminService
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(svc *AdminService) *AdminHandler { return &AdminHandler{svc: svc} }

// List handles GET /admin/users.
func (h *AdminHandler) List(c *gin.Context) {
	q := c.Query("q")
	role := c.Query("role")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	users, total, err := h.svc.ListUsers(c.Request.Context(), q, role, page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	items := make([]UserResponse, len(users))
	for i := range users {
		items[i] = ToResponse(&users[i])
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"page":      page,
		"page_size": size,
		"total":     total,
	})
}

// Create handles POST /admin/users.
func (h *AdminHandler) Create(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": err.Error()})
		return
	}
	u, err := h.svc.CreateUser(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			c.JSON(http.StatusConflict, gin.H{"code": "ERR_EMAIL_TAKEN", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": ToResponse(u)})
}

// Update handles PATCH /admin/users/:id.
func (h *AdminHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": "invalid id"})
		return
	}
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": err.Error()})
		return
	}
	// Self-protection: a user cannot change their own role or status.
	// Name/phone edits remain allowed so admins can still fix typos on their
	// own profile.
	if actor := CurrentUser(c); actor.ID == id && (req.RoleCode != nil || req.Status != nil) {
		c.JSON(http.StatusConflict, gin.H{
			"code":    "ERR_SELF_PROTECTED",
			"message": "cannot change your own role or status",
		})
		return
	}
	u, err := h.svc.UpdateUser(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND", "message": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": ToResponse(u)})
}

// ResetPassword handles POST /admin/users/:id/reset-password.
func (h *AdminHandler) ResetPassword(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": "invalid id"})
		return
	}
	// Self-protection: admins cannot reset their own password via this flow —
	// rely on another admin or a dedicated change-password endpoint.
	if actor := CurrentUser(c); actor.ID == id {
		c.JSON(http.StatusConflict, gin.H{
			"code":    "ERR_SELF_PROTECTED",
			"message": "cannot reset your own password",
		})
		return
	}
	pw, err := h.svc.ResetPassword(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND", "message": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ResetPasswordResponse{Password: pw})
}
