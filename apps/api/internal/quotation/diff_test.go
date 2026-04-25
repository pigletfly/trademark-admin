package quotation

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestComputeSnapshotDiff_ChangesAndTotals verifies computeSnapshotDiff
// reports added, removed, and updated lines plus the before/after totals
// in a form serialisable to the JSONB history column. The test checks
// for presence of specific field fragments in the marshalled JSON so
// the shape is pinned without over-fitting to field ordering.
func TestComputeSnapshotDiff_ChangesAndTotals(t *testing.T) {
	prev := Snapshot{Lines: []SnapshotLine{
		{FeeItem: "A", AmountCNYCents: 100},
		{FeeItem: "B", AmountCNYCents: 200},
	}, TotalCNYCents: 300}
	next := Snapshot{Lines: []SnapshotLine{
		{FeeItem: "A", AmountCNYCents: 150}, // changed
		{FeeItem: "C", AmountCNYCents: 50},  // added; B removed
	}, TotalCNYCents: 200}

	diff := computeSnapshotDiff(prev, next)
	b, err := json.Marshal(diff)
	if err != nil {
		t.Fatalf("marshal diff: %v", err)
	}
	// Presence checks — exact format can evolve.
	for _, want := range []string{
		`"fee_item":"A"`, `"before":100`, `"after":150`, // update
		`"fee_item":"C"`, `"after":50`, // add
		`"fee_item":"B"`, `"before":200`, // remove
		`"total_before":300`, `"total_after":200`,
	} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("diff JSON missing %q — got: %s", want, b)
		}
	}
}
