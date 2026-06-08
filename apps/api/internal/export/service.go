package export

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PDFRenderer is the interface Service needs to produce PDF bytes from
// HTML. Real impl: *Gotenberg. Tests can swap in a fake that returns
// deterministic bytes.
type PDFRenderer interface {
	RenderPDF(ctx context.Context, html []byte) ([]byte, error)
}

// Clock is injectable for deterministic expires_at in tests.
type Clock interface{ Now() time.Time }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Service orchestrates the full export pipeline:
// view → render HTML/DOCX → write to Storage → record in Repository.
type Service struct {
	repo        *Repository
	storage     *Storage
	pdfRenderer PDFRenderer
	ttl         time.Duration
	clock       Clock
}

func NewService(
	repo *Repository,
	storage *Storage,
	pdf PDFRenderer,
	ttl time.Duration,
) *Service {
	return &Service{repo: repo, storage: storage, pdfRenderer: pdf, ttl: ttl, clock: realClock{}}
}

// WithClock returns a shallow copy whose Now() is pluggable. Tests use
// this to control expires_at; production callers never touch it.
func (s *Service) WithClock(c Clock) *Service {
	cp := *s
	cp.clock = c
	return &cp
}

// Repo exposes the underlying repository so the handler can look up
// export-file metadata for downloads without adding a passthrough
// method for every read.
func (s *Service) Repo() *Repository { return s.repo }

// Storage exposes the file-system layer so the handler can stream
// file bytes without the service in between.
func (s *Service) Storage() *Storage { return s.storage }

// GeneratePDF renders a PDF of the quotation in the requested language,
// writes it to storage, and records the metadata. Returns the persisted row.
func (s *Service) GeneratePDF(
	ctx context.Context,
	view QuotationView,
	lang Language,
	qid, actorID uuid.UUID,
) (*ExportFile, error) {
	html, err := RenderHTML(view, lang)
	if err != nil {
		return nil, err
	}
	pdf, err := s.pdfRenderer.RenderPDF(ctx, html)
	if err != nil {
		return nil, err
	}
	return s.writeAndRecord(ctx, qid, actorID, FormatPDF, lang, bytes.NewReader(pdf))
}

// GenerateDOCX renders a .docx (reusing RenderDOCX from docx.go), writes
// it to storage, and records metadata. The language argument is recorded
// on the row but docx rendering is bilingual-only today.
func (s *Service) GenerateDOCX(
	ctx context.Context,
	view QuotationView,
	lang Language,
	qid, actorID uuid.UUID,
) (*ExportFile, error) {
	var buf bytes.Buffer
	if err := RenderDOCX(&buf, view); err != nil {
		return nil, err
	}
	return s.writeAndRecord(ctx, qid, actorID, FormatDOCX, lang, bytes.NewReader(buf.Bytes()))
}

// GenerateXLSX renders a spreadsheet workbook, writes it to storage,
// and records metadata.
func (s *Service) GenerateXLSX(
	ctx context.Context,
	view QuotationView,
	lang Language,
	qid, actorID uuid.UUID,
) (*ExportFile, error) {
	xlsx, err := RenderXLSX(view)
	if err != nil {
		return nil, err
	}
	return s.writeAndRecord(ctx, qid, actorID, FormatXLSX, lang, bytes.NewReader(xlsx))
}

// writeAndRecord is the common tail of GeneratePDF/GenerateDOCX.
func (s *Service) writeAndRecord(
	ctx context.Context,
	qid, actorID uuid.UUID,
	format Format,
	lang Language,
	content *bytes.Reader,
) (*ExportFile, error) {
	stored, err := s.storage.Write(qid, format, lang, content)
	if err != nil {
		return nil, err
	}
	f := &ExportFile{
		QuotationID: qid,
		Format:      format,
		Language:    lang,
		FilePath:    stored.Path,
		FileSize:    stored.Size,
		SHA256:      stored.SHA256,
		ExpiresAt:   s.clock.Now().Add(s.ttl),
		CreatedBy:   actorID,
	}
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, fmt.Errorf("export: record: %w", err)
	}
	return f, nil
}
