-- name: ListNonprofitRaffles :many
-- Public list of one nonprofit's raffles, for the org profile screen.
--   status_filter: 'active' | 'past' | 'all'
-- Drafts and cancelled raffles are never returned (this is the public view).
-- Reads straight from raffle_items — the client adapter only needs these columns,
-- so no category/nonprofit joins are required here.
SELECT
    id,
    title,
    slug,
    description,
    status,
    ticket_strategy,
    ticket_assignment_strategy,
    draw_strategy,
    ticket_price,
    max_tickets,
    tickets_sold,
    attributes,
    image_urls,
    draw_at,
    drawn_at,
    category_id,
    category_path,
    created_at,
    updated_at
FROM raffle_items
WHERE created_by = sqlc.arg(nonprofit_id)::uuid
  AND (
    CASE sqlc.arg(status_filter)::text
        WHEN 'active' THEN status = 'active'
        WHEN 'past'   THEN status = 'completed'
        ELSE status IN ('active', 'completed')
    END
  )
ORDER BY
    -- active raffles: ending soonest first; everything else: newest first
    CASE WHEN status = 'active' THEN draw_at END ASC NULLS LAST,
    created_at DESC
LIMIT sqlc.arg(result_limit)
OFFSET sqlc.arg(result_offset);
