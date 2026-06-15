package export

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
)

func TestRenderDOCX_UsesBundledTemplateAndReplacesDynamicSections(t *testing.T) {
	var buf bytes.Buffer
	v := sampleTemplateView()
	if err := RenderDOCX(&buf, v); err != nil {
		t.Fatalf("render: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}

	wantMembers := map[string]bool{
		"[Content_Types].xml": false,
		"word/document.xml":   false,
		"word/styles.xml":     false,
		"docProps/core.xml":   false,
	}
	var doc string
	for _, f := range r.File {
		if _, ok := wantMembers[f.Name]; ok {
			wantMembers[f.Name] = true
		}
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open document.xml: %v", err)
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read document.xml: %v", err)
		}
		doc = string(raw)
	}
	for name, seen := range wantMembers {
		if !seen {
			t.Fatalf("missing template member %s", name)
		}
	}
	if doc == "" {
		t.Fatal("word/document.xml missing")
	}
	plain := extractTextRuns(t, doc)

	wantSubstrings := []string{
		"国际商标申请报价方案",
		"致:Acme 有限公司",
		"拟注册类别：第19类、第35类",
		"拟注册国家/地区：美国、阿根廷（共2个国家/地区）",
		"（一）通过马德里方式申请",
		"1.国家（共1个国家/地区）：美国",
		"2.类别：第19类、第35类",
		"基础费用",
		"美国",
		"653",
		"266",
		"300",
		"合计（RMB）：7245元人民币（不含税）/7679元人民币（含税，6%）",
		"（二）通过单一注册方式申请",
		"1.国家（共1个国家/地区）：阿根廷",
		"4000",
		"2700",
		"12-18",
		"10",
		"合计（RMB）：6700【含税，6%】",
		"（三）总的报价方案",
		"总的报价为14379元",
		"单一国",
		"14379",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(plain, want) {
			t.Fatalf("document text missing %q\n%s", want, plain)
		}
	}

	notWanted := []string{
		"致: ABBC",
	}
	for _, s := range notWanted {
		if strings.Contains(plain, s) {
			t.Fatalf("document text still contains template placeholder %q\n%s", s, plain)
		}
	}
}

func TestRenderDOCX_UsesGeneratedAtForChineseDateParagraph(t *testing.T) {
	var buf bytes.Buffer
	v := sampleTemplateView()
	v.GeneratedAt = time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)

	if err := RenderDOCX(&buf, v); err != nil {
		t.Fatalf("render: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}
	var doc string
	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open document.xml: %v", err)
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read document.xml: %v", err)
		}
		doc = string(raw)
		break
	}
	if doc == "" {
		t.Fatal("word/document.xml missing")
	}
	plain := extractTextRuns(t, doc)

	if !strings.Contains(plain, "2026年6月14日") {
		t.Fatalf("generated date missing from document\n%s", plain)
	}
	if strings.Contains(plain, "2026年6月1日") {
		t.Fatalf("template date still present\n%s", plain)
	}
}

