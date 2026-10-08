-- name: GetNonprofitByID :one
SELECT
    *
FROM
    nonprofits
WHERE
    id = $1;

-- name: GetNonprofitBySlug :one
SELECT
    *
FROM
    nonprofits
WHERE
    slug = $1;

-- name: GetNonprofitForUser :one
-- The nonprofit a user manages (via membership).
SELECT
    n.*
FROM
    nonprofits n
    JOIN nonprofit_users nu ON nu.nonprofit_id = n.id
WHERE
    nu.user_id = $1
LIMIT
    1;

-- name: CreateNonprofit :one
INSERT INTO
    nonprofits (
        name,
        slug,
        description,
        image,
        address,
        phone,
        ein,
        "hasToPay",
        website,
        categories
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING
    *;

-- name: UpdateNonprofitProfile :one
UPDATE
    nonprofits
SET
    name = $2,
    description = $3,
    image = $4,
    categories = $5
WHERE
    id = $1
RETURNING
    *;

-- name: UpdateNonprofitContact :one
UPDATE
    nonprofits
SET
    website = $2,
    phone = $3,
    address = $4
WHERE
    id = $1
RETURNING
    *;

-- name: GetNonprofitVerification :one
SELECT
    *
FROM
    nonprofit_verifications
WHERE
    nonprofit_id = $1
ORDER BY
    created_at DESC
LIMIT
    1;

-- name: SearchNonprofits :many
SELECT
    *
FROM
    nonprofits
WHERE
    name ILIKE '%' || sqlc.arg(query) || '%'
ORDER BY
    follower_count DESC
LIMIT
    sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- ── Team members ───────────────────────────────────────────────────────────────
-- name: ListNonprofitTeam :many
SELECT
    nu.*,
    u.name AS user_name,
    u.email AS user_email
FROM
    nonprofit_users nu
    JOIN users u ON u.id = nu.user_id
WHERE
    nu.nonprofit_id = $1
ORDER BY
    nu.created_at;

-- name: AddTeamMember :one
INSERT INTO
    nonprofit_users (nonprofit_id, user_id, role, MODE)
VALUES
    ($1, $2, $3, $4)
RETURNING
    *;

-- name: UpdateTeamMemberRole :one
UPDATE
    nonprofit_users
SET
    role = $3
WHERE
    nonprofit_id = $1
    AND user_id = $2
RETURNING
    *;

-- name: RemoveTeamMember :exec
DELETE FROM
    nonprofit_users
WHERE
    nonprofit_id = $1
    AND user_id = $2;

-- name: GetUserRoleInNonprofit :one
SELECT
    role
FROM
    nonprofit_users
WHERE
    nonprofit_id = $1
    AND user_id = $2;

-- ── Social links ─────────────────────────────────────────────────────────────
-- name: ListSocialLinks :many
SELECT
    *
FROM
    nonprofit_social_links
WHERE
    nonprofit_id = $1;

-- name: ReplaceSocialLink :one
INSERT INTO
    nonprofit_social_links (nonprofit_id, platform, url)
VALUES
    ($1, $2, $3)
RETURNING
    *;

-- name: DeleteSocialLinks :exec
DELETE FROM
    nonprofit_social_links
WHERE
    nonprofit_id = $1;

-- ── Bank account & payouts ───────────────────────────────────────────────────
-- name: GetBankAccount :one
SELECT
    *
FROM
    nonprofit_bank_accounts
WHERE
    nonprofit_id = $1
ORDER BY
    created_at DESC
LIMIT
    1;

-- name: UpsertBankAccount :one
INSERT INTO
    nonprofit_bank_accounts (
        nonprofit_id,
        processor,
        processor_account_id,
        bank_name,
        last4,
        account_type,
        is_verified
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7)
RETURNING
    *;

-- name: ListPayouts :many
SELECT
    *
FROM
    payouts
WHERE
    nonprofit_id = sqlc.arg(nonprofit_id)
ORDER BY
    created_at DESC
LIMIT
    sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: CreatePayout :one
INSERT INTO
    payouts (
        nonprofit_id,
        raffle_item_id,
        amount_cents,
        STATUS,
        processor_payout_id
    )
VALUES
    ($1, $2, $3, $4, $5)
RETURNING
    *;

-- name: MarkPayoutPaid :exec
UPDATE
    payouts
SET
    STATUS = 'paid',
    paid_at = NOW()
WHERE
    id = $1;

-- ── Dashboard / Org all-time stats ────────────────────────────────────────────
-- name: GetNonprofitAllTimeStats :one
SELECT
    n.id AS org_id,
    n.name AS org_name,
    n.total_raised_cents,
    n.raffles_run,
    n.follower_count,
    COALESCE(
        (
            SELECT
                COUNT(*)
            FROM
                tickets t
                JOIN raffle_items ri ON ri.id = t.raffle_item_id
            WHERE
                ri.created_by = n.id
        ),
        0
    ) AS total_tickets_sold
FROM
    nonprofits n
WHERE
    n.id = $1;

-- name: GetNonprofitMonthStats :one
-- Raised + tickets in a [window_start, window_end) range.
SELECT
    COALESCE(SUM(t.price_paid), 0)::NUMERIC AS raised,
    COUNT(*)::INT AS tickets_sold
FROM
    tickets t
    JOIN raffle_items ri ON ri.id = t.raffle_item_id
WHERE
    ri.created_by = sqlc.arg(nonprofit_id)
    AND t.purchased_at >= sqlc.arg(window_start)
    AND t.purchased_at < sqlc.arg(window_end);

-- name: GetTicketsSoldSince :one
-- For "tickets today" / "tickets in last hour" dashboard metrics.
SELECT
    COUNT(*)::INT AS count
FROM
    tickets t
    JOIN raffle_items ri ON ri.id = t.raffle_item_id
WHERE
    ri.created_by = sqlc.arg(nonprofit_id)
    AND t.purchased_at >= sqlc.arg(since);
