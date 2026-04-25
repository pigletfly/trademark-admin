package quotation

import (
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