func TestRenderDOCX_SingleOnlyRemovesMadridSectionAndRenumbersSummary(t *testing.T) {
	var buf bytes.Buffer
	v := sampleSingleOnlyTemplateView()
	if err := RenderDOCX(&buf, v); err != nil {
		t.Fatalf("render: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}
	var doc string
	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open document.xml: %v", err)
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read document.xml: %v", err)
		}
		doc = string(raw)
		break
	}
	if doc == "" {
		t.Fatal("word/document.xml missing")
	}
	plain := extractTextRuns(t, doc)

	if strings.Contains(plain, "通过马德里方式申请") {
		t.Fatalf("madrid section should be removed\n%s", plain)
	}
	for _, want := range []string{
		"（一）通过单一注册方式申请",
		"（二）总的报价方案",
		"中国香港（共1个国家/地区）",
		"3800",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("document text missing %q\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "（三）总的报价方案") {
		t.Fatalf("summary numbering not updated\n%s", plain)
	}
}

func TestRenderDOCX_EscapesXMLSpecialChars(t *testing.T) {
	var buf bytes.Buffer
	v := sampleTemplateView()
	v.CustomerName = `Evil & Co <script>`
	v.CountrySummary = []CountryView{{Code: "US", NameZH: `A "B"`, NameEN: "US"}}
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
	if strings.Contains(s, "<script>") {
		t.Fatal("raw XML-breaking script tag leaked into document")
	}
	if !strings.Contains(s, "Evil &amp; Co") {
		t.Fatal("customer name ampersand not escaped")
	}
	if !strings.Contains(s, "A &#34;B&#34;") {
		t.Fatal("country name quotes not escaped")
	}
}

func TestRenderDOCX_DerivesCountryListsFromQuoteSectionsWhenMethodListsMissing(t *testing.T) {
	var buf bytes.Buffer
	v := sampleTemplateView()
	v.Countries = nil
	v.CountrySummary = nil
	v.MadridCountries = nil
	v.SingleCountries = nil

	if err := RenderDOCX(&buf, v); err != nil {
		t.Fatalf("render: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}
	var doc string
	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open document.xml: %v", err)
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read document.xml: %v", err)
		}
		doc = string(raw)
		break
	}
	if doc == "" {
		t.Fatal("word/document.xml missing")
	}
	plain := extractTextRuns(t, doc)

	for _, want := range []string{
		"拟注册国家/地区：美国、阿根廷（共2个国家/地区）",
		"1.国家（共1个国家/地区）：美国",
		"1.国家（共1个国家/地区）：阿根廷",
		"美国（共1个国家/地区）",
		"阿根廷（共1个国家/地区）",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("document text missing %q\n%s", want, plain)
		}
	}
}

func TestPopulateTemplateSections_DerivesMethodCountriesBeforeSummary(t *testing.T) {
	usID := uuid.New()
	madridOfficialFee := int64(26600)

	h := &Handler{pricingRepo: &pricing.Repository{}}
	v := QuotationView{
		NiceCategoryCodes: []int{19, 35},
	}
	snap := quotation.Snapshot{
		Lines: []quotation.SnapshotLine{
			{
				RegistrationMethod:  pricing.RegistrationMethodMadrid,
				CountryID:           &usID,
				CountryArea:         "美国",
				OfficialFeeCHFCents: &madridOfficialFee,
				AmountCNYCents:      192000,
			},
			{
				RegistrationMethod: pricing.RegistrationMethodSingle,
				CountryArea:        "阿根廷",
				AmountCNYCents:     400000,
			},
		},
	}

	if err := h.populateTemplateSections(context.Background(), &v, snap); err != nil {
		t.Fatalf("populate sections: %v", err)
	}

	if len(v.MadridCountries) != 1 || v.MadridCountries[0].NameZH != "美国" {
		t.Fatalf("madrid countries = %#v", v.MadridCountries)
	}
	if len(v.SingleCountries) != 1 || v.SingleCountries[0].NameZH != "阿根廷" {
		t.Fatalf("single countries = %#v", v.SingleCountries)
	}
	if len(v.CountrySummary) != 2 {
		t.Fatalf("country summary = %#v", v.CountrySummary)
	}

	wantSummary := []string{
		"美国（共1个国家/地区）",
		"阿根廷（共1个国家/地区）",
	}
	if len(v.SummaryQuote.Rows) != 2 {
		t.Fatalf("summary rows = %#v", v.SummaryQuote.Rows)
	}
	for i, want := range wantSummary {
		if v.SummaryQuote.Rows[i].CountryAreaSummary != want {
			t.Fatalf("summary row %d country area = %q want %q", i, v.SummaryQuote.Rows[i].CountryAreaSummary, want)
		}
	}
	for _, want := range []string{
		"在美国（共1个国家/地区）通过马德里途径提起针对第19类、第35类的新申请",
		"在阿根廷（共1个国家/地区）通过单一注册方式提起针对第19类、第35类的新申请",
	} {
		if !strings.Contains(v.SummaryNarrative, want) {
			t.Fatalf("summary narrative missing %q: %s", want, v.SummaryNarrative)
		}
	}
}

