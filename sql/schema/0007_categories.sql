-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Tree structure (Adjacency List + Materialized Path hybrid)
    parent_id UUID REFERENCES categories(id) ON DELETE RESTRICT,
    path TEXT NOT NULL,
    depth INT NOT NULL DEFAULT 0,
    -- Display
    name VARCHAR(120) NOT NULL,
    slug VARCHAR(120) NOT NULL,
    icon VARCHAR(80),
    sort_order INT NOT NULL DEFAULT 0,
    -- Attribute schema: defines what fields items in this category have.
    -- Example: {"make":"string","year":"number","condition":["new","used"]}
    attribute_schema JSONB,
    -- Soft state
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Constraints
    CONSTRAINT categories_slug_unique UNIQUE (slug),
    CONSTRAINT categories_path_unique UNIQUE (path),
    CONSTRAINT categories_depth_nonneg CHECK (depth >= 0)
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_categories_path_text ON categories USING btree (path text_pattern_ops);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_categories_parent_id ON categories (parent_id);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_categories_name_trgm ON categories USING gin (name gin_trgm_ops);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_categories_active ON categories (is_active)
WHERE
    is_active = TRUE;

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_categories_updated_at BEFORE
UPDATE
    ON categories FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_categories_updated_at ON categories;

-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_categories_active;

-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_categories_name_trgm;

-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_categories_parent_id;

-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_categories_path_text;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS categories;

-- +goose StatementEnd
