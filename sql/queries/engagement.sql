-- ── Saved raffles ─────────────────────────────────────────────────────────────
-- name: SaveRaffle :one
INSERT INTO
    saved_raffles (user_id, raffle_item_id)
VALUES
    ($1, $2) ON CONFLICT (user_id, raffle_item_id) DO NOTHING
RETURNING
    *;

-- name: UnsaveRaffle :exec
DELETE FROM
    saved_raffles
WHERE
    user_id = $1
    AND raffle_item_id = $2;

-- name: IsRaffleSaved :one
SELECT
    EXISTS(
        SELECT
            1
        FROM
            saved_raffles
        WHERE
            user_id = $1
            AND raffle_item_id = $2
    ) AS saved;

-- name: ListSavedRaffles :many
SELECT
    ri.*,
    c.icon AS category_icon,
    n.name AS nonprofit_name,
    sr.created_at AS saved_at
FROM
    saved_raffles sr
    JOIN raffle_items ri ON ri.id = sr.raffle_item_id
    JOIN categories c ON c.id = ri.category_id
    LEFT JOIN nonprofits n ON n.id = ri.created_by
WHERE
    sr.user_id = $1
ORDER BY
    sr.created_at DESC;

-- ── Org follows ───────────────────────────────────────────────────────────────
-- name: FollowOrg :one
INSERT INTO
    org_follows (user_id, nonprofit_id)
VALUES
    ($1, $2) ON CONFLICT (user_id, nonprofit_id) DO NOTHING
RETURNING
    *;

-- name: UnfollowOrg :exec
DELETE FROM
    org_follows
WHERE
    user_id = $1
    AND nonprofit_id = $2;

-- name: IsFollowingOrg :one
SELECT
    EXISTS(
        SELECT
            1
        FROM
            org_follows
        WHERE
            user_id = $1
            AND nonprofit_id = $2
    ) AS following;

-- name: ListFollowedOrgs :many
SELECT
    n.*,
    of.created_at AS followed_at,
    COALESCE(v."isVerified", FALSE)::bool AS is_verified,
    (
        SELECT
            COUNT(*)
        FROM
            raffle_items ri
        WHERE
            ri.created_by = n.id
            AND ri.status = 'active'::raffle_item_status
    )::INT AS active_raffles
FROM
    org_follows of
    JOIN nonprofits n ON n.id = of.nonprofit_id
    LEFT JOIN LATERAL (
        SELECT
            nv."isVerified"
        FROM
            nonprofit_verifications nv
        WHERE
            nv.nonprofit_id = n.id
        ORDER BY
            nv.created_at DESC
        LIMIT
            1
    ) v ON TRUE
WHERE
    of.user_id = $1
ORDER BY
    of.created_at DESC;

-- name: ListFollowerIDsForOrg :many
-- For "new raffle from followed org" notifications.
SELECT
    user_id
FROM
    org_follows
WHERE
    nonprofit_id = $1;

-- ── Reviews ───────────────────────────────────────────────────────────────────
-- name: ListReviewsForOrg :many
SELECT
    r.*,
    u.name AS user_name,
    u.image AS user_image
FROM
    nonprofit_reviews r
    JOIN users u ON u.id = r.user_id
WHERE
    r.nonprofit_id = sqlc.arg(nonprofit_id)
ORDER BY
    r.created_at DESC
LIMIT
    sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: GetOrgReviewSummary :one
SELECT
    COUNT(*)::INT AS review_count,
    COALESCE(AVG(rating), 0)::NUMERIC(3, 2) AS average_rating
FROM
    nonprofit_reviews
WHERE
    nonprofit_id = $1;

-- name: UpsertReview :one
INSERT INTO
    nonprofit_reviews (nonprofit_id, user_id, rating, body)
VALUES
    ($1, $2, $3, $4) ON CONFLICT (nonprofit_id, user_id) DO
UPDATE
SET
    rating = EXCLUDED.rating,
    body = EXCLUDED.body
RETURNING
    *;

-- name: DeleteReview :exec
DELETE FROM
    nonprofit_reviews
WHERE
    nonprofit_id = $1
    AND user_id = $2;
