package quotation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
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
