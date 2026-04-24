-- Countries dictionary. Code is ISO 3166-1 alpha-2 (2 letters).
CREATE TABLE IF NOT EXISTS countries (
  id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code                        TEXT UNIQUE NOT NULL,
  name_zh                     TEXT NOT NULL,
  name_en                     TEXT NOT NULL,
  is_madrid_member            BOOLEAN NOT NULL DEFAULT FALSE,
  default_acceptance_days     INTEGER,
  default_registration_months INTEGER,
  requires_notarization       BOOLEAN NOT NULL DEFAULT FALSE,
  notes_zh                    TEXT,
  notes_en                    TEXT,
  sort_order                  INTEGER NOT NULL DEFAULT 0,
  enabled                     BOOLEAN NOT NULL DEFAULT TRUE,
  created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_countries_enabled_sort ON countries(enabled, sort_order);
CREATE INDEX IF NOT EXISTS idx_countries_madrid ON countries(is_madrid_member) WHERE enabled;

-- Nice (international trademark) classes. Codes are fixed 1..45.
CREATE TABLE IF NOT EXISTS nice_categories (
  code            INTEGER PRIMARY KEY CHECK (code BETWEEN 1 AND 45),
  name_zh         TEXT NOT NULL,
  name_en         TEXT NOT NULL,
  description_zh  TEXT,
  description_en  TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Customers. Unique (name) among non-deleted rows.
CREATE TABLE IF NOT EXISTS customers (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name            TEXT NOT NULL,
  industry        TEXT,
  is_returning    BOOLEAN NOT NULL DEFAULT FALSE,
  price_sensitive BOOLEAN NOT NULL DEFAULT FALSE,
  contact_name    TEXT,
  contact_phone   TEXT,
  contact_email   TEXT,
  notes           TEXT,
  created_by      UUID NOT NULL REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_customers_name_alive
  ON customers(name)
  WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customers_owner
  ON customers(created_by)
  WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customers_search
  ON customers USING GIN (to_tsvector('simple', coalesce(name, '') || ' ' || coalesce(industry, '')))
  WHERE deleted_at IS NULL;
