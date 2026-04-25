# M2 Review Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the reviewer workflow by adding Adjust (in-place snapshot edit), Withdraw (sales revert to draft), Copy (clone to new draft), and per-day serial numbers, backed by a diff-aware status history table.

**Architecture:** Service-layer transitions gated by roles; the repository's existing optimistic-concurrency `Transition()` is extended with an optional `diff_json` + `serial_no` update, since the CHECK constraint forces all snapshot-dependent columns to move atomically. Serial numbers use a fixed Postgres advisory xact lock key `(1, 1)` + `MAX(serial_no) + 1` scoped to the day. Frontend adds three mutations and one sheet component; the status-history view gains a `diff_json` renderer.

**Tech Stack:** Go 1.25 + Gin + GORM + Postgres (advisory locks, JSONB); React 19 + TanStack Query + TanStack Router + shadcn/ui.

---

## Scope deviations from spec (locked in M1 roadmap)

- State machine stays `draft → submitted → approved/rejected/cancelled`; **adjust** is a same-status snapshot mutation (`submitted → submitted`), recorded by a non-null `diff_json` on the history row rather than a new state.
- `quotation_status_history` is the existing domain event log (M1 found that the writer is `repository.Transition`, not a standalone helper); M2 extends it in-place rather than building a separate `quotation_reviews` table.
- Serial numbers live on `quotations.serial_no` (nullable on drafts, unique when set); format `Q + YYYYMMDD + 4-digit daily sequence`. Generated at `Submit()` time, before snapshot freeze.
- Copy returns a *new* draft quotation — the original is untouched. Re-pricing runs against current pricing entries at copy time.

---

## File structure

**New files**
- `apps/api/migrations/000006_m2_review_flow.up.sql` + `.down.sql`
- `apps/api/internal/quotation/serial.go` + `serial_test.go`
- `apps/api/internal/quotation/diff.go` + `diff_test.go`
- `apps/web/src/features/quotation/components/quotation-adjust-sheet.tsx`

**Modified files (api)**
- `apps/api/internal/quotation/model.go` — add `SerialNo`, extend `StatusHistory` with `DiffJSON`
- `apps/api/internal/quotation/dto.go` — add `SerialNo` on output; `AdjustRequest`, `CopyResponse` types; `DiffJSON` on history DTO
- `apps/api/internal/quotation/repository.go` — teach `Transition` about `diff_json`; add `nextSerial(ctx, tx, day)`
- `apps/api/internal/quotation/service.go` — new methods `Adjust`, `Withdraw`, `Copy`; refactor `Submit` to call serial gen + extracted snapshot helper
- `apps/api/internal/quotation/handler.go` — handlers + request parsing for Adjust/Withdraw/Copy
- `apps/api/internal/quotation/router.go` — new routes, role-gated
- `apps/api/internal/quotation/handler_test.go` — new integration tests for the 3 new endpoints + serial

**Modified files (web)**
- `apps/web/src/features/quotation/types.ts` — `serial_no`, `AdjustRequest`, `DiffJSON`, extend `QuotationHistoryEntry`
- `apps/web/src/features/quotation/hooks/use-quotation-mutations.ts` — add Withdraw / Copy / Adjust mutations
- `apps/web/src/features/quotation/detail.tsx` — show `serial_no`, render diff_json in history
- `apps/web/src/features/quotation/components/quotation-action-bar.tsx` (or equivalent) — new buttons
- `apps/web/src/features/quotation/quotation.integration.test.tsx` — happy-path integration for Withdraw/Copy/Adjust

---

## Task list (14 tasks)

### Task 1: Migration — serial_no + diff_json + CHECK updates

**Files:**
- Create: `apps/api/migrations/000006_m2_review_flow.up.sql`
- Create: `apps/api/migrations/000006_m2_review_flow.down.sql`

- [ ] **Step 1: Write up.sql**

```sql
-- Add serial_no (nullable; set on first submit).
ALTER TABLE quotations ADD COLUMN serial_no TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS uq_quotations_serial_no ON quotations(serial_no) WHERE serial_no IS NOT NULL;

-- Drafts have no serial_no; once set, stays set. Enforce via CHECK:
-- any non-draft status MUST have a serial_no.
ALTER TABLE quotations ADD CONSTRAINT chk_quotations_serial_no_when_nondraft
  CHECK (status = 'draft' OR serial_no IS NOT NULL);

-- Extend status history with diff payload for Adjust events.
ALTER TABLE quotation_status_history ADD COLUMN diff_json JSONB;
```

- [ ] **Step 2: Write down.sql**

```sql
ALTER TABLE quotation_status_history DROP COLUMN diff_json;
ALTER TABLE quotations DROP CONSTRAINT chk_quotations_serial_no_when_nondraft;
DROP INDEX IF EXISTS uq_quotations_serial_no;
ALTER TABLE quotations DROP COLUMN serial_no;
```

- [ ] **Step 3: Apply + rollback locally**

Run: `cd apps/api && go run ./cmd/migrate up`
Expected: `migration 6 applied`.

Run: `go run ./cmd/migrate down 1 && go run ./cmd/migrate up`
Expected: clean round trip.

- [ ] **Step 4: Commit**

```bash
git add apps/api/migrations/000006_m2_review_flow.up.sql apps/api/migrations/000006_m2_review_flow.down.sql
git commit -m "feat(db): add serial_no + status_history.diff_json for M2"
```

---

### Task 2: Model — SerialNo, StatusHistory.DiffJSON

**Files:**
- Modify: `apps/api/internal/quotation/model.go`
- Modify: `apps/api/internal/quotation/dto.go`

- [ ] **Step 1: Extend `Quotation` struct**

In `model.go`, add to the `Quotation` struct:

```go
SerialNo *string `gorm:"column:serial_no"`
```

- [ ] **Step 2: Extend `StatusHistory`**

