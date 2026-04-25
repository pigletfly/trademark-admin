package quotation

// SnapshotDiff summarizes what changed between two snapshots. Marshaled
// into the JSONB quotation_status_history.diff_json column so reviewers
// and auditors can see precisely what an Adjust altered without having
// to load and compare two full snapshot blobs.
//
// All three line buckets use SnapshotLineDelta so the downstream shape
// is uniform: added lines carry Before=0, removed lines carry After=0,
// updated lines carry both. This is stricter than raw SnapshotLine and
// makes the diff self-describing when rendered in the UI.
type SnapshotDiff struct {
	LinesAdded   []SnapshotLineDelta `json:"lines_added,omitempty"`
	LinesRemoved []SnapshotLineDelta `json:"lines_removed,omitempty"`
	LinesUpdated []SnapshotLineDelta `json:"lines_updated,omitempty"`
	TotalBefore  int64               `json:"total_before"`
	TotalAfter   int64               `json:"total_after"`
}

// SnapshotLineDelta is one fee item's movement between two snapshots.
// FeeItem is the natural key; Before and After are the raw integer
// cents. For added lines Before is zero; for removed lines After is
// zero; for updated lines both are non-zero and differ.
type SnapshotLineDelta struct {
	FeeItem string `json:"fee_item"`
	Before  int64  `json:"before"`
	After   int64  `json:"after"`
}

// computeSnapshotDiff produces a structured diff of two snapshots.
// Matching is by FeeItem (the natural key for a snapshot line): items
// present in `next` but not `prev` are added, items in `prev` but not
// `next` are removed, and items in both with differing AmountCNYCents
// are updated. Totals are copied from each snapshot verbatim — we do
// NOT recompute them here because the caller owns the authoritative
// total and the diff row should reflect exactly what was persisted.
//
// Iteration order follows the input slices (not the internal maps) so
// the resulting JSON is deterministic for a given input pair.
func computeSnapshotDiff(prev, next Snapshot) SnapshotDiff {
	out := SnapshotDiff{TotalBefore: prev.TotalCNYCents, TotalAfter: next.TotalCNYCents}
	prevByItem := make(map[string]int64, len(prev.Lines))
	for _, l := range prev.Lines {
		prevByItem[l.FeeItem] = l.AmountCNYCents
	}
	nextByItem := make(map[string]int64, len(next.Lines))
	for _, l := range next.Lines {
		nextByItem[l.FeeItem] = l.AmountCNYCents
	}
	for _, l := range next.Lines {
		prevAmt, ok := prevByItem[l.FeeItem]
		if !ok {
			out.LinesAdded = append(out.LinesAdded, SnapshotLineDelta{
				FeeItem: l.FeeItem, Before: 0, After: l.AmountCNYCents,
			})
			continue
		}
		if prevAmt != l.AmountCNYCents {
			out.LinesUpdated = append(out.LinesUpdated, SnapshotLineDelta{
				FeeItem: l.FeeItem, Before: prevAmt, After: l.AmountCNYCents,
			})
		}
	}
	for _, l := range prev.Lines {
		if _, ok := nextByItem[l.FeeItem]; !ok {
			out.LinesRemoved = append(out.LinesRemoved, SnapshotLineDelta{
				FeeItem: l.FeeItem, Before: l.AmountCNYCents, After: 0,
			})
		}
	}
	return out
}
