package pricing

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
)

// CalcInput feeds Calculate. At MVP we price per (country, tier); future
// plans add multi-country aggregation at the quotation layer.
type CalcInput struct {
	CountryID   uuid.UUID `json:"country_id"`
	ServiceTier string    `json:"service_tier"`
}

// CalcLine is one fee item included in the total. SourcePricingEntryID
// is the ID of the PricingEntry row this line was derived from — lets
// downstream snapshot consumers trace the line back for audit.
type CalcLine struct {
	FeeItem              string     `json:"fee_item"`
	AmountCNYCents       int64      `json:"amount_cny_cents"`
	SourcePricingEntryID uuid.UUID  `json:"source_pricing_entry_id"`
	SourcePricingTable   string     `json:"source_pricing_table,omitempty"`
	SourcePricingID      *uuid.UUID `json:"source_pricing_id,omitempty"`
	RegistrationMethod   string     `json:"registration_method,omitempty"`
	CountryID            *uuid.UUID `json:"country_id,omitempty"`
	CountryArea          string     `json:"country_area,omitempty"`
	Quantity             int        `json:"quantity,omitempty"`
	UnitAmountCNYCents   *int64     `json:"unit_amount_cny_cents,omitempty"`
	OfficialFeeCHFCents  *int64     `json:"official_fee_chf_cents,omitempty"`
}

// CalcResult is the deterministic output of Calculate.
type CalcResult struct {
	Lines         []CalcLine `json:"lines"`
	TotalCNYCents int64      `json:"total_cny_cents"`
	// Signature is a SHA-256 over input + sorted lines + total — lets the
	// quotation layer detect tampering when it re-calls the engine later.
	Signature string `json:"signature"`
}

// ErrNoMatchingEntries is returned when the filtered slice produces no
// lines — usually the country/tier combination is unpriced.
var ErrNoMatchingEntries = errors.New("pricing: no active entries for input")

// Calculate deterministically reduces entries → CalcResult.
//
// Rules:
//   - Only active entries (effective_to == nil) matching input are used.
//   - Lines are sorted by fee_item ascending so signature is stable.
//   - Total is simple sum — no discounts / promos at MVP.
//
// Calculate does NOT touch the DB. Callers fetch entries (usually via
// repo.ListActive with a CountryID filter) and hand the slice in.
func Calculate(entries []PricingEntry, input CalcInput) (CalcResult, error) {
	if !IsValidServiceTier(input.ServiceTier) {
		return CalcResult{}, ErrInvalidTier
	}
	var lines []CalcLine
	var total int64
	for _, e := range entries {
		if e.EffectiveTo != nil {
			continue
		}
		if e.CountryID != input.CountryID {
			continue
		}
		if e.ServiceTier != input.ServiceTier {
			continue
		}
		lines = append(lines, CalcLine{
			FeeItem:              e.FeeItem,
			AmountCNYCents:       e.AmountCNYCents,
			SourcePricingEntryID: e.ID,
		})
		total += e.AmountCNYCents
	}
	if len(lines) == 0 {
		return CalcResult{}, ErrNoMatchingEntries
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].FeeItem < lines[j].FeeItem })
	sig := signature(input, lines, total)
	return CalcResult{Lines: lines, TotalCNYCents: total, Signature: sig}, nil
}

func signature(in CalcInput, lines []CalcLine, total int64) string {
	// Length-prefix every variable-width field so a malicious fee_item
	// value like "a=1;b" cannot forge a collision with a two-line set.
	// Format: v2|<ctry>|<tier>|<len>:<fee_item>=<cents>;...|=<total>
	h := sha256.New()
	fmt.Fprintf(h, "v2|%s|%s|", in.CountryID, in.ServiceTier)
	for _, l := range lines {
		fmt.Fprintf(h, "%d:%s=%d;", len(l.FeeItem), l.FeeItem, l.AmountCNYCents)
	}
	fmt.Fprintf(h, "=%d", total)
	return hex.EncodeToString(h.Sum(nil))
}
