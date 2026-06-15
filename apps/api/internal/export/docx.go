package export

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

//go:embed ABBC-国际商标报价模版.docx
var docxTemplateBytes []byte

// QuotationView is the render input. Caller resolves it from the
// quotation + customer + country repos; the renderer has no DB access.
type QuotationView struct {
	QuotationID       string
	QuotationIDShort  string // first 8 chars; filled by RenderHTML if blank
	Status            string
	ServiceTier       string
	CustomerName      string
	Countries         []CountryView
	CountrySummary    []CountryView
	MadridCountries   []CountryView
	SingleCountries   []CountryView
	CountryNameZH     string
	CountryNameEN     string
	CountryCode       string
	NiceCategoryCodes []int
	TotalCNYCents     int64
	Signature         string
	Lines             []ExportLine
	SubmittedAt       *time.Time
	ReviewedAt        *time.Time
	ReviewComment     string
	Notes             string
	GeneratedAt       time.Time
	SummaryNarrative  string
	MadridQuote       *MadridQuoteSection
	SingleQuote       *SingleQuoteSection
	SummaryQuote      SummaryQuoteSection
}

type CountryView struct {
	ID     uuid.UUID
	Code   string
	NameZH string
	NameEN string
}

// ExportLine is one priced fee item from the quotation snapshot.
type ExportLine struct {
	FeeItem             string
	RegistrationMethod  string
	CountryArea         string
	Quantity            int
	UnitAmountCNYCents  *int64
	OfficialFeeCHFCents *int64
	AmountCNYCents      int64
}

type MadridQuoteSection struct {
	BaseOfficialFeeCHFCents int64
	BaseAgencyFeeCNYCents   int64
	BaseFeeNote             string
	Rows                    []MadridQuoteRow
	OfficialTotalCHFCents   int64
	OfficialTotalCNYCents   int64
	AgencyTotalCNYCents     int64
	SubtotalCNYCents        int64
	TotalWithTaxCNYCents    int64
}

type MadridQuoteRow struct {
	SequenceNo          int
	CountryArea         string
	OfficialFeeCHFCents int64
	AgencyFeeCNYCents   int64
	ValidityText        string
}

type SingleQuoteSection struct {
	Rows          []SingleQuoteRow
	TotalCNYCents int64
}

type SingleQuoteRow struct {
	SequenceNo              int
	CountryArea             string
	ApplicationFeeCNYCents  int64
	NotarizationFeeText     string
	NotarizationFeeCNYCents int64
	SubmissionMethodText    string
	RegistrationMonthsText  string
	ValidityYearsText       string
}

type SummaryQuoteSection struct {
	Rows          []SummaryQuoteRow
	TotalCNYCents int64
}

type SummaryQuoteRow struct {
	MethodLabel        string
	CategoryCodeText   string
	CountryAreaSummary string
	FeeCNYCents        int64
}

var textRunRE = regexp.MustCompile(`(?s)<w:t\b[^>]*>(.*?)</w:t>`)
var chineseDateParagraphRE = regexp.MustCompile(`^\d{4}年\d{1,2}月\d{1,2}日$`)

func (v QuotationView) allCountries() []CountryView {
	if len(v.CountrySummary) > 0 {
		return v.CountrySummary
	}
	if len(v.Countries) > 0 {
		return v.Countries
	}
	if v.CountryCode == "" && v.CountryNameZH == "" && v.CountryNameEN == "" {
		return nil
	}
	return []CountryView{{
		Code:   v.CountryCode,
		NameZH: v.CountryNameZH,
		NameEN: v.CountryNameEN,
	}}
}

// RenderDOCX writes a .docx file based on the bundled quotation template.
// The template structure and styling are preserved; only dynamic text and
// table rows are replaced.
func RenderDOCX(w io.Writer, v QuotationView) error {
	buf, err := renderTemplateDOCX(normalizeDOCXView(v))
	if err != nil {
		return err
	}
	_, err = w.Write(buf)
	return err
}

func writeFile(zw *zip.Writer, name, body string) error {
	f, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, bytes.NewReader([]byte(body)))
	return err
}

