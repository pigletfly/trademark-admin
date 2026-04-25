package export_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
)

type fakePDFRenderer struct {
	out   []byte
	err   error
	calls int
}

func (f *fakePDFRenderer) RenderPDF(_ context.Context, _ []byte) ([]byte, error) {
	f.calls++
	return f.out, f.err
}

type fixedClock time.Time

func (c fixedClock) Now() time.Time { return time.Time(c) }

// buildService wires a fresh DB + tempdir storage + fake renderer and
// returns (service, qid, actorID).
func buildService(t *testing.T, pdf export.PDFRenderer, ttl time.Duration) (*export.Service, uuid.UUID, uuid.UUID) {
	t.Helper()
	db := bootPg(t)
	repo := export.NewRepository(db)
	storage := export.NewStorage(t.TempDir())
	svc := export.NewService(repo, storage, pdf, ttl)
	qid, actorID := seedApprovedQuotation(t, db)
	return svc, qid, actorID
}

func TestService_GeneratePDF_Persists(t *testing.T) {
	pdf := &fakePDFRenderer{out: []byte("%PDF-fake")}
	svc, qid, actorID := buildService(t, pdf, 24*time.Hour)

	frozen := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	svc = svc.WithClock(fixedClock(frozen))

	view := baseView()
	view.QuotationID = qid.String()

	f, err := svc.GeneratePDF(context.Background(), view, export.LanguageBilingual, qid, actorID)
	if err != nil {
		t.Fatalf("generate pdf: %v", err)
	}
	if pdf.calls != 1 {
		t.Fatalf("pdf renderer calls: got %d want 1", pdf.calls)
	}
	if f.ID == uuid.Nil {
		t.Fatalf("empty id")
	}
	if f.Format != export.FormatPDF {
		t.Errorf("format: got %q want pdf", f.Format)
	}
	if f.Language != export.LanguageBilingual {
		t.Errorf("language: got %q want bilingual", f.Language)
	}
	wantExpiry := frozen.Add(24 * time.Hour)
	if !f.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("expires_at: got %v want %v", f.ExpiresAt, wantExpiry)
	}

	// Assert row actually landed in DB and sha matches.
	got, err := svc.Repo().Get(context.Background(), f.ID)
	if err != nil {
		t.Fatalf("repo get: %v", err)
	}
	if got.SHA256 != f.SHA256 {
		t.Errorf("sha mismatch: db=%s returned=%s", got.SHA256, f.SHA256)
	}

	// Assert file actually on disk.
	if _, err := os.Stat(f.FilePath); err != nil {
		t.Errorf("file not written: %v", err)
	}
}

func TestService_GenerateDOCX_Persists(t *testing.T) {
	svc, qid, actorID := buildService(t, &fakePDFRenderer{}, time.Hour)

	view := baseView()
	view.QuotationID = qid.String()

	f, err := svc.GenerateDOCX(context.Background(), view, export.LanguageBilingual, qid, actorID)
	if err != nil {
		t.Fatalf("generate docx: %v", err)
	}
	if f.Format != export.FormatDOCX {
		t.Errorf("format: got %q want docx", f.Format)
	}
	if _, err := os.Stat(f.FilePath); err != nil {
		t.Errorf("file not written: %v", err)
	}

	// DOCX has a ZIP magic header (PK\x03\x04).
	data, err := os.ReadFile(f.FilePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if len(data) < 4 || string(data[:2]) != "PK" {
		t.Errorf("does not look like a .docx ZIP; first bytes=%q", string(data[:min(len(data), 8)]))
	}
}

func TestService_GeneratePDF_RenderError_NoPersistence(t *testing.T) {
	pdf := &fakePDFRenderer{err: errors.New("boom")}
	svc, qid, actorID := buildService(t, pdf, time.Hour)

	view := baseView()
	view.QuotationID = qid.String()

	_, err := svc.GeneratePDF(context.Background(), view, export.LanguageBilingual, qid, actorID)
	if err == nil {
		t.Fatal("expected render error")
	}

	// No row should have been inserted.
	rows, err := svc.Repo().ByQuotation(context.Background(), qid, 10)
	if err != nil {
		t.Fatalf("by quotation: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}
