-- +goose Up
-- ── Enums ──────────────────────────────────────────────────────────────────────
-- raffle status — includes 'drawing' (live draw in progress) and 'cancelled'
-- +goose StatementBegin
CREATE TYPE raffle_item_status AS ENUM (
    'draft',
    'active',
    'closed',
    'drawing',
    'completed',
    'cancelled'
);

-- +goose StatementEnd
-- ticket_strategy — how tickets are priced
--   fixed             every ticket the same price
--   bundle            discounted multi-ticket packs
--   donation          pay-what-you-want with a suggested amount
--   free              all tickets free
--   pay_what_you_pull the ticket NUMBER is its price (#1=$1, #65=$65)
-- +goose StatementBegin
CREATE TYPE ticket_strategy AS ENUM (
    'fixed',
    'bundle',
    'donation',
    'free',
    'pay_what_you_pull'
);

-- +goose StatementEnd
-- ticket_assignment_strategy — how a BUYER gets their ticket number
--   random      system assigns the next number
--   user_picks  buyer chooses an available number
-- +goose StatementBegin
CREATE TYPE ticket_assignment_strategy AS ENUM ('random', 'user_picks');

-- +goose StatementEnd
-- draw_strategy — how WINNERS are pulled at draw time
--   single          one winner, one prize
--   ranked          N winners, different prize each, first pulled = 1st
--   multiple_equal  N winners, same prize, order irrelevant
--   countdown       last ticket standing wins the grand prize
-- +goose StatementBegin
CREATE TYPE draw_strategy AS ENUM (
    'single',
    'ranked',
    'multiple_equal',
    'countdown'
);

-- +goose StatementEnd
-- ── raffle_items ────────────────────────────────────────────────────────────────
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS raffle_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Category linkage
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    category_path TEXT NOT NULL,  -- denormalized, kept in sync by trigger
    -- Core fields
    title VARCHAR(255) NOT NULL,
    slug VARCHAR(140),  -- shareable URL slug
    description TEXT,
    STATUS raffle_item_status NOT NULL DEFAULT 'draft',
    -- Strategies
    ticket_strategy ticket_strategy NOT NULL DEFAULT 'fixed',
    ticket_assignment_strategy ticket_assignment_strategy NOT NULL DEFAULT 'random',
    draw_strategy draw_strategy NOT NULL DEFAULT 'single',
    -- Ticket pricing
    ticket_price NUMERIC(10, 2) CHECK (ticket_price >= 0),
    free_tickets INT NOT NULL DEFAULT 0,
    free_ticket_start_range INT CHECK (free_ticket_start_range >= 1),
    free_ticket_end_range INT CHECK (free_ticket_end_range >= free_ticket_start_range),
    max_tickets INT CHECK (
        max_tickets IS NULL
        OR max_tickets > 0
    ),
    tickets_sold INT NOT NULL DEFAULT 0 CHECK (tickets_sold >= 0),
    -- Dynamic per-category attributes (prize details) stored as JSONB
    attributes JSONB NOT NULL DEFAULT '{}',
    -- Media
    image_urls TEXT [] NOT NULL DEFAULT '{}',
    -- Draw scheduling — draw_at is NULL until the raffle sells out / is scheduled
    draw_at TIMESTAMPTZ,
    drawn_at TIMESTAMPTZ,
    -- Audit
    created_by UUID REFERENCES nonprofits(id) ON DELETE
    SET
        NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE UNIQUE INDEX idx_items_slug ON raffle_items (slug)
WHERE
    slug IS NOT NULL;

-- +goose StatementEnd
-- Primary browse query: all active items in a category subtree
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_items_category_path_status ON raffle_items (category_path text_pattern_ops, STATUS);

-- +goose StatementEnd
-- Direct category lookup
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_items_category_id_status ON raffle_items (category_id, STATUS);

-- +goose StatementEnd
-- Ticket price filtering/sorting
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_items_ticket_price ON raffle_items (ticket_price)
WHERE
    STATUS = 'active';

-- +goose StatementEnd
-- Upcoming draws
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_items_draw_at ON raffle_items (draw_at)
WHERE
    STATUS = 'active'
    AND draw_at IS NOT NULL;

-- +goose StatementEnd
-- Owner's manage-raffles list
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_items_created_by ON raffle_items (created_by, STATUS);

-- +goose StatementEnd
-- Full-text search on title + description
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_items_fts ON raffle_items USING gin (
    to_tsvector(
        'english',
        coalesce(title, '') || ' ' || coalesce(description, '')
    )
);

-- +goose StatementEnd
-- JSONB attribute filtering (attributes @> '{"brand":"Nike"}')
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_items_attributes ON raffle_items USING gin (attributes jsonb_path_ops);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_raffle_items_updated_at BEFORE
UPDATE
    ON raffle_items FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd
-- Now that raffle_items exists, attach the deferred payouts FK.
-- +goose StatementBegin
ALTER TABLE
    payouts
ADD
    CONSTRAINT payouts_raffle_item_fk FOREIGN KEY (raffle_item_id) REFERENCES raffle_items(id) ON DELETE
SET
    NULL;

-- +goose StatementEnd
-- sync_item_category_path: keeps category_path in sync with category_id.
-- +goose StatementBegin
CREATE
OR REPLACE FUNCTION sync_item_category_path() RETURNS TRIGGER AS
$$
BEGIN
SELECT
    path INTO NEW.category_path
FROM
    categories
WHERE
    id = NEW.category_id;

IF NOT FOUND THEN RAISE EXCEPTION 'category_id % does not exist',
NEW.category_id;

END IF;

RETURN NEW;

END;

$$
LANGUAGE plpgsql;

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_sync_item_category_path BEFORE
INSERT
    OR
UPDATE
    OF category_id ON raffle_items FOR EACH ROW EXECUTE FUNCTION sync_item_category_path();

-- +goose StatementEnd
-- Maintain nonprofits.raffles_run counter when a raffle becomes active.
-- +goose StatementBegin
CREATE
OR REPLACE FUNCTION bump_raffles_run() RETURNS TRIGGER AS
$$
BEGIN
IF NEW.status = 'active'
AND (
    TG_OP = 'INSERT'
    OR OLD.status <> 'active'
) THEN
UPDATE
    nonprofits
SET
    raffles_run = raffles_run + 1
WHERE
    id = NEW.created_by;

END IF;

RETURN NEW;

END;

$$
LANGUAGE plpgsql;

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_bump_raffles_run
AFTER
INSERT
    OR
UPDATE
    OF STATUS ON raffle_items FOR EACH ROW EXECUTE FUNCTION bump_raffles_run();

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_bump_raffles_run ON raffle_items;

-- +goose StatementEnd
-- +goose StatementBegin
DROP FUNCTION IF EXISTS bump_raffles_run();

-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_sync_item_category_path ON raffle_items;

-- +goose StatementEnd
-- +goose StatementBegin
DROP FUNCTION IF EXISTS sync_item_category_path();

-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE
    payouts DROP CONSTRAINT IF EXISTS payouts_raffle_item_fk;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_raffle_items_updated_at ON raffle_items;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS raffle_items;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TYPE IF EXISTS draw_strategy;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TYPE IF EXISTS ticket_assignment_strategy;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TYPE IF EXISTS ticket_strategy;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TYPE IF EXISTS raffle_item_status;

-- +goose StatementEnd
