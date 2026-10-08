-- sort: 'most_followed' (default) | 'most_raised' | 'most_active' | 'alphabetical'
-- name: ListNonprofits :many
SELECT
    n.id,
    n.name,
    n.description,
    n.image,
    n.ein,
    n.categories,
    n.follower_count,
    n.total_raised_cents,
    COALESCE(v."isVerified", FALSE)::bool AS is_verified,
    COALESCE((
        SELECT COUNT(*) FROM raffle_items ri
        WHERE ri.created_by = n.id
          AND ri.status = 'active'::raffle_item_status
    ), 0)::int AS active_raffles,
    COALESCE((
        SELECT TRUE FROM org_follows f
        WHERE f.nonprofit_id = n.id
          AND f.user_id = sqlc.narg(viewer_id)
    ), FALSE)::bool AS is_following
FROM nonprofits n
LEFT JOIN LATERAL (
    SELECT nv."isVerified"
    FROM nonprofit_verifications nv
    WHERE nv.nonprofit_id = n.id
    ORDER BY nv.created_at DESC
    LIMIT 1
) v ON TRUE
WHERE (
    sqlc.narg(search)::text IS NULL
    OR n.name ILIKE '%' || sqlc.narg(search)::text || '%'
)
ORDER BY
    CASE WHEN sqlc.arg(sort)::text = 'most_raised' THEN n.total_raised_cents END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'most_active' THEN (
        SELECT COUNT(*) FROM raffle_items ri
        WHERE ri.created_by = n.id AND ri.status = 'active'::raffle_item_status
    ) END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'alphabetical' THEN n.name END ASC NULLS LAST,
    n.follower_count DESC,   -- default = most followed
    n.name
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);
