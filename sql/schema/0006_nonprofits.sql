-- +goose Up
-- Assumes the better-auth "users" table already exists.
-- +goose StatementBegin
CREATE TABLE nonprofits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug VARCHAR(140),
    description TEXT,
    image TEXT,
    address TEXT,
    phone TEXT,
    ein TEXT,
    "hasToPay" BOOLEAN NOT NULL DEFAULT TRUE,
    website TEXT,
    categories TEXT [] NOT NULL DEFAULT '{}',
    -- Denormalized counters (maintained by triggers in later migrations)
    follower_count INT NOT NULL DEFAULT 0,
    total_raised_cents BIGINT NOT NULL DEFAULT 0,
    raffles_run INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE UNIQUE INDEX idx_nonprofits_slug ON nonprofits (slug)
WHERE
    slug IS NOT NULL;

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_nonprofits_updated_at BEFORE
UPDATE
    ON nonprofits FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TABLE nonprofit_verifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nonprofit_id UUID NOT NULL REFERENCES nonprofits(id) ON DELETE CASCADE,
    "isVerified" BOOLEAN DEFAULT FALSE,
    "verificationMethod" TEXT,
    "exemptStatus" INT,
    subsection INT,
    "nteeCd" TEXT,
    "rulingDate" DATE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_nonprofit_verifications_org ON nonprofit_verifications (nonprofit_id);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_nonprofit_verifications_updated_at BEFORE
UPDATE
    ON nonprofit_verifications FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TABLE nonprofit_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nonprofit_id UUID NOT NULL REFERENCES nonprofits(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(255) NOT NULL,  -- owner | admin | member
    MODE VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (nonprofit_id, user_id)
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_nonprofit_users_user ON nonprofit_users (user_id);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_nonprofit_users_org ON nonprofit_users (nonprofit_id);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_nonprofit_users_updated_at BEFORE
UPDATE
    ON nonprofit_users FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd
-- Social links (Org screen → Contact & links)
-- +goose StatementBegin
CREATE TABLE nonprofit_social_links (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nonprofit_id UUID NOT NULL REFERENCES nonprofits(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,  -- instagram | twitter | facebook | linkedin | youtube
    url TEXT NOT NULL
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_social_links_org ON nonprofit_social_links (nonprofit_id);

-- +goose StatementEnd
-- Bank account for payouts (Org screen → Payouts)
-- +goose StatementBegin
CREATE TABLE nonprofit_bank_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nonprofit_id UUID NOT NULL REFERENCES nonprofits(id) ON DELETE CASCADE,
    processor TEXT NOT NULL,  -- 'stripe_connect'
    processor_account_id TEXT NOT NULL,
    bank_name TEXT,
    last4 TEXT,
    account_type TEXT,  -- checking | savings
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_bank_accounts_org ON nonprofit_bank_accounts (nonprofit_id);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_bank_accounts_updated_at BEFORE
UPDATE
    ON nonprofit_bank_accounts FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd
-- Payout history (Org screen → Payout history)
-- references raffle_items, which is created later; FK added in migration 003.
-- +goose StatementBegin
CREATE TABLE payouts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nonprofit_id UUID NOT NULL REFERENCES nonprofits(id) ON DELETE CASCADE,
    raffle_item_id UUID,  -- FK added after raffle_items exists
    amount_cents INT NOT NULL,
    STATUS TEXT NOT NULL,  -- pending | paid | failed
    processor_payout_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at TIMESTAMPTZ
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_payouts_org ON payouts (nonprofit_id, created_at DESC);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS payouts;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS nonprofit_bank_accounts;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS nonprofit_social_links;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS nonprofit_users;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS nonprofit_verifications;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS nonprofits;

-- +goose StatementEnd
