package quotation

import (
	"encoding/json"
	"testing"
)

func TestComputeAdjustSignature_Deterministic(t *testing.T) {
	lines := []SnapshotLine{
		{FeeItem: "Agent fee", AmountCNYCents: 120000},
		{FeeItem: "Application fee", AmountCNYCents: 30000},
	}
	a := computeAdjustSignature(lines)
	b := computeAdjustSignature(lines)
	if a != b {
		t.Fatalf("not deterministic: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("signature length %d, want 64 (sha256 hex)", len(a))
	}
}

func TestComputeAdjustSignature_OrderIndependent(t *testing.T) {
	// Same items, different caller order should produce the same signature:
	// Adjust signs the CANONICAL (sorted) representation so two reviewers
	// submitting the same edit in different orders agree.
	a := computeAdjustSignature([]SnapshotLine{
		{FeeItem: "A", AmountCNYCents: 100},
		{FeeItem: "B", AmountCNYCents: 200},
	})
	b := computeAdjustSignature([]SnapshotLine{
		{FeeItem: "B", AmountCNYCents: 200},
		{FeeItem: "A", AmountCNYCents: 100},
	})
	if a != b {
		t.Fatalf("signature must be order-independent, got %s vs %s", a, b)
	}
}

func TestComputeAdjustSignature_AmountMatters(t *testing.T) {
	a := computeAdjustSignature([]SnapshotLine{
		{FeeItem: "A", AmountCNYCents: 100},
	})
	b := computeAdjustSignature([]SnapshotLine{
		{FeeItem: "A", AmountCNYCents: 101},
	})
	if a == b {
		t.Fatalf("signature must differ when amount differs")
	}
}

func TestComputeAdjustSignature_FeeItemMatters(t *testing.T) {
	a := computeAdjustSignature([]SnapshotLine{
		{FeeItem: "A", AmountCNYCents: 100},
	})
	b := computeAdjustSignature([]SnapshotLine{
		{FeeItem: "B", AmountCNYCents: 100},
	})
	if a == b {
		t.Fatalf("signature must differ when fee item differs")
	}
}

func TestComputeAdjustSignature_EmptyLines(t *testing.T) {
	sig := computeAdjustSignature(nil)
	if len(sig) != 64 {
		t.Fatalf("empty input should still produce a valid sha256 hex signature, got len %d", len(sig))
	}
}

// TestDecodeLegacySnapshot_SourceNil verifies that a snapshot JSONB
// blob written before M4 (missing the source_pricing_entry_id key)
// decodes with Lines[i].SourcePricingEntryID == nil, without error.
// This is the behavior json.Unmarshal gives us for free on a pointer
// field tagged with omitempty — this test locks it against future
// regressions (e.g. if someone adds a required tag or a custom
// UnmarshalJSON).
func TestDecodeLegacySnapshot_SourceNil(t *testing.T) {
	legacy := []byte(`{"lines":[{"fee_item":"application","amount_cny_cents":10000}],"total_cny_cents":10000,"signature":"abc"}`)
	var s Snapshot
	if err := json.Unmarshal(legacy, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s.Lines) != 1 {
		t.Fatalf("lines: want 1, got %d", len(s.Lines))
	}
	if s.Lines[0].SourcePricingEntryID != nil {
		t.Errorf("legacy source: want nil, got %v", s.Lines[0].SourcePricingEntryID)
	}
	if s.Lines[0].FeeItem != "application" {
		t.Errorf("fee_item: want application, got %s", s.Lines[0].FeeItem)
	}
}
