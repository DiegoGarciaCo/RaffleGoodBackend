-- ── Raffle views ──────────────────────────────────────────────────────────────
-- name: RecordRaffleView :exec
-- Records one raffle view for conversion analytics. Called (fire-and-forget)
-- from the public GET /raffles/{id} handler. Anonymous views are fine.
-- NOTE: if raffle_views has a nullable user_id column you want to populate,
-- add `, user_id` here and `, sqlc.narg(user_id)` to VALUES.
INSERT INTO raffle_views (raffle_item_id)
VALUES (sqlc.arg(raffle_item_id)::uuid);

-- name: CountRaffleViews :one
SELECT
    COUNT(*)::INT AS count
FROM
    raffle_views
WHERE
    raffle_item_id = $1;

-- ── Search events ─────────────────────────────────────────────────────────────
-- name: RecordSearch :exec
INSERT INTO
    search_events (user_id, query, result_count, clicked_raffle_id)
VALUES
    (
        sqlc.narg(user_id),
        sqlc.arg(query),
        sqlc.arg(result_count),
        sqlc.narg(clicked_raffle_id)
    );

-- name: TopSearchQueries :many
-- Demand sensing: most common searches over a window.
SELECT
    query,
    COUNT(*)::INT AS count
FROM
    search_events
WHERE
    created_at >= sqlc.arg(window_start)
GROUP BY
    query
ORDER BY
    count DESC
LIMIT
    sqlc.arg(result_limit);

-- name: TopZeroResultSearches :many
-- Searches that returned nothing — prizes people want that you don't have.
SELECT
    query,
    COUNT(*)::INT AS count
FROM
    search_events
WHERE
    result_count = 0
    AND created_at >= sqlc.arg(window_start)
GROUP BY
    query
ORDER BY
    count DESC
LIMIT
    sqlc.arg(result_limit);

-- ── Activity events (dashboard feed) ──────────────────────────────────────────
-- name: RecordActivityEvent :exec
INSERT INTO
    activity_events (
        actor_user_id,
        nonprofit_id,
        raffle_item_id,
        event_type,
        metadata
    )
VALUES
    (
        sqlc.narg(actor_user_id),
        sqlc.narg(nonprofit_id),
        sqlc.narg(raffle_item_id),
        sqlc.arg(event_type),
        sqlc.arg(metadata)
    );

-- name: ListActivityForNonprofit :many
-- Powers the dashboard recent-activity feed.
SELECT
    *
FROM
    activity_events
WHERE
    nonprofit_id = sqlc.arg(nonprofit_id)
ORDER BY
    created_at DESC
LIMIT
    sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- ── Shares ────────────────────────────────────────────────────────────────────
-- name: RecordShare :one
INSERT INTO
    shares (
        user_id,
        raffle_item_id,
        nonprofit_id,
        channel,
        share_token
    )
VALUES
    (
        sqlc.narg(user_id),
        sqlc.narg(raffle_item_id),
        sqlc.narg(nonprofit_id),
        sqlc.arg(channel),
        sqlc.arg(share_token)
    )
RETURNING
    *;

-- name: GetShareByToken :one
SELECT
    *
FROM
    shares
WHERE
    share_token = $1;

-- name: CountSharesForRaffle :one
SELECT
    COUNT(*)::INT AS count
FROM
    shares
WHERE
    raffle_item_id = $1;