func renderTemplateDOCX(v QuotationView) ([]byte, error) {
	docXML, err := renderTemplateDocument(v)
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(docxTemplateBytes), int64(len(docxTemplateBytes)))
	if err != nil {
		return nil, fmt.Errorf("export: open docx template: %w", err)
	}

	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		var payload []byte
		if f.Name == "word/document.xml" {
			payload = []byte(docXML)
		} else {
			rc, err := f.Open()
			if err != nil {
				_ = zw.Close()
				return nil, fmt.Errorf("export: open template member %s: %w", f.Name, err)
			}
			payload, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				_ = zw.Close()
				return nil, fmt.Errorf("export: read template member %s: %w", f.Name, err)
			}
		}
		h := f.FileHeader
		wc, err := zw.CreateHeader(&h)
		if err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("export: create zip member %s: %w", f.Name, err)
		}
		if _, err := wc.Write(payload); err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("export: write zip member %s: %w", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func renderTemplateDocument(v QuotationView) (string, error) {
	raw, err := templateDocumentXML()
	if err != nil {
		return "", err
	}
	openDocument, documentChildren, closeDocument, err := splitElementChildren(raw, "w:document")
	if err != nil {
		return "", err
	}
	bodyIndex := -1
	for i, child := range documentChildren {
		if fragmentName(child) == "w:body" {
			bodyIndex = i
			break
		}
	}
	if bodyIndex < 0 {
		return "", fmt.Errorf("export: template body missing")
	}
	openBody, bodyChildren, closeBody, err := splitElementChildren(documentChildren[bodyIndex], "w:body")
	if err != nil {
		return "", err
	}
	layout, err := detectDOCXLayout(bodyChildren)
	if err != nil {
		return "", err
	}

	var errReplace error
	bodyChildren[layout.customerParagraphIndex], errReplace = replaceTextRunsFlexible(
		bodyChildren[layout.customerParagraphIndex],
		[]string{"致", ":", " ", v.CustomerName},
	)
	if errReplace != nil {
		return "", errReplace
	}
	bodyChildren[layout.categoryParagraphIndex], errReplace = replaceTextRunsFlexible(
		bodyChildren[layout.categoryParagraphIndex],
		[]string{"拟注册类别：第", niceCategoryMiddleText(v.NiceCategoryCodes), "类"},
	)
	if errReplace != nil {
		return "", errReplace
	}
	bodyChildren[layout.countryParagraphIndex], errReplace = replaceTextRunsFlexible(
		bodyChildren[layout.countryParagraphIndex],
		[]string{
			"拟注册国家",
			"/",
			"地区：" + fmtCountryListZH(v.allCountries()) + "（共",
			strconv.Itoa(len(v.allCountries())),
			"个国家",
			"/",
			"地区）",
		},
	)
	if errReplace != nil {
		return "", errReplace
	}
	bodyChildren[layout.generatedDateIndex], errReplace = replaceTextRunsFlexible(
		bodyChildren[layout.generatedDateIndex],
		[]string{formatChineseDate(v.GeneratedAt)},
	)
	if errReplace != nil {
		return "", errReplace
	}

	nextNumber := 0
	madridTitle := ""
	singleTitle := ""
	summaryTitle := ""
	if v.MadridQuote != nil {
		madridTitle = sectionNumber(nextNumber)
		nextNumber++
	}
	if v.SingleQuote != nil {
		singleTitle = sectionNumber(nextNumber)
		nextNumber++
	}
	summaryTitle = sectionNumber(nextNumber)

	if v.MadridQuote != nil {
		bodyChildren[layout.madridTitleIndex], errReplace = replaceTextRunsFlexible(
			bodyChildren[layout.madridTitleIndex],
			[]string{madridTitle + "通过马德里方式申请"},
		)
		if errReplace != nil {
			return "", errReplace
		}
		bodyChildren[layout.madridCountryIndex], errReplace = replaceTextRunsFlexible(
			bodyChildren[layout.madridCountryIndex],
			[]string{
				"1.",
				"国家",
				"（共",
				strconv.Itoa(len(v.MadridCountries)),
				"个国家",
				"/",
				"地区）",
				"：",
				fmtCountryListZH(v.MadridCountries),
			},
		)
		if errReplace != nil {
			return "", errReplace
		}
		bodyChildren[layout.madridCategoryIndex], errReplace = replaceTextRunsFlexible(
			bodyChildren[layout.madridCategoryIndex],
			[]string{"2.", "类别：", "第", niceCategoryMiddleText(v.NiceCategoryCodes), "类"},
		)
		if errReplace != nil {
			return "", errReplace
		}
		bodyChildren[layout.madridTableIndex], errReplace = renderMadridTable(bodyChildren[layout.madridTableIndex], v.MadridQuote)
		if errReplace != nil {
			return "", errReplace
		}
	}

	if v.SingleQuote != nil {
		bodyChildren[layout.singleTitleIndex], errReplace = replaceTextRunsFlexible(
			bodyChildren[layout.singleTitleIndex],
			[]string{singleTitle + "通过单一注册方式申请"},
		)
		if errReplace != nil {
			return "", errReplace
		}
		bodyChildren[layout.singleCountryIndex], errReplace = replaceTextRunsFlexible(
			bodyChildren[layout.singleCountryIndex],
			[]string{
				"1.",
				"国家",
				"（共",
				strconv.Itoa(len(v.SingleCountries)),
				"个国家",
				"/",
				"地区）",
				"：",
				fmtCountryListZH(v.SingleCountries),
			},
		)
		if errReplace != nil {
			return "", errReplace
		}
		bodyChildren[layout.singleCategoryIndex], errReplace = replaceTextRunsFlexible(
			bodyChildren[layout.singleCategoryIndex],
			[]string{"2.", "类别：", "第", niceCategoryMiddleText(v.NiceCategoryCodes), "类"},
		)
		if errReplace != nil {
			return "", errReplace
		}
		bodyChildren[layout.singleTableIndex], errReplace = renderSingleTable(bodyChildren[layout.singleTableIndex], v.SingleQuote)
		if errReplace != nil {
			return "", errReplace
		}
	}

	bodyChildren[layout.summaryTitleIndex], errReplace = replaceTextRunsFlexible(
		bodyChildren[layout.summaryTitleIndex],
		[]string{summaryTitle + "总的报价方案"},
	)
	if errReplace != nil {
		return "", errReplace
	}
	bodyChildren[layout.summaryParagraphIndex], errReplace = replaceFirstTextAndClearRest(
		bodyChildren[layout.summaryParagraphIndex],
		v.SummaryNarrative,
	)
	if errReplace != nil {
		return "", errReplace
	}
	bodyChildren[layout.summaryTableIndex], errReplace = renderSummaryTable(bodyChildren[layout.summaryTableIndex], v.SummaryQuote)
	if errReplace != nil {
		return "", errReplace
	}

	if v.MadridQuote == nil {
		nextSectionStart := nextIndexAfter(
			layout.madridTitleIndex,
			layout.singleTitleIndex,
			layout.summaryTitleIndex,
		)
		bodyChildren = removeChildRange(
			bodyChildren,
			layout.madridTitleIndex,
			nextSectionStart-1,
		)
	}
	if v.SingleQuote == nil {
		singleTitleIndex := layout.singleTitleIndex
		summaryTitleIndex := layout.summaryTitleIndex
		if v.MadridQuote == nil && layout.madridTitleIndex < layout.singleTitleIndex {
			removedCount := nextIndexAfter(
				layout.madridTitleIndex,
				layout.singleTitleIndex,
				layout.summaryTitleIndex,
			) - layout.madridTitleIndex
			singleTitleIndex -= removedCount
			summaryTitleIndex -= removedCount
		}
		bodyChildren = removeChildRange(
			bodyChildren,
			singleTitleIndex,
			summaryTitleIndex-1,
		)
	}

	documentChildren[bodyIndex] = openBody + strings.Join(bodyChildren, "") + closeBody
	return openDocument + strings.Join(documentChildren, "") + closeDocument, nil
}

type docxLayout struct {
	customerParagraphIndex int
	categoryParagraphIndex int
	countryParagraphIndex  int
	generatedDateIndex     int

	madridTitleIndex    int
	madridCountryIndex  int
	madridCategoryIndex int
	madridTableIndex    int

	singleTitleIndex    int
	singleCountryIndex  int
	singleCategoryIndex int
	singleTableIndex    int

	summaryTitleIndex     int
	summaryParagraphIndex int
	summaryTableIndex     int
}

func detectDOCXLayout(bodyChildren []string) (docxLayout, error) {
	layout := docxLayout{
		customerParagraphIndex: findParagraphIndexContaining(bodyChildren, 0, -1, "致"),
		categoryParagraphIndex: findParagraphIndexContaining(bodyChildren, 0, -1, "拟注册类别"),
		countryParagraphIndex:  findParagraphIndexContaining(bodyChildren, 0, -1, "拟注册国家"),
		generatedDateIndex:     findParagraphIndexMatching(bodyChildren, 0, -1, chineseDateParagraphRE.MatchString),
		madridTitleIndex:       findParagraphIndexContaining(bodyChildren, 0, -1, "通过马德里方式申请"),
		singleTitleIndex:       findParagraphIndexContaining(bodyChildren, 0, -1, "通过单一注册方式申请"),
		summaryTitleIndex:      findParagraphIndexContaining(bodyChildren, 0, -1, "总的报价方案"),
	}
	if layout.customerParagraphIndex < 0 {
		return layout, fmt.Errorf("export: template anchor customer paragraph missing")
	}
	if layout.categoryParagraphIndex < 0 {
		return layout, fmt.Errorf("export: template anchor category paragraph missing")
	}
	if layout.countryParagraphIndex < 0 {
		return layout, fmt.Errorf("export: template anchor country paragraph missing")
	}
	if layout.generatedDateIndex < 0 {
		return layout, fmt.Errorf("export: template anchor generated date paragraph missing")
	}
	if layout.madridTitleIndex < 0 {
		return layout, fmt.Errorf("export: template anchor madrid title missing")
	}
	if layout.singleTitleIndex < 0 {
		return layout, fmt.Errorf("export: template anchor single title missing")
	}
	if layout.summaryTitleIndex < 0 {
		return layout, fmt.Errorf("export: template anchor summary title missing")
	}

	madridStop := nextIndexAfter(layout.madridTitleIndex, layout.singleTitleIndex, layout.summaryTitleIndex)
	layout.madridCountryIndex = findParagraphIndexContaining(bodyChildren, layout.madridTitleIndex+1, madridStop, "国家")
	layout.madridCategoryIndex = findParagraphIndexContaining(bodyChildren, layout.madridCountryIndex+1, madridStop, "类别")
	layout.madridTableIndex = findFirstChildIndex(bodyChildren, layout.madridCategoryIndex+1, madridStop, "w:tbl", nil)

	singleStop := nextIndexAfter(layout.singleTitleIndex, layout.summaryTitleIndex)
	layout.singleCountryIndex = findParagraphIndexContaining(bodyChildren, layout.singleTitleIndex+1, singleStop, "国家")
	layout.singleCategoryIndex = findParagraphIndexContaining(bodyChildren, layout.singleCountryIndex+1, singleStop, "类别")
	layout.singleTableIndex = findFirstChildIndex(bodyChildren, layout.singleCategoryIndex+1, singleStop, "w:tbl", nil)

	layout.summaryParagraphIndex = findFirstNonEmptyParagraphIndex(bodyChildren, layout.summaryTitleIndex+1, -1)
	layout.summaryTableIndex = findFirstChildIndex(bodyChildren, layout.summaryParagraphIndex+1, -1, "w:tbl", nil)

	if layout.madridCountryIndex < 0 || layout.madridCategoryIndex < 0 || layout.madridTableIndex < 0 {
		return layout, fmt.Errorf("export: template madrid block incomplete")
	}
	if layout.singleCountryIndex < 0 || layout.singleCategoryIndex < 0 || layout.singleTableIndex < 0 {
		return layout, fmt.Errorf("export: template single block incomplete")
	}
	if layout.summaryParagraphIndex < 0 || layout.summaryTableIndex < 0 {
		return layout, fmt.Errorf("export: template summary block incomplete")
	}
	return layout, nil
}

func templateDocumentXML() (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(docxTemplateBytes), int64(len(docxTemplateBytes)))
	if err != nil {
		return "", fmt.Errorf("export: open template zip: %w", err)
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("export: open template document: %w", err)
		}
		defer rc.Close()
		raw, err := io.ReadAll(rc)
		if err != nil {
			return "", fmt.Errorf("export: read template document: %w", err)
		}
		return string(raw), nil
	}
	return "", fmt.Errorf("export: template document.xml missing")
}

