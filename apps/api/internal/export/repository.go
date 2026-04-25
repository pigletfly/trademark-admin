package export

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrNotFound signals the requested export file is absent or already expired.
var ErrNotFound = errors.New("export: file not found")

// Repository persists ExportFile rows. Files on disk are written by
// storage.go (added in Task 3); this layer only tracks metadata.
type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// Create inserts one export_files row. Generates ID and CreatedAt if the caller left them zero.
func (r *Repository) Create(ctx context.Context, f *ExportFile) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}
	return r.db.WithContext(ctx).Create(f).Error
}

// Get fetches one export by id. Returns ErrNotFound if missing or expired.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*ExportFile, error) {
	var f ExportFile
	err := r.db.WithContext(ctx).
		Where("id = ? AND expires_at > ?", id, time.Now()).
		Take(&f).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// ByQuotation lists still-valid export records for a quotation,
// newest first. Expired rows are excluded.
func (r *Repository) ByQuotation(ctx context.Context, qid uuid.UUID, limit int) ([]ExportFile, error) {
	if limit <= 0 {
		limit = 10
	}
	var out []ExportFile
	err := r.db.WithContext(ctx).
		Where("quotation_id = ? AND expires_at > ?", qid, time.Now()).
		Order("created_at DESC").
		Limit(limit).
		Find(&out).Error
	return out, err
}
