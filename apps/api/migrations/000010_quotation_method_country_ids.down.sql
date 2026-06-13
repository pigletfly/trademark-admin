ALTER TABLE quotations
  DROP COLUMN IF EXISTS single_country_ids,
  DROP COLUMN IF EXISTS madrid_country_ids;
