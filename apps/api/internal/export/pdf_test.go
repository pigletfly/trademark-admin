package export_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
)

func baseView() export.QuotationView {
	now := time.Date(2026, 4, 25, 10, 30, 0, 0, time.UTC)
	return export.QuotationView{
		QuotationID:   uuid.New().String(),
		Status:        "approved",
		ServiceTier:   "standard",
		CustomerName:  "北京示例科技",
		CountryNameZH: "中国",
		CountryNameEN: "China",
		CountryCode:   "CN",
		TotalCNYCents: 123456,
		Signature:     "v2|cn|standard|1:official=123456;|=123456",
		Lines: []export.ExportLine{
			{FeeItem: "official", AmountCNYCents: 100000},
			{FeeItem: "agent", AmountCNYCents: 23456},
		},
		SubmittedAt: &now,
		ReviewedAt:  &now,
		GeneratedAt: now,
	}
}

func TestRenderHTML_Bilingual_ContainsBothLanguages(t *testing.T) {
	html, err := export.RenderHTML(baseView(), export.LanguageBilingual)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(html)
	for _, want := range []string{
		"报价书 / Quotation",
		"客户 Customer",
		"北京示例科技",
		"¥ 1,234.56", // fmtCNY of 123456 cents
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in output:\n%s", want, s)
		}
	}
}

func TestRenderHTML_Bilingual_IncludesMethodPricingColumns(t *testing.T) {
	v := baseView()
	unit := int64(574640)
	chf := int64(65300)
	v.Lines = []export.ExportLine{
		{
			FeeItem:             "Madrid base official fee",
			RegistrationMethod:  "madrid",
			CountryArea:         "Basic registration fee - black and white mark",
			Quantity:            1,
			UnitAmountCNYCents:  &unit,
			OfficialFeeCHFCents: &chf,
			AmountCNYCents:      574640,
		},
	}
	html, err := export.RenderHTML(v, export.LanguageBilingual)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(html)
	for _, want := range []string{
		"注册方式 Method",
		"国家/地区 Country/Region",
		"madrid",
		"Basic registration fee - black and white mark",
		"CHF 653.00",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in output:\n%s", want, s)
		}
	}
}

func TestRenderHTML_ZH_NoEnglishLabels(t *testing.T) {
	html, err := export.RenderHTML(baseView(), export.LanguageZH)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(html)
	if !strings.Contains(s, "客户") {
		t.Fatalf("missing 客户 label")
	}
	// Label "Customer" MUST NOT appear. Data may still be Chinese; that's fine.
	if strings.Contains(s, "Customer") {
		t.Fatalf("Customer label leaked into zh template")
	}
}

func TestRenderHTML_EN_NoChineseLabels(t *testing.T) {
	html, err := export.RenderHTML(baseView(), export.LanguageEN)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(html)
	if !strings.Contains(s, "Customer") {
		t.Fatalf("missing Customer label in en template")
	}
	// Chinese "报价书" label must not appear in an English template.
	if strings.Contains(s, "报价书") {
		t.Fatalf("Chinese 报价书 label leaked into en template")
	}
}

func TestRenderHTML_XSSEscape(t *testing.T) {
	v := baseView()
	v.CustomerName = "<script>alert(1)</script>"
	html, err := export.RenderHTML(v, export.LanguageBilingual)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(html), "<script>alert(1)</script>") {
		t.Fatalf("raw script leaked")
	}
	if !strings.Contains(string(html), "&lt;script&gt;") {
		t.Fatalf("missing escaped form")
	}
}

func TestRenderHTML_ShortIDAutoFilled(t *testing.T) {
	v := baseView()
	v.QuotationIDShort = "" // force derivation
	html, err := export.RenderHTML(v, export.LanguageBilingual)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	short := v.QuotationID[:8]
	if !strings.Contains(strings.ToLower(string(html)), strings.ToLower(short)) {
		t.Fatalf("missing derived short id %q in output", short)
	}
}