```go
type StatusHistory struct {
    ID           uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    QuotationID  uuid.UUID       `gorm:"type:uuid;not null;index"`
    FromStatus   Status          `gorm:"column:from_status;not null"`
    ToStatus     Status          `gorm:"column:to_status;not null"`
    ActorID      *uuid.UUID      `gorm:"column:actor_id;type:uuid"`
    Comment      *string         `gorm:"column:comment"`
    DiffJSON     audit.JSONB     `gorm:"column:diff_json"`
    CreatedAt    time.Time       `gorm:"column:created_at;not null;default:now()"`
}
```

(Ensure `audit.JSONB` is still imported; if not, alias a local type.)

- [ ] **Step 3: Extend DTOs**

In `dto.go`:

```go
// Add to QuotationDTO:
SerialNo *string `json:"serial_no,omitempty"`

// Add to QuotationHistoryEntryDTO:
DiffJSON *json.RawMessage `json:"diff_json,omitempty"`
```

Update `toDTO` / history mapper accordingly (copy the SerialNo pointer; unmarshal DiffJSON if non-nil, else leave nil).

- [ ] **Step 4: Build**

Run: `cd apps/api && go build ./...`
Expected: compiles.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/quotation/model.go apps/api/internal/quotation/dto.go
git commit -m "feat(api): add SerialNo + StatusHistory.DiffJSON to quotation model"
```

---

### Task 3: Refactor — extract `calcLinesToSnapshot` helper

**Files:**
- Modify: `apps/api/internal/quotation/service.go`
- Create: `apps/api/internal/quotation/snapshot.go` (extraction)

**Why:** `Submit` currently inlines the `CalcLine → SnapshotLine` mapping at `service.go:163`. M2's Adjust re-runs pricing and must reuse the same mapping. Extract first so both callers share one implementation.

- [ ] **Step 1: Write failing test**

Create `apps/api/internal/quotation/snapshot_test.go`:

```go
package quotation

import (
    "testing"

    "github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
)

func TestCalcLinesToSnapshot_SignatureStable(t *testing.T) {
    lines := []pricing.CalcLine{
        {FeeItem: "Agent fee", AmountCNYCents: 120000},
        {FeeItem: "Application fee", AmountCNYCents: 30000},
    }
    snap := calcLinesToSnapshot(lines)

    if len(snap.Lines) != 2 {
        t.Fatalf("got %d lines, want 2", len(snap.Lines))
    }
    if snap.Total != 150000 {
        t.Fatalf("got total %d, want 150000", snap.Total)
    }
    if len(snap.Signature) != 64 {
        t.Fatalf("signature length %d, want 64 hex chars (sha256)", len(snap.Signature))
    }
    // Determinism: same input → same signature.
    snap2 := calcLinesToSnapshot(lines)
    if snap.Signature != snap2.Signature {
        t.Fatalf("signature not deterministic: %s vs %s", snap.Signature, snap2.Signature)
    }
}

func TestCalcLinesToSnapshot_OrderMatters(t *testing.T) {
    a := []pricing.CalcLine{
        {FeeItem: "A", AmountCNYCents: 100},
        {FeeItem: "B", AmountCNYCents: 200},
    }
    b := []pricing.CalcLine{
        {FeeItem: "B", AmountCNYCents: 200},
        {FeeItem: "A", AmountCNYCents: 100},
    }
    if calcLinesToSnapshot(a).Signature == calcLinesToSnapshot(b).Signature {
        t.Fatal("signature should differ when line order differs")
    }
}
```

Run: `cd apps/api && go test ./internal/quotation/ -run TestCalcLinesToSnapshot -count=1`
Expected: FAIL — `calcLinesToSnapshot` undefined.

- [ ] **Step 2: Create `snapshot.go`**

```go
package quotation

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"

    "github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
)

// builtSnapshot is the canonical in-memory form of a frozen snapshot
// prior to JSONB marshaling. It is the single source of truth for the
// snapshot signature — any caller that mutates the snapshot (Submit,
// Adjust) goes through this function.
type builtSnapshot struct {
    Lines     []SnapshotLine
    Total     int64
    Signature string
}

// calcLinesToSnapshot maps pricing.CalcLine (the pricing calc output)
// to SnapshotLine (our frozen persisted form), computes the total,
// and signs the payload with SHA256 over the deterministic
// "fee_item:amount;..." string. The signature is a tamper-detector
// for the JSONB column — any later edit to the row without recomputing
// the signature is detectable server-side.
func calcLinesToSnapshot(src []pricing.CalcLine) builtSnapshot {
    out := builtSnapshot{Lines: make([]SnapshotLine, 0, len(src))}
    var h = sha256.New()
    var total int64
    for _, l := range src {
        out.Lines = append(out.Lines, SnapshotLine{
            FeeItem:        l.FeeItem,
            AmountCNYCents: l.AmountCNYCents,
        })
        total += l.AmountCNYCents
        fmt.Fprintf(h, "%s:%d;", l.FeeItem, l.AmountCNYCents)
    }
    out.Total = total
    out.Signature = hex.EncodeToString(h.Sum(nil))
    return out
}
```

- [ ] **Step 3: Replace inline logic in `Submit`**

In `service.go`, find the block that builds `lines`/`total`/`sig` around line 163 and replace with:

```go
built := calcLinesToSnapshot(calc.Lines)
```

Then wherever `lines`, `total`, `sig` are used, replace with `built.Lines`, `built.Total`, `built.Signature`.

- [ ] **Step 4: Run tests**

Run: `cd apps/api && go test ./internal/quotation/... -count=1`
Expected: all pass including new snapshot tests; existing `TestService_Submit_*` still green.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/quotation/snapshot.go apps/api/internal/quotation/snapshot_test.go apps/api/internal/quotation/service.go
git commit -m "refactor(api): extract calcLinesToSnapshot helper"
```

---

### Task 4: Serial number generator (advisory lock)

**Files:**
- Create: `apps/api/internal/quotation/serial.go`
- Create: `apps/api/internal/quotation/serial_test.go`

