package quotation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// computeAdjustSignature produces a deterministic SHA-256 hex digest
// over a canonical representation of the snapshot lines.
//
// Why it exists: pricing.Calculate produces its own signature for the
// Submit path (incorporates country_id, service_tier, and sorted lines).
// Adjust does not flow through pricing — the reviewer hand-edits
// amounts — so we cannot reuse the pricing signature. This helper gives
// Adjust a tamper-detection signature of its own, over the same
// canonical form (sorted by fee_item, amount as integer cents, semicolon
// separators) so the JSONB column is always self-verifiable.
//
// The resulting 64-char hex matches pricing's format, so any later
// verification tool can treat both kinds of signatures uniformly.
func computeAdjustSignature(lines []SnapshotLine) string {
	// Sort a defensive copy — caller may hold a reference they don't
	// expect us to mutate.
	sorted := make([]SnapshotLine, len(lines))
	copy(sorted, lines)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].FeeItem < sorted[j].FeeItem
	})

	var b strings.Builder
	for _, l := range sorted {
		fmt.Fprintf(&b, "%s:%d;", l.FeeItem, l.AmountCNYCents)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func computeMethodSnapshotSignature(niceCategoryCount int, lines []SnapshotLine, total int64) string {
	sortedLines := make([]SnapshotLine, len(lines))
	copy(sortedLines, lines)
	sort.Slice(sortedLines, func(i, j int) bool {
		if sortedLines[i].RegistrationMethod != sortedLines[j].RegistrationMethod {
			return sortedLines[i].RegistrationMethod < sortedLines[j].RegistrationMethod
		}
		leftCountryID, rightCountryID := "", ""
		if sortedLines[i].CountryID != nil {
			leftCountryID = sortedLines[i].CountryID.String()
		}
		if sortedLines[j].CountryID != nil {
			rightCountryID = sortedLines[j].CountryID.String()
		}
		if leftCountryID != rightCountryID {
			return leftCountryID < rightCountryID
		}
		if sortedLines[i].CountryArea != sortedLines[j].CountryArea {
			return sortedLines[i].CountryArea < sortedLines[j].CountryArea
		}
		if sortedLines[i].FeeItem != sortedLines[j].FeeItem {
			return sortedLines[i].FeeItem < sortedLines[j].FeeItem
		}
		return sortedLines[i].AmountCNYCents < sortedLines[j].AmountCNYCents
	})

	var b strings.Builder
	fmt.Fprintf(&b, "method-v2|classes=%d|", niceCategoryCount)
	for _, line := range sortedLines {
		countryID := ""
		if line.CountryID != nil {
			countryID = line.CountryID.String()
		}
		sourceEntryID := ""
		if line.SourcePricingEntryID != nil {
			sourceEntryID = line.SourcePricingEntryID.String()
		}
		sourcePricingID := ""
		if line.SourcePricingID != nil {
			sourcePricingID = line.SourcePricingID.String()
		}
		unit := int64(0)
		if line.UnitAmountCNYCents != nil {
			unit = *line.UnitAmountCNYCents
		}
		chf := int64(0)
		if line.OfficialFeeCHFCents != nil {
			chf = *line.OfficialFeeCHFCents
		}
		fmt.Fprintf(
			&b,
			"%s|%s|%s|%s|%s|%s|%d:%s|qty=%d|unit=%d|chf=%d|amount=%d;",
			line.RegistrationMethod,
			countryID,
			line.CountryArea,
			sourceEntryID,
			line.SourcePricingTable,
			sourcePricingID,
			len(line.FeeItem),
			line.FeeItem,
			line.Quantity,
			unit,
			chf,
			line.AmountCNYCents,
		)
	}
	fmt.Fprintf(&b, "=%d", total)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func computeQuotationSignature(countryIDs []uuid.UUID, serviceTier string, lines []SnapshotLine, total int64) string {
	sortedCountries := make([]uuid.UUID, len(countryIDs))
	copy(sortedCountries, countryIDs)
	sort.Slice(sortedCountries, func(i, j int) bool {
		return sortedCountries[i].String() < sortedCountries[j].String()
	})

	sortedLines := make([]SnapshotLine, len(lines))
	copy(sortedLines, lines)
	sort.Slice(sortedLines, func(i, j int) bool {
		if sortedLines[i].FeeItem != sortedLines[j].FeeItem {
			return sortedLines[i].FeeItem < sortedLines[j].FeeItem
		}
		leftSource, rightSource := "", ""
		if sortedLines[i].SourcePricingEntryID != nil {
			leftSource = sortedLines[i].SourcePricingEntryID.String()
		}
		if sortedLines[j].SourcePricingEntryID != nil {
			rightSource = sortedLines[j].SourcePricingEntryID.String()
		}
		if leftSource != rightSource {
			return leftSource < rightSource
		}
		return sortedLines[i].AmountCNYCents < sortedLines[j].AmountCNYCents
	})

	var b strings.Builder
	b.WriteString("quotation-v1|")
	for _, id := range sortedCountries {
		fmt.Fprintf(&b, "%s;", id)
	}
	fmt.Fprintf(&b, "|%s|", serviceTier)
	for _, line := range sortedLines {
		source := ""
		if line.SourcePricingEntryID != nil {
			source = line.SourcePricingEntryID.String()
		}
		fmt.Fprintf(&b, "%d:%s:%s=%d;", len(line.FeeItem), line.FeeItem, source, line.AmountCNYCents)
	}
	fmt.Fprintf(&b, "=%d", total)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
