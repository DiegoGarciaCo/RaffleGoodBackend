-- ── Draw results ──────────────────────────────────────────────────────────────
-- name: CreateDrawResult :one
INSERT INTO
    draw_results (
        raffle_item_id,
        draw_strategy,
        reveal_order,
        winner_count,
        commitment_hash,
        total_tickets,
        total_participants,
        verification_url,
        STATUS
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING
    *;

-- name: GetDrawResultForRaffle :one
SELECT
    *
FROM
    draw_results
WHERE
    raffle_item_id = $1;

-- name: StartDrawReveal :one
-- Fires the reveal: stamps start time, duration, reveals the seed, sets status.
UPDATE
    draw_results
SET
    STATUS = 'in_progress',
    draw_started_at = NOW(),
    draw_duration_ms = $2,
    seed = $3
WHERE
    raffle_item_id = $1
RETURNING
    *;

-- name: CompleteDrawResult :exec
UPDATE
    draw_results
SET
    STATUS = 'completed'
WHERE
    raffle_item_id = $1;

-- name: SetCommitmentHash :exec
-- Published BEFORE sales close (at raffle publish time).
UPDATE
    draw_results
SET
    commitment_hash = $2
WHERE
    raffle_item_id = $1;

-- ── Draw winners ──────────────────────────────────────────────────────────────
-- name: CreateDrawWinner :one
INSERT INTO
    draw_winners (
        draw_result_id,
        pull_index,
        rank,
        ticket_id,
        ticket_number,
        user_id,
        prize_title,
        prize_description,
        prize_value_cents,
        revealed_at_ms
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING
    *;

-- name: ListDrawWinners :many
-- The frozen ordered result the reveal engine animates (pull order).
SELECT
    dw.*,
    u.name AS user_name
FROM
    draw_winners dw
    JOIN users u ON u.id = dw.user_id
WHERE
    dw.draw_result_id = $1
ORDER BY
    dw.pull_index;

-- name: ListDrawWinnersByRaffle :many
SELECT
    dw.*
FROM
    draw_winners dw
    JOIN draw_results dr ON dr.id = dw.draw_result_id
WHERE
    dr.raffle_item_id = $1
ORDER BY
    dw.pull_index;

-- name: GetUserWinForRaffle :one
-- Did this user win anything in this raffle?
SELECT
    dw.*
FROM
    draw_winners dw
    JOIN draw_results dr ON dr.id = dw.draw_result_id
WHERE
    dr.raffle_item_id = sqlc.arg(raffle_item_id)
    AND dw.user_id = sqlc.arg(user_id)
LIMIT
    1;

-- name: ClaimPrize :one
UPDATE
    draw_winners
SET
    prize_claimed = TRUE,
    claimed_at = NOW()
WHERE
    id = sqlc.arg(winner_id)
    AND user_id = sqlc.arg(user_id)
RETURNING
    *;

-- name: ListUnclaimedPrizesForNonprofit :many
-- Dashboard alert: winners who haven't claimed.
SELECT
    dw.*,
    ri.title AS raffle_title,
    ri.id AS raffle_item_id
FROM
    draw_winners dw
    JOIN draw_results dr ON dr.id = dw.draw_result_id
    JOIN raffle_items ri ON ri.id = dr.raffle_item_id
WHERE
    ri.created_by = $1
    AND dw.prize_claimed = FALSE
ORDER BY
    dw.created_at;

-- ── User's wins (My Tickets → Won tab) ────────────────────────────────────────
-- name: ListUserWins :many
SELECT
    dw.*,
    ri.id AS raffle_item_id,
    ri.title AS raffle_title,
    ri.image_urls,
    n.name AS nonprofit_name
FROM
    draw_winners dw
    JOIN draw_results dr ON dr.id = dw.draw_result_id
    JOIN raffle_items ri ON ri.id = dr.raffle_item_id
    LEFT JOIN nonprofits n ON n.id = ri.created_by
WHERE
    dw.user_id = $1
ORDER BY
    dw.created_at DESC;

-- name: CountUserWins :one
SELECT
    COUNT(*)::INT AS count
FROM
    draw_winners
WHERE
    user_id = $1;

-- ── Verification ──────────────────────────────────────────────────────────────
-- name: GetVerificationData :one
-- Everything the client needs to independently verify the draw.
SELECT
    dr.commitment_hash,
    dr.seed,
    dr.draw_strategy,
    dr.reveal_order,
    dr.winner_count,
    dr.raffle_item_id
FROM
    draw_results dr
WHERE
    dr.raffle_item_id = $1;
