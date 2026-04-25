package export

import (
	"bytes"
	"fmt"
	"html/template"
)

// RenderHTML builds the bytes of the PDF-source HTML for a quotation.
// This is exposed for testing separately from any PDF engine round-trip.
// The returned bytes are the HTML input that a subsequent Task-6
// gotenberg call will convert to PDF.
func RenderHTML(v QuotationView, lang Language) ([]byte, error) {
	name := fmt.Sprintf("template_%s.html", lang)
	raw, err := templatesFS.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("export: load template %s: %w", name, err)
	}
	tpl, err := template.New(name).Funcs(template.FuncMap{
		"fmtCNY": fmtCNY,
	}).Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("export: parse template: %w", err)
	}
	// Fill QuotationIDShort if caller left it blank.
	view := v
	if view.QuotationIDShort == "" && len(view.QuotationID) > 8 {
		view.QuotationIDShort = view.QuotationID[:8]
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("export: execute template: %w", err)
	}
	return buf.Bytes(), nil
}
