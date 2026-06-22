-- ── Payment methods ───────────────────────────────────────────────────────────
-- name: ListPaymentMethods :many
SELECT
    *
FROM
    payment_methods
WHERE
    user_id = $1
ORDER BY
    is_default DESC,
    created_at DESC;

-- name: GetDefaultPaymentMethod :one
SELECT
    *
FROM
    payment_methods
WHERE
    user_id = $1
    AND is_default = TRUE
LIMIT
    1;

-- name: AddPaymentMethod :one
INSERT INTO
    payment_methods (
        user_id,
        processor,
        processor_pm_id,
        brand,
        last4,
        exp_month,
        exp_year,
        is_default
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING
    *;

-- name: SetDefaultPaymentMethod :exec
-- Clear other defaults, then set this one (call in a tx).
UPDATE
    payment_methods
SET
    is_default = (id = sqlc.arg(payment_method_id))
WHERE
    user_id = sqlc.arg(user_id);

-- name: DeletePaymentMethod :exec
DELETE FROM
    payment_methods
WHERE
    id = sqlc.arg(payment_method_id)
    AND user_id = sqlc.arg(user_id);

-- ── Shipping addresses ────────────────────────────────────────────────────────
-- name: ListShippingAddresses :many
SELECT
    *
FROM
    shipping_addresses
WHERE
    user_id = $1
ORDER BY
    is_default DESC,
    created_at DESC;

-- name: GetDefaultShippingAddress :one
SELECT
    *
FROM
    shipping_addresses
WHERE
    user_id = $1
    AND is_default = TRUE
LIMIT
    1;

-- name: AddShippingAddress :one
INSERT INTO
    shipping_addresses (
        user_id,
        line1,
        line2,
        city,
        state,
        zip,
        country,
        is_default
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING
    *;

-- name: UpdateShippingAddress :one
UPDATE
    shipping_addresses
SET
    line1 = sqlc.arg(line1),
    line2 = sqlc.arg(line2),
    city = sqlc.arg(city),
    state = sqlc.arg(state),
    zip = sqlc.arg(zip),
    country = sqlc.arg(country)
WHERE
    id = sqlc.arg(address_id)
    AND user_id = sqlc.arg(user_id)
RETURNING
    *;

-- name: SetDefaultShippingAddress :exec
UPDATE
    shipping_addresses
SET
    is_default = (id = sqlc.arg(address_id))
WHERE
    user_id = sqlc.arg(user_id);

-- name: DeleteShippingAddress :exec
DELETE FROM
    shipping_addresses
WHERE
    id = sqlc.arg(address_id)
    AND user_id = sqlc.arg(user_id);

-- ── Notification preferences ──────────────────────────────────────────────────
-- name: GetUserNotificationPrefs :one
SELECT
    *
FROM
    notification_preferences
WHERE
    user_id = $1;

-- name: UpsertUserNotificationPrefs :one
INSERT INTO
    notification_preferences (user_id, prefs)
VALUES
    ($1, $2) ON CONFLICT (user_id) DO
UPDATE
SET
    prefs = EXCLUDED.prefs,
    updated_at = NOW()
RETURNING
    *;

-- name: GetOrgNotificationPrefs :one
SELECT
    *
FROM
    notification_preferences
WHERE
    nonprofit_id = $1;

-- name: UpsertOrgNotificationPrefs :one
INSERT INTO
    notification_preferences (nonprofit_id, prefs)
VALUES
    ($1, $2) ON CONFLICT (nonprofit_id) DO
UPDATE
SET
    prefs = EXCLUDED.prefs,
    updated_at = NOW()
RETURNING
    *;

-- ── User profile basics + stats ───────────────────────────────────────────────
-- name: UpdateUserPreferredCategories :exec
UPDATE
    users
SET
    preferred_categories = $2
WHERE
    id = $1;

-- name: UpdateUserLocationRegion :exec
UPDATE
    users
SET
    location_region = $2
WHERE
    id = $1;
