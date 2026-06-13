ALTER TABLE quotations
  ADD COLUMN IF NOT EXISTS madrid_country_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS single_country_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE quotations
SET
  madrid_country_ids = CASE
    WHEN registration_methods ? 'madrid' THEN country_ids
    ELSE '[]'::jsonb
  END,
  single_country_ids = CASE
    WHEN jsonb_array_length(registration_methods) = 0 OR registration_methods ? 'single' THEN country_ids
    ELSE '[]'::jsonb
  END
WHERE
  madrid_country_ids = '[]'::jsonb
  AND single_country_ids = '[]'::jsonb;