func TestPopulateTemplateSections_MergesIncompleteCountrySummaryWithMethodCountries(t *testing.T) {
	usID := uuid.New()
	madridOfficialFee := int64(26600)

	h := &Handler{pricingRepo: &pricing.Repository{}}
	v := QuotationView{
		Countries:         []CountryView{{ID: usID, NameZH: "美国"}},
		CountrySummary:    []CountryView{{ID: usID, NameZH: "美国"}},
		NiceCategoryCodes: []int{19, 35},
	}
	snap := quotation.Snapshot{
		Lines: []quotation.SnapshotLine{
			{
				RegistrationMethod:  pricing.RegistrationMethodMadrid,
				CountryID:           &usID,
				CountryArea:         "美国",
				OfficialFeeCHFCents: &madridOfficialFee,
				AmountCNYCents:      192000,
			},
			{
				RegistrationMethod: pricing.RegistrationMethodSingle,
				CountryArea:        "阿根廷",
				AmountCNYCents:     400000,
			},
		},
	}

	if err := h.populateTemplateSections(context.Background(), &v, snap); err != nil {
		t.Fatalf("populate sections: %v", err)
	}

	if len(v.CountrySummary) != 2 {
		t.Fatalf("country summary = %#v", v.CountrySummary)
	}
	if v.CountrySummary[0].NameZH != "美国" || v.CountrySummary[1].NameZH != "阿根廷" {
		t.Fatalf("country summary order = %#v", v.CountrySummary)
	}
}

func TestPopulateTemplateSections_KeepsCountriesAlignedWithMergedSummary(t *testing.T) {
	usID := uuid.New()
	madridOfficialFee := int64(26600)

	h := &Handler{pricingRepo: &pricing.Repository{}}
	v := QuotationView{
		Countries:         []CountryView{{ID: usID, NameZH: "美国"}},
		CountrySummary:    []CountryView{{ID: usID, NameZH: "美国"}},
		NiceCategoryCodes: []int{19, 35},
	}
	snap := quotation.Snapshot{
		Lines: []quotation.SnapshotLine{
			{
				RegistrationMethod:  pricing.RegistrationMethodMadrid,
				CountryID:           &usID,
				CountryArea:         "美国",
				OfficialFeeCHFCents: &madridOfficialFee,
				AmountCNYCents:      192000,
			},
			{
				RegistrationMethod: pricing.RegistrationMethodSingle,
				CountryArea:        "阿根廷",
				AmountCNYCents:     400000,
			},
		},
	}

	if err := h.populateTemplateSections(context.Background(), &v, snap); err != nil {
		t.Fatalf("populate sections: %v", err)
	}

	if len(v.Countries) != 2 {
		t.Fatalf("countries = %#v", v.Countries)
	}
	if v.Countries[0].NameZH != "美国" || v.Countries[1].NameZH != "阿根廷" {
		t.Fatalf("countries order = %#v", v.Countries)
	}
}