func normalizeDOCXView(v QuotationView) QuotationView {
	v = normalizeCountryLists(v)
	if v.MadridQuote != nil {
		if v.MadridQuote.BaseFeeNote == "" {
			v.MadridQuote.BaseFeeNote = "（黑白商标）"
		}
		if v.MadridQuote.SubtotalCNYCents == 0 {
			v.MadridQuote.SubtotalCNYCents = v.MadridQuote.OfficialTotalCNYCents + v.MadridQuote.AgencyTotalCNYCents
		}
		if v.MadridQuote.TotalWithTaxCNYCents == 0 {
			v.MadridQuote.TotalWithTaxCNYCents = addVAT6(v.MadridQuote.SubtotalCNYCents)
		}
		for i := range v.MadridQuote.Rows {
			if v.MadridQuote.Rows[i].ValidityText == "" {
				v.MadridQuote.Rows[i].ValidityText = "10年"
			}
		}
	}
	if v.SingleQuote != nil {
		for i := range v.SingleQuote.Rows {
			if v.SingleQuote.Rows[i].SubmissionMethodText == "" {
				v.SingleQuote.Rows[i].SubmissionMethodText = "一标一类"
			}
			if v.SingleQuote.Rows[i].NotarizationFeeText == "" {
				v.SingleQuote.Rows[i].NotarizationFeeText = "0"
			}
		}
		if v.SingleQuote.TotalCNYCents == 0 {
			for _, row := range v.SingleQuote.Rows {
				v.SingleQuote.TotalCNYCents += row.ApplicationFeeCNYCents + row.NotarizationFeeCNYCents
			}
		}
	}
	if len(v.SummaryQuote.Rows) == 0 {
		if v.MadridQuote != nil {
			v.SummaryQuote.Rows = append(v.SummaryQuote.Rows, SummaryQuoteRow{
				MethodLabel:        "马德里",
				CategoryCodeText:   niceCategoryCodeText(v.NiceCategoryCodes),
				CountryAreaSummary: countryAreaSummary(v.MadridCountries),
				FeeCNYCents:        v.MadridQuote.TotalWithTaxCNYCents,
			})
		}
		if v.SingleQuote != nil {
			v.SummaryQuote.Rows = append(v.SummaryQuote.Rows, SummaryQuoteRow{
				MethodLabel:        "单一国",
				CategoryCodeText:   niceCategoryCodeText(v.NiceCategoryCodes),
				CountryAreaSummary: countryAreaSummary(v.SingleCountries),
				FeeCNYCents:        v.SingleQuote.TotalCNYCents,
			})
		}
	}
	if v.SummaryQuote.TotalCNYCents == 0 {
		for _, row := range v.SummaryQuote.Rows {
			v.SummaryQuote.TotalCNYCents += row.FeeCNYCents
		}
	}
	if v.SummaryNarrative == "" {
		v.SummaryNarrative = buildSummaryNarrative(v)
	}
	return v
}

