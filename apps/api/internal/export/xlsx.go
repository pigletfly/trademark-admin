package export

import (
	"archive/zip"
	"bytes"
	"fmt"
	"time"
)

// RenderXLSX writes a compact Excel workbook containing the same quote
// view used by PDF/DOCX exports.
func RenderXLSX(v QuotationView) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	files := map[string]string{
		"[Content_Types].xml":        xlsxContentTypesXML,
		"_rels/.rels":                xlsxRootRelsXML,
		"xl/workbook.xml":            xlsxWorkbookXML,
		"xl/_rels/workbook.xml.rels": xlsxWorkbookRelsXML,
		"docProps/app.xml":           xlsxAppXML,
		"docProps/core.xml":          xlsxCoreXML(v.GeneratedAt),
		"xl/worksheets/sheet1.xml":   renderXLSXSheet(v),
	}
	for name, body := range files {
		if err := writeFile(zw, name, body); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderXLSXSheet(v QuotationView) string {
	rows := [][]string{
		{"Quotation"},
		{"Quotation ID", short(v.QuotationID)},
		{"Customer", v.CustomerName},
		{"Countries", fmtCountryListEN(v.allCountries())},
		{"Madrid Registration", fmtCountryListEN(v.MadridCountries)},
		{"Single Filing", fmtCountryListEN(v.SingleCountries)},
		{"Status", v.Status},
		{"Service Tier", v.ServiceTier},
		{"Generated At", v.GeneratedAt.Format("2006-01-02 15:04")},
		{},
		{"Registration Method", "Country/Region", "Fee Item", "Qty", "Unit (CNY)", "Official (CHF)", "Amount (CNY)"},
	}
	for _, line := range v.Lines {
		rows = append(rows, []string{
			line.RegistrationMethod,
			line.CountryArea,
			line.FeeItem,
			fmtQuantity(line.Quantity),
			fmtCNYPtr(line.UnitAmountCNYCents),
			fmtCHFPtr(line.OfficialFeeCHFCents),
			fmtCNY(line.AmountCNYCents),
		})
	}
	rows = append(rows, []string{"", "", "Total", "", "", "", fmtCNY(v.TotalCNYCents)})
	rows = append(rows, []string{})
	rows = append(rows, []string{"Signature", v.Signature})

	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	b.WriteString(`<sheetViews><sheetView workbookViewId="0"/></sheetViews>`)
	b.WriteString(`<cols><col min="1" max="7" width="22" customWidth="1"/></cols>`)
	b.WriteString(`<sheetData>`)
	for r, row := range rows {
		rowNum := r + 1
		fmt.Fprintf(&b, `<row r="%d">`, rowNum)
		for c, value := range row {
			if value == "" {
				continue
			}
			fmt.Fprintf(
				&b,
				`<c r="%s%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`,
				xlsxColumn(c),
				rowNum,
				xmlEscape(value),
			)
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func xlsxColumn(index int) string {
	if index < 26 {
		return string(rune('A' + index))
	}
	return "A"
}

func xlsxCoreXML(generatedAt time.Time) string {
	ts := generatedAt.UTC().Format(time.RFC3339)
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <dc:title>Quotation</dc:title>
  <dc:creator>trademark-admin</dc:creator>
  <cp:lastModifiedBy>trademark-admin</cp:lastModifiedBy>
  <dcterms:created xsi:type="dcterms:W3CDTF">` + ts + `</dcterms:created>
  <dcterms:modified xsi:type="dcterms:W3CDTF">` + ts + `</dcterms:modified>
</cp:coreProperties>`
}

const xlsxContentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
  <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
</Types>`

const xlsxRootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`

const xlsxWorkbookXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="Quotation" sheetId="1" r:id="rId1"/></sheets>
</workbook>`

const xlsxWorkbookRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`

const xlsxAppXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
  <Application>trademark-admin</Application>
</Properties>`
