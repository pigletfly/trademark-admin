package quotation

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"
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

// transitionInTx is the transactional body of Transition. It uses the
// provided tx directly — the CALLER is responsible for the transaction
// envelope and commit/rollback. Used by Transition (which wraps itself
// in a tx) and by SubmitWithSerial / other multi-step flows that need
// to pin several operations to one tx.
func (r *Repository) transitionInTx(
	tx *gorm.DB,
	q *Quotation,
	to Status,
	actorID uuid.UUID,
	comment *string,
	diffJSON []byte,
) error {
	from := q.Status
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
	if q.SerialNo != nil {
		updates["serial_no"] = *q.SerialNo
	}

	res := tx.Model(&Quotation{}).
		Where("id = ? AND status = ?", q.ID, from).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrInvalidTransition
	}
	h := StatusHistory{
		ID: uuid.New(), QuotationID: q.ID,
		FromStatus: from, ToStatus: to,
		ActorID: &actorID, Comment: comment,
		At: time.Now(),
	}
	if len(diffJSON) > 0 {
		h.DiffJSON = audit.JSONB(diffJSON)
	}
	return tx.Create(&h).Error
}

// Transition updates the quotation row and appends a StatusHistory row
// in a single transaction. Preserves the existing signature; new flows
// that need serial generation or diff payloads use other wrappers that
// share transitionInTx.
func (r *Repository) Transition(
	ctx context.Context,
	q *Quotation,
	to Status,
	actorID uuid.UUID,
	comment *string,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.transitionInTx(tx, q, to, actorID, comment, nil)
	})
}

// SubmitWithSerial transitions a draft to submitted while generating
// and persisting the daily serial_no atomically. Calls GenerateSerialAt
// inside the SAME transaction that writes the status change and the
// history row, so the advisory lock + MAX query + UPDATE form one
// serializable unit.
//
// `q` is expected to already carry the fresh snapshot/total/signature/
// submitted_at fields — SubmitWithSerial does not compute them. Passing
// a non-draft q is a programmer error; the WHERE status = 'draft'
// predicate on the inner UPDATE will catch it by returning
// ErrInvalidTransition.
func (r *Repository) SubmitWithSerial(
	ctx context.Context,
	q *Quotation,
	actorID uuid.UUID,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		serial, err := GenerateSerialAt(ctx, tx, now)
		if err != nil {
			return err
		}
		q.SerialNo = &serial
		return r.transitionInTx(tx, q, StatusSubmitted, actorID, nil, nil)
	})
}

// TransitionWithHistory is like Transition but records a structured
// diff_json payload on the history row. Used by Adjust (same-status
// snapshot mutation) and any future transition that needs to capture
// "what changed" in a typed form. Pass nil/empty diffJSON to get
// identical behavior to plain Transition.
func (r *Repository) TransitionWithHistory(
	ctx context.Context,
	q *Quotation,
	to Status,
	actorID uuid.UUID,
	comment *string,
	diffJSON []byte,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.transitionInTx(tx, q, to, actorID, comment, diffJSON)
	})
}

// Withdraw reverts a submitted quotation to draft. It explicitly NULLs
// snapshot_json/total_cny_cents/signature to satisfy
// chk_quotations_snapshot_when_nondraft, and preserves serial_no and
// submitted_at so a later Submit can reuse/overwrite them.
//
// Cannot delegate to transitionInTx: that helper only writes columns
// when the in-memory pointer is non-nil, so it has no explicit NULL
// path. We write Withdraw inline using a map[string]any (GORM's
// Updates(map) writes NULL for map values that are literal nil).
//
// The guarded UPDATE (WHERE id = ? AND status = 'submitted') returns
// ErrInvalidTransition if the row is not in submitted state — same
// concurrency-safety pattern as transitionInTx.
func (r *Repository) Withdraw(ctx context.Context, q *Quotation, actorID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"status":          StatusDraft,
			"updated_at":      time.Now(),
			"snapshot_json":   nil,
			"total_cny_cents": nil,
			"signature":       nil,
		}
		res := tx.Model(&Quotation{}).
			Where("id = ? AND status = ?", q.ID, StatusSubmitted).
			Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInvalidTransition
		}
		h := StatusHistory{
			ID: uuid.New(), QuotationID: q.ID,
			FromStatus: StatusSubmitted, ToStatus: StatusDraft,
			ActorID: &actorID, Comment: nil,
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

// StatusCount is one {status: count} row — used by CountByStatus.
type StatusCount struct {
	Status Status
	Count  int64
}

// CountByStatus groups non-deleted quotations by status. When ownerID
// is non-nil, the scope is narrowed to one creator (salesperson view).
func (r *Repository) CountByStatus(ctx context.Context, ownerID *uuid.UUID) ([]StatusCount, error) {
	q := r.db.WithContext(ctx).Model(&Quotation{})
	if ownerID != nil {
		q = q.Where("created_by = ?", *ownerID)
	}
	var rows []StatusCount
	err := q.Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&rows).Error
	return rows, err
}

// SumApprovedCNYCents returns the total CNY cents across approved
// quotations, optionally scoped to a single creator. Returns 0 when
// there are no approved rows.
func (r *Repository) SumApprovedCNYCents(ctx context.Context, ownerID *uuid.UUID) (int64, error) {
	q := r.db.WithContext(ctx).Model(&Quotation{}).
		Where("status = ?", StatusApproved)
	if ownerID != nil {
		q = q.Where("created_by = ?", *ownerID)
	}
	var total *int64
	if err := q.Select("COALESCE(SUM(total_cny_cents), 0)").Row().Scan(&total); err != nil {
		return 0, err
	}
	if total == nil {
		return 0, nil
	}
	return *total, nil
}

// RecentQuotation is a trimmed row for the activity feed — avoids
// dragging the full snapshot JSON into the dashboard response.
type RecentQuotation struct {
	ID            uuid.UUID
	Status        Status
	ServiceTier   string
	TotalCNYCents *int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Recent returns the most recently updated quotations (newest first).
// Scoped to ownerID when non-nil.
func (r *Repository) Recent(ctx context.Context, ownerID *uuid.UUID, limit int) ([]RecentQuotation, error) {
	if limit <= 0 || limit > 50 {
		limit = 5
	}
	q := r.db.WithContext(ctx).Model(&Quotation{})
	if ownerID != nil {
		q = q.Where("created_by = ?", *ownerID)
	}
	var rows []RecentQuotation
	err := q.Select("id, status, service_tier, total_cny_cents, created_at, updated_at").
		Order("updated_at DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}
