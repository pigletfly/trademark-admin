package pricing

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func mustEntry(t *testing.T, c uuid.UUID, tier, item string, cents int64, deprecated bool) PricingEntry {
	t.Helper()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e := PricingEntry{
		ID:             uuid.New(),
		CountryID:      c,
		ServiceTier:    tier,
		FeeItem:        item,
		AmountCNYCents: cents,
		EffectiveFrom:  from,
		CreatedBy:      uuid.New(),
	}
	if deprecated {
		to := from.Add(24 * time.Hour)
		e.EffectiveTo = &to
	}
	return e
}

func TestCalculate_InvalidTier(t *testing.T) {
	_, err := Calculate(nil, CalcInput{CountryID: uuid.New(), ServiceTier: "vip"})
	if err != ErrInvalidTier {
		t.Fatalf("want ErrInvalidTier, got %v", err)
	}
}

func TestCalculate_NoMatchingEntries(t *testing.T) {
	c := uuid.New()
	entries := []PricingEntry{
		mustEntry(t, uuid.New(), "basic", "application", 10000, false), // wrong country
	}
	_, err := Calculate(entries, CalcInput{CountryID: c, ServiceTier: "basic"})
	if err != ErrNoMatchingEntries {
		t.Fatalf("want ErrNoMatchingEntries, got %v", err)
	}
}

func TestCalculate_FiltersDeprecatedAndOtherTier(t *testing.T) {
	c := uuid.New()
	entries := []PricingEntry{
		mustEntry(t, c, "basic", "application", 10000, false),
		mustEntry(t, c, "basic", "agent", 5000, false),
		mustEntry(t, c, "basic", "deprecated_fee", 9999, true),  // deprecated
		mustEntry(t, c, "premium", "application", 20000, false), // wrong tier
	}
	res, err := Calculate(entries, CalcInput{CountryID: c, ServiceTier: "basic"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.TotalCNYCents != 15000 {
		t.Fatalf("total: want 15000, got %d", res.TotalCNYCents)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("lines: want 2, got %d", len(res.Lines))
	}
	// Lines must be sorted alphabetically by fee_item so signature is
	// stable regardless of input order.
	if res.Lines[0].FeeItem != "agent" || res.Lines[1].FeeItem != "application" {
		t.Fatalf("expected [agent, application] order, got %+v", res.Lines)
	}
}

func TestCalculate_SignatureStableAcrossInputOrder(t *testing.T) {
	c := uuid.New()
	a := mustEntry(t, c, "standard", "aa_fee", 1000, false)
	b := mustEntry(t, c, "standard", "bb_fee", 2000, false)

	r1, _ := Calculate([]PricingEntry{a, b}, CalcInput{c, "standard"})
	r2, _ := Calculate([]PricingEntry{b, a}, CalcInput{c, "standard"})

	if r1.Signature != r2.Signature {
		t.Fatalf("signature differs across input order: %s vs %s", r1.Signature, r2.Signature)
	}
	if r1.TotalCNYCents != 3000 {
		t.Fatalf("total: want 3000, got %d", r1.TotalCNYCents)
	}
}

func TestCalculate_SignatureChangesWithAmount(t *testing.T) {
	c := uuid.New()
	a1 := mustEntry(t, c, "standard", "fee", 1000, false)
	a2 := mustEntry(t, c, "standard", "fee", 1001, false)

	r1, _ := Calculate([]PricingEntry{a1}, CalcInput{c, "standard"})
	r2, _ := Calculate([]PricingEntry{a2}, CalcInput{c, "standard"})

	if r1.Signature == r2.Signature {
		t.Fatal("signature should change when amount changes")
	}
}
