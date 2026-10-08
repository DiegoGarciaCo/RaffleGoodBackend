-- +goose Up
-- +goose StatementBegin
DROP VIEW IF EXISTS v_active_raffles;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE VIEW v_active_raffles AS
SELECT ri.id,
    ri.title,
    ri.slug,
    ri.description,
    ri.status,
    ri.ticket_strategy,
    ri.draw_strategy,
    ri.ticket_price,
    ri.max_tickets,
    ri.tickets_sold,
    ri.draw_at,
    ri.attributes,
    ri.image_urls,
    ri.category_path,
    ri.created_by AS nonprofit_id,
    c.id AS category_id,
    c.name AS category_name,
    c.slug AS category_slug,
    c.icon AS category_icon,
    c.depth AS category_depth,
    n.name AS nonprofit_name            -- NEW: joined org name for browse cards
FROM raffle_items ri
    JOIN categories c ON c.id = ri.category_id
    LEFT JOIN nonprofits n ON n.id = ri.created_by
WHERE ri.status = 'active'::raffle_item_status
    AND c.is_active = true;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS v_active_raffles;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE VIEW v_active_raffles AS
SELECT ri.id,
    ri.title,
    ri.slug,
    ri.description,
    ri.status,
    ri.ticket_strategy,
    ri.draw_strategy,
    ri.ticket_price,
    ri.max_tickets,
    ri.tickets_sold,
    ri.draw_at,
    ri.attributes,
    ri.image_urls,
    ri.category_path,
    ri.created_by AS nonprofit_id,
    c.id AS category_id,
    c.name AS category_name,
    c.slug AS category_slug,
    c.icon AS category_icon,
    c.depth AS category_depth
FROM raffle_items ri
    JOIN categories c ON c.id = ri.category_id
WHERE ri.status = 'active'::raffle_item_status
    AND c.is_active = true;
-- +goose StatementEnd
