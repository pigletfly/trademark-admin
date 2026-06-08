CREATE TABLE madrid_pricing_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    country_id UUID REFERENCES countries(id) ON DELETE RESTRICT,
    sequence_no INTEGER,
    country_area TEXT NOT NULL CHECK (length(country_area) BETWEEN 1 AND 120),
    official_fee_chf_cents BIGINT NOT NULL CHECK (official_fee_chf_cents >= 0),
    agency_fee_cny_cents BIGINT NOT NULL CHECK (agency_fee_cny_cents >= 0),
    is_base_fee BOOLEAN NOT NULL DEFAULT FALSE,
    notes TEXT,
    effective_from DATE NOT NULL,
    effective_to DATE
        CHECK (effective_to IS NULL OR effective_to > effective_from),
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_madrid_pricing_country_scope
        CHECK (
            (is_base_fee = TRUE AND country_id IS NULL)
            OR (is_base_fee = FALSE AND country_id IS NOT NULL)
        )
);

CREATE UNIQUE INDEX idx_madrid_pricing_active_base
    ON madrid_pricing_entries (is_base_fee)
    WHERE effective_to IS NULL AND is_base_fee = TRUE;

CREATE UNIQUE INDEX idx_madrid_pricing_active_country
    ON madrid_pricing_entries (country_id)
    WHERE effective_to IS NULL AND is_base_fee = FALSE;

CREATE INDEX idx_madrid_pricing_active_lookup
    ON madrid_pricing_entries (country_id, is_base_fee)
    WHERE effective_to IS NULL;

CREATE TABLE single_class_pricing_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    country_id UUID NOT NULL REFERENCES countries(id) ON DELETE RESTRICT,
    continent TEXT NOT NULL CHECK (length(continent) BETWEEN 1 AND 80),
    country_area TEXT NOT NULL CHECK (length(country_area) BETWEEN 1 AND 120),
    first_class_fee_cny_cents BIGINT NOT NULL CHECK (first_class_fee_cny_cents >= 0),
    first_class_fee_tax6_cny_cents BIGINT NOT NULL CHECK (first_class_fee_tax6_cny_cents >= 0),
    first_class_fee_tax1_cny_cents BIGINT NOT NULL CHECK (first_class_fee_tax1_cny_cents >= 0),
    additional_class_fee_cny_cents BIGINT NOT NULL CHECK (additional_class_fee_cny_cents >= 0),
    additional_class_fee_tax6_cny_cents BIGINT NOT NULL CHECK (additional_class_fee_tax6_cny_cents >= 0),
    additional_class_fee_tax1_cny_cents BIGINT NOT NULL CHECK (additional_class_fee_tax1_cny_cents >= 0),
    required_documents TEXT NOT NULL DEFAULT '',
    notarization_fee TEXT NOT NULL DEFAULT '',
    acceptance_time TEXT NOT NULL DEFAULT '',
    registration_months TEXT NOT NULL DEFAULT '',
    validity_years INTEGER CHECK (validity_years IS NULL OR validity_years >= 0),
    note1 TEXT,
    note2 TEXT,
    effective_from DATE NOT NULL,
    effective_to DATE
        CHECK (effective_to IS NULL OR effective_to > effective_from),
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_single_class_pricing_active_country
    ON single_class_pricing_entries (country_id)
    WHERE effective_to IS NULL;

CREATE INDEX idx_single_class_pricing_active_lookup
    ON single_class_pricing_entries (country_id)
    WHERE effective_to IS NULL;
