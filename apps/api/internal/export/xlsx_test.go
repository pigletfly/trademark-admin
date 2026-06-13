package export_test

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
)

func TestRenderXLSX_ContainsQuotationAndMethodPricingLines(t *testing.T) {
	v := baseView()
	v.Lines = []export.ExportLine{
		{
			FeeItem:            "Single filing first class fee",
			RegistrationMethod: "single",
			CountryArea:        "Singapore",
			Quantity:           1,
			AmountCNYCents:     360000,
		},
	}
	data, err := export.RenderXLSX(v)
	if err != nil {
		t.Fatalf("render xlsx: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open xlsx zip: %v", err)
	}
	var sheet string
	for _, file := range zr.File {
		if file.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open sheet: %v", err)
		}
		raw, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read sheet: %v", err)
		}
		sheet = string(raw)
	}
	if sheet == "" {
		t.Fatal("sheet1.xml not found")
	}
	for _, want := range []string{
		"Quotation",
		"Countries",
		"Madrid Registration",
		"Single Filing",
		"US — United States",
		"AR — Argentina",
		"Registration Method",
		"single",
		"Singapore",
		"Single filing first class fee",
	} {
		if !strings.Contains(sheet, want) {
			t.Fatalf("missing %q in sheet:\n%s", want, sheet)
		}
	}
}