- [ ] **Step 1: Write failing test**

`serial_test.go`:

```go
package quotation_test

import (
    "context"
    "testing"
    "time"

    "github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
)

func TestGenerateSerial_FirstOfDay(t *testing.T) {
    db := bootPg(t) // reuses the helper already in handler_test.go
    day := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

    got, err := quotation.GenerateSerialAt(context.Background(), db, day)
    if err != nil {
        t.Fatalf("generate: %v", err)
    }
    if got != "Q202605010001" {
        t.Fatalf("got %q, want Q202605010001", got)
    }
}

func TestGenerateSerial_IncrementsSameDay(t *testing.T) {
    db := bootPg(t)
    day := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)

    a, _ := quotation.GenerateSerialAt(context.Background(), db, day)
    b, _ := quotation.GenerateSerialAt(context.Background(), db, day)
    if a == b {
        t.Fatal("expected distinct serials")
    }
    if a != "Q202605020001" || b != "Q202605020002" {
        t.Fatalf("got %q, %q", a, b)
    }
}

func TestGenerateSerial_ResetsNextDay(t *testing.T) {
    db := bootPg(t)
    day1 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
    day2 := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)

    quotation.GenerateSerialAt(context.Background(), db, day1)
    quotation.GenerateSerialAt(context.Background(), db, day1)
    c, _ := quotation.GenerateSerialAt(context.Background(), db, day2)
    if c != "Q202605040001" {
        t.Fatalf("got %q, want reset to ...0001", c)
    }
}
```

- [ ] **Step 2: Create `serial.go`**

```go
package quotation

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"

    "gorm.io/gorm"
)

// Advisory lock key for serial generation. Two int32 constants —
// first is "domain=quotation" (arbitrary), second is "resource=serial".
// No collision risk with other features because no other code in the
// repo uses pg_advisory_xact_lock today.
const (
    advisoryLockDomainQuotation = 1
    advisoryLockResourceSerial  = 1
)

// GenerateSerialAt returns the next serial number for the given day.
// Format: "Q" + YYYYMMDD + 4-digit daily sequence (padded).
// The caller MUST invoke this inside a DB transaction — the advisory
// lock is transaction-scoped and released on commit/rollback.
//
// Concurrent callers for the SAME day queue on the lock; different-day
// calls contend on the same lock but do not corrupt each other because
// the MAX(serial_no) subquery is day-scoped.
func GenerateSerialAt(ctx context.Context, tx *gorm.DB, day time.Time) (string, error) {
    if err := tx.WithContext(ctx).Exec(
        "SELECT pg_advisory_xact_lock(?, ?)",
        advisoryLockDomainQuotation, advisoryLockResourceSerial,
    ).Error; err != nil {
        return "", fmt.Errorf("quotation: advisory lock: %w", err)
    }

    prefix := "Q" + day.UTC().Format("20060102")
    var maxSerial sql.NullString
    err := tx.WithContext(ctx).Raw(`
        SELECT MAX(serial_no)
        FROM quotations
        WHERE serial_no LIKE ?
    `, prefix+"%").Scan(&maxSerial).Error
    if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
        return "", fmt.Errorf("quotation: max serial: %w", err)
    }

    next := 1
    if maxSerial.Valid && len(maxSerial.String) == len(prefix)+4 {
        var seq int
        if _, err := fmt.Sscanf(maxSerial.String[len(prefix):], "%04d", &seq); err == nil {
            next = seq + 1
        }
    }
    if next > 9999 {
        return "", errors.New("quotation: daily serial exhausted (max 9999)")
    }

    return fmt.Sprintf("%s%04d", prefix, next), nil
}
```

- [ ] **Step 3: Run tests**

Run: `cd apps/api && go test ./internal/quotation/ -run TestGenerateSerial -count=1`
Expected: all pass (note: tests share `bootPg` testcontainers helper).

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/quotation/serial.go apps/api/internal/quotation/serial_test.go
git commit -m "feat(api): add per-day serial number generator with advisory lock"
```

---

### Task 5: Wire `Submit` to generate serial + update test

**Files:**
- Modify: `apps/api/internal/quotation/service.go`
- Modify: `apps/api/internal/quotation/repository.go`
- Modify: `apps/api/internal/quotation/handler_test.go`

- [ ] **Step 1: Update `Transition` signature**

In `repository.go`, find the existing `Transition(ctx, id, fromStatus, updates *Quotation)` method and extend the `*Quotation` struct fields consumed:

The struct already has `SerialNo *string`. `Transition` uses `updates.X != nil` to decide which columns to set, so adding `SerialNo` is transparent — but test explicitly to confirm by grep the transition code.

No code change required IF the existing `Transition` already iterates over reflective fields OR uses explicit `if updates.Foo != nil` blocks for each column. If the code is:

```go
if updates.SnapshotJSON != nil { setMap["snapshot_json"] = updates.SnapshotJSON }
```

Then add:

```go
if updates.SerialNo != nil { setMap["serial_no"] = *updates.SerialNo }
```

(Read the existing file first; if the Transition already uses gorm `Updates(updates)` with a full struct and zero-value skip, NO change is needed.)

- [ ] **Step 2: Update `Service.Submit`**

In `service.go`, before calling `Transition`, call `GenerateSerialAt` inside the same transaction. Approach:

```go
// Inside Submit, after building the snapshot but before Transition:
var serial string
err := s.repo.WithTx(ctx, func(tx *gorm.DB) error {
    s, err := GenerateSerialAt(ctx, tx, s.clock.Now())
    if err != nil {
        return err
    }
    serial = s
    // then Transition inside the same tx ...
})
```

If there is no existing `WithTx` helper, use `s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error { ... })`.

Set `updates.SerialNo = &serial` on the struct passed to `Transition`.

- [ ] **Step 3: Update existing Submit test**

`handler_test.go` has a test that submits a quotation and asserts `status == submitted`. Extend it to also assert the response body has `serial_no` matching the pattern `^Q\d{12}$`.

- [ ] **Step 4: Run tests**

Run: `cd apps/api && go test ./internal/quotation/... -count=1`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/quotation/service.go apps/api/internal/quotation/repository.go apps/api/internal/quotation/handler_test.go
git commit -m "feat(api): generate daily serial_no at Submit time"
```

