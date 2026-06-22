-- +goose Up
-- ── saved_raffles (participant Saved tab) ───────────────────────────────────────
-- +goose StatementBegin
CREATE TABLE saved_raffles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    raffle_item_id UUID NOT NULL REFERENCES raffle_items(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, raffle_item_id)
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_saved_user ON saved_raffles (user_id);

-- +goose StatementEnd
-- ── org_follows (Saved → Following tab; powers follower_count) ───────────────────
-- +goose StatementBegin
CREATE TABLE org_follows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    nonprofit_id UUID NOT NULL REFERENCES nonprofits(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, nonprofit_id)
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_follows_user ON org_follows (user_id);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_follows_org ON org_follows (nonprofit_id);

-- +goose StatementEnd
-- Maintain nonprofits.follower_count on follow / unfollow.
-- +goose StatementBegin
CREATE
OR REPLACE FUNCTION sync_follower_count() RETURNS TRIGGER AS
$$
BEGIN
IF TG_OP = 'INSERT' THEN
UPDATE
    nonprofits
SET
    follower_count = follower_count + 1
WHERE
    id = NEW.nonprofit_id;

RETURN NEW;

ELSIF TG_OP = 'DELETE' THEN
UPDATE
    nonprofits
SET
    follower_count = GREATEST(follower_count - 1, 0)
WHERE
    id = OLD.nonprofit_id;

RETURN OLD;

END IF;

RETURN NULL;

END;

$$
LANGUAGE plpgsql;

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_sync_follower_count
AFTER
INSERT
    OR DELETE ON org_follows FOR EACH ROW EXECUTE FUNCTION sync_follower_count();

-- +goose StatementEnd
-- ── nonprofit_reviews (participant nonprofit profile) ───────────────────────────
-- +goose StatementBegin
CREATE TABLE nonprofit_reviews (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nonprofit_id UUID NOT NULL REFERENCES nonprofits(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    body TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (nonprofit_id, user_id)
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_reviews_org ON nonprofit_reviews (nonprofit_id, created_at DESC);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_reviews_updated_at BEFORE
UPDATE
    ON nonprofit_reviews FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd
-- ── notification_preferences (participant + org) ────────────────────────────────
-- Exactly one of user_id / nonprofit_id is set per row.
-- +goose StatementBegin
CREATE TABLE notification_preferences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    nonprofit_id UUID REFERENCES nonprofits(id) ON DELETE CASCADE,
    prefs JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT notif_owner_chk CHECK (
        (
            user_id IS NOT NULL
            AND nonprofit_id IS NULL
        )
        OR (
            user_id IS NULL
            AND nonprofit_id IS NOT NULL
        )
    )
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE UNIQUE INDEX idx_notif_user ON notification_preferences (user_id)
WHERE
    user_id IS NOT NULL;

-- +goose StatementEnd
-- +goose StatementBegin
CREATE UNIQUE INDEX idx_notif_org ON notification_preferences (nonprofit_id)
WHERE
    nonprofit_id IS NOT NULL;

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_notif_prefs_updated_at BEFORE
UPDATE
    ON notification_preferences FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd
-- ── payment_methods (participant profile) ───────────────────────────────────────
-- Never store raw card data — only the processor token + display metadata.
-- +goose StatementBegin
CREATE TABLE payment_methods (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    processor TEXT NOT NULL,  -- 'stripe'
    processor_pm_id TEXT NOT NULL,  -- Stripe PaymentMethod id
    brand TEXT NOT NULL,
    last4 TEXT NOT NULL,
    exp_month INT NOT NULL,
    exp_year INT NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_pm_user ON payment_methods (user_id);

-- +goose StatementEnd
-- ── shipping_addresses (participant profile → prize delivery) ───────────────────
-- +goose StatementBegin
CREATE TABLE shipping_addresses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    line1 TEXT NOT NULL,
    line2 TEXT,
    city TEXT NOT NULL,
    state TEXT NOT NULL,
    zip TEXT NOT NULL,
    country TEXT NOT NULL DEFAULT 'US',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_addr_user ON shipping_addresses (user_id);

-- +goose StatementEnd
-- ── user preference signals (for recommendations) ───────────────────────────────
-- +goose StatementBegin
ALTER TABLE
    users
ADD
    COLUMN IF NOT EXISTS preferred_categories TEXT [] DEFAULT '{}';

-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE
    users
ADD
    COLUMN IF NOT EXISTS location_region TEXT;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE
    users DROP COLUMN IF EXISTS location_region;

-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE
    users DROP COLUMN IF EXISTS preferred_categories;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS shipping_addresses;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS payment_methods;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_notif_prefs_updated_at ON notification_preferences;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS notification_preferences;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_reviews_updated_at ON nonprofit_reviews;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS nonprofit_reviews;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_sync_follower_count ON org_follows;

-- +goose StatementEnd
-- +goose StatementBegin
DROP FUNCTION IF EXISTS sync_follower_count();

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS org_follows;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS saved_raffles;

-- +goose StatementEnd
