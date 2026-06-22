-- name: GetSessionByToken :one
-- Validates a better-auth session. Columns are aliased to snake_case so sqlc
-- generates clean Go field names from the camelCase better-auth schema.
SELECT
    s.id AS session_id,
    s."userId" AS user_id,
    s."expiresAt" AS expires_at,
    u.name AS user_name,
    u.email AS user_email
FROM
    "session" s
    JOIN users u ON u.id = s."userId"
WHERE
    s.token = sqlc.arg(token);

-- name: GetUserMembership :one
-- The nonprofit a user manages, if any. No row = participant (not an org member).
-- A user could belong to several orgs; we take one for now (org-switching later).
SELECT
    nonprofit_id,
    role
FROM
    nonprofit_users
WHERE
    user_id = sqlc.arg(user_id)
ORDER BY
    created_at
LIMIT
    1;
