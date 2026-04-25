ALTER TABLE quotation_status_history DROP COLUMN IF EXISTS diff_json;
ALTER TABLE quotations DROP CONSTRAINT IF EXISTS chk_quotations_serial_no_when_nondraft;
DROP INDEX IF EXISTS uq_quotations_serial_no;
ALTER TABLE quotations DROP COLUMN IF EXISTS serial_no;
