-- name: InsertAccountSuspension :one
INSERT INTO identity.account_suspensions (account_id, reason_category, reason_text, source_ref, suspended_at)
VALUES (sqlc.arg(account_id), sqlc.arg(reason_category), sqlc.arg(reason_text), sqlc.narg(source_ref), sqlc.arg(now)::timestamptz)
RETURNING *;

-- name: ReleaseAccountSuspension :one
UPDATE identity.account_suspensions
SET released_at = sqlc.arg(now)::timestamptz, release_reason = sqlc.arg(release_reason)
WHERE account_id = sqlc.arg(account_id) AND released_at IS NULL
RETURNING *;

-- name: CurrentAccountSuspension :one
SELECT * FROM identity.account_suspensions WHERE account_id = $1 AND released_at IS NULL;
