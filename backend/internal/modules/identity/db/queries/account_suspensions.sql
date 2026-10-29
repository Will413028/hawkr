-- name: InsertAccountSuspension :one
-- 與 accounts.suspended_at 在同一個 transaction 寫入；同一帳號未解除的紀錄由 uidx_account_suspensions_active 保證最多一筆。
INSERT INTO identity.account_suspensions (account_id, reason_category, reason_text, source_ref, suspended_at)
VALUES (sqlc.arg(account_id), sqlc.arg(reason_category), sqlc.arg(reason_text), sqlc.narg(source_ref), sqlc.arg(now)::timestamptz)
RETURNING *;

-- name: ReleaseAccountSuspension :one
-- 解除帳號目前未解除的紀錄；沒有未解除的紀錄時沒有列（pgx.ErrNoRows）。
UPDATE identity.account_suspensions
SET released_at = sqlc.arg(now)::timestamptz, release_reason = sqlc.arg(release_reason)
WHERE account_id = sqlc.arg(account_id) AND released_at IS NULL
RETURNING *;

-- name: CurrentAccountSuspension :one
-- 帳號目前未解除的停權紀錄；沒有時沒有列（pgx.ErrNoRows）。
SELECT * FROM identity.account_suspensions WHERE account_id = $1 AND released_at IS NULL;
