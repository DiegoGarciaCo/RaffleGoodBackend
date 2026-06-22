-- name: CreateDrawResultWithSeed :one
-- Used at publish time: stores the secret seed alongside the public commitment.
-- (The API never returns seed until status = 'completed'.) Also used as a dev
-- fallback by the worker if a raffle reached its draw time without a result row.
INSERT INTO
    draw_results (
        raffle_item_id,
        draw_strategy,
        reveal_order,
        winner_count,
        commitment_hash,
        seed,
        total_tickets,
        total_participants,
        STATUS
    )
VALUES
    (
        sqlc.arg(raffle_item_id),
        sqlc.arg(draw_strategy),
        sqlc.arg(reveal_order),
        sqlc.arg(winner_count),
        sqlc.arg(commitment_hash),
        sqlc.arg(seed),
        sqlc.arg(total_tickets),
        sqlc.arg(total_participants),
        'scheduled'
    )
RETURNING
    *;

-- name: MarkRaffleDrawing :exec
UPDATE
    raffle_items
SET
    STATUS = 'drawing'
WHERE
    id = sqlc.arg(id);

-- name: MarkRaffleDrawn :exec
UPDATE
    raffle_items
SET
    STATUS = 'completed',
    drawn_at = NOW()
WHERE
    id = sqlc.arg(id);