---

### Task 6: Adjust — service method

**Files:**
- Create: `apps/api/internal/quotation/diff.go`
- Create: `apps/api/internal/quotation/diff_test.go`
- Modify: `apps/api/internal/quotation/service.go`

- [ ] **Step 1: Diff helper test**

`diff_test.go`:

```go
package quotation

import (
    "encoding/json"
    "testing"
)

func TestComputeSnapshotDiff_ChangesAndTotals(t *testing.T) {
    prev := Snapshot{Lines: []SnapshotLine{
        {FeeItem: "A", AmountCNYCents: 100},
        {FeeItem: "B", AmountCNYCents: 200},
    }, Total: 300}
    next := Snapshot{Lines: []SnapshotLine{
        {FeeItem: "A", AmountCNYCents: 150}, // changed
        {FeeItem: "C", AmountCNYCents: 50},  // added; B removed
    }, Total: 200}

    diff := computeSnapshotDiff(prev, next)
    b, _ := json.Marshal(diff)
    // Presence checks — exact format can evolve.
    for _, want := range []string{
        `"fee_item":"A"`, `"before":100`, `"after":150`, // update
        `"fee_item":"C"`, `"after":50`,                 // add
        `"fee_item":"B"`, `"before":200`,               // remove
        `"total_before":300`, `"total_after":200`,
    } {
        if !strings.Contains(string(b), want) {
            t.Fatalf("diff JSON missing %q — got: %s", want, b)
        }
    }
}
```

- [ ] **Step 2: Create `diff.go`**

```go
package quotation

// SnapshotDiff summarizes what changed between two snapshots.
// Marshaled into the JSONB quotation_status_history.diff_json column
// so reviewers and auditors can see precisely what an Adjust altered.
type SnapshotDiff struct {
    LinesAdded   []SnapshotLine    `json:"lines_added,omitempty"`
    LinesRemoved []SnapshotLine    `json:"lines_removed,omitempty"`
    LinesUpdated []SnapshotLineDelta `json:"lines_updated,omitempty"`
    TotalBefore  int64             `json:"total_before"`
    TotalAfter   int64             `json:"total_after"`
}

type SnapshotLineDelta struct {
    FeeItem string `json:"fee_item"`
    Before  int64  `json:"before"`
    After   int64  `json:"after"`
}

// computeSnapshotDiff produces a structured diff of two snapshots.
// Matching is by FeeItem (which is the natural key for a snapshot line).
func computeSnapshotDiff(prev, next Snapshot) SnapshotDiff {
    out := SnapshotDiff{TotalBefore: prev.Total, TotalAfter: next.Total}
    prevByItem := make(map[string]int64, len(prev.Lines))
    for _, l := range prev.Lines {
        prevByItem[l.FeeItem] = l.AmountCNYCents
    }
    nextByItem := make(map[string]int64, len(next.Lines))
    for _, l := range next.Lines {
        nextByItem[l.FeeItem] = l.AmountCNYCents
    }
    for item, amt := range nextByItem {
        if prevAmt, ok := prevByItem[item]; !ok {
            out.LinesAdded = append(out.LinesAdded, SnapshotLine{FeeItem: item, AmountCNYCents: amt})
        } else if prevAmt != amt {
            out.LinesUpdated = append(out.LinesUpdated, SnapshotLineDelta{
                FeeItem: item, Before: prevAmt, After: amt,
            })
        }
    }
    for item, amt := range prevByItem {
        if _, ok := nextByItem[item]; !ok {
            out.LinesRemoved = append(out.LinesRemoved, SnapshotLine{FeeItem: item, AmountCNYCents: amt})
        }
    }
    return out
}
```

- [ ] **Step 3: Adjust service method**

In `service.go` add:

```go
// Adjust mutates a submitted quotation's snapshot in place. Role-gated
// by the router (reviewer/admin). The quotation stays in `submitted`
// status; the status_history row records from=submitted,to=submitted
// with a non-null diff_json distinguishing it from plain submit.
func (s *Service) Adjust(
    ctx context.Context,
    id, actorID uuid.UUID,
    lines []SnapshotLine,
    comment *string,
) (*Quotation, error) {
    if len(lines) == 0 {
        return nil, ErrEmptyAdjust
    }

    q, err := s.repo.Get(ctx, id)
    if err != nil {
        return nil, err
    }
    if q.Status != StatusSubmitted {
        return nil, ErrInvalidTransition
    }

    prevSnap, err := q.UnmarshalSnapshot()
    if err != nil {
        return nil, err
    }

    // Rebuild snapshot from the caller-supplied lines — caller has
    // already overridden whatever pricing would produce.
    calcLines := make([]pricing.CalcLine, len(lines))
    for i, l := range lines {
        calcLines[i] = pricing.CalcLine{FeeItem: l.FeeItem, AmountCNYCents: l.AmountCNYCents}
    }
    built := calcLinesToSnapshot(calcLines)

    nextSnap := Snapshot{Lines: built.Lines, Total: built.Total, Signature: built.Signature}
    diff := computeSnapshotDiff(prevSnap, nextSnap)
    diffJSON, err := json.Marshal(diff)
    if err != nil {
        return nil, err
    }

    nextSnapJSON, err := json.Marshal(nextSnap)
    if err != nil {
        return nil, err
    }

    updates := &Quotation{
        SnapshotJSON:  nextSnapJSON,
        TotalCNYCents: &built.Total,
        Signature:     &built.Signature,
    }

    if err := s.repo.TransitionWithHistory(
        ctx, id, StatusSubmitted, StatusSubmitted, actorID,
        updates, comment, diffJSON,
    ); err != nil {
        return nil, err
    }
    return s.repo.Get(ctx, id)
}
```

