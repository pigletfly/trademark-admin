package export_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
)

func TestStorage_WriteAndOpen(t *testing.T) {
	s := export.NewStorage(t.TempDir())
	qid := uuid.New()

	stored, err := s.Write(qid, export.FormatPDF, export.LanguageBilingual,
		strings.NewReader("hello-pdf-bytes"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if stored.Size <= 0 {
		t.Fatalf("size zero: %+v", stored)
	}
	if stored.SHA256 == "" {
		t.Fatalf("sha256 empty")
	}

	f, err := s.Open(stored.Path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	buf, _ := io.ReadAll(f)
	if string(buf) != "hello-pdf-bytes" {
		t.Fatalf("content mismatch: %q", string(buf))
	}
}

func TestStorage_Open_PathTraversal(t *testing.T) {
	s := export.NewStorage(t.TempDir())
	_, err := s.Open("/etc/passwd")
	if err == nil {
		t.Fatalf("expected error for traversal")
	}
	if !strings.Contains(err.Error(), "escapes storage root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStorage_Open_InsideRoot_OK(t *testing.T) {
	root := t.TempDir()
	s := export.NewStorage(root)
	qid := uuid.New()

	stored, err := s.Write(qid, export.FormatPDF, export.LanguageZH,
		strings.NewReader("hi"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := s.Open(stored.Path)
	if err != nil {
		t.Fatalf("open valid path: %v", err)
	}
	_ = f.Close()
}

func TestStorage_Write_ContentRoundTrip_Bytes(t *testing.T) {
	s := export.NewStorage(t.TempDir())
	qid := uuid.New()

	payload := bytes.Repeat([]byte("中文 PDF "), 1024) // ~10 KB mixed bytes
	stored, err := s.Write(qid, export.FormatPDF, export.LanguageZH, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	onDisk, err := os.ReadFile(stored.Path)
	if err != nil {
		t.Fatalf("readfile: %v", err)
	}
	if !bytes.Equal(onDisk, payload) {
		t.Fatalf("content differs (len on-disk=%d, len expected=%d)", len(onDisk), len(payload))
	}
	if int64(len(onDisk)) != stored.Size {
		t.Fatalf("size mismatch: reported=%d actual=%d", stored.Size, len(onDisk))
	}
}

func TestStorage_Write_DirectoryLayout(t *testing.T) {
	root := t.TempDir()
	s := export.NewStorage(root)
	qid := uuid.New()

	stored, err := s.Write(qid, export.FormatDOCX, export.LanguageBilingual,
		strings.NewReader("x"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	// Path should be under <root>/quotations/<qid>/<name>.
	expectedPrefix := root + "/quotations/" + qid.String() + "/"
	if !strings.HasPrefix(stored.Path, expectedPrefix) {
		t.Fatalf("unexpected path %q (want prefix %q)", stored.Path, expectedPrefix)
	}
	if !strings.HasSuffix(stored.Path, "-docx-bilingual.docx") {
		t.Fatalf("unexpected filename tail: %q", stored.Path)
	}
}
