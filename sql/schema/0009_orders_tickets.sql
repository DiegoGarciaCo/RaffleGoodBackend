-- +goose Up
-- ── orders ──────────────────────────────────────────────────────────────────────
-- Groups a multi-ticket purchase into one transaction (for receipts + tickets tab).
-- +goose StatementBegin
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    raffle_item_id UUID NOT NULL REFERENCES raffle_items(id),
    ticket_count INT NOT NULL CHECK (ticket_count > 0),
    subtotal_cents INT NOT NULL DEFAULT 0,
    donation_cents INT NOT NULL DEFAULT 0,  -- optional add-on donation
    total_cents INT NOT NULL DEFAULT 0,
    processor TEXT NOT NULL,  -- 'stripe'
    processor_payment_id TEXT,
    STATUS TEXT NOT NULL DEFAULT 'pending',  -- pending | paid | refunded | failed
    receipt_number TEXT UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_orders_user ON orders (user_id, created_at DESC);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_orders_raffle ON orders (raffle_item_id);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_orders_updated_at BEFORE
UPDATE
    ON orders FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd
-- ── tickets ─────────────────────────────────────────────────────────────────────
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tickets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    raffle_item_id UUID NOT NULL REFERENCES raffle_items(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_id UUID REFERENCES orders(id),
    ticket_number INT NOT NULL,
    price_paid NUMERIC(10, 2) NOT NULL,
    purchased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tickets_item_number_unique UNIQUE (raffle_item_id, ticket_number)
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_tickets_item_id ON tickets (raffle_item_id);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON tickets (user_id);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_tickets_order_id ON tickets (order_id);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_tickets_purchased ON tickets (purchased_at DESC);

-- +goose StatementEnd
-- assign_ticket_number: for system-assigned ('random') numbering, picks the next
-- sequential number and computes price_paid based on the raffle's ticket_strategy
-- (fixed price, or pay_what_you_pull where the NUMBER is the price, honoring any
-- free range). For 'user_picks' assignment, the app supplies ticket_number and
-- price_paid explicitly and this still validates / fills price if zero.
-- +goose StatementBegin
CREATE
OR REPLACE FUNCTION assign_ticket_number() RETURNS TRIGGER AS
$$
DECLARE
v_strategy ticket_strategy;

v_assign ticket_assignment_strategy;

v_price NUMERIC(10, 2);

v_free_start INT;

v_free_end INT;

v_is_free BOOLEAN;

BEGIN
SELECT
    ticket_strategy,
    ticket_assignment_strategy,
    ticket_price,
    free_ticket_start_range,
    free_ticket_end_range INTO v_strategy,
    v_assign,
    v_price,
    v_free_start,
    v_free_end
FROM
    raffle_items
WHERE
    id = NEW.raffle_item_id FOR
UPDATE
;

-- Assign the number if the system controls it
IF v_assign = 'random'
OR NEW.ticket_number IS NULL THEN
SELECT
    COALESCE(MAX(ticket_number), 0) + 1 INTO NEW.ticket_number
FROM
    tickets
WHERE
    raffle_item_id = NEW.raffle_item_id;

END IF;

-- Is this ticket within the free range?
v_is_free := v_free_start IS NOT NULL
AND NEW.ticket_number BETWEEN v_free_start AND v_free_end;

-- Compute price_paid when the caller didn't set one (0)
IF NEW.price_paid IS NULL
OR NEW.price_paid = 0 THEN IF v_is_free
OR v_strategy = 'free' THEN NEW.price_paid := 0;

ELSIF v_strategy = 'pay_what_you_pull' THEN
-- the ticket NUMBER is the price
NEW.price_paid := NEW.ticket_number;

ELSE NEW.price_paid := COALESCE(v_price, 0);

END IF;

END IF;

-- Increment sold counter
UPDATE
    raffle_items
SET
    tickets_sold = tickets_sold + 1
WHERE
    id = NEW.raffle_item_id;

RETURN NEW;

END;

$$
LANGUAGE plpgsql;

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_assign_ticket_number BEFORE
INSERT
    ON tickets FOR EACH ROW EXECUTE FUNCTION assign_ticket_number();

-- +goose StatementEnd
-- Maintain nonprofits.total_raised_cents when a ticket is recorded.
-- +goose StatementBegin
CREATE
OR REPLACE FUNCTION bump_total_raised() RETURNS TRIGGER AS
$$
DECLARE
v_org UUID;

BEGIN
SELECT
    created_by INTO v_org
FROM
    raffle_items
WHERE
    id = NEW.raffle_item_id;

IF v_org IS NOT NULL THEN
UPDATE
    nonprofits
SET
    total_raised_cents = total_raised_cents + ROUND(NEW.price_paid * 100)::BIGINT
WHERE
    id = v_org;

END IF;

RETURN NEW;

END;

$$
LANGUAGE plpgsql;

-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_bump_total_raised
AFTER
INSERT
    ON tickets FOR EACH ROW EXECUTE FUNCTION bump_total_raised();

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_bump_total_raised ON tickets;

-- +goose StatementEnd
-- +goose StatementBegin
DROP FUNCTION IF EXISTS bump_total_raised();

-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_assign_ticket_number ON tickets;

-- +goose StatementEnd
-- +goose StatementBegin
DROP FUNCTION IF EXISTS assign_ticket_number();

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS tickets;

-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS orders;

-- +goose StatementEnd
