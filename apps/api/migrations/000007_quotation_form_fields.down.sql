ALTER TABLE quotations
  DROP CONSTRAINT IF EXISTS chk_quotations_agent_level,
  DROP COLUMN IF EXISTS info_sections,
  DROP COLUMN IF EXISTS agent_level,
  DROP COLUMN IF EXISTS registration_methods,
  DROP COLUMN IF EXISTS nice_category_codes,
  DROP COLUMN IF EXISTS country_ids;
