package export

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func TestRenderDOCX_ZipStructure(t *testing.T) {
	var buf bytes.Buffer
	v := sampleView()
	if err := RenderDOCX(&buf, v); err != nil {
		t.Fatalf("render: %v", err)
	}
	// Open the bytes as a zip.
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}
	want := map[string]bool{
		"[Content_Types].xml": false,
		"_rels/.rels":         false,
		"word/document.xml":   false,
	}
	for _, f := range r.File {
		if _, ok := want[f.Name]; ok {
			want[f.Name] = true
		}
	}
	for name, got := range want {
		if !got {
			t.Errorf("missing expected member: %s", name)
		}
	}
}

func TestRenderDOCX_BodyContainsBilingualContent(t *testing.T) {
	var buf bytes.Buffer
	v := sampleView()
	if err := RenderDOCX(&buf, v); err != nil {
		t.Fatalf("render: %v", err)
	}
	r, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	var doc *zip.File
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			doc = f
			break
		}
	}
	if doc == nil {
		t.Fatal("word/document.xml missing")
	}
	rc, _ := doc.Open()
	defer rc.Close()
	body, _ := io.ReadAll(rc)
	s := string(body)

	wantSubstrings := []string{
		"报价书 / Quotation",
		"Acme 有限公司",
		"US",
		"美国",
		"United States",
		"AR",
		"阿根廷",
		"Argentina",
		"马德里注册 / Madrid Registration",
		"单一注册 / Single Filing",
		"application",
		"agent",
		"¥ 150.00",
		"¥ 100.00",
		"¥ 50.00",
		"sig-abc",
	}
	for _, w := range wantSubstrings {
		if !strings.Contains(s, w) {
			t.Errorf("body missing %q", w)
		}
	}
}

func TestRenderDOCX_EscapesXMLSpecialChars(t *testing.T) {
	var buf bytes.Buffer
	v := sampleView()
	v.CustomerName = `Evil & Co <script>`
	v.Notes = `with "quotes" and 'apostrophes'`
	if err := RenderDOCX(&buf, v); err != nil {
		t.Fatalf("render: %v", err)
	}
	r, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	var body []byte
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			body, _ = io.ReadAll(rc)
			rc.Close()
			break
		}
	}
	s := string(body)

	// Raw XML-breaking substrings must NOT appear.
	if strings.Contains(s, "<script>") {
		t.Error("unescaped <script> leaked into body")
	}
	// The escaped form must.
	if !strings.Contains(s, "Evil &amp; Co") {
		t.Error("& not escaped")
	}
	if !strings.Contains(s, "&#34;quotes&#34;") {
		t.Error(`" not escaped`)
	}
}

func TestHumanMoney(t *testing.T) {
	cases := map[float64]string{
		0:        "0.00",
		12.34:    "12.34",
		1234:     "1,234.00",
		1234567:  "1,234,567.00",
		12345678: "12,345,678.00",
	}
	for in, want := range cases {
		got := humanMoney(in)
		if got != want {
			t.Errorf("humanMoney(%v) = %q, want %q", in, got, want)
		}
	}
}

func sampleView() QuotationView {
	submitted := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	reviewed := submitted.Add(2 * time.Hour)
	return QuotationView{
		QuotationID:  "11111111-2222-3333-4444-555555555555",
		Status:       "approved",
		ServiceTier:  "basic",
		CustomerName: "Acme 有限公司",
		Countries: []CountryView{
			{Code: "US", NameZH: "美国", NameEN: "United States"},
			{Code: "AR", NameZH: "阿根廷", NameEN: "Argentina"},
		},
		MadridCountries: []CountryView{
			{Code: "US", NameZH: "美国", NameEN: "United States"},
		},
		SingleCountries: []CountryView{
			{Code: "AR", NameZH: "阿根廷", NameEN: "Argentina"},
		},
		CountryNameZH: "中国",
		CountryNameEN: "China",
		CountryCode:   "CN",
		TotalCNYCents: 15000,
		Signature:     "sig-abc",
		Lines: []ExportLine{
			{FeeItem: "application", AmountCNYCents: 10000},
			{FeeItem: "agent", AmountCNYCents: 5000},
		},
		SubmittedAt:   &submitted,
		ReviewedAt:    &reviewed,
		ReviewComment: "通过 / Approved",
		Notes:         "备注 / note",
		GeneratedAt:   time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC),
	}
}
