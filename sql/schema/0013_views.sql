-- +goose Up
-- v_active_raffles: joins active items with their category for list/detail endpoints.
-- +goose StatementBegin
CREATE
OR REPLACE VIEW v_active_raffles AS
SELECT
    ri.id,
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
FROM
    raffle_items ri
    JOIN categories c ON c.id = ri.category_id
WHERE
    ri.status = 'active'
    AND c.is_active = TRUE;

-- +goose StatementEnd
-- v_category_item_counts: powers the browse sidebar. The LIKE (path || '%') join
-- counts items in a category AND all its subcategories without recursion.
-- +goose StatementBegin
CREATE
OR REPLACE VIEW v_category_item_counts AS
SELECT
    c.id,
    c.name,
    c.slug,
    c.path,
    c.parent_id,
    c.depth,
    c.icon,
    c.sort_order,
    c.attribute_schema,
    COUNT(ri.id) AS active_item_count
FROM
    categories c
    LEFT JOIN raffle_items ri ON ri.category_path LIKE (c.path || '%')
    AND ri.status = 'active'
WHERE
    c.is_active = TRUE
GROUP BY
    c.id,
    c.name,
    c.slug,
    c.path,
    c.parent_id,
    c.depth,
    c.icon,
    c.sort_order,
    c.attribute_schema
ORDER BY
    c.depth,
    c.sort_order,
    c.name;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS v_category_item_counts;

-- +goose StatementEnd
-- +goose StatementBegin
DROP VIEW IF EXISTS v_active_raffles;

-- +goose StatementEnd
