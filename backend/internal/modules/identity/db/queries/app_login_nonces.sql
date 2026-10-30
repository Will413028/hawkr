-- name: InsertAppLoginNonce :exec
INSERT INTO identity.app_login_nonces (nonce_hash, created_at, expires_at)
VALUES (sqlc.arg(nonce_hash), sqlc.arg(now), sqlc.arg(expires_at));

-- name: ConsumeAppLoginNonce :one
-- 單次使用：取出同時刪除，逾時的判斷在呼叫方。
DELETE FROM identity.app_login_nonces WHERE nonce_hash = $1 RETURNING expires_at;

-- name: DeleteExpiredAppLoginNonces :execrows
DELETE FROM identity.app_login_nonces WHERE expires_at <= $1;
