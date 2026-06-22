-- name: GetCategoryByID :one
SELECT
    *
FROM
    categories
WHERE
    id = $1
    AND is_active = TRUE;

-- name: GetCategoryBySlug :one
SELECT
    *
FROM
    categories
WHERE
    slug = $1
    AND is_active = TRUE;

-- name: ListRootCategories :many
SELECT
    *
FROM
    categories
WHERE
    parent_id IS NULL
    AND is_active = TRUE
ORDER BY
    sort_order,
    name;

-- name: ListChildCategories :many
SELECT
    *
FROM
    categories
WHERE
    parent_id = $1
    AND is_active = TRUE
ORDER BY
    sort_order,
    name;

-- name: ListAllActiveCategories :many
SELECT
    *
FROM
    categories
WHERE
    is_active = TRUE
ORDER BY
    depth,
    sort_order,
    name;

-- name: ListCategoryItemCounts :many
-- Powers the browse sidebar (item counts include subcategories).
SELECT
    *
FROM
    v_category_item_counts;

-- name: SearchCategoriesByName :many
SELECT
    *
FROM
    categories
WHERE
    is_active = TRUE
    AND name ILIKE '%' || sqlc.arg(query) || '%'
ORDER BY
    name
LIMIT
    sqlc.arg(result_limit);

-- name: CreateCategory :one
INSERT INTO
    categories (
        parent_id,
        path,
        depth,
        name,
        slug,
        icon,
        sort_order,
        attribute_schema
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING
    *;

-- name: UpdateCategory :one
UPDATE
    categories
SET
    name = $2,
    icon = $3,
    sort_order = $4,
    attribute_schema = $5
WHERE
    id = $1
RETURNING
    *;

-- name: DeactivateCategory :exec
UPDATE
    categories
SET
    is_active = FALSE
WHERE
    id = $1;
