package audit_test

import (
	"testing"

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"
)

func TestRedactPassword(t *testing.T) {
	in := []byte(`{"email":"a@b.com","password":"hunter2","remember":true}`)
	out := audit.RedactForTest(in)
	want := `{"email":"a@b.com","password":"[REDACTED]","remember":true}`
	if string(out) != want {
		t.Fatalf("got %q\nwant %q", string(out), want)
	}
}

func TestRedactWhenNoPassword(t *testing.T) {
	in := []byte(`{"email":"a@b.com"}`)
	out := audit.RedactForTest(in)
	if string(out) != string(in) {
		t.Fatalf("body must be unchanged; got %q", out)
	}
}