func renderMadridTable(fragment string, section *MadridQuoteSection) (string, error) {
	prefix, rows, suffix, err := splitTable(fragment)
	if err != nil {
		return "", err
	}
	if len(rows) < 6 {
		return "", fmt.Errorf("export: unexpected madrid table rows %d", len(rows))
	}
	header := rows[0]
	baseTemplate := rows[1]
	countryTemplate := rows[2]
	officialSubtotalTemplate := rows[len(rows)-3]
	agencySubtotalTemplate := rows[len(rows)-2]
	totalTemplate := rows[len(rows)-1]

	baseRow, err := replaceTextRuns(baseTemplate, []string{
		"/",
		"基础费用",
		formatWholeAmount(section.BaseOfficialFeeCHFCents),
		section.BaseFeeNote,
		formatWholeAmount(section.BaseAgencyFeeCNYCents),
	})
	if err != nil {
		return "", err
	}
	newRows := []string{header, baseRow}
	for _, row := range section.Rows {
		years, suffixText := splitYearText(row.ValidityText)
		replaced, err := replaceTextRuns(countryTemplate, []string{
			strconv.Itoa(row.SequenceNo),
			row.CountryArea,
			formatWholeAmount(row.OfficialFeeCHFCents),
			formatWholeAmount(row.AgencyFeeCNYCents),
			years,
			suffixText,
		})
		if err != nil {
			return "", err
		}
		newRows = append(newRows, replaced)
	}

	rateText := formatCHFCNYRate(section.OfficialTotalCHFCents, section.OfficialTotalCNYCents)
	officialSubtotal, err := replaceTextRuns(officialSubtotalTemplate, []string{
		"官费小计",
		formatWholeAmount(section.OfficialTotalCHFCents),
		"瑞郎，合",
		formatWholeAmount(section.OfficialTotalCNYCents),
		"元人民币",
		"【",
		"1",
		"瑞士法郎",
		"=" + rateText,
		"元人民币】",
	})
	if err != nil {
		return "", err
	}
	agencySubtotal, err := replaceTextRuns(agencySubtotalTemplate, []string{
		"代理费小计",
		formatWholeAmount(section.AgencyTotalCNYCents),
		"元人民币",
	})
	if err != nil {
		return "", err
	}
	totalRow, err := replaceTextRuns(totalTemplate, []string{
		"合计（",
		"RMB",
		"）：",
		formatWholeAmount(section.SubtotalCNYCents),
		"元人民币（不含税）",
		"/" + formatWholeAmount(section.TotalWithTaxCNYCents),
		"元人民币（含税，",
		"6%",
		"）",
	})
	if err != nil {
		return "", err
	}
	newRows = append(newRows, officialSubtotal, agencySubtotal, totalRow)
	return prefix + strings.Join(newRows, "") + suffix, nil
}

