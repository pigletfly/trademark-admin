ALTER TABLE export_files
  DROP CONSTRAINT IF EXISTS chk_export_files_format;

ALTER TABLE export_files
  ADD CONSTRAINT chk_export_files_format
  CHECK (format IN ('pdf','docx'));
