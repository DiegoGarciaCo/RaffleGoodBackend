-- All analytics queries take a nonprofit_id and a time window (window_start).
-- The handler computes the previous-period window for trend comparisons.
-- ── KPIs ──────────────────────────────────────────────────────────────────────
-- name: GetAnalyticsKPIs :one
-- Total raised, tickets sold, and distinct buyers in a window.
SELECT
    COALESCE(SUM(t.price_paid), 0)::NUMERIC AS total_raised,
    COUNT(t.id)::INT AS tickets_sold,
    COUNT(DISTINCT t.user_id)::INT AS unique_buyers
FROM
    tickets t
    JOIN raffle_items ri ON ri.id = t.raffle_item_id
WHERE
    ri.created_by = sqlc.arg(nonprofit_id)
    AND t.purchased_at >= sqlc.arg(window_start)
    AND t.purchased_at < sqlc.arg(window_end);

-- name: GetNewFollowersInWindow :one
SELECT
    COUNT(*)::INT AS count
FROM
    org_follows
WHERE
    nonprofit_id = sqlc.arg(nonprofit_id)
    AND created_at >= sqlc.arg(window_start)
    AND created_at < sqlc.arg(window_end);

-- ── Revenue series (bar chart) ────────────────────────────────────────────────
-- name: GetRevenueByDay :many
-- Daily revenue for the chart. Handler buckets into weeks/months as needed.
SELECT
    DATE_TRUNC('day', t.purchased_at)::DATE AS DAY,
    SUM(t.price_paid)::NUMERIC AS amount
FROM
    tickets t
    JOIN raffle_items ri ON ri.id = t.raffle_item_id
WHERE
    ri.created_by = sqlc.arg(nonprofit_id)
    AND t.purchased_at >= sqlc.arg(window_start)
    AND t.purchased_at < sqlc.arg(window_end)
GROUP BY
    DAY
ORDER BY
    DAY;

-- name: GetRevenueByMonth :many
-- Monthly buckets for longer ranges (year / all).
SELECT
    DATE_TRUNC('month', t.purchased_at)::DATE AS MONTH,
    SUM(t.price_paid)::NUMERIC AS amount
FROM
    tickets t
    JOIN raffle_items ri ON ri.id = t.raffle_item_id
WHERE
    ri.created_by = sqlc.arg(nonprofit_id)
    AND t.purchased_at >= sqlc.arg(window_start)
GROUP BY
    MONTH
ORDER BY
    MONTH;

-- ── Top performing raffles ────────────────────────────────────────────────────
-- name: GetTopRaffles :many
SELECT
    ri.id,
    ri.title,
    ri.tickets_sold,
    ri.max_tickets,
    COALESCE(SUM(t.price_paid), 0)::NUMERIC AS raised
FROM
    raffle_items ri
    LEFT JOIN tickets t ON t.raffle_item_id = ri.id
WHERE
    ri.created_by = sqlc.arg(nonprofit_id)
    AND ri.created_at >= sqlc.arg(window_start)
GROUP BY
    ri.id,
    ri.title,
    ri.tickets_sold,
    ri.max_tickets
ORDER BY
    raised DESC
LIMIT
    sqlc.arg(result_limit);

-- ── Buyer breakdown (repeat vs first-time vs from followers) ───────────────────
-- name: GetBuyerBreakdown :one
-- Classifies buyers in the window:
--   repeat       = bought from this org before the window too
--   first_time   = first ever purchase from this org is in the window
--   from_follower = buyer follows the org
WITH window_buyers AS (
    SELECT
        DISTINCT t.user_id
    FROM
        tickets t
        JOIN raffle_items ri ON ri.id = t.raffle_item_id
    WHERE
        ri.created_by = sqlc.arg(nonprofit_id)
        AND t.purchased_at >= sqlc.arg(window_start)
        AND t.purchased_at < sqlc.arg(window_end)
),
prior_buyers AS (
    SELECT
        DISTINCT t.user_id
    FROM
        tickets t
        JOIN raffle_items ri ON ri.id = t.raffle_item_id
    WHERE
        ri.created_by = sqlc.arg(nonprofit_id)
        AND t.purchased_at < sqlc.arg(window_start)
)
SELECT
    COUNT(*) FILTER (
        WHERE
            wb.user_id IN (
                SELECT
                    user_id
                FROM
                    prior_buyers
            )
    )::INT AS repeat_buyers,
    COUNT(*) FILTER (
        WHERE
            wb.user_id NOT IN (
                SELECT
                    user_id
                FROM
                    prior_buyers
            )
    )::INT AS first_time_buyers,
    COUNT(*) FILTER (
        WHERE
            wb.user_id IN (
                SELECT
                    user_id
                FROM
                    org_follows
                WHERE
                    nonprofit_id = sqlc.arg(nonprofit_id)
            )
    )::INT AS follower_buyers,
    COUNT(*)::INT AS total_buyers
FROM
    window_buyers wb;

-- ── Conversion (views → purchases) ─────────────────────────────────────────────
-- name: GetConversionStats :one
WITH org_views AS (
    SELECT
        COUNT(*)::INT AS views
    FROM
        raffle_views rv
        JOIN raffle_items ri ON ri.id = rv.raffle_item_id
    WHERE
        ri.created_by = sqlc.arg(nonprofit_id)
        AND rv.created_at >= sqlc.arg(window_start)
        AND rv.created_at < sqlc.arg(window_end)
),
org_buyers AS (
    SELECT
        COUNT(DISTINCT t.user_id)::INT AS buyers
    FROM
        tickets t
        JOIN raffle_items ri ON ri.id = t.raffle_item_id
    WHERE
        ri.created_by = sqlc.arg(nonprofit_id)
        AND t.purchased_at >= sqlc.arg(window_start)
        AND t.purchased_at < sqlc.arg(window_end)
)
SELECT
    (
        SELECT
            views
        FROM
            org_views
    ) AS views,
    (
        SELECT
            buyers
        FROM
            org_buyers
    ) AS buyers;

-- ── Pricing insight support (PWP vs fixed performance) ─────────────────────────
-- name: GetConversionByTicketStrategy :many
-- Feeds the automated "your $5 raffles convert better" style insight.
SELECT
    ri.ticket_strategy,
    ri.ticket_price,
    COUNT(DISTINCT rv.id)::INT AS views,
    COUNT(DISTINCT t.id)::INT AS tickets_sold
FROM
    raffle_items ri
    LEFT JOIN raffle_views rv ON rv.raffle_item_id = ri.id
    LEFT JOIN tickets t ON t.raffle_item_id = ri.id
WHERE
    ri.created_by = sqlc.arg(nonprofit_id)
GROUP BY
    ri.ticket_strategy,
    ri.ticket_price;