func renderSingleTable(fragment string, section *SingleQuoteSection) (string, error) {
	prefix, rows, suffix, err := splitTable(fragment)
	if err != nil {
		return "", err
	}
	if len(rows) < 3 {
		return "", fmt.Errorf("export: unexpected single table rows %d", len(rows))
	}
	header := rows[0]
	rowTemplate := rows[1]
	totalTemplate := rows[len(rows)-1]

	newRows := []string{header}
	for _, row := range section.Rows {
		replaced, err := replaceTextRuns(rowTemplate, []string{
			strconv.Itoa(row.SequenceNo),
			row.CountryArea,
			formatWholeAmount(row.ApplicationFeeCNYCents),
			row.NotarizationFeeText,
			row.SubmissionMethodText,
			row.RegistrationMonthsText,
			row.ValidityYearsText,
		})
		if err != nil {
			return "", err
		}
		newRows = append(newRows, replaced)
	}
	totalRow, err := replaceTextRuns(totalTemplate, []string{
		"合计（",
		"RMB",
		"）：",
		formatWholeAmount(section.TotalCNYCents),
		"【含税，",
		"6%",
		"】",
	})
	if err != nil {
		return "", err
	}
	newRows = append(newRows, totalRow)
	return prefix + strings.Join(newRows, "") + suffix, nil
}

func renderSummaryTable(fragment string, section SummaryQuoteSection) (string, error) {
	prefix, rows, suffix, err := splitTable(fragment)
	if err != nil {
		return "", err
	}
	if len(rows) < 4 {
		return "", fmt.Errorf("export: unexpected summary table rows %d", len(rows))
	}
	header := rows[0]
	madridTemplate := rows[1]
	singleTemplate := rows[2]
	totalTemplate := rows[len(rows)-1]

	newRows := []string{header}
	for _, row := range section.Rows {
		templateRow := madridTemplate
		if row.MethodLabel == "单一国" {
			templateRow = singleTemplate
		}
		replaced, err := replaceTextRuns(templateRow, []string{
			row.MethodLabel,
			row.CategoryCodeText,
			row.CountryAreaSummary,
			"",
			"",
			"",
			"",
			formatWholeAmount(row.FeeCNYCents),
		})
		if err != nil {
			return "", err
		}
		newRows = append(newRows, replaced)
	}
	totalRow, err := replaceTextRuns(totalTemplate, []string{
		"合计（",
		"RMB",
		"）：",
		formatWholeAmount(section.TotalCNYCents),
	})
	if err != nil {
		return "", err
	}
	newRows = append(newRows, totalRow)
	return prefix + strings.Join(newRows, "") + suffix, nil
}

func splitTable(fragment string) (string, []string, string, error) {
	open, children, close, err := splitElementChildren(fragment, "w:tbl")
	if err != nil {
		return "", nil, "", err
	}
	var leading []string
	var rows []string
	var trailing []string
	seenRows := false
	for _, child := range children {
		if fragmentName(child) == "w:tr" {
			seenRows = true
			rows = append(rows, child)
			continue
		}
		if !seenRows {
			leading = append(leading, child)
			continue
		}
		trailing = append(trailing, child)
	}
	return open + strings.Join(leading, ""), rows, strings.Join(trailing, "") + close, nil
}

func splitElementChildren(fragment string, wantRoot string) (string, []string, string, error) {
	rootName, open, inner, close, err := splitRootElement(fragment)
	if err != nil {
		return "", nil, "", err
	}
	if wantRoot != "" && rootName != wantRoot {
		return "", nil, "", fmt.Errorf("export: expected root %s, got %s", wantRoot, rootName)
	}
	children, err := splitDirectChildren(inner)
	if err != nil {
		return "", nil, "", err
	}
	return open, children, close, nil
}

