package quotation

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

// Handler wires HTTP to Service. Role gating is enforced by the
// middleware at the router level — inside handlers we only do ownership
// checks (e.g. a salesperson can't read another salesperson's draft).
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// POST /quotations — create draft. Any authenticated user may create.
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	user := auth.CurrentUser(c)
	if user.ID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "ERR_UNAUTHORIZED"})
		return
	}
	q, err := h.svc.Create(c.Request.Context(), user.ID, req)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, ToResponse(q))
}

// GET /quotations/:id — read one. Salesperson may only read their own.
func (h *Handler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	q, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	if !h.canRead(c, q) {
		c.JSON(http.StatusForbidden, gin.H{"code": "ERR_FORBIDDEN", "message": "forbidden"})
		return
	}
	c.JSON(http.StatusOK, ToResponse(q))
}

// GET /quotations — list. Role shapes the scope:
//   - salesperson → only their own
//   - reviewer/admin → all
func (h *Handler) List(c *gin.Context) {
	user := auth.CurrentUser(c)
	f := ListFilter{
		Page:     atoiDefault(c.Query("page"), 1),
		PageSize: atoiDefault(c.Query("page_size"), 20),
	}
	if user.Role == "salesperson" {
		uid := user.ID
		f.OwnerID = &uid
	}
	if s := c.Query("status"); s != "" {
		st := Status(s)
		f.Status = &st
	}
	if cid := c.Query("customer_id"); cid != "" {
		if cuid, err := uuid.Parse(cid); err == nil {
			f.CustomerID = &cuid
		}
	}
	rows, total, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	items := make([]Response, 0, len(rows))
	for i := range rows {
		items = append(items, ToResponse(&rows[i]))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": f.Page, "page_size": f.PageSize})
}

// PATCH /quotations/:id — update editable fields while draft.
func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req UpdateDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	user := auth.CurrentUser(c)
	if err := h.svc.UpdateDraft(c.Request.Context(), id, user.ID, req); err != nil {
		h.writeServiceErr(c, err)
		return
	}
	q, _ := h.svc.Get(c.Request.Context(), id)
	c.JSON(http.StatusOK, ToResponse(q))
}

// POST /quotations/:id/submit — draft → submitted.
func (h *Handler) Submit(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	user := auth.CurrentUser(c)
	q, err := h.svc.Submit(c.Request.Context(), id, user.ID)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, ToResponse(q))
}

// POST /quotations/:id/approve | /reject — reviewer only.
func (h *Handler) Review(approve bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req ReviewRequest
		_ = c.ShouldBindJSON(&req) // body optional
		user := auth.CurrentUser(c)
		q, err := h.svc.Review(c.Request.Context(), id, user.ID, approve, req.Comment)
		if err != nil {
			h.writeServiceErr(c, err)
			return
		}
		c.JSON(http.StatusOK, ToResponse(q))
	}
}

// POST /quotations/:id/cancel — owner cancels a draft.
func (h *Handler) Cancel(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req ReviewRequest // reusing shape for comment
	_ = c.ShouldBindJSON(&req)
	user := auth.CurrentUser(c)
	if err := h.svc.Cancel(c.Request.Context(), id, user.ID, req.Comment); err != nil {
		h.writeServiceErr(c, err)
		return
	}
	q, _ := h.svc.Get(c.Request.Context(), id)
	c.JSON(http.StatusOK, ToResponse(q))
}

// POST /quotations/:id/withdraw — owner returns a submitted quotation
// to draft. Service enforces owner + status; we just marshal.
func (h *Handler) Withdraw(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	user := auth.CurrentUser(c)
	q, err := h.svc.Withdraw(c.Request.Context(), id, user.ID)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, ToResponse(q))
}

// POST /quotations/:id/copy — any authed user may clone a quotation
// they can see. We deliberately do NOT call canRead here because the
// product rule is: if you can list/search a quotation, you can copy it.
// (List scoping already filters salespeople to their own rows.)
func (h *Handler) Copy(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	user := auth.CurrentUser(c)
	q, err := h.svc.Copy(c.Request.Context(), id, user.ID)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, ToResponse(q))
}

// POST /quotations/preview — non-persistent pricing lookup for the
// 5-step wizard. Any authenticated user may call; the service returns
// the same error taxonomy as Submit.
func (h *Handler) Preview(c *gin.Context) {
	var req PreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	resp, err := h.svc.Preview(c.Request.Context(), req)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// POST /quotations/:id/adjust — reviewer/admin rewrites the submitted
// snapshot in place. Router-level RequireRole gates the route.
func (h *Handler) Adjust(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req AdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	user := auth.CurrentUser(c)
	q, err := h.svc.Adjust(c.Request.Context(), id, user.ID, req.Lines, req.Comment)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, ToResponse(q))
}

// GET /quotations/:id/history.
func (h *Handler) History(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	q, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	if !h.canRead(c, q) {
		c.JSON(http.StatusForbidden, gin.H{"code": "ERR_FORBIDDEN", "message": "forbidden"})
		return
	}
	rows, err := h.svc.History(c.Request.Context(), id)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	out := make([]HistoryEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, HistoryEntry{
			FromStatus: r.FromStatus, ToStatus: r.ToStatus,
			ActorID: r.ActorID, Comment: r.Comment, At: r.At,
			DiffJSON: json.RawMessage(r.DiffJSON),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

// canRead returns true when the current user can read this quotation.
// Salespeople can only read their own; reviewer + admin read all.
func (h *Handler) canRead(c *gin.Context, q *Quotation) bool {
	u := auth.CurrentUser(c)
	switch u.Role {
	case "admin", "reviewer":
		return true
	case "salesperson":
		return q.CreatedBy == u.ID
	}
	return false
}

func (h *Handler) writeServiceErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND", "message": "quotation not found"})
	case errors.Is(err, ErrNotOwner):
		c.JSON(http.StatusForbidden, gin.H{"code": "ERR_NOT_OWNER", "message": "not owner"})
	case errors.Is(err, ErrInvalidTransition):
		c.JSON(http.StatusConflict, gin.H{"code": "ERR_INVALID_TRANSITION", "message": "invalid status transition"})
	case errors.Is(err, ErrInvalidTier):
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_TIER", "message": "invalid service tier"})
	case errors.Is(err, ErrMissingPricing):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": "ERR_MISSING_PRICING", "message": "no active pricing for country+tier"})
	case errors.Is(err, ErrEmptyAdjust):
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_EMPTY_ADJUST", "message": "adjust requires at least one line"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
	}
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID", "message": "invalid uuid"})
		return uuid.Nil, false
	}
	return id, true
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}
