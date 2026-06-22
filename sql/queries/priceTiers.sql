-- name: ListPrizeTiers :many
SELECT
    *
FROM
    prize_tiers
WHERE
    raffle_item_id = $1
ORDER BY
    rank;

-- name: CreatePrizeTier :one
INSERT INTO
    prize_tiers (
        raffle_item_id,
        rank,
        title,
        description,
        value_cents,
        image_url
    )
VALUES
    ($1, $2, $3, $4, $5, $6)
RETURNING
    *;

-- name: DeletePrizeTiersForRaffle :exec
DELETE FROM
    prize_tiers
WHERE
    raffle_item_id = $1;