func sampleTemplateView() QuotationView {
	return QuotationView{
		QuotationID:       "11111111-2222-3333-4444-555555555555",
		Status:            "approved",
		ServiceTier:       "basic",
		CustomerName:      "Acme 有限公司",
		NiceCategoryCodes: []int{19, 35},
		CountrySummary: []CountryView{
			{Code: "US", NameZH: "美国", NameEN: "United States"},
			{Code: "AR", NameZH: "阿根廷", NameEN: "Argentina"},
		},
		MadridCountries: []CountryView{
			{Code: "US", NameZH: "美国", NameEN: "United States"},
		},
		SingleCountries: []CountryView{
			{Code: "AR", NameZH: "阿根廷", NameEN: "Argentina"},
		},
		MadridQuote: &MadridQuoteSection{
			BaseOfficialFeeCHFCents: 65300,
			BaseAgencyFeeCNYCents:   30000,
			Rows: []MadridQuoteRow{{
				SequenceNo:          1,
				CountryArea:         "美国",
				OfficialFeeCHFCents: 26600,
				AgencyFeeCNYCents:   30000,
				ValidityText:        "10年",
			}},
			OfficialTotalCHFCents: 91900,
			OfficialTotalCNYCents: 664500,
			AgencyTotalCNYCents:   60000,
			SubtotalCNYCents:      724500,
			TotalWithTaxCNYCents:  767900,
		},
		SingleQuote: &SingleQuoteSection{
			Rows: []SingleQuoteRow{{
				SequenceNo:              1,
				CountryArea:             "阿根廷",
				ApplicationFeeCNYCents:  400000,
				NotarizationFeeText:     "2700",
				NotarizationFeeCNYCents: 270000,
				SubmissionMethodText:    "一标一类",
				RegistrationMonthsText:  "12-18",
				ValidityYearsText:       "10",
			}},
			TotalCNYCents: 670000,
		},
		SummaryQuote: SummaryQuoteSection{
			Rows: []SummaryQuoteRow{
				{
					MethodLabel:        "马德里",
					CategoryCodeText:   "19、35",
					CountryAreaSummary: "美国（共1个国家/地区）",
					FeeCNYCents:        767900,
				},
				{
					MethodLabel:        "单一国",
					CategoryCodeText:   "19、35",
					CountryAreaSummary: "阿根廷（共1个国家/地区）",
					FeeCNYCents:        670000,
				},
			},
			TotalCNYCents: 1437900,
		},
		SummaryNarrative: "针对贵司的需求，我司建议在美国等 1 个国家/地区通过马德里途径提起针对第19类、第35类的新申请，在阿根廷等 1 个国家/地区通过单一注册方式提起针对第19类、第35类的新申请，总的报价为14379元，请贵司查阅。",
		Signature:        "sig-template",
		GeneratedAt:      time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC),
	}
}

func sampleSingleOnlyTemplateView() QuotationView {
	v := sampleTemplateView()
	v.CountrySummary = []CountryView{{Code: "HK", NameZH: "中国香港", NameEN: "Hong Kong"}}
	v.MadridCountries = nil
	v.SingleCountries = []CountryView{{Code: "HK", NameZH: "中国香港", NameEN: "Hong Kong"}}
	v.MadridQuote = nil
	v.SingleQuote = &SingleQuoteSection{
		Rows: []SingleQuoteRow{{
			SequenceNo:             1,
			CountryArea:            "中国香港",
			ApplicationFeeCNYCents: 380000,
			NotarizationFeeText:    "0",
			SubmissionMethodText:   "一标一类",
			RegistrationMonthsText: "6-8",
			ValidityYearsText:      "10",
		}},
		TotalCNYCents: 380000,
	}
	v.SummaryQuote = SummaryQuoteSection{
		Rows: []SummaryQuoteRow{{
			MethodLabel:        "单一国",
			CategoryCodeText:   "19、35",
			CountryAreaSummary: "中国香港（共1个国家/地区）",
			FeeCNYCents:        380000,
		}},
		TotalCNYCents: 380000,
	}
	v.SummaryNarrative = "针对贵司的需求，我司建议在中国香港等 1 个国家/地区通过单一注册方式提起针对第19类、第35类的新申请，总的报价为3800元，请贵司查阅。"
	return v
}

func extractTextRuns(t *testing.T, raw string) string {
	t.Helper()
	decoder := xml.NewDecoder(strings.NewReader(raw))
	var b strings.Builder
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode xml: %v", err)
		}
		charData, ok := tok.(xml.CharData)
		if !ok {
			continue
		}
		text := string(charData)
		if strings.TrimSpace(text) == "" {
			continue
		}
		b.WriteString(text)
	}
	return b.String()
}
