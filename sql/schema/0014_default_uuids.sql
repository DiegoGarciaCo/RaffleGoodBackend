-- +goose Up
-- +goose StatementBegin

-- Let Postgres generate UUIDs for the better-auth tables. better-auth is
-- configured with generateId:false, so it sends no id and relies on the
-- database default. gen_random_uuid() is built into Postgres 13+ (via pgcrypto,
-- which recent Postgres/Neon include by default).

ALTER TABLE "users"        ALTER COLUMN "id" SET DEFAULT gen_random_uuid();
ALTER TABLE "session"      ALTER COLUMN "id" SET DEFAULT gen_random_uuid();
ALTER TABLE "accounts"     ALTER COLUMN "id" SET DEFAULT gen_random_uuid();
ALTER TABLE "verification" ALTER COLUMN "id" SET DEFAULT gen_random_uuid();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE "users"        ALTER COLUMN "id" DROP DEFAULT;
ALTER TABLE "session"      ALTER COLUMN "id" DROP DEFAULT;
ALTER TABLE "accounts"     ALTER COLUMN "id" DROP DEFAULT;
ALTER TABLE "verification" ALTER COLUMN "id" DROP DEFAULT;

-- +goose StatementEnd
