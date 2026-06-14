package export

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"time"
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
		"fmtCNY":                  fmtCNY,
		"fmtCNYPtr":               fmtCNYPtr,
		"fmtCHFPtr":               fmtCHFPtr,
		"fmtQuantity":             fmtQuantity,
		"fmtCountryListBilingual": fmtCountryListBilingual,
		"fmtCountryListZH":        fmtCountryListZH,
		"fmtCountryListEN":        fmtCountryListEN,
	}).Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("export: parse template: %w", err)
	}
	// Fill QuotationIDShort if caller left it blank.
	view := normalizeCountryLists(v)
	if view.QuotationIDShort == "" && len(view.QuotationID) > 8 {
		view.QuotationIDShort = view.QuotationID[:8]
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("export: execute template: %w", err)
	}
	return buf.Bytes(), nil
}

// Gotenberg POSTs HTML to a gotenberg chromium service and streams back PDF.
// Ref: https://gotenberg.dev/docs/routes#html-file-into-pdf-route
type Gotenberg struct {
	baseURL string
	client  *http.Client
}

func NewGotenberg(baseURL string) *Gotenberg {
	return &Gotenberg{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// RenderPDF takes HTML bytes and returns PDF bytes. The gotenberg route
// /forms/chromium/convert/html expects a multipart form with an "index.html"
// file (and optional assets); we send only index.html for MVP.
func (g *Gotenberg) RenderPDF(ctx context.Context, html []byte) ([]byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, fmt.Errorf("gotenberg: create part: %w", err)
	}
	if _, err := part.Write(html); err != nil {
		return nil, fmt.Errorf("gotenberg: write html: %w", err)
	}
	// A4 (inches, per gotenberg docs).
	_ = mw.WriteField("paperWidth", "8.27")
	_ = mw.WriteField("paperHeight", "11.69")
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("gotenberg: close multipart: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.baseURL+"/forms/chromium/convert/html", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("gotenberg: status %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}