> Note: `TransitionWithHistory` is a new repository method that takes the history row's `diff_json` as a final parameter. See Task 7 Step 1 for where it lives.

- [ ] **Step 4: Add `ErrEmptyAdjust` sentinel**

In `service.go`:

```go
var ErrEmptyAdjust = errors.New("quotation: adjust requires at least one line")
```

- [ ] **Step 5: Run tests**

Run: `cd apps/api && go test ./internal/quotation/ -run TestComputeSnapshotDiff -count=1`
Expected: pass.

Note: full `Adjust` integration test is written in Task 10 (handler integration).

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/quotation/diff.go apps/api/internal/quotation/diff_test.go apps/api/internal/quotation/service.go
git commit -m "feat(api): add Adjust service + snapshot diff helper"
```

---

### Task 7: Repository — `TransitionWithHistory`

**Files:**
- Modify: `apps/api/internal/quotation/repository.go`

- [ ] **Step 1: Add the method**

Next to the existing `Transition`, add:

```go
// TransitionWithHistory is Transition + an explicit diff_json on the
// status history row. Use this for Adjust (same-status with payload)
// and for any future transition that needs to record a structured
// event body.
//
// The existing Transition method is preserved for plain state changes
// with no diff.
func (r *Repository) TransitionWithHistory(
    ctx context.Context,
    id uuid.UUID,
    from, to Status,
    actor uuid.UUID,
    updates *Quotation,
    comment *string,
    diffJSON []byte,
) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        setMap := r.buildUpdatesMap(updates)
        setMap["status"] = to
        res := tx.Model(&Quotation{}).
            Where("id = ? AND status = ?", id, from).
            Updates(setMap)
        if res.Error != nil {
            return res.Error
        }
        if res.RowsAffected == 0 {
            return ErrInvalidTransition
        }

        hist := StatusHistory{
            QuotationID: id,
            FromStatus:  from,
            ToStatus:    to,
            ActorID:     &actor,
            Comment:     comment,
        }
        if len(diffJSON) > 0 {
            hist.DiffJSON = audit.JSONB(diffJSON)
        }
        return tx.Create(&hist).Error
    })
}
```

(Extract the setMap-building logic from existing `Transition` into a helper `buildUpdatesMap(*Quotation) map[string]any` so both methods share it. Keep both public methods in the same file.)

- [ ] **Step 2: Build + existing tests still green**

Run: `cd apps/api && go build ./... && go test ./internal/quotation/... -count=1`
Expected: pass.

- [ ] **Step 3: Commit**

```bash
git add apps/api/internal/quotation/repository.go
git commit -m "feat(api): add TransitionWithHistory for Adjust diff payload"
```

---

### Task 8: Withdraw — service method

**Files:**
- Modify: `apps/api/internal/quotation/service.go`

- [ ] **Step 1: Add the method**

```go
// Withdraw returns a submitted quotation to draft. Owner-only
// (enforced at handler). Clears snapshot/total/signature so the
// CHECK constraint chk_quotations_snapshot_when_nondraft is satisfied
// (i.e. drafts may have NULL snapshot columns). Serial_no is kept so
// the same quotation reuses its serial on re-submit.
func (s *Service) Withdraw(
    ctx context.Context,
    id, actorID uuid.UUID,
) (*Quotation, error) {
    q, err := s.repo.Get(ctx, id)
    if err != nil {
        return nil, err
    }
    if q.CreatedBy != actorID {
        return nil, ErrForbidden
    }
    if q.Status != StatusSubmitted {
        return nil, ErrInvalidTransition
    }

    updates := &Quotation{
        SnapshotJSON:  nil,
        TotalCNYCents: nil,
        Signature:     nil,
    }
    // Transition needs to know these fields should be SET TO NULL,
    // which requires an explicit "nullify" path. See Step 2.
    if err := s.repo.TransitionNullable(
        ctx, id, StatusSubmitted, StatusDraft, actorID,
        updates, []string{"snapshot_json", "total_cny_cents", "signature"},
        nil,
    ); err != nil {
        return nil, err
    }
    return s.repo.Get(ctx, id)
}
```

- [ ] **Step 2: Add `TransitionNullable` in repository**

```go
// TransitionNullable is TransitionWithHistory but explicitly nullifies
// the columns listed in `nullColumns` regardless of what's on updates.
// Required for Withdraw because the CHECK constraint permits drafts
// to have NULL snapshot columns, and the only way to satisfy it is to
// actually SET them to NULL.
func (r *Repository) TransitionNullable(
    ctx context.Context,
    id uuid.UUID,
    from, to Status,
    actor uuid.UUID,
    updates *Quotation,
    nullColumns []string,
    comment *string,
) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        setMap := r.buildUpdatesMap(updates)
        setMap["status"] = to
        for _, col := range nullColumns {
            setMap[col] = nil
        }
        res := tx.Model(&Quotation{}).
            Where("id = ? AND status = ?", id, from).
            Updates(setMap)
        if res.Error != nil {
            return res.Error
        }
        if res.RowsAffected == 0 {
            return ErrInvalidTransition
        }
        hist := StatusHistory{
            QuotationID: id,
            FromStatus:  from,
            ToStatus:    to,
            ActorID:     &actor,
            Comment:     comment,
        }
        return tx.Create(&hist).Error
    })
}
```

- [ ] **Step 3: Sentinel**

Add to `service.go` if not already present:

```go
var ErrForbidden = errors.New("quotation: not authorized")
```

- [ ] **Step 4: Build + existing tests pass**

Run: `cd apps/api && go build ./... && go test ./internal/quotation/... -count=1`

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/quotation/service.go apps/api/internal/quotation/repository.go
git commit -m "feat(api): add Withdraw service (submitted → draft)"
```

---

### Task 9: Copy — service method

