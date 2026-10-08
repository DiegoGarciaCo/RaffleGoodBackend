-- Verification + fee-flag queries. Place this file in your sqlc queries
-- directory (alongside the nonprofits queries) and run `sqlc generate`.

-- name: CreateNonprofitVerification :one
-- Records a verification attempt. We insert a new row each time (history);
-- GetNonprofitVerification already returns the most recent one.
INSERT INTO
    nonprofit_verifications (
        nonprofit_id,
        "isVerified",
        "verificationMethod",
        "exemptStatus",
        subsection,
        "nteeCd",
        "rulingDate",
        foundation
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING
    *;

-- name: SetNonprofitHasToPay :one
UPDATE
    nonprofits
SET
    "hasToPay" = $2
WHERE
    id = $1
RETURNING
    *;

-- name: SetNonprofitEIN :one
UPDATE
    nonprofits
SET
    ein = $2
WHERE
    id = $1
RETURNING
    *;
