-- name: GetIdentity :one
SELECT * FROM identity.account_identities WHERE provider = $1 AND subject = $2;

-- name: InsertIdentity :one
-- 同一個 (provider, subject) 並發連結時只有一個成功：沒有列回傳表示它已存在，呼叫方放棄這個 transaction 重來。
INSERT INTO identity.account_identities (provider, subject, account_id, email, email_verified, created_at, updated_at)
VALUES (sqlc.arg(provider), sqlc.arg(subject), sqlc.arg(account_id), sqlc.narg(email), sqlc.arg(email_verified), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (provider, subject) DO NOTHING
RETURNING account_id;

-- name: UpdateIdentityClaims :exec
-- 每次登入以 provider 最近回報的 email 更新。
UPDATE identity.account_identities
SET email = sqlc.narg(email), email_verified = sqlc.arg(email_verified), updated_at = sqlc.arg(now)::timestamptz
WHERE provider = sqlc.arg(provider) AND subject = sqlc.arg(subject);

-- name: DeleteIdentitiesOfAccount :exec
DELETE FROM identity.account_identities WHERE account_id = $1;