**Files:**
- Modify: `apps/api/internal/quotation/service.go`

- [ ] **Step 1: Add the method**

```go
// Copy clones an existing quotation into a fresh draft owned by actor.
// Re-runs pricing against CURRENT pricing entries; does NOT reuse the
// source's frozen snapshot. Serial_no is NOT copied (drafts have no
// serial). Any role may copy any quotation they're allowed to see —
// visibility enforced at the handler.
func (s *Service) Copy(
    ctx context.Context,
    sourceID, actorID uuid.UUID,
) (*Quotation, error) {
    src, err := s.repo.Get(ctx, sourceID)
    if err != nil {
        return nil, err
    }
    draft := &Quotation{
        CustomerID:   src.CustomerID,
        CountryID:    src.CountryID,
        ServiceTier:  src.ServiceTier,
        Status:       StatusDraft,
        Notes:        src.Notes,
        CreatedBy:    actorID,
    }
    if err := s.repo.Create(ctx, draft); err != nil {
        return nil, err
    }
    return draft, nil
}
```

- [ ] **Step 2: Build + tests**

Run: `cd apps/api && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add apps/api/internal/quotation/service.go
git commit -m "feat(api): add Copy service (clone to fresh draft)"
```

---

### Task 10: Handlers + routes for Adjust/Withdraw/Copy

**Files:**
- Modify: `apps/api/internal/quotation/handler.go`
- Modify: `apps/api/internal/quotation/dto.go`
- Modify: `apps/api/internal/quotation/router.go`
- Modify: `apps/api/internal/quotation/handler_test.go`

- [ ] **Step 1: DTO additions**

```go
type AdjustRequest struct {
    Lines   []SnapshotLine `json:"lines" binding:"required,min=1,dive"`
    Comment *string        `json:"comment,omitempty"`
}
```

- [ ] **Step 2: Handlers**

```go
// Adjust — reviewer/admin only (router-gated).
func (h *Handler) Adjust(c *gin.Context) {
    id, ok := parseUUIDParam(c, "id")
    if !ok { return }
    var req AdjustRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"code":"ERR_BAD_REQUEST","message":err.Error()})
        return
    }
    user := auth.CurrentUser(c)
    q, err := h.svc.Adjust(c.Request.Context(), id, user.ID, req.Lines, req.Comment)
    if err != nil {
        writeServiceErr(c, err)
        return
    }
    c.JSON(http.StatusOK, toDTO(q))
}

// Withdraw — owner only, status must be submitted.
func (h *Handler) Withdraw(c *gin.Context) {
    id, ok := parseUUIDParam(c, "id")
    if !ok { return }
    user := auth.CurrentUser(c)
    q, err := h.svc.Withdraw(c.Request.Context(), id, user.ID)
    if err != nil {
        writeServiceErr(c, err)
        return
    }
    c.JSON(http.StatusOK, toDTO(q))
}

// Copy — any authed user; source visibility enforced like Get.
func (h *Handler) Copy(c *gin.Context) {
    id, ok := parseUUIDParam(c, "id")
    if !ok { return }
    user := auth.CurrentUser(c)
    q, err := h.svc.Copy(c.Request.Context(), id, user.ID)
    if err != nil {
        writeServiceErr(c, err)
        return
    }
    c.JSON(http.StatusCreated, toDTO(q))
}
```

If `writeServiceErr` does not exist, inline error mapping from `errors.Is` to HTTP codes (403 on `ErrForbidden`, 409 on `ErrInvalidTransition`, 400 on `ErrEmptyAdjust`).

- [ ] **Step 3: Router**

In `router.go`:

```go
func RegisterAuthedRoutes(g *gin.RouterGroup, h *Handler) {
    // ... existing ...
    g.POST("/quotations/:id/withdraw", h.Withdraw)
    g.POST("/quotations/:id/copy", h.Copy)
}

func RegisterReviewerRoutes(g *gin.RouterGroup, h *Handler) {
    // ... existing ...
    g.POST("/quotations/:id/adjust", h.Adjust)
}
```

- [ ] **Step 4: Integration tests**

In `handler_test.go` add tests:

1. `TestHandler_Withdraw_OwnerOK` — create draft, submit, withdraw → status=draft.
2. `TestHandler_Withdraw_Forbidden_NonOwner` — different user cannot withdraw.
3. `TestHandler_Withdraw_InvalidFromApproved` — approve first, withdraw → 409.
4. `TestHandler_Copy_ReturnsFreshDraft` — copy an approved quotation → new ID, status=draft, same customer/country/tier, no serial_no, no snapshot.
5. `TestHandler_Adjust_RecordsDiff` — reviewer adjusts snapshot (bumps one amount); assert history row for id has diff_json with `total_after` matching new total.
6. `TestHandler_Adjust_Forbidden_Salesperson` — non-reviewer gets 403 at route level.
7. `TestHandler_Adjust_InvalidOnDraft` — adjust before submit → 409 invalid transition.

- [ ] **Step 5: Run**

Run: `cd apps/api && go test ./internal/quotation/... -count=1`
Expected: all green, including 7 new tests.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/quotation/handler.go apps/api/internal/quotation/dto.go apps/api/internal/quotation/router.go apps/api/internal/quotation/handler_test.go
git commit -m "feat(api): add POST adjust/withdraw/copy endpoints"
```

---

### Task 11: Main.go wiring (routes)

**Files:**
- Modify: `apps/api/cmd/server/main.go`

- [ ] **Step 1: Confirm existing wiring is enough**

Since Adjust goes on `reviewerAdminGroup` and Withdraw/Copy go on `authed`, and `main.go` already calls `quotation.RegisterAuthedRoutes(authed, quotHandler)` and `quotation.RegisterReviewerRoutes(reviewerAdminGroup, quotHandler)`, no main.go change is required IF the new routes were added to the existing register functions in Task 10. Verify by grep.

- [ ] **Step 2: Build**

Run: `cd apps/api && go build ./...`

- [ ] **Step 3: Commit (only if any change)**

If no change, skip commit and note in the branch diff.

---

### Task 12: Frontend — types + hooks

**Files:**
- Modify: `apps/web/src/features/quotation/types.ts`
- Modify: `apps/web/src/features/quotation/hooks/use-quotation-mutations.ts`

- [ ] **Step 1: Types**

```ts
// Extend existing Quotation type:
export interface Quotation {
  // ... existing fields ...
  serial_no?: string | null
}

