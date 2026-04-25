package export

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/catalog"
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
)

// Error sentinels buildView returns; handlers map to HTTP codes via writeViewErr.
var (
	ErrNotApproved = errors.New("export: quotation not approved")
	ErrForbidden   = errors.New("export: not authorized")
)

// Handler exposes the export endpoints. It depends on the catalog,
// customer, and quotation read paths to resolve the view model; the
// Service + Signer power the new POST /export + public download route.
//
// exportSvc and signer may be nil for callers that only need the
// legacy ExportDOCX path — the new Export/Download handlers will panic
// when invoked with nil deps, which is by design so misconfiguration
// fails loudly rather than silently serving 500s.
type Handler struct {
	quotSvc   *quotation.Service
	custSvc   *customer.Service
	catRepo   *catalog.Repository
	exportSvc *Service
	signer    *Signer
}

// NewHandler constructs a Handler. Pass nil for svc/signer if you only
// need the legacy ExportDOCX route.
func NewHandler(
	q *quotation.Service,
	c *customer.Service,
	cat *catalog.Repository,
	svc *Service,
	signer *Signer,
) *Handler {
	return &Handler{quotSvc: q, custSvc: c, catRepo: cat, exportSvc: svc, signer: signer}
}

// ExportDOCX handles GET /quotations/:id/export.docx — the legacy path
// that streams a .docx inline. Kept for backward compatibility with
// existing handler_test.go and frontend; the POST /export route is the
// new, preferred interface.
func (h *Handler) ExportDOCX(c *gin.Context) {
	id, ok := parseExportID(c, "id")
	if !ok {
		return
	}
	q, err := h.quotSvc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND"})
		return
	}
	user := auth.CurrentUser(c)
	view, err := h.buildView(c.Request.Context(), q, user)
	if err != nil {
		writeViewErr(c, err)
		return
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="quotation-%s.docx"`, q.ID.String()[:8]))
	c.Status(http.StatusOK)
	if err := RenderDOCX(c.Writer, view); err != nil {
		// Response has already started — just drop it. Gin will surface via recover middleware.
		_ = err
	}
}

// Export handles POST /quotations/:id/export with JSON body
// {format, language}. It renders the requested artifact via the
// Service, stores it, and returns ExportFileDTO with a signed
// short-lived download_url.
func (h *Handler) Export(c *gin.Context) {
	id, ok := parseExportID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Format   Format   `json:"format"   binding:"required"`
		Language Language `json:"language" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	if !isValidFormat(req.Format) || !isValidLanguage(req.Language) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_EXPORT_OPTS"})
		return
	}
	q, err := h.quotSvc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND"})
		return
	}
	user := auth.CurrentUser(c)
	view, err := h.buildView(c.Request.Context(), q, user)
	if err != nil {
		writeViewErr(c, err)
		return
	}

	var rec *ExportFile
	switch req.Format {
	case FormatPDF:
		rec, err = h.exportSvc.GeneratePDF(c.Request.Context(), view, req.Language, id, user.ID)
	case FormatDOCX:
		rec, err = h.exportSvc.GenerateDOCX(c.Request.Context(), view, req.Language, id, user.ID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_EXPORT_FAILED", "message": err.Error()})
		return
	}
	token := h.signer.Sign(rec.ID, rec.ExpiresAt)
	c.JSON(http.StatusCreated, exportFileDTO(rec, token))
}

// Download handles GET /exports/:id/download?token=... — PUBLIC endpoint.
// The HMAC'd token authorizes; we still verify expires_at server-side
// because a token alone must not outlive the stored file.
func (h *Handler) Download(c *gin.Context) {
	id, ok := parseExportID(c, "id")
	if !ok {
		return
	}
	token := c.Query("token")
	verifiedID, err := h.signer.Verify(token)
	if err != nil || verifiedID != id {
		c.JSON(http.StatusForbidden, gin.H{"code": "ERR_INVALID_TOKEN"})
		return
	}
	f, err := h.exportSvc.Repo().Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "ERR_EXPORT_EXPIRED_OR_MISSING"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	file, err := h.exportSvc.Storage().Open(f.FilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_STORAGE_READ"})
		return
	}
	defer file.Close()

	ctype := "application/pdf"
	ext := "pdf"
	if f.Format == FormatDOCX {
		ctype = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		ext = "docx"
	}
	fname := "quotation-" + f.QuotationID.String()[:8] + "-" + string(f.Language) + "." + ext
	c.Header("Content-Type", ctype)
	c.Header("Content-Disposition", `attachment; filename="`+fname+`"`)
	c.Header("Content-Length", strconv.FormatInt(f.FileSize, 10))
	c.Header("X-Content-SHA256", f.SHA256)
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, file); err != nil {
		// Headers already sent; nothing useful to surface here.
		_ = err
	}
}

