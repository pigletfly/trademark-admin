package quotation

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository is the GORM-backed persistence layer.
type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, q *Quotation) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(q).Error
}

// Get returns the quotation or nil + nil err when not found.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Quotation, error) {
	var q Quotation
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&q).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// UpdateDraft applies a map of column updates. Caller guarantees status == draft.
func (r *Repository) UpdateDraft(ctx context.Context, id uuid.UUID, patch map[string]any) error {
	res := r.db.WithContext(ctx).
		Model(&Quotation{}).
		Where("id = ? AND status = ?", id, StatusDraft).
		Updates(patch)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrInvalidTransition
	}
	return nil
}

// Transition updates the quotation row and appends a StatusHistory row
// in a single transaction. `q` is assumed to already reflect the target
// state on all snapshot/reviewer fields — Transition only writes status
// itself plus the history row.
func (r *Repository) Transition(
	ctx context.Context,
	q *Quotation,
	to Status,
	actorID uuid.UUID,
	comment *string,
) error {
	from := q.Status
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update the main row with status + any set snapshot/reviewer fields.
		updates := map[string]any{
			"status":     to,
			"updated_at": time.Now(),
		}
		if q.SnapshotJSON != nil {
			updates["snapshot_json"] = q.SnapshotJSON
		}
		if q.TotalCNYCents != nil {
			updates["total_cny_cents"] = *q.TotalCNYCents
		}
		if q.Signature != nil {
			updates["signature"] = *q.Signature
		}
		if q.SubmittedAt != nil {
			updates["submitted_at"] = *q.SubmittedAt
		}
		if q.ReviewedAt != nil {
			updates["reviewed_at"] = *q.ReviewedAt
		}
		if q.ReviewedBy != nil {
			updates["reviewed_by"] = *q.ReviewedBy
		}
		if q.ReviewComment != nil {
			updates["review_comment"] = *q.ReviewComment
		}
		if err := tx.Model(&Quotation{}).
			Where("id = ? AND status = ?", q.ID, from).
			Updates(updates).Error; err != nil {
			return err
		}
		// Append the history row.
		h := StatusHistory{
			ID: uuid.New(), QuotationID: q.ID,
			FromStatus: from, ToStatus: to,
			ActorID: &actorID, Comment: comment,
			At: time.Now(),
		}
		return tx.Create(&h).Error
	})
}

// List returns quotations matching the filter plus the total count.
func (r *Repository) List(ctx context.Context, f ListFilter) ([]Quotation, int64, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 200 {
		f.PageSize = 20
	}
	q := r.db.WithContext(ctx).Model(&Quotation{})
	if f.OwnerID != nil {
		q = q.Where("created_by = ?", *f.OwnerID)
	}
	if f.CustomerID != nil {
		q = q.Where("customer_id = ?", *f.CustomerID)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []Quotation
	err := q.Order("created_at DESC").
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		Find(&rows).Error
	return rows, total, err
}

// History returns the append-only transition log, oldest first.
func (r *Repository) History(ctx context.Context, id uuid.UUID) ([]StatusHistory, error) {
	var rows []StatusHistory
	err := r.db.WithContext(ctx).
		Where("quotation_id = ?", id).
		Order("at ASC").
		Find(&rows).Error
	return rows, err
}

// SoftDelete sets deleted_at. Only draft quotations should be deletable
// — that rule lives at the service layer.
func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Quotation{}, "id = ?", id).Error
}
