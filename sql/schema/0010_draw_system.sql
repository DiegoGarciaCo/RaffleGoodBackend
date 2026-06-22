-- +goose Up
-- ── prize_tiers ─────────────────────────────────────────────────────────────────
-- Set at create time (the wizard / draw-config screen). Defines what each rank wins.
-- +goose StatementBegin
CREATE TABLE prize_tiers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    raffle_item_id UUID NOT NULL REFERENCES raffle_items(id) ON DELETE CASCADE,
    rank INT NOT NULL,  -- 1 = first place / grand prize
    title TEXT NOT NULL,
    description TEXT,
    value_cents INT,
    image_url TEXT,
    UNIQUE (raffle_item_id, rank)
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_prize_tiers_raffle ON prize_tiers (raffle_item_id);

-- +goose StatementEnd
-- ── draw_results ────────────────────────────────────────────────────────────────
-- One per raffle. Created when the draw fires; frozen and revealed gradually.
-- Provable fairness via commit-reveal: commitment_hash published before sales
-- close, seed revealed after the draw.
-- +goose StatementBegin
CREATE TABLE draw_results (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    raffle_item_id UUID NOT NULL UNIQUE REFERENCES raffle_items(id) ON DELETE CASCADE,
    draw_strategy draw_strategy NOT NULL,
    reveal_order TEXT NOT NULL DEFAULT 'forward',  -- forward | reverse
    winner_count INT NOT NULL,
    -- Commit-reveal
    commitment_hash TEXT NOT NULL,  -- sha256(seed), published BEFORE sales close
    seed TEXT,  -- revealed AFTER the draw (NULL until then)
    -- Reveal timing
    draw_started_at TIMESTAMPTZ,
    draw_duration_ms INT NOT NULL DEFAULT 0,
    -- Snapshot stats at draw time
    total_tickets INT NOT NULL DEFAULT 0,
    total_participants INT NOT NULL DEFAULT 0,
    verification_url TEXT,
    STATUS TEXT NOT NULL DEFAULT 'scheduled',  -- scheduled | in_progress | completed
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_draw_results_updated_at BEFORE
UPDATE
    ON draw_results FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd
-- ── draw_winners ────────────────────────────────────────────────────────────────
-- The frozen, ordered winning tickets. Array position (pull_index) IS pull order.
-- +goose StatementBegin
CREATE TABLE draw_winners (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    draw_result_id UUID NOT NULL REFERENCES draw_results(id) ON DELETE CASCADE,
    pull_index INT NOT NULL,  -- 0-based order pulled
    rank INT NOT NULL,  -- 1 = grand prize (after reveal_order)
    ticket_id UUID NOT NULL REFERENCES tickets(id),
    ticket_number INT NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id),
    prize_title TEXT NOT NULL,
    prize_description TEXT,
    prize_value_cents INT,
    revealed_at_ms INT NOT NULL,  -- ms offset from draw_started_at
    prize_claimed BOOLEAN NOT NULL DEFAULT FALSE,
    claimed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (draw_result_id, pull_index)
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_draw_winners_result ON draw_winners (draw_result_id);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_draw_winners_user ON draw_winners (user_id);

-- +goose StatementEnd
-- Unclaimed-prize lookups (dashboard alerts)
-- +goose StatementBegin
CREATE INDEX idx_draw_winners_unclaimed ON draw_winners (prize_claimed)
WHERE
    prize_claimed = FALSE;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS draw_winners;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_draw_results_updated_at ON draw_results;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS draw_results;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS prize_tiers;

-- +goose StatementEnd
