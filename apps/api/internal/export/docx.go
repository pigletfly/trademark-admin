package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"strings"
	"time"
)

// QuotationView is the render input. Caller resolves it from the
// quotation + customer + country repos; the renderer has no DB access.
type QuotationView struct {
	QuotationID      string
	QuotationIDShort string // first 8 chars; filled by RenderHTML if blank
	Status           string
	ServiceTier      string
	CustomerName     string
	Countries        []CountryView
	MadridCountries  []CountryView
	SingleCountries  []CountryView
	CountryNameZH    string
	CountryNameEN    string
	CountryCode      string
	TotalCNYCents    int64
	Signature        string
	Lines            []ExportLine
	SubmittedAt      *time.Time
	ReviewedAt       *time.Time
	ReviewComment    string
	Notes            string
	GeneratedAt      time.Time
}

type CountryView struct {
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

func (v QuotationView) allCountries() []CountryView {
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

// RenderDOCX writes a .docx file's bytes to w. Format is bilingual —
// labels render in 中文 and English, values stay as-is.
func RenderDOCX(w io.Writer, v QuotationView) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	// 1. [Content_Types].xml — mandatory.
	if err := writeFile(zw, "[Content_Types].xml", contentTypesXML); err != nil {
		return err
	}
	// 2. _rels/.rels — mandatory root rels pointing to word/document.xml.
	if err := writeFile(zw, "_rels/.rels", rootRelsXML); err != nil {
		return err
	}
	// 3. word/document.xml — the body content.
	body, err := renderBody(v)
	if err != nil {
		return err
	}
	if err := writeFile(zw, "word/document.xml", body); err != nil {
		return err
	}
	return nil
}

func writeFile(zw *zip.Writer, name, body string) error {
	f, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, bytes.NewReader([]byte(body)))
	return err
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`

const rootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

// renderBody builds word/document.xml. We don't use html/template to
// avoid any accidental HTML escaping in XML contexts — every value is
// explicitly escaped via xml-safe helpers.
func renderBody(v QuotationView) (string, error) {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>`)

	heading(&b, "报价书 / Quotation", 28, true)
	para(&b, fmt.Sprintf("编号 / No.: %s", short(v.QuotationID)))
	para(&b, fmt.Sprintf("生成时间 / Generated: %s", v.GeneratedAt.Format("2006-01-02 15:04")))
	para(&b, "")

	heading(&b, "1. 基本信息 / Basic Info", 20, false)
	kv(&b, "客户 / Customer", v.CustomerName)
	kv(&b, "国家 / Countries", fmtCountryListBilingual(v.allCountries()))
	if len(v.MadridCountries) > 0 {
		kv(&b, "马德里注册 / Madrid Registration", fmtCountryListBilingual(v.MadridCountries))
	}
	if len(v.SingleCountries) > 0 {
		kv(&b, "单一注册 / Single Filing", fmtCountryListBilingual(v.SingleCountries))
	}
	kv(&b, "服务级别 / Service Tier", v.ServiceTier)
	kv(&b, "状态 / Status", v.Status)
	if v.SubmittedAt != nil {
		kv(&b, "提交时间 / Submitted At", v.SubmittedAt.Format("2006-01-02 15:04"))
	}
	if v.ReviewedAt != nil {
		kv(&b, "审核时间 / Reviewed At", v.ReviewedAt.Format("2006-01-02 15:04"))
	}
	if v.ReviewComment != "" {
		kv(&b, "审核备注 / Review Comment", v.ReviewComment)
	}
	if v.Notes != "" {
		kv(&b, "备注 / Notes", v.Notes)
	}
	para(&b, "")

	heading(&b, "2. 报价明细 / Fee Breakdown", 20, false)
	// Table header.
	b.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="5000" w:type="pct"/></w:tblPr>`)
	tableRow(&b, []string{
		"注册方式 / Method",
		"国家/地区 / Country",
		"费用项 / Fee Item",
		"数量 / Qty",
		"单价 / Unit (CNY)",
		"官费 / Official (CHF)",
		"金额 / Amount (CNY)",
	}, true)
	for _, l := range v.Lines {
		tableRow(&b, []string{
			l.RegistrationMethod,
			l.CountryArea,
			l.FeeItem,
			fmtQuantity(l.Quantity),
			fmtCNYPtr(l.UnitAmountCNYCents),
			fmtCHFPtr(l.OfficialFeeCHFCents),
			fmtCNY(l.AmountCNYCents),
		}, false)
	}
	tableRow(&b, []string{"", "", "合计 / Total", "", "", "", fmtCNY(v.TotalCNYCents)}, true)
	b.WriteString(`</w:tbl>`)
	para(&b, "")

	heading(&b, "3. 签名 / Signature", 20, false)
	para(&b, v.Signature)
	para(&b, "")
	para(&b, "—— 本文档由系统自动生成 / Auto-generated document ——")

	b.WriteString(`</w:body></w:document>`)
	return b.String(), nil
}

// heading writes a paragraph with the Heading1 style and size. `sz` is
// in half-points (OOXML convention).
func heading(b *bytes.Buffer, text string, halfPt int, doubleAfter bool) {
	spacing := ""
	if doubleAfter {
		spacing = `<w:spacing w:after="240"/>`
	}
	fmt.Fprintf(b, `<w:p><w:pPr>%s</w:pPr><w:r><w:rPr><w:b/><w:sz w:val="%d"/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r></w:p>`,
		spacing, halfPt, xmlEscape(text))
}

func para(b *bytes.Buffer, text string) {
	if text == "" {
		b.WriteString(`<w:p/>`)
		return
	}
	fmt.Fprintf(b, `<w:p><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, xmlEscape(text))
}

func kv(b *bytes.Buffer, k, v string) {
	fmt.Fprintf(b, `<w:p><w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">%s: </w:t></w:r><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p>`,
		xmlEscape(k), xmlEscape(v))
}

func tableRow(b *bytes.Buffer, cells []string, bold bool) {
	b.WriteString(`<w:tr>`)
	for _, c := range cells {
		weight := ``
		if bold {
			weight = `<w:rPr><w:b/></w:rPr>`
		}
		fmt.Fprintf(b, `<w:tc><w:tcPr><w:tcW w:w="2500" w:type="pct"/></w:tcPr><w:p><w:r>%s<w:t xml:space="preserve">%s</w:t></w:r></w:p></w:tc>`,
			weight, xmlEscape(c))
	}
	b.WriteString(`</w:tr>`)
}

// xmlEscape escapes < > & ' " inside text nodes / attribute values.
// encoding/xml's EscapeText handles these for us when writing through
// xml.Encoder, but we build the document with fmt.Fprintf for speed
// and control, so we escape explicitly. We use html.EscapeString
// which over-escapes `<`, `>`, `&` which is safe in XML.
func xmlEscape(s string) string {
	// html.EscapeString handles &, <, >, and ". It leaves ' alone.
	s = html.EscapeString(s)
	// Convert single quote explicitly for attribute safety.
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
	name := firstNonEmpty(country.NameZH, country.NameEN)
	switch {
	case country.Code != "" && name != "":
		return fmt.Sprintf("%s — %s", country.Code, name)
	case name != "":
		return name
	default:
		return country.Code
	}
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
	// Locate decimal point.
	intPart, fracPart := s, ""
	for i, c := range s {
		if c == '.' {
			intPart, fracPart = s[:i], s[i:]
			break
		}
	}
	// Insert commas into intPart.
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

// Sentinel to make encoding/xml package usage explicit if we expand.
var _ = xml.NewEncoder
