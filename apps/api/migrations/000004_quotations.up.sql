-- apps/api/migrations/000004_quotations.up.sql

CREATE TABLE IF NOT EXISTS quotations (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id       UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
  country_id        UUID NOT NULL REFERENCES countries(id) ON DELETE RESTRICT,
  service_tier      TEXT NOT NULL,
  status            TEXT NOT NULL DEFAULT 'draft',
  -- Snapshot captured at submit time. NULL while draft.
  snapshot_json     JSONB,
  total_cny_cents   BIGINT,
  signature         TEXT,
  submitted_at      TIMESTAMPTZ,
  reviewed_at       TIMESTAMPTZ,
  reviewed_by       UUID REFERENCES users(id) ON DELETE SET NULL,
  review_comment    TEXT,
  notes             TEXT,
  created_by        UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at        TIMESTAMPTZ,

  CONSTRAINT chk_quotations_tier
    CHECK (service_tier IN ('basic','standard','premium')),
  CONSTRAINT chk_quotations_status
    CHECK (status IN ('draft','submitted','approved','rejected','cancelled')),
  -- Non-draft statuses must carry the snapshot.
  CONSTRAINT chk_quotations_snapshot_when_nondraft
    CHECK (
      status = 'draft'
      OR (snapshot_json IS NOT NULL AND total_cny_cents IS NOT NULL AND signature IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_quotations_customer ON quotations(customer_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_quotations_status   ON quotations(status)      WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_quotations_created_by_status ON quotations(created_by, status) WHERE deleted_at IS NULL;

-- Status transitions log. Append-only — rows are never updated or deleted.
CREATE TABLE IF NOT EXISTS quotation_status_history (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  quotation_id    UUID NOT NULL REFERENCES quotations(id) ON DELETE CASCADE,
  from_status     TEXT NOT NULL,
  to_status       TEXT NOT NULL,
  actor_id        UUID REFERENCES users(id) ON DELETE SET NULL,
  comment         TEXT,
  at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_quotation_history_qid ON quotation_status_history(quotation_id, at DESC);
