package export

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Storage writes generated files to disk under a per-quotation layout:
//   <root>/quotations/<quotation_id>/<timestamp>-<format>-<language>.<ext>
// and returns the path + size + sha256 so the caller can persist metadata.
type Storage struct{ root string }

func NewStorage(root string) *Storage { return &Storage{root: root} }

// StoredFile describes a file freshly written to disk.
type StoredFile struct {
	Path   string
	Size   int64
	SHA256 string
}

// Write streams content into a fresh file under the storage root. The
// caller provides format+language+quotationID to derive the filename;
// Storage is oblivious to the content.
func (s *Storage) Write(
	quotationID uuid.UUID,
	format Format,
	language Language,
	content io.Reader,
) (StoredFile, error) {
	dir := filepath.Join(s.root, "quotations", quotationID.String())
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return StoredFile{}, fmt.Errorf("export: mkdir %s: %w", dir, err)
	}
	name := fmt.Sprintf("%s-%s-%s.%s",
		time.Now().UTC().Format("20060102T150405.000Z"),
		format, language, format,
	)
	full := filepath.Join(dir, name)
	f, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return StoredFile{}, fmt.Errorf("export: create %s: %w", full, err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), content)
	if err != nil {
		_ = os.Remove(full)
		return StoredFile{}, fmt.Errorf("export: write: %w", err)
	}
	return StoredFile{
		Path:   full,
		Size:   size,
		SHA256: hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// Open returns a reader for an existing file. Caller must close.
// Refuses paths that resolve outside the storage root (defence in depth
// against a maliciously stored file_path row in the DB).
func (s *Storage) Open(path string) (*os.File, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil || rel == ".." || (len(rel) >= 3 && rel[:3] == "../") {
		return nil, fmt.Errorf("export: path %q escapes storage root", path)
	}
	return os.Open(abs)
}
