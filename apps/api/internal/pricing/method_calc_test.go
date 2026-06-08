package pricing

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCalculateMethodPricing_SingleClassUsesFirstAndAdditionalClassFees(t *testing.T) {
	countryID := uuid.New()
	sourceID := uuid.New()
	result, err := CalculateMethodPricing(MethodPricingSet{
		SingleClass: []SingleClassPricingEntry{{
			ID:                             sourceID,
			CountryID:                      countryID,
			Continent:                      "Asia",
			CountryArea:                    "Singapore",
			FirstClassFeeCNYCents:          360000,
			FirstClassFeeTax6CNYCents:      381600,
			FirstClassFeeTax1CNYCents:      363600,
			AdditionalClassFeeCNYCents:     270000,
			AdditionalClassFeeTax6CNYCents: 286200,
			AdditionalClassFeeTax1CNYCents: 272700,
			RequiredDocuments:              "Power of attorney",
			NotarizationFee:                "0",
			AcceptanceTime:                 "2 days",
			RegistrationMonths:             "6--8",
			EffectiveFrom:                  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy:                      uuid.New(),
		}},
	}, MethodCalcInput{
		CountryIDs:          []uuid.UUID{countryID},
		RegistrationMethods: []string{RegistrationMethodSingle},
		NiceCategoryCount:   3,
	})
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if result.TotalCNYCents != 900000 {
		t.Fatalf("total: got %d want 900000", result.TotalCNYCents)
	}
	if len(result.Lines) != 2 {
		t.Fatalf("lines: got %d want 2", len(result.Lines))
	}
	first := result.Lines[0]
	if first.RegistrationMethod != RegistrationMethodSingle {
		t.Fatalf("method: got %q want %q", first.RegistrationMethod, RegistrationMethodSingle)
	}
	if first.CountryID == nil || *first.CountryID != countryID {
		t.Fatalf("country source: got %v want %s", first.CountryID, countryID)
	}
	if first.SourcePricingTable != SingleClassPricingTable || first.SourcePricingID == nil || *first.SourcePricingID != sourceID {
		t.Fatalf("source: got table=%q id=%v", first.SourcePricingTable, first.SourcePricingID)
	}
	if first.Quantity != 1 || first.UnitAmountCNYCents == nil || *first.UnitAmountCNYCents != 360000 {
		t.Fatalf("first line unit/quantity mismatch: %+v", first)
	}
	additional := result.Lines[1]
	if additional.Quantity != 2 || additional.UnitAmountCNYCents == nil || *additional.UnitAmountCNYCents != 270000 {
		t.Fatalf("additional line unit/quantity mismatch: %+v", additional)
	}
}

func TestCalculateMethodPricing_MadridUsesBaseAndDesignatedCountryFees(t *testing.T) {
	countryID := uuid.New()
	baseID := uuid.New()
	countrySourceID := uuid.New()
	result, err := CalculateMethodPricing(MethodPricingSet{
		Madrid: []MadridPricingEntry{
			{
				ID:                  baseID,
				CountryArea:         "Basic registration fee - black and white mark",
				OfficialFeeCHFCents: 65300,
				AgencyFeeCNYCents:   400000,
				IsBaseFee:           true,
				EffectiveFrom:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				CreatedBy:           uuid.New(),
			},
			{
				ID:                  countrySourceID,
				CountryID:           &countryID,
				SequenceNo:          intPtr(1),
				CountryArea:         "Singapore",
				OfficialFeeCHFCents: 26100,
				AgencyFeeCNYCents:   40000,
				EffectiveFrom:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				CreatedBy:           uuid.New(),
			},
		},
	}, MethodCalcInput{
		CountryIDs:                    []uuid.UUID{countryID},
		RegistrationMethods:           []string{RegistrationMethodMadrid},
		NiceCategoryCount:             1,
		MadridCHFExchangeRateCNYCents: 880,
	})
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if result.TotalCNYCents != 1244320 {
		t.Fatalf("total: got %d want 1244320", result.TotalCNYCents)
	}
	if len(result.Lines) != 4 {
		t.Fatalf("lines: got %d want 4", len(result.Lines))
	}
	if result.Lines[0].OfficialFeeCHFCents == nil || *result.Lines[0].OfficialFeeCHFCents != 65300 {
		t.Fatalf("base official CHF missing: %+v", result.Lines[0])
	}
	if result.Lines[2].SourcePricingTable != MadridPricingTable || result.Lines[2].SourcePricingID == nil || *result.Lines[2].SourcePricingID != countrySourceID {
		t.Fatalf("country source mismatch: %+v", result.Lines[2])
	}
}

func intPtr(v int) *int {
	return &v
}
