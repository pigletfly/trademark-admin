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

// A fee_item containing the delimiter characters must not collide with
// a legitimate two-line set. Length-prefixing in signature() defeats
// the "a=1;b" trick.
func TestCalculate_SignatureResistsDelimiterInjection(t *testing.T) {
	c := uuid.New()
	// Attacker case: single fee named "a=1;b" with amount 2
	injected := mustEntry(t, c, "basic", "a=1;b", 2, false)
	// Honest case: two fees "a" = 1 and "b" = 2
	honestA := mustEntry(t, c, "basic", "a", 1, false)
	honestB := mustEntry(t, c, "basic", "b", 2, false)

	rInjected, err := Calculate([]PricingEntry{injected}, CalcInput{c, "basic"})
	if err != nil {
		t.Fatalf("injected calc err: %v", err)
	}
	rHonest, err := Calculate([]PricingEntry{honestA, honestB}, CalcInput{c, "basic"})
	if err != nil {
		t.Fatalf("honest calc err: %v", err)
	}

	if rInjected.Signature == rHonest.Signature {
		t.Fatalf("signature collision via delimiter injection: both %q", rInjected.Signature)
	}
}

// TestCalculate_CarriesSourceIDs verifies every CalcLine in the result
// carries the originating PricingEntry.ID — M4 traceability requirement.
func TestCalculate_CarriesSourceIDs(t *testing.T) {
	c := uuid.New()
	a := mustEntry(t, c, "basic", "aa_fee", 1000, false)
	b := mustEntry(t, c, "basic", "bb_fee", 2000, false)

	res, err := Calculate([]PricingEntry{a, b}, CalcInput{c, "basic"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("lines: want 2, got %d", len(res.Lines))
	}
	// Lines sort by fee_item so aa_fee comes first.
	if res.Lines[0].SourcePricingEntryID != a.ID {
		t.Errorf("line[0] source: want %s, got %s", a.ID, res.Lines[0].SourcePricingEntryID)
	}
	if res.Lines[1].SourcePricingEntryID != b.ID {
		t.Errorf("line[1] source: want %s, got %s", b.ID, res.Lines[1].SourcePricingEntryID)
	}
}