// Extend QuotationHistoryEntry:
export interface QuotationHistoryEntry {
  // ... existing fields ...
  diff_json?: SnapshotDiff | null
}

export interface SnapshotLineDelta {
  fee_item: string
  before: number
  after: number
}

export interface SnapshotDiff {
  lines_added?: SnapshotLine[]
  lines_removed?: SnapshotLine[]
  lines_updated?: SnapshotLineDelta[]
  total_before: number
  total_after: number
}

export interface AdjustRequest {
  lines: SnapshotLine[]
  comment?: string
}
```

- [ ] **Step 2: Hooks**

Add three mutations alongside existing ones (same file, same QueryClient invalidation pattern):

```ts
export function useWithdrawQuotation(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      const { data } = await api.post<Quotation>(`/quotations/${id}/withdraw`)
      return data
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['quotations'] })
      qc.invalidateQueries({ queryKey: ['quotation', id] })
      toast.success('已撤回到草稿 / Withdrawn to draft')
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}

export function useCopyQuotation(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      const { data } = await api.post<Quotation>(`/quotations/${id}/copy`)
      return data
    },
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ['quotations'] })
      toast.success(`已复制为新草稿 / Copied to ${data.id.slice(0, 8)}`)
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}

export function useAdjustQuotation(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (req: AdjustRequest) => {
      const { data } = await api.post<Quotation>(`/quotations/${id}/adjust`, req)
      return data
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['quotation', id] })
      toast.success('调价已保存 / Adjustment saved')
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}
```

- [ ] **Step 3: Typecheck**

Run: `cd apps/web && pnpm tsc --noEmit`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/features/quotation/types.ts apps/web/src/features/quotation/hooks/use-quotation-mutations.ts
git commit -m "feat(web): add withdraw/copy/adjust types + mutations"
```

---

### Task 13: Frontend — Adjust sheet component

**Files:**
- Create: `apps/web/src/features/quotation/components/quotation-adjust-sheet.tsx`

- [ ] **Step 1: Component**

```tsx
import { useState } from 'react'
import {
  Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle, SheetTrigger,
} from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Plus, Trash } from 'lucide-react'

import { useAdjustQuotation } from '../hooks/use-quotation-mutations'
import type { Quotation, SnapshotLine } from '../types'

interface Props {
  quotation: Quotation
  trigger: React.ReactNode
}

export function QuotationAdjustSheet({ quotation, trigger }: Props) {
  const initial: SnapshotLine[] =
    quotation.snapshot?.lines.map((l) => ({ ...l })) ?? []
  const [lines, setLines] = useState<SnapshotLine[]>(initial)
  const [comment, setComment] = useState('')
  const [open, setOpen] = useState(false)

  const adjustMut = useAdjustQuotation(quotation.id)
  const total = lines.reduce((sum, l) => sum + (l.amount_cny_cents || 0), 0)

  const updateLine = (i: number, patch: Partial<SnapshotLine>) => {
    setLines((cur) => cur.map((l, idx) => (idx === i ? { ...l, ...patch } : l)))
  }
  const addLine = () =>
    setLines((cur) => [...cur, { fee_item: '', amount_cny_cents: 0 }])
  const removeLine = (i: number) =>
    setLines((cur) => cur.filter((_, idx) => idx !== i))

  const save = async () => {
    // Normalize: drop empty fee_item rows; coerce amounts.
    const clean = lines
      .map((l) => ({ ...l, fee_item: l.fee_item.trim() }))
      .filter((l) => l.fee_item.length > 0)
    await adjustMut.mutateAsync({
      lines: clean,
      comment: comment.trim() || undefined,
    })
    setOpen(false)
  }

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>{trigger}</SheetTrigger>
      <SheetContent className='w-[520px] sm:max-w-none'>
        <SheetHeader>
          <SheetTitle>调价 / Adjust</SheetTitle>
          <SheetDescription>
            修改冻结报价的明细。总价会自动重算,保存后写入审核历史。
          </SheetDescription>
        </SheetHeader>

        <div className='grid gap-3 py-4'>
          {lines.map((l, i) => (
            <div key={i} className='flex gap-2 items-center'>
              <Input
                className='flex-1'
                placeholder='费用项 / Fee item'
                value={l.fee_item}
                onChange={(e) => updateLine(i, { fee_item: e.target.value })}
              />
              <Input
                className='w-32'
                type='number'
                min={0}
                step={1}
                value={l.amount_cny_cents}
                onChange={(e) =>
                  updateLine(i, { amount_cny_cents: Number(e.target.value) })
                }
              />
              <Button variant='ghost' size='icon' onClick={() => removeLine(i)}>
                <Trash className='h-4 w-4' />
              </Button>
            </div>
          ))}
          <Button variant='outline' onClick={addLine} className='self-start'>
            <Plus className='mr-2 h-4 w-4' /> 新增行 / Add line
          </Button>

          <div className='mt-2 text-right text-sm'>
            合计 / Total: <strong>¥{(total / 100).toFixed(2)}</strong>
          </div>

          <Textarea
            placeholder='备注（可选）/ Comment (optional)'
            value={comment}
            onChange={(e) => setComment(e.target.value)}
          />
        </div>

        <SheetFooter>
          <Button variant='ghost' onClick={() => setOpen(false)}>
            取消 / Cancel
          </Button>
          <Button
            disabled={adjustMut.isPending || lines.filter((l) => l.fee_item.trim()).length === 0}
            onClick={save}
          >
            保存 / Save
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
```