func splitRootElement(fragment string) (string, string, string, string, error) {
	start := strings.IndexByte(fragment, '<')
	if start < 0 {
		return "", "", "", "", fmt.Errorf("export: missing root start tag")
	}
	if strings.HasPrefix(fragment[start:], "<?xml") {
		prologEnd := strings.Index(fragment[start:], "?>")
		if prologEnd < 0 {
			return "", "", "", "", fmt.Errorf("export: unterminated xml declaration")
		}
		start += prologEnd + 2
		for start < len(fragment) && isXMLSpace(fragment[start]) {
			start++
		}
		if start >= len(fragment) || fragment[start] != '<' {
			return "", "", "", "", fmt.Errorf("export: missing root element after xml declaration")
		}
	}
	end, err := findTagEnd(fragment, start)
	if err != nil {
		return "", "", "", "", err
	}
	rootName := tagName(fragment[start+1 : end])
	closeTag := "</" + rootName + ">"
	closeStart := strings.LastIndex(fragment, closeTag)
	if closeStart < 0 {
		return "", "", "", "", fmt.Errorf("export: missing closing tag %s", closeTag)
	}
	return rootName, fragment[:end+1], fragment[end+1 : closeStart], fragment[closeStart:], nil
}

func splitDirectChildren(raw string) ([]string, error) {
	children := make([]string, 0)
	for i := 0; i < len(raw); {
		for i < len(raw) && isXMLSpace(raw[i]) {
			i++
		}
		if i >= len(raw) {
			break
		}
		if raw[i] != '<' {
			return nil, fmt.Errorf("export: unexpected non-tag content near %q", raw[i:])
		}
		elem, next, err := consumeElement(raw, i)
		if err != nil {
			return nil, err
		}
		children = append(children, elem)
		i = next
	}
	return children, nil
}

func consumeElement(raw string, start int) (string, int, error) {
	end, err := findTagEnd(raw, start)
	if err != nil {
		return "", 0, err
	}
	tag := strings.TrimSpace(raw[start+1 : end])
	if strings.HasSuffix(tag, "/") {
		return raw[start : end+1], end + 1, nil
	}
	depth := 1
	i := end + 1
	for depth > 0 {
		next := strings.IndexByte(raw[i:], '<')
		if next < 0 {
			return "", 0, fmt.Errorf("export: unterminated element starting %q", raw[start:])
		}
		i += next
		tagEnd, err := findTagEnd(raw, i)
		if err != nil {
			return "", 0, err
		}
		tagBody := strings.TrimSpace(raw[i+1 : tagEnd])
		switch {
		case strings.HasPrefix(tagBody, "/"):
			depth--
		case strings.HasSuffix(tagBody, "/"):
			// self-closing nested tag
		default:
			depth++
		}
		i = tagEnd + 1
	}
	return raw[start:i], i, nil
}

func findTagEnd(raw string, start int) (int, error) {
	var quote byte
	for i := start + 1; i < len(raw); i++ {
		switch raw[i] {
		case '\'', '"':
			if quote == 0 {
				quote = raw[i]
			} else if quote == raw[i] {
				quote = 0
			}
		case '>':
			if quote == 0 {
				return i, nil
			}
		}
	}
	return 0, fmt.Errorf("export: unterminated tag near %q", raw[start:])
}

func fragmentName(fragment string) string {
	start := strings.IndexByte(fragment, '<')
	if start < 0 {
		return ""
	}
	end, err := findTagEnd(fragment, start)
	if err != nil {
		return ""
	}
	return tagName(fragment[start+1 : end])
}

func tagName(tag string) string {
	tag = strings.TrimSpace(tag)
	if strings.HasPrefix(tag, "/") {
		tag = tag[1:]
	}
	for i := 0; i < len(tag); i++ {
		switch tag[i] {
		case ' ', '\t', '\n', '\r', '/':
			return tag[:i]
		}
	}
	return tag
}

func replaceTextRunsFlexible(fragment string, replacements []string) (string, error) {
	indexes := textRunRE.FindAllStringSubmatchIndex(fragment, -1)
	switch {
	case len(indexes) == 0:
		return "", fmt.Errorf("export: no text runs to replace")
	case len(indexes) < len(replacements):
		return replaceFirstTextAndClearRest(fragment, strings.Join(replacements, ""))
	default:
		return replaceTextRuns(fragment, replacements)
	}
}

func replaceTextRuns(fragment string, replacements []string) (string, error) {
	indexes := textRunRE.FindAllStringSubmatchIndex(fragment, -1)
	if len(indexes) < len(replacements) {
		return "", fmt.Errorf("export: text run replacement overflow: have %d need %d", len(indexes), len(replacements))
	}
	var b strings.Builder
	last := 0
	for i, loc := range indexes {
		contentStart, contentEnd := loc[2], loc[3]
		b.WriteString(fragment[last:contentStart])
		value := fragment[contentStart:contentEnd]
		if i < len(replacements) {
			value = xmlEscape(replacements[i])
		}
		b.WriteString(value)
		last = contentEnd
	}
	b.WriteString(fragment[last:])
	return b.String(), nil
}

func replaceFirstTextAndClearRest(fragment, text string) (string, error) {
	count := len(textRunRE.FindAllStringSubmatchIndex(fragment, -1))
	if count == 0 {
		return "", fmt.Errorf("export: no text runs to replace")
	}
	replacements := make([]string, count)
	replacements[0] = text
	for i := 1; i < count; i++ {
		replacements[i] = ""
	}
	return replaceTextRuns(fragment, replacements)
}

