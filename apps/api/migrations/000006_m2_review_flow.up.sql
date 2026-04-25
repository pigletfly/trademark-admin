-- M2: add serial_no to quotations (per-day generated Q+YYYYMMDD+NNNN,
-- nullable on drafts) and diff_json to quotation_status_history for
-- Adjust event payloads.

ALTER TABLE quotations ADD COLUMN serial_no TEXT;

-- Unique when set; NULLs allowed (drafts have none).
CREATE UNIQUE INDEX IF NOT EXISTS uq_quotations_serial_no
  ON quotations (serial_no)
  WHERE serial_no IS NOT NULL;

-- A quotation past draft MUST have a serial_no. Complements
-- chk_quotations_snapshot_when_nondraft.
ALTER TABLE quotations ADD CONSTRAINT chk_quotations_serial_no_when_nondraft
  CHECK (status = 'draft' OR serial_no IS NOT NULL);

-- diff_json captures the structured delta for Adjust events
-- (same-status submitted→submitted rows with a payload).
ALTER TABLE quotation_status_history ADD COLUMN diff_json JSONB;