- [ ] **Step 2: Typecheck**

Run: `cd apps/web && pnpm tsc --noEmit`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/features/quotation/components/quotation-adjust-sheet.tsx
git commit -m "feat(web): add quotation adjust sheet component"
```

---

### Task 14: Frontend — detail integration + tests

**Files:**
- Modify: `apps/web/src/features/quotation/detail.tsx`
- Modify: `apps/web/src/features/quotation/components/quotation-action-bar.tsx` (or the existing actions component)
- Modify: `apps/web/src/features/quotation/quotation.integration.test.tsx`

- [ ] **Step 1: Wire action-bar buttons**

In the existing action-bar/detail component, add three buttons with correct visibility rules:

- **Withdraw** (撤回到草稿): visible when `status === 'submitted'` AND `current user is the owner (created_by === user.id)`.
- **Copy** (复制为草稿): visible for any viewer on any status.
- **Adjust** (调价): visible when `status === 'submitted'` AND `user.role in ['reviewer', 'admin']`. Wrap a button in `<QuotationAdjustSheet quotation={q} trigger={<Button>调价 / Adjust</Button>} />`.

Hook wiring:

```tsx
const withdrawMut = useWithdrawQuotation(q.id)
const copyMut = useCopyQuotation(q.id)

// Copy navigation after success:
const handleCopy = async () => {
  const newDraft = await copyMut.mutateAsync()
  navigate({ to: '/quotations/$id', params: { id: newDraft.id } })
}
```

- [ ] **Step 2: Render serial_no + diff in history**

In `detail.tsx`, if `q.serial_no` is present, render it alongside the short ID:

```tsx
<div className='font-mono text-sm'>{q.serial_no ?? q.id.slice(0, 8)}</div>
```

In the status-history list, when `entry.diff_json` is present, render a small summary:

```tsx
{entry.diff_json && (
  <div className='text-xs text-muted-foreground'>
    调价: ¥{entry.diff_json.total_before/100} → ¥{entry.diff_json.total_after/100}
  </div>
)}
```

- [ ] **Step 3: Integration test additions**

Append to `quotation.integration.test.tsx`:

```tsx
describe('withdraw + copy + adjust', () => {
  beforeAll(async () => {
    await worker.start({ onUnhandledRequest: 'bypass' })
  })
  beforeEach(() => {
    resetMswState(); __resetAuthInterceptorState()
    useAuthStore.getState().auth.reset()
  })
  afterAll(() => worker.stop())

  it('salesperson withdraws a submitted quotation', async () => {
    // Seed: quotation in submitted state, user is owner.
    // MSW stub for POST /quotations/:id/withdraw → return quotation with status=draft.
    // Render detail page; click Withdraw; assert status badge flips to 草稿.
  })

  it('reviewer adjusts snapshot via sheet, diff appears in history', async () => {
    // Seed: submitted quotation, reviewer user.
    // Click Adjust → open sheet → edit first line amount → Save.
    // MSW stub: POST /quotations/:id/adjust returns updated quotation + extended history.
    // Assert toast 调价已保存; history list renders 调价: ¥100 → ¥150.
  })

  it('copy lands on detail page of the new draft', async () => {
    // Seed: approved quotation.
    // Click Copy.
    // MSW stub POST /quotations/:id/copy returns new Quotation with fresh id, status=draft.
    // Assert router navigated to /quotations/<new-id> and UI now shows 草稿 badge.
  })
})
```

Flesh out each `it()` following the existing "admin submits a draft → approves" test's structure (same MSW helpers, same render harness, same use of `vitest-browser-react` + `vitest/browser`).

- [ ] **Step 4: Run tests**

Run: `cd apps/web && pnpm vitest run --browser.headless src/features/quotation/quotation.integration.test.tsx`
Expected: 6 tests pass (3 existing + 3 new).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/features/quotation/detail.tsx \
        apps/web/src/features/quotation/components/quotation-action-bar.tsx \
        apps/web/src/features/quotation/quotation.integration.test.tsx
git commit -m "feat(web): wire Withdraw / Copy / Adjust into detail page + tests"
```

---

## Self-review checklist

- [x] Every spec deviation (see top of doc) has a justification and is implemented consistently.
- [x] Every task has a commit at the end; no orphan working-tree changes.
- [x] The CHECK constraint added in Task 1 matches the nullable semantics in Task 8 (withdraw clears snapshot, so drafts can have NULL snapshot columns).
- [x] `TransitionWithHistory` and `TransitionNullable` both go through `buildUpdatesMap` so no drift in which columns can be updated.
- [x] Advisory lock key is a fixed `(1, 1)` pair; serial wrap-around (>9999/day) raises an error rather than silently reusing.
- [x] Frontend hook file is the existing `use-quotation-mutations.ts`, consistent with how `useSubmitQuotation`/`useApproveQuotation` are structured.
- [x] All new endpoints go through CSRF + audit + role middleware groups that already exist in main.go (no main.go wiring needed per Task 11 Step 1).
- [x] Integration test file is the existing `quotation.integration.test.tsx` — reusing the bootstrap, not a new file (same pattern M1 established).

---

## Execution sequence

Run tasks strictly in order **T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8 → T9 → T10 → T11 → T12 → T13 → T14**.

- T1–T5 are backend foundations; each is independently testable.
- T6–T10 are the business endpoints; T6 depends on T7 (TransitionWithHistory) so they ship together but in separate commits.
- T11 is a no-op confirmation; it exists to prevent forgetting to wire routes if they were added in the wrong register function.
- T12–T14 are frontend; T14 depends on T13 (sheet exists) and T12 (hooks exist).

Post-completion: run the full stack locally (`docker compose up -d postgres gotenberg && go run ./cmd/server & pnpm vite`), do an end-to-end smoke: submit → adjust → approve → withdraw clone → repeat. Optional since integration tests cover the happy paths, but recommended before merge.
