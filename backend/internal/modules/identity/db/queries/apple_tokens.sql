-- name: UpsertAppleToken :exec
-- 每個帳號一筆：重新連結時取代。
INSERT INTO identity.apple_tokens (account_id, client_id, token, created_at, updated_at)
VALUES (sqlc.arg(account_id), sqlc.arg(client_id), sqlc.arg(token), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (account_id) DO UPDATE
SET client_id = EXCLUDED.client_id, token = EXCLUDED.token, updated_at = EXCLUDED.updated_at;

-- name: TakeAppleToken :one
-- 刪除帳號時取走保存的 token，交給撤銷的 job；沒有列表示這個帳號沒有交出過 Apple 的憑證。
DELETE FROM identity.apple_tokens WHERE account_id = $1 RETURNING client_id, token;

-- name: ReferencedAppleKeyIDs :many
-- 保存的密文開頭是 1 byte 的金鑰 ID 長度與金鑰 ID：啟動檢查用，資料庫參照的金鑰都要在金鑰環裡。
SELECT DISTINCT convert_from(substring(token FROM 2 FOR get_byte(token, 0)), 'UTF8')::text AS key_id
FROM identity.apple_tokens;
