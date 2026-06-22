-- +goose Up
-- ── raffle_views (powers conversion analytics + trending + recs) ────────────────
-- High volume → BIGSERIAL, nullable user_id for anonymous browsing.
-- +goose StatementBegin
CREATE TABLE raffle_views (
    id BIGSERIAL PRIMARY KEY,
    raffle_item_id UUID NOT NULL REFERENCES raffle_items(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE
    SET
        NULL,
        session_id TEXT,  -- correlate anonymous → signup
        source TEXT,  -- home | explore | search | share | org_page
        referrer TEXT,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_views_raffle ON raffle_views (raffle_item_id, created_at);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_views_user ON raffle_views (user_id, created_at);

-- +goose StatementEnd
-- ── search_events (demand sensing + search tuning) ──────────────────────────────
-- +goose StatementBegin
CREATE TABLE search_events (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE
    SET
        NULL,
        query TEXT NOT NULL,
        result_count INT NOT NULL DEFAULT 0,
        clicked_raffle_id UUID REFERENCES raffle_items(id) ON DELETE
    SET
        NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_search_query_trgm ON search_events USING gin (query gin_trgm_ops);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_search_created ON search_events (created_at DESC);

-- +goose StatementEnd
-- ── activity_events (dashboard feed + ML feature source) ────────────────────────
-- Append-only event stream. Powers the nonprofit dashboard's recent-activity feed
-- directly, and becomes the feature source for churn/LTV/recommendation models.
-- +goose StatementBegin
CREATE TABLE activity_events (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id UUID REFERENCES users(id) ON DELETE
    SET
        NULL,
        nonprofit_id UUID REFERENCES nonprofits(id) ON DELETE CASCADE,
        raffle_item_id UUID REFERENCES raffle_items(id) ON DELETE CASCADE,
        event_type TEXT NOT NULL,  -- ticket_purchase | follow | unfollow |
        -- view | raffle_created | draw_complete |
        -- prize_claimed | share | milestone
        metadata JSONB NOT NULL DEFAULT '{}',
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_activity_org ON activity_events (nonprofit_id, created_at DESC);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_activity_type ON activity_events (event_type, created_at DESC);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_activity_actor ON activity_events (actor_user_id, created_at DESC);

-- +goose StatementEnd
-- ── shares (virality + attribution) ─────────────────────────────────────────────
-- +goose StatementBegin
CREATE TABLE shares (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE
    SET
        NULL,
        raffle_item_id UUID REFERENCES raffle_items(id) ON DELETE CASCADE,
        nonprofit_id UUID REFERENCES nonprofits(id) ON DELETE CASCADE,
        channel TEXT,  -- copy_link | sms | email | social
        share_token TEXT UNIQUE,  -- attribute resulting signups/purchases
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_shares_raffle ON shares (raffle_item_id);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_shares_user ON shares (user_id);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS shares;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS activity_events;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS search_events;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS raffle_views;

-- +goose StatementEnd
