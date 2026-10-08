-- name: GetRaffleByID :one
SELECT
    ri.*,
    c.name AS category_name,
    c.slug AS category_slug,
    c.icon AS category_icon,
    n.name AS nonprofit_name,
    n.slug AS nonprofit_slug,
    n.image AS nonprofit_image
FROM
    raffle_items ri
    JOIN categories c ON c.id = ri.category_id
    LEFT JOIN nonprofits n ON n.id = ri.created_by
WHERE
    ri.id = $1;

-- name: GetRaffleBySlug :one
SELECT
    ri.*,
    c.name AS category_name,
    c.slug AS category_slug,
    c.icon AS category_icon,
    n.name AS nonprofit_name,
    n.slug AS nonprofit_slug,
    n.image AS nonprofit_image
FROM
    raffle_items ri
    JOIN categories c ON c.id = ri.category_id
    LEFT JOIN nonprofits n ON n.id = ri.created_by
WHERE
    ri.slug = sqlc.arg(slug)::text;

-- ── Browse / feed (participant) ──────────────────────────────────────────────
-- sort: 'ending_soon' (default) | 'newest' | 'most_funded' | 'price_asc' | 'price_desc'
-- name: ListActiveRaffles :many
SELECT
    *
FROM
    v_active_raffles
ORDER BY
    CASE WHEN sqlc.arg(sort)::text = 'most_funded' THEN tickets_sold END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'price_asc'   THEN ticket_price END ASC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'price_desc'  THEN ticket_price END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'newest'      THEN created_at   END DESC NULLS LAST,
    draw_at ASC NULLS LAST   -- default = ending soon
LIMIT
    sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: ListFeaturedRaffles :many
-- Hero / featured: active raffles with the most tickets sold.
SELECT
    *
FROM
    v_active_raffles
ORDER BY
    tickets_sold DESC
LIMIT
    sqlc.arg(result_limit);

-- name: ListEndingSoonRaffles :many
SELECT
    *
FROM
    v_active_raffles
WHERE
    draw_at IS NOT NULL
    AND draw_at > NOW()
ORDER BY
    draw_at ASC
LIMIT
    sqlc.arg(result_limit);

-- name: ListRafflesByCategory :many
SELECT
    *
FROM
    v_active_raffles
WHERE
    category_path LIKE sqlc.arg(category_path_prefix)::text || '%'
ORDER BY
    CASE WHEN sqlc.arg(sort)::text = 'most_funded' THEN tickets_sold END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'price_asc'   THEN ticket_price END ASC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'price_desc'  THEN ticket_price END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'newest'      THEN created_at   END DESC NULLS LAST,
    tickets_sold DESC   -- default within a category = most funded
LIMIT
    sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: SearchRaffles :many
-- Full-text search on title + description.
SELECT
    ri.*,
    c.name AS category_name,
    c.icon AS category_icon
FROM
    raffle_items ri
    JOIN categories c ON c.id = ri.category_id
WHERE
    ri.status = 'active'
    AND to_tsvector(
        'english',
        coalesce(ri.title, '') || ' ' || coalesce(ri.description, '')
    ) @@ plainto_tsquery('english', sqlc.arg(query))
ORDER BY
    ri.tickets_sold DESC
LIMIT
    sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: FilterRafflesByAttribute :many
-- JSONB attribute filter, e.g. attributes @> '{"brand":"Nike"}'.
SELECT
    *
FROM
    v_active_raffles
WHERE
    attributes @> sqlc.arg(attribute_filter)
ORDER BY
    tickets_sold DESC
LIMIT
    sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- ── Nonprofit's own raffles (manage screen) ───────────────────────────────────
-- name: ListRafflesForNonprofit :many
SELECT
    ri.*,
    c.name AS category_name,
    c.icon AS category_icon,
    COALESCE(dr.winner_count, 0)::INT AS winner_count,
    COALESCE(
        (
            SELECT
                bool_and(dw.prize_claimed)
            FROM
                draw_winners dw
            WHERE
                dw.draw_result_id = dr.id
        ),
        FALSE
    ) AS prize_claimed
FROM
    raffle_items ri
    JOIN categories c ON c.id = ri.category_id
    LEFT JOIN draw_results dr ON dr.raffle_item_id = ri.id
WHERE
    ri.created_by = sqlc.arg(nonprofit_id)
ORDER BY
    ri.created_at DESC
LIMIT
    sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: ListRafflesForNonprofitByStatus :many
