-- name: InsertLoginFlow :exec
INSERT INTO identity.login_flows (state_hash, provider, code_verifier, nonce, return_path, binding_hash, link_account_id, created_at, expires_at)
VALUES (sqlc.arg(state_hash), sqlc.arg(provider), sqlc.arg(code_verifier), sqlc.arg(nonce), sqlc.arg(return_path), sqlc.arg(binding_hash),
        sqlc.narg(link_account_id), sqlc.arg(now), sqlc.arg(expires_at));

-- name: ConsumeLoginFlow :one
-- 單次使用：取出同時刪除；之後的比對（逾時、provider、登入前 cookie）失敗也不會留下這一列。
DELETE FROM identity.login_flows WHERE state_hash = $1 RETURNING *;

-- name: DeleteExpiredLoginFlows :execrows
DELETE FROM identity.login_flows WHERE expires_at <= $1;
