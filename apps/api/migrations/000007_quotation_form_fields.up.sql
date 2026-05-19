ALTER TABLE quotations
  ADD COLUMN IF NOT EXISTS country_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS nice_category_codes JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS registration_methods JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS agent_level TEXT NOT NULL DEFAULT 'agent_a',
  ADD COLUMN IF NOT EXISTS info_sections JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE quotations
SET country_ids = jsonb_build_array(country_id)
WHERE country_ids = '[]'::jsonb;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_quotations_agent_level'
  ) THEN
    ALTER TABLE quotations
      ADD CONSTRAINT chk_quotations_agent_level
      CHECK (agent_level IN ('agent_a', 'agent_b'));
  END IF;
END $$;