func removeChildRange(children []string, start, end int) []string {
	if start < 0 || end >= len(children) || start > end {
		return children
	}
	out := make([]string, 0, len(children)-(end-start+1))
	out = append(out, children[:start]...)
	out = append(out, children[end+1:]...)
	return out
}

func fragmentText(fragment string) string {
	matches := textRunRE.FindAllStringSubmatch(fragment, -1)
	if len(matches) == 0 {
		return ""
	}
	var b strings.Builder
	for _, match := range matches {
		b.WriteString(html.UnescapeString(match[1]))
	}
	return strings.TrimSpace(b.String())
}

func findParagraphIndexContaining(children []string, start, stop int, substrings ...string) int {
	return findFirstChildIndex(children, start, stop, "w:p", func(fragment string) bool {
		text := fragmentText(fragment)
		if text == "" {
			return false
		}
		for _, substring := range substrings {
			if !strings.Contains(text, substring) {
				return false
			}
		}
		return true
	})
}

func findParagraphIndexMatching(children []string, start, stop int, matcher func(string) bool) int {
	return findFirstChildIndex(children, start, stop, "w:p", func(fragment string) bool {
		text := fragmentText(fragment)
		if text == "" {
			return false
		}
		return matcher(text)
	})
}

func findFirstNonEmptyParagraphIndex(children []string, start, stop int) int {
	return findFirstChildIndex(children, start, stop, "w:p", func(fragment string) bool {
		return fragmentText(fragment) != ""
	})
}

func findFirstChildIndex(
	children []string,
	start int,
	stop int,
	wantName string,
	matcher func(fragment string) bool,
) int {
	if start < 0 {
		start = 0
	}
	if stop < 0 || stop > len(children) {
		stop = len(children)
	}
	for i := start; i < stop; i++ {
		if wantName != "" && fragmentName(children[i]) != wantName {
			continue
		}
		if matcher != nil && !matcher(children[i]) {
			continue
		}
		return i
	}
	return -1
}

func nextIndexAfter(after int, candidates ...int) int {
	next := -1
	for _, candidate := range candidates {
		if candidate <= after {
			continue
		}
		if next < 0 || candidate < next {
			next = candidate
		}
	}
	return next
}

func sectionNumber(index int) string {
	labels := []string{"（一）", "（二）", "（三）"}
	if index < 0 || index >= len(labels) {
		return fmt.Sprintf("（%d）", index+1)
	}
	return labels[index]
}

func formatChineseDate(t time.Time) string {
	return fmt.Sprintf("%d年%d月%d日", t.Year(), int(t.Month()), t.Day())
}

func buildSummaryNarrative(v QuotationView) string {
	clauses := make([]string, 0, 2)
	categoryText := niceCategoryLabel(v.NiceCategoryCodes)
	if v.MadridQuote != nil {
		clauses = append(clauses,
			fmt.Sprintf("在%s通过马德里途径提起针对%s的新申请", countryAreaSummary(v.MadridCountries), categoryText),
		)
	}
	if v.SingleQuote != nil {
		clauses = append(clauses,
			fmt.Sprintf("在%s通过单一注册方式提起针对%s的新申请", countryAreaSummary(v.SingleCountries), categoryText),
		)
	}
	if len(clauses) == 0 {
		return ""
	}
	return fmt.Sprintf(
		"针对贵司的需求，我司建议%s，总的报价为%s元，请贵司查阅。",
		strings.Join(clauses, "，"),
		formatWholeAmount(v.SummaryQuote.TotalCNYCents),
	)
}

func countryAreaSummary(countries []CountryView) string {
	return fmtCountryListZH(countries) + "（共" + strconv.Itoa(len(countries)) + "个国家/地区）"
}

func niceCategoryLabel(codes []int) string {
	if len(codes) == 0 {
		return "第1类"
	}
	parts := make([]string, 0, len(codes))
	for _, code := range codes {
		parts = append(parts, fmt.Sprintf("第%d类", code))
	}
	return strings.Join(parts, "、")
}

func niceCategoryMiddleText(codes []int) string {
	label := niceCategoryLabel(codes)
	label = strings.TrimPrefix(label, "第")
	label = strings.TrimSuffix(label, "类")
	return label
}

func niceCategoryCodeText(codes []int) string {
	if len(codes) == 0 {
		return "1"
	}
	parts := make([]string, 0, len(codes))
	for _, code := range codes {
		parts = append(parts, strconv.Itoa(code))
	}
	return strings.Join(parts, "、")
}

func splitYearText(value string) (string, string) {
	if value == "" {
		return "10", "年"
	}
	if strings.HasSuffix(value, "年") {
		return strings.TrimSuffix(value, "年"), "年"
	}
	return value, ""
}

func addVAT6(cents int64) int64 {
	return cents * 106 / 100
}

func formatCHFCNYRate(chfCents, cnyCents int64) string {
	if chfCents <= 0 || cnyCents <= 0 {
		return "0"
	}
	rate := float64(cnyCents) / float64(chfCents)
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(rate, 'f', 2, 64), "0"), ".")
}

func formatWholeAmount(cents int64) string {
	if cents <= 0 {
		return "0"
	}
	return strconv.FormatInt(cents/100, 10)
}

