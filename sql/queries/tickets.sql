-- ── Orders ───────────────────────────────────────────────────────────────────
-- name: CreateOrder :one
INSERT INTO
    orders (
        user_id,
        raffle_item_id,
        ticket_count,
        subtotal_cents,
        donation_cents,
        total_cents,
        processor,
        processor_payment_id,
        STATUS,
        receipt_number
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING
    *;

-- name: MarkOrderPaid :one
UPDATE
    orders
SET
    STATUS = 'paid',
    processor_payment_id = $2
WHERE
    id = $1
RETURNING
    *;

-- name: GetOrderByID :one
SELECT
    *
FROM
    orders
WHERE
    id = $1;

-- name: GetOrderByReceiptNumber :one
SELECT
    o.*,
    ri.title AS raffle_title,
    n.name AS nonprofit_name,
    n.ein AS nonprofit_ein
FROM
    orders o
    JOIN raffle_items ri ON ri.id = o.raffle_item_id
    LEFT JOIN nonprofits n ON n.id = ri.created_by
WHERE
    o.receipt_number = $1;

-- name: ListOrdersForUser :many
SELECT
    o.*,
    ri.title AS raffle_title
FROM
    orders o
    JOIN raffle_items ri ON ri.id = o.raffle_item_id
WHERE
    o.user_id = sqlc.arg(user_id)
ORDER BY
    o.created_at DESC
LIMIT
    sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- ── Tickets ──────────────────────────────────────────────────────────────────
-- name: CreateTicket :one
-- ticket_number and price_paid are auto-filled by the assign_ticket_number trigger
-- when passed as NULL / 0 (system assignment). For user_picks, pass them explicitly.
INSERT INTO
    tickets (
        raffle_item_id,
        user_id,
        order_id,
        ticket_number,
        price_paid
    )
VALUES
    (
        sqlc.arg(raffle_item_id),
        sqlc.arg(user_id),
        sqlc.narg(order_id),
        sqlc.narg(ticket_number),
        sqlc.arg(price_paid)
    )
RETURNING
    *;

-- name: GetTicketByID :one
SELECT
    *
FROM
    tickets
WHERE
    id = $1;

-- name: ListTicketsForRaffle :many
SELECT
    *
FROM
    tickets
WHERE
    raffle_item_id = $1
ORDER BY
    ticket_number;

-- name: ListTicketNumbersForRaffle :many
-- For the draw engine: all numbers in order (input to the deterministic shuffle).
SELECT
    ticket_number,
    id AS ticket_id,
    user_id
FROM
    tickets
WHERE
    raffle_item_id = $1
ORDER BY
    ticket_number;

-- name: IsTicketNumberTaken :one
SELECT
    EXISTS(
        SELECT
            1
        FROM
            tickets
        WHERE
            raffle_item_id = $1
            AND ticket_number = $2
    ) AS taken;

-- name: ListAvailableTicketNumbers :many
-- For user_picks assignment: which numbers in 1..max are still free.
SELECT
    n AS ticket_number
FROM
    generate_series(1, sqlc.arg(max_tickets)::INT) AS n
WHERE
    n NOT IN (
        SELECT
            ticket_number
        FROM
            tickets
        WHERE
            raffle_item_id = sqlc.arg(raffle_item_id)
    )
ORDER BY
    n;

-- name: CountParticipantsForRaffle :one
SELECT
    COUNT(DISTINCT user_id)::INT AS count
FROM
    tickets
WHERE
    raffle_item_id = $1;

-- ── User's tickets (My Tickets screen) ────────────────────────────────────────
-- name: ListUserTicketsGrouped :many
-- One row per raffle the user has tickets in, with their numbers aggregated.
SELECT
    ri.id AS raffle_item_id,
    ri.title,
    ri.status,
    ri.draw_at,
    ri.image_urls,
    n.name AS nonprofit_name,
    ARRAY_AGG(
        t.ticket_number
        ORDER BY
            t.ticket_number
    )::INT[] AS ticket_numbers,
    COUNT(t.id)::INT AS ticket_count,
    SUM(t.price_paid)::NUMERIC AS total_spent,
    MAX(t.purchased_at)::TIMESTAMPTZ AS purchased_at
FROM
    tickets t
    JOIN raffle_items ri ON ri.id = t.raffle_item_id
    LEFT JOIN nonprofits n ON n.id = ri.created_by
WHERE
    t.user_id = $1
GROUP BY
    ri.id,
    ri.title,
    ri.status,
    ri.draw_at,
    ri.image_urls,
    n.name
ORDER BY
    ri.draw_at NULLS LAST;

-- name: GetUserTicketStats :one
SELECT
    COUNT(*)::INT AS total_tickets_purchased,
    COUNT(DISTINCT raffle_item_id)::INT AS total_raffles_entered,
    COALESCE(SUM(price_paid), 0)::NUMERIC AS total_spent
FROM
    tickets
WHERE
    user_id = $1;
