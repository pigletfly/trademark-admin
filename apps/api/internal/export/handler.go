package export

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/catalog"
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
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
	quotSvc     *quotation.Service
	custSvc     *customer.Service
	catRepo     *catalog.Repository
	pricingRepo *pricing.Repository
	exportSvc   *Service
	signer      *Signer
}

type singleGroup struct {
	CountryID          uuid.UUID
	CountryArea        string
	SourcePricingID    uuid.UUID
	ApplicationUntaxed int64
}

// NewHandler constructs a Handler. Pass nil for svc/signer if you only
// need the legacy ExportDOCX route.
func NewHandler(
	q *quotation.Service,
	c *customer.Service,
	cat *catalog.Repository,
	pricingRepo *pricing.Repository,
	svc *Service,
	signer *Signer,
) *Handler {
	return &Handler{
		quotSvc:     q,
		custSvc:     c,
		catRepo:     cat,
		pricingRepo: pricingRepo,
		exportSvc:   svc,
		signer:      signer,
	}
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
	c.Header("Content-Disposition", `attachment; filename="`+docxAttachmentFilename(view.CustomerName, time.Now())+`"`)
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
	case FormatXLSX:
		rec, err = h.exportSvc.GenerateXLSX(c.Request.Context(), view, req.Language, id, user.ID)
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
	if f.Format == FormatXLSX {
		ctype = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		ext = "xlsx"
	}
	fname := "quotation-" + f.QuotationID.String()[:8] + "-" + string(f.Language) + "." + ext
	if f.Format == FormatDOCX {
		fname = h.docxDownloadFilename(c.Request.Context(), f.QuotationID, fname)
	}
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

func (h *Handler) docxDownloadFilename(ctx context.Context, quotationID uuid.UUID, fallback string) string {
	customerName, err := h.lookupCustomerName(ctx, quotationID)
	if err != nil {
		return fallback
	}
	return docxAttachmentFilename(customerName, time.Now())
}

func (h *Handler) lookupCustomerName(ctx context.Context, quotationID uuid.UUID) (string, error) {
	q, err := h.quotSvc.Get(ctx, quotationID)
	if err != nil {
		return "", err
	}
	cust, err := h.custSvc.Get(ctx, uuid.Nil, customer.RoleAdmin, q.CustomerID)
	if err != nil {
		return "", err
	}
	return cust.Name, nil
}

func docxAttachmentFilename(customerName string, now time.Time) string {
	return sanitizeAttachmentFilenameBase(customerName) + "-" + now.Format("20060102") + ".docx"
}

func sanitizeAttachmentFilenameBase(name string) string {
	replacer := strings.NewReplacer(
		"\r", " ",
		"\n", " ",
		"/", " ",
		"\\", " ",
		"\"", "",
		":", " ",
		"*", " ",
		"?", " ",
		"<", " ",
		">", " ",
		"|", " ",
	)
	cleaned := strings.Join(strings.Fields(replacer.Replace(strings.TrimSpace(name))), " ")
	if cleaned == "" {
		return "quotation"
	}
	return cleaned
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
	resp := quotation.ToResponse(q)
	countries, err := h.resolveCountries(ctx, resp.CountryIDs)
	if err != nil {
		return QuotationView{}, err
	}
	madridCountries, err := h.resolveCountries(ctx, resp.MadridCountryIDs)
	if err != nil {
		return QuotationView{}, err
	}
	singleCountries, err := h.resolveCountries(ctx, resp.SingleCountryIDs)
	if err != nil {
		return QuotationView{}, err
	}
	snap, err := q.DecodeSnapshot()
	if err != nil {
		return QuotationView{}, fmt.Errorf("export: decode snapshot: %w", err)
	}
	primaryCountry := CountryView{}
	if len(countries) > 0 {
		primaryCountry = countries[0]
	}
	v := QuotationView{
		QuotationID:       q.ID.String(),
		Status:            string(q.Status),
		ServiceTier:       q.ServiceTier,
		CustomerName:      cust.Name,
		Countries:         countries,
		CountrySummary:    countries,
		MadridCountries:   madridCountries,
		SingleCountries:   singleCountries,
		CountryNameZH:     primaryCountry.NameZH,
		CountryNameEN:     primaryCountry.NameEN,
		CountryCode:       primaryCountry.Code,
		NiceCategoryCodes: resp.NiceCategoryCodes,
		TotalCNYCents:     derefInt64(q.TotalCNYCents),
		Signature:         derefString(q.Signature),
		SubmittedAt:       q.SubmittedAt,
		ReviewedAt:        q.ReviewedAt,
		ReviewComment:     derefString(q.ReviewComment),
		Notes:             derefString(q.Notes),
		GeneratedAt:       time.Now(),
	}
	for _, l := range snap.Lines {
		v.Lines = append(v.Lines, ExportLine{
			FeeItem:             l.FeeItem,
			RegistrationMethod:  l.RegistrationMethod,
			CountryArea:         l.CountryArea,
			Quantity:            l.Quantity,
			UnitAmountCNYCents:  l.UnitAmountCNYCents,
			OfficialFeeCHFCents: l.OfficialFeeCHFCents,
			AmountCNYCents:      l.AmountCNYCents,
		})
	}
	if err := h.populateTemplateSections(ctx, &v, snap); err != nil {
		return QuotationView{}, err
	}
	return v, nil
}

// --- helpers ---

func (h *Handler) resolveCountries(ctx context.Context, ids []uuid.UUID) ([]CountryView, error) {
	out := make([]CountryView, 0, len(ids))
	for _, id := range ids {
		country, err := h.catRepo.GetCountry(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("export: lookup country %s: %w", id, err)
		}
		if country == nil {
			return nil, fmt.Errorf("export: lookup country %s: empty result", id)
		}
		out = append(out, CountryView{
			ID:     country.ID,
			Code:   country.Code,
			NameZH: country.NameZh,
			NameEN: country.NameEn,
		})
	}
	return out, nil
}

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

func isValidFormat(f Format) bool { return f == FormatPDF || f == FormatDOCX || f == FormatXLSX }
func isValidLanguage(l Language) bool {
	return l == LanguageZH || l == LanguageEN || l == LanguageBilingual
}

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

func (h *Handler) populateTemplateSections(
	ctx context.Context,
	v *QuotationView,
	snap quotation.Snapshot,
) error {
	var err error
	madridIDs, hasMadridLines := snapshotMethodCountryIDs(snap.Lines, pricing.RegistrationMethodMadrid)
	if h.catRepo != nil && len(madridIDs) > 0 {
		v.MadridCountries, err = h.resolveCountries(ctx, madridIDs)
		if err != nil {
			return err
		}
	}

	singleIDs, hasSingleLines := snapshotMethodCountryIDs(snap.Lines, pricing.RegistrationMethodSingle)
	if h.catRepo != nil && len(singleIDs) > 0 {
		v.SingleCountries, err = h.resolveCountries(ctx, singleIDs)
		if err != nil {
			return err
		}
	}

	v.MadridQuote = buildMadridQuoteSection(v.MadridCountries, snap.Lines)
	singleQuote, err := h.buildSingleQuoteSection(ctx, v.SingleCountries, len(v.NiceCategoryCodes), snap.Lines)
	if err != nil {
		return err
	}
	v.SingleQuote = singleQuote

	if hasMadridLines && v.MadridQuote != nil {
		v.MadridCountries = mergeCountryViews(v.MadridCountries, countryViewsFromMadridRows(v.MadridQuote.Rows))
	} else if len(v.MadridCountries) == 0 && v.MadridQuote != nil {
		v.MadridCountries = countryViewsFromMadridRows(v.MadridQuote.Rows)
	}
	if hasSingleLines && v.SingleQuote != nil {
		v.SingleCountries = mergeCountryViews(v.SingleCountries, countryViewsFromSingleRows(v.SingleQuote.Rows))
	} else if len(v.SingleCountries) == 0 && v.SingleQuote != nil {
		v.SingleCountries = countryViewsFromSingleRows(v.SingleQuote.Rows)
	}

	mergedCountries := mergeCountryViews(
		v.CountrySummary,
		v.Countries,
		v.MadridCountries,
		v.SingleCountries,
	)
	v.CountrySummary = mergedCountries
	v.Countries = mergedCountries
	v.SummaryQuote = buildSummaryQuoteSection(*v)
	v.SummaryNarrative = buildSummaryNarrative(*v)
	return nil
}

func snapshotMethodCountryIDs(lines []quotation.SnapshotLine, method string) ([]uuid.UUID, bool) {
	ids := make([]uuid.UUID, 0)
	seen := make(map[uuid.UUID]struct{})
	hasMethodLines := false
	for _, line := range lines {
		if line.RegistrationMethod != method {
			continue
		}
		hasMethodLines = true
		if line.CountryID == nil || *line.CountryID == uuid.Nil {
			continue
		}
		if _, ok := seen[*line.CountryID]; ok {
			continue
		}
		seen[*line.CountryID] = struct{}{}
		ids = append(ids, *line.CountryID)
	}
	return ids, hasMethodLines
}

func buildMadridQuoteSection(
	countries []CountryView,
	lines []quotation.SnapshotLine,
) *MadridQuoteSection {
	nameByID := make(map[uuid.UUID]string, len(countries))
	for _, country := range countries {
		nameByID[country.ID] = firstNonEmpty(country.NameZH, country.NameEN, country.Code)
	}

	rowByKey := make(map[string]*MadridQuoteRow)
	discoveryOrder := make([]string, 0)
	section := &MadridQuoteSection{BaseFeeNote: "（黑白商标）"}
	hasMadrid := false

	for _, line := range lines {
		if line.RegistrationMethod != pricing.RegistrationMethodMadrid {
			continue
		}
		hasMadrid = true
		if line.CountryID == nil {
			if line.OfficialFeeCHFCents != nil {
				section.BaseOfficialFeeCHFCents += *line.OfficialFeeCHFCents
				section.OfficialTotalCHFCents += *line.OfficialFeeCHFCents
				section.OfficialTotalCNYCents += line.AmountCNYCents
			} else {
				section.BaseAgencyFeeCNYCents += line.AmountCNYCents
				section.AgencyTotalCNYCents += line.AmountCNYCents
			}
			continue
		}

		key := line.CountryID.String()
		row, ok := rowByKey[key]
		if !ok {
			row = &MadridQuoteRow{
				CountryArea:  firstNonEmpty(nameByID[*line.CountryID], line.CountryArea),
				ValidityText: "10年",
			}
			rowByKey[key] = row
			discoveryOrder = append(discoveryOrder, key)
		}
		if line.OfficialFeeCHFCents != nil {
			row.OfficialFeeCHFCents += *line.OfficialFeeCHFCents
			section.OfficialTotalCHFCents += *line.OfficialFeeCHFCents
			section.OfficialTotalCNYCents += line.AmountCNYCents
			continue
		}
		row.AgencyFeeCNYCents += line.AmountCNYCents
		section.AgencyTotalCNYCents += line.AmountCNYCents
	}

	if !hasMadrid {
		return nil
	}
	section.Rows = orderMadridRows(countries, rowByKey, discoveryOrder)
	for i := range section.Rows {
		section.Rows[i].SequenceNo = i + 1
	}
	section.SubtotalCNYCents = section.OfficialTotalCNYCents + section.AgencyTotalCNYCents
	section.TotalWithTaxCNYCents = addVAT6(section.SubtotalCNYCents)
	return section
}

func orderMadridRows(
	countries []CountryView,
	rowByKey map[string]*MadridQuoteRow,
	discoveryOrder []string,
) []MadridQuoteRow {
	rows := make([]MadridQuoteRow, 0, len(rowByKey))
	seen := make(map[string]struct{}, len(rowByKey))
	for _, country := range countries {
		key := country.ID.String()
		row, ok := rowByKey[key]
		if !ok {
			continue
		}
		row.CountryArea = firstNonEmpty(country.NameZH, row.CountryArea)
		rows = append(rows, *row)
		seen[key] = struct{}{}
	}
	for _, key := range discoveryOrder {
		if _, ok := seen[key]; ok {
			continue
		}
		rows = append(rows, *rowByKey[key])
	}
	return rows
}

func (h *Handler) buildSingleQuoteSection(
	ctx context.Context,
	countries []CountryView,
	classCount int,
	lines []quotation.SnapshotLine,
) (*SingleQuoteSection, error) {
	if classCount < 1 {
		classCount = 1
	}

	groupByKey := make(map[string]*singleGroup)
	discoveryOrder := make([]string, 0)
	hasSingle := false
	for _, line := range lines {
		if line.RegistrationMethod != pricing.RegistrationMethodSingle {
			continue
		}
		hasSingle = true
		key := line.CountryArea
		if line.CountryID != nil {
			key = line.CountryID.String()
		}
		group, ok := groupByKey[key]
		if !ok {
			group = &singleGroup{CountryArea: line.CountryArea}
			if line.CountryID != nil {
				group.CountryID = *line.CountryID
			}
			groupByKey[key] = group
			discoveryOrder = append(discoveryOrder, key)
		}
		if line.SourcePricingID != nil && group.SourcePricingID == uuid.Nil {
			group.SourcePricingID = *line.SourcePricingID
		}
		group.ApplicationUntaxed += line.AmountCNYCents
	}
	if !hasSingle {
		return nil, nil
	}
	if h.pricingRepo == nil {
		return nil, fmt.Errorf("export: pricing repository is required for single-class docx export")
	}

	nameByID := make(map[uuid.UUID]string, len(countries))
	for _, country := range countries {
		nameByID[country.ID] = firstNonEmpty(country.NameZH, country.NameEN, country.Code)
	}
	orderedGroups := orderSingleGroups(countries, groupByKey, discoveryOrder)
	section := &SingleQuoteSection{Rows: make([]SingleQuoteRow, 0, len(orderedGroups))}
	for i, group := range orderedGroups {
		row := SingleQuoteRow{
			SequenceNo:           i + 1,
			CountryArea:          firstNonEmpty(nameByID[group.CountryID], group.CountryArea),
			SubmissionMethodText: "一标一类",
		}
		if group.SourcePricingID != uuid.Nil {
			entry, err := h.pricingRepo.GetSingleClassByID(ctx, group.SourcePricingID)
			if err != nil {
				return nil, fmt.Errorf("export: lookup single pricing %s: %w", group.SourcePricingID, err)
			}
			row.ApplicationFeeCNYCents = entry.FirstClassFeeTax6CNYCents
			if classCount > 1 {
				row.ApplicationFeeCNYCents += int64(classCount-1) * entry.AdditionalClassFeeTax6CNYCents
			}
			if row.ApplicationFeeCNYCents == 0 {
				row.ApplicationFeeCNYCents = group.ApplicationUntaxed
			}
			row.NotarizationFeeText = strings.TrimSpace(entry.NotarizationFee)
			if row.NotarizationFeeText == "" {
				row.NotarizationFeeText = "0"
			}
			notarizationCents, err := parseMoneyTextToCents(row.NotarizationFeeText)
			if err != nil {
				return nil, fmt.Errorf("export: parse notarization fee %q: %w", row.NotarizationFeeText, err)
			}
			row.NotarizationFeeCNYCents = notarizationCents
			row.RegistrationMonthsText = entry.RegistrationMonths
			if entry.ValidityYears != nil {
				row.ValidityYearsText = strconv.Itoa(*entry.ValidityYears)
			}
			row.CountryArea = firstNonEmpty(nameByID[entry.CountryID], row.CountryArea, entry.CountryArea)
		} else {
			row.ApplicationFeeCNYCents = group.ApplicationUntaxed
			row.NotarizationFeeText = "0"
		}
		section.TotalCNYCents += row.ApplicationFeeCNYCents + row.NotarizationFeeCNYCents
		section.Rows = append(section.Rows, row)
	}
	return section, nil
}

func orderSingleGroups(
	countries []CountryView,
	groupByKey map[string]*singleGroup,
	discoveryOrder []string,
) []*singleGroup {
	ordered := make([]*singleGroup, 0, len(groupByKey))
	seen := make(map[string]struct{}, len(groupByKey))
	for _, country := range countries {
		key := country.ID.String()
		group, ok := groupByKey[key]
		if !ok {
			continue
		}
		ordered = append(ordered, group)
		seen[key] = struct{}{}
	}
	for _, key := range discoveryOrder {
		if _, ok := seen[key]; ok {
			continue
		}
		ordered = append(ordered, groupByKey[key])
	}
	return ordered
}

func buildSummaryQuoteSection(v QuotationView) SummaryQuoteSection {
	section := SummaryQuoteSection{}
	if v.MadridQuote != nil {
		section.Rows = append(section.Rows, SummaryQuoteRow{
			MethodLabel:        "马德里",
			CategoryCodeText:   niceCategoryCodeText(v.NiceCategoryCodes),
			CountryAreaSummary: countryAreaSummary(v.MadridCountries),
			FeeCNYCents:        v.MadridQuote.TotalWithTaxCNYCents,
		})
		section.TotalCNYCents += v.MadridQuote.TotalWithTaxCNYCents
	}
	if v.SingleQuote != nil {
		section.Rows = append(section.Rows, SummaryQuoteRow{
			MethodLabel:        "单一国",
			CategoryCodeText:   niceCategoryCodeText(v.NiceCategoryCodes),
			CountryAreaSummary: countryAreaSummary(v.SingleCountries),
			FeeCNYCents:        v.SingleQuote.TotalCNYCents,
		})
		section.TotalCNYCents += v.SingleQuote.TotalCNYCents
	}
	return section
}

func parseMoneyTextToCents(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	replacer := strings.NewReplacer(",", "", "，", "", "元", "", "RMB", "", "人民币", "")
	value = strings.TrimSpace(replacer.Replace(value))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(parsed * 100)), nil
}