func countryViewsFromMadridRows(rows []MadridQuoteRow) []CountryView {
	derived := make([]CountryView, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.CountryArea)
		if name == "" {
			continue
		}
		derived = append(derived, CountryView{NameZH: name})
	}
	return dedupeCountryViews(derived)
}

func countryViewsFromSingleRows(rows []SingleQuoteRow) []CountryView {
	derived := make([]CountryView, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.CountryArea)
		if name == "" {
			continue
		}
		derived = append(derived, CountryView{NameZH: name})
	}
	return dedupeCountryViews(derived)
}

func normalizeCountryLists(v QuotationView) QuotationView {
	if len(v.MadridCountries) == 0 && v.MadridQuote != nil {
		v.MadridCountries = countryViewsFromMadridRows(v.MadridQuote.Rows)
	}
	if len(v.SingleCountries) == 0 && v.SingleQuote != nil {
		v.SingleCountries = countryViewsFromSingleRows(v.SingleQuote.Rows)
	}

	merged := mergeCountryViews(
		v.CountrySummary,
		v.Countries,
		v.MadridCountries,
		v.SingleCountries,
	)
	if len(merged) == 0 {
		merged = v.allCountries()
	}
	v.CountrySummary = merged
	v.Countries = merged
	return v
}

func mergeCountryViews(groups ...[]CountryView) []CountryView {
	merged := make([]CountryView, 0)
	for _, group := range groups {
		merged = append(merged, group...)
	}
	return dedupeCountryViews(merged)
}

func dedupeCountryViews(countries []CountryView) []CountryView {
	if len(countries) == 0 {
		return nil
	}
	out := make([]CountryView, 0, len(countries))
	seen := make(map[string]struct{}, len(countries))
	for _, country := range countries {
		key := countryViewKey(country)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, country)
	}
	return out
}

func countryViewKey(country CountryView) string {
	key := firstNonEmpty(
		strings.TrimSpace(country.NameZH),
		strings.TrimSpace(country.NameEN),
		strings.TrimSpace(country.Code),
	)
	if key != "" {
		return key
	}
	if country.ID != uuid.Nil {
		return country.ID.String()
	}
	return ""
}

func isXMLSpace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t'
}

func xmlEscape(s string) string {
	s = html.EscapeString(s)
	var buf bytes.Buffer
	for _, r := range s {
		if r == '\'' {
			buf.WriteString("&#39;")
		} else {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

func fmtCNY(cents int64) string {
	yuan := float64(cents) / 100
	return fmt.Sprintf("¥ %s", humanMoney(yuan))
}

func fmtCNYPtr(cents *int64) string {
	if cents == nil {
		return ""
	}
	return fmtCNY(*cents)
}

func fmtCHFPtr(cents *int64) string {
	if cents == nil {
		return ""
	}
	return fmt.Sprintf("CHF %s", humanMoney(float64(*cents)/100))
}

func fmtQuantity(quantity int) string {
	if quantity <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", quantity)
}

func fmtCountryListBilingual(countries []CountryView) string {
	return joinCountryViews(countries, formatCountryBilingual, "、")
}

func fmtCountryListZH(countries []CountryView) string {
	return joinCountryViews(countries, formatCountryZH, "、")
}

func fmtCountryListEN(countries []CountryView) string {
	return joinCountryViews(countries, formatCountryEN, "; ")
}

func joinCountryViews(
	countries []CountryView,
	formatter func(CountryView) string,
	separator string,
) string {
	parts := make([]string, 0, len(countries))
	for _, country := range countries {
		value := formatter(country)
		if value == "" {
			continue
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, separator)
}

func formatCountryBilingual(country CountryView) string {
	label := formatCountryNames(country.NameZH, country.NameEN, " / ")
	switch {
	case country.Code != "" && label != "":
		return fmt.Sprintf("%s %s", country.Code, label)
	case label != "":
		return label
	default:
		return country.Code
	}
}

func formatCountryZH(country CountryView) string {
	return firstNonEmpty(country.NameZH, country.NameEN, country.Code)
}

func formatCountryEN(country CountryView) string {
	name := firstNonEmpty(country.NameEN, country.NameZH)
	switch {
	case country.Code != "" && name != "":
		return fmt.Sprintf("%s — %s", country.Code, name)
	case name != "":
		return name
	default:
		return country.Code
	}
}

func formatCountryNames(primary, secondary, separator string) string {
	switch {
	case primary != "" && secondary != "" && primary != secondary:
		return primary + separator + secondary
	case primary != "":
		return primary
	default:
		return secondary
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// humanMoney formats 1234567.89 as "1,234,567.89".
func humanMoney(x float64) string {
	s := fmt.Sprintf("%.2f", x)
	intPart, fracPart := s, ""
	for i, c := range s {
		if c == '.' {
			intPart, fracPart = s[:i], s[i:]
			break
		}
	}
	n := len(intPart)
	if n <= 3 {
		return intPart + fracPart
	}
	var b bytes.Buffer
	first := n % 3
	if first > 0 {
		b.WriteString(intPart[:first])
		if n > first {
			b.WriteByte(',')
		}
	}
	for i := first; i < n; i += 3 {
		b.WriteString(intPart[i : i+3])
		if i+3 < n {
			b.WriteByte(',')
		}
	}
	return b.String() + fracPart
}

// short returns the first 8 chars of a UUID for user-facing display.
func short(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
