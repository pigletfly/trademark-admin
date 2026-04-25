-- service_tier + fee_item 用 CHECK 约束而非 ENUM：CHECK 放宽/扩容时只需 ALTER TABLE
-- 单条语句；ENUM 改值要 DROP & recreate，在生产上痛苦。
CREATE TABLE pricing_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    country_id UUID NOT NULL REFERENCES countries(id) ON DELETE RESTRICT,
    service_tier TEXT NOT NULL
        CHECK (service_tier IN ('basic', 'standard', 'premium')),
    fee_item TEXT NOT NULL
        CHECK (length(fee_item) BETWEEN 1 AND 80),
    amount_cny_cents BIGINT NOT NULL CHECK (amount_cny_cents >= 0),
    notes TEXT,
    effective_from DATE NOT NULL,
    effective_to DATE
        CHECK (effective_to IS NULL OR effective_to > effective_from),
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 同一 (country, tier, fee_item) 任一时刻只能有一条 effective_to IS NULL 的 active 行
CREATE UNIQUE INDEX idx_pricing_active_unique
    ON pricing_entries (country_id, service_tier, fee_item)
    WHERE effective_to IS NULL;

-- 历史查询：按维度查所有版本，倒序
CREATE INDEX idx_pricing_history
    ON pricing_entries (country_id, service_tier, fee_item, effective_from DESC);

-- 列表查询：admin 查 active by country+tier 时走这个覆盖
CREATE INDEX idx_pricing_active_lookup
    ON pricing_entries (country_id, service_tier)
    WHERE effective_to IS NULL;
