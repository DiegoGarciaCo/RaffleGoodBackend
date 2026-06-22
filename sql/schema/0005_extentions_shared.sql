-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- +goose StatementEnd
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- +goose StatementEnd
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "btree_gist";

-- +goose StatementEnd
-- set_updated_at: reusable trigger function shared by every table that needs
-- automatic updated_at maintenance. Created once here.
-- +goose StatementBegin
CREATE
OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS
$$
BEGIN
NEW.updated_at = NOW();

RETURN NEW;

END;

$$
LANGUAGE plpgsql;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS set_updated_at();

-- +goose StatementEnd
-- +goose StatementBegin
DROP EXTENSION IF EXISTS "btree_gist";

-- +goose StatementEnd
-- +goose StatementBegin
DROP EXTENSION IF EXISTS "pg_trgm";

-- +goose StatementEnd
-- +goose StatementBegin
DROP EXTENSION IF EXISTS "uuid-ossp";

-- +goose StatementEnd
