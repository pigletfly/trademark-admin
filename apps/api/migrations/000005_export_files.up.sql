-- apps/api/migrations/000005_export_files.up.sql

CREATE TABLE IF NOT EXISTS export_files (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  quotation_id   UUID NOT NULL REFERENCES quotations(id) ON DELETE CASCADE,
  format         TEXT NOT NULL,
  language       TEXT NOT NULL,
  file_path      TEXT NOT NULL,
  file_size      BIGINT NOT NULL,
  sha256         TEXT NOT NULL,
  expires_at     TIMESTAMPTZ NOT NULL,
  created_by     UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT chk_export_files_format
    CHECK (format IN ('pdf','docx')),
  CONSTRAINT chk_export_files_language
    CHECK (language IN ('zh','en','bilingual'))
);

CREATE INDEX IF NOT EXISTS idx_export_files_quotation
  ON export_files(quotation_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_export_files_expiry
  ON export_files(expires_at);