// buildView collects the data needed to render a quotation. It enforces
// role-based visibility (salespeople see only their own), refuses to
// export anything that isn't approved, and resolves customer/country/
// snapshot into the flat QuotationView the renderers consume. Returns
// ErrForbidden / ErrNotApproved sentinels or a wrapped error.
func (h *Handler) buildView(
	ctx context.Context,
	q *quotation.Quotation,
	user auth.CurrentUserSummary,
) (QuotationView, error) {
	switch user.Role {
	case "admin", "reviewer":
		// ok
	case "salesperson":
		if q.CreatedBy != user.ID {
			return QuotationView{}, ErrForbidden
		}
	default:
		return QuotationView{}, ErrForbidden
	}
	if q.Status != quotation.StatusApproved {
		return QuotationView{}, ErrNotApproved
	}
	if len(q.SnapshotJSON) == 0 {
		return QuotationView{}, fmt.Errorf("export: missing snapshot")
	}
	cust, err := h.custSvc.Get(ctx, user.ID, user.Role, q.CustomerID)
	if err != nil || cust == nil {
		return QuotationView{}, fmt.Errorf("export: lookup customer: %w", err)
	}
	country, err := h.catRepo.GetCountry(ctx, q.CountryID)
	if err != nil || country == nil {
		return QuotationView{}, fmt.Errorf("export: lookup country: %w", err)
	}
	snap, err := q.DecodeSnapshot()
	if err != nil {
		return QuotationView{}, fmt.Errorf("export: decode snapshot: %w", err)
	}
	v := QuotationView{
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
		v.Lines = append(v.Lines, ExportLine{FeeItem: l.FeeItem, AmountCNYCents: l.AmountCNYCents})
	}
	return v, nil
}

// --- helpers ---

// parseExportID reads :key as a UUID, writes a 400 and returns false on error.
func parseExportID(c *gin.Context, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(key))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID", "message": "invalid uuid"})
		return uuid.Nil, false
	}
	return id, true
}

// writeViewErr maps buildView sentinels to HTTP responses.
func writeViewErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": "ERR_FORBIDDEN"})
	case errors.Is(err, ErrNotApproved):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": "ERR_NOT_APPROVED", "message": "only approved quotations may be exported"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
	}
}

func isValidFormat(f Format) bool     { return f == FormatPDF || f == FormatDOCX }
func isValidLanguage(l Language) bool { return l == LanguageZH || l == LanguageEN || l == LanguageBilingual }

// ExportFileDTO is the JSON response for POST /quotations/:id/export.
type ExportFileDTO struct {
	ID          uuid.UUID `json:"id"`
	QuotationID uuid.UUID `json:"quotation_id"`
	Format      Format    `json:"format"`
	Language    Language  `json:"language"`
	SHA256      string    `json:"sha256"`
	FileSize    int64     `json:"file_size"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	DownloadURL string    `json:"download_url"`
}

func exportFileDTO(f *ExportFile, token string) ExportFileDTO {
	return ExportFileDTO{
		ID:          f.ID,
		QuotationID: f.QuotationID,
		Format:      f.Format,
		Language:    f.Language,
		SHA256:      f.SHA256,
		FileSize:    f.FileSize,
		ExpiresAt:   f.ExpiresAt,
		CreatedAt:   f.CreatedAt,
		DownloadURL: "/api/v1/exports/" + f.ID.String() + "/download?token=" + token,
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
