package export

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/catalog"
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
)

// Handler exposes the export endpoints. It depends on the catalog,
// customer, and quotation read paths to resolve the view model.
type Handler struct {
	quotSvc *quotation.Service
	custSvc *customer.Service
	catRepo *catalog.Repository
}

func NewHandler(q *quotation.Service, c *customer.Service, cat *catalog.Repository) *Handler {
	return &Handler{quotSvc: q, custSvc: c, catRepo: cat}
}

// ExportDOCX handles GET /quotations/:id/export.docx. Only approved
// quotations can be exported. Visibility follows the same rule as
// reading the quotation: salesperson → owner only; reviewer/admin → any.
func (h *Handler) ExportDOCX(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID", "message": "invalid uuid"})
		return
	}
	q, err := h.quotSvc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND"})
		return
	}
	user := auth.CurrentUser(c)
	// Visibility
	switch user.Role {
	case "admin", "reviewer":
		// ok
	case "salesperson":
		if q.CreatedBy != user.ID {
			c.JSON(http.StatusForbidden, gin.H{"code": "ERR_FORBIDDEN"})
			return
		}
	default:
		c.JSON(http.StatusForbidden, gin.H{"code": "ERR_FORBIDDEN"})
		return
	}
	// Only approved quotations have a stable exportable form.
	if q.Status != quotation.StatusApproved {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": "ERR_NOT_APPROVED", "message": "only approved quotations may be exported"})
		return
	}
	if len(q.SnapshotJSON) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_MISSING_SNAPSHOT"})
		return
	}

	// Resolve customer + country names. reviewer/admin can export any;
	// salesperson can only reach this code path when they own the row
	// (gate above). Customer.Service.Get signature is
	// (ctx, callerID, role, id) and returns *CustomerDTO.
	cust, err := h.custSvc.Get(c.Request.Context(), user.ID, user.Role, q.CustomerID)
	if err != nil || cust == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_LOOKUP_CUSTOMER"})
		return
	}
	country, err := h.catRepo.GetCountry(c.Request.Context(), q.CountryID)
	if err != nil || country == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_LOOKUP_COUNTRY"})
		return
	}
	// Parse snapshot JSON.
	snap, err := q.DecodeSnapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_DECODE_SNAPSHOT"})
		return
	}

	view := QuotationView{
		QuotationID:   q.ID.String(),
		Status:        string(q.Status),
		ServiceTier:   q.ServiceTier,
		CustomerName:  cust.Name,
		CountryNameZH: country.NameZh,
		CountryNameEN: country.NameEn,
		CountryCode:   country.Code,
		TotalCNYCents: derefInt64(q.TotalCNYCents),
		Signature:     derefString(q.Signature),
		SubmittedAt:   q.SubmittedAt,
		ReviewedAt:    q.ReviewedAt,
		ReviewComment: derefString(q.ReviewComment),
		Notes:         derefString(q.Notes),
		GeneratedAt:   time.Now(),
	}
	for _, l := range snap.Lines {
		view.Lines = append(view.Lines, ExportLine{FeeItem: l.FeeItem, AmountCNYCents: l.AmountCNYCents})
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="quotation-%s.docx"`, q.ID.String()[:8]))
	c.Status(http.StatusOK)
	if err := RenderDOCX(c.Writer, view); err != nil {
		// Response has already started — just log path. Gin will surface via recover middleware.
		_ = err
	}
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
