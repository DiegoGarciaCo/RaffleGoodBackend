-- +goose Up
-- +goose StatementBegin
ALTER TABLE nonprofit_verifications
    ADD COLUMN IF NOT EXISTS foundation INT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE nonprofit_verifications
    DROP COLUMN IF EXISTS foundation;
-- +goose StatementEnd