SELECT
    ri.*,
    c.name AS category_name,
    c.icon AS category_icon,
    COALESCE(dr.winner_count, 0)::INT AS winner_count,
    COALESCE(
        (
            SELECT
                bool_and(dw.prize_claimed)
            FROM
                draw_winners dw
            WHERE
                dw.draw_result_id = dr.id
        ),
        FALSE
    ) AS prize_claimed
FROM
    raffle_items ri
    JOIN categories c ON c.id = ri.category_id
    LEFT JOIN draw_results dr ON dr.raffle_item_id = ri.id
WHERE
    ri.created_by = sqlc.arg(nonprofit_id)
    AND ri.status = sqlc.arg(STATUS)
ORDER BY
    ri.created_at DESC
LIMIT
    sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: CountRafflesByStatusForNonprofit :many
-- For the manage-raffles header counts.
SELECT
    STATUS,
    COUNT(*)::INT AS count
FROM
    raffle_items
WHERE
    created_by = $1
GROUP BY
    STATUS;

-- name: ListActiveRafflesForNonprofit :many
-- Dashboard "active raffles" section.
SELECT
    ri.*,
    c.name AS category_name
FROM
    raffle_items ri
    JOIN categories c ON c.id = ri.category_id
WHERE
    ri.created_by = sqlc.arg(nonprofit_id)
    AND ri.status IN ('active', 'drawing')
ORDER BY
    ri.draw_at NULLS LAST
LIMIT
    sqlc.arg(result_limit);

-- ── Create / update / lifecycle ───────────────────────────────────────────────
-- name: CreateRaffle :one
INSERT INTO
    raffle_items (
        category_id,
        title,
        slug,
        description,
        STATUS,
        ticket_strategy,
        ticket_assignment_strategy,
        draw_strategy,
        ticket_price,
        free_tickets,
        free_ticket_start_range,
        free_ticket_end_range,
        max_tickets,
        attributes,
        image_urls,
        draw_at,
        created_by
    )
VALUES
    (
        $1,
        $2,
        $3,
        $4,
        $5,
        $6,
        $7,
        $8,
        $9,
        $10,
        $11,
        $12,
        $13,
        $14,
        $15,
        $16,
        $17
    )
RETURNING
    *;

-- name: UpdateRaffle :one
UPDATE
    raffle_items
SET
    title = sqlc.arg(title),
    description = sqlc.arg(description),
    ticket_strategy = sqlc.arg(ticket_strategy),
    ticket_assignment_strategy = sqlc.arg(ticket_assignment_strategy),
    draw_strategy = sqlc.arg(draw_strategy),
    ticket_price = sqlc.arg(ticket_price),
    free_tickets = sqlc.arg(free_tickets),
    free_ticket_start_range = sqlc.arg(free_ticket_start_range),
    free_ticket_end_range = sqlc.arg(free_ticket_end_range),
    max_tickets = sqlc.arg(max_tickets),
    attributes = sqlc.arg(attributes),
    image_urls = sqlc.arg(image_urls)
WHERE
    id = sqlc.arg(raffle_id)
    AND created_by = sqlc.arg(nonprofit_id)
RETURNING
    *;

-- name: PublishRaffle :one
UPDATE
    raffle_items
SET
    STATUS = 'active'
WHERE
    id = sqlc.arg(raffle_id)
    AND created_by = sqlc.arg(nonprofit_id)
    AND STATUS = 'draft'
RETURNING
    *;

-- name: SetRaffleStatus :one
UPDATE
    raffle_items
SET
    STATUS = $2
WHERE
    id = $1
RETURNING
    *;

-- name: SetRaffleDrawAt :one
-- Called when the raffle sells out and a draw is scheduled.
UPDATE
    raffle_items
SET
    draw_at = $2
WHERE
    id = $1
RETURNING
    *;

-- name: DeleteRaffle :exec
DELETE FROM
    raffle_items
WHERE
    id = sqlc.arg(raffle_id)
    AND created_by = sqlc.arg(nonprofit_id)
    AND STATUS = 'draft';

-- name: SlugExists :one
SELECT
    EXISTS(
        SELECT
            1
        FROM
            raffle_items
        WHERE
            slug = $1
    ) AS EXISTS;

-- name: ListRafflesReadyToDraw :many
-- For the draw worker: active raffles whose draw_at has passed.
SELECT
    *
FROM
    raffle_items
WHERE
    STATUS = 'active'
    AND draw_at IS NOT NULL
    AND draw_at <= NOW();

