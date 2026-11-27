-- name: InsertSession :one
INSERT INTO identity.sessions (token_hash, account_id, client, created_at, last_used_at, absolute_expires_at, auth_time)
VALUES (sqlc.arg(token_hash), sqlc.arg(account_id), sqlc.arg(client), sqlc.arg(now), sqlc.arg(now), sqlc.arg(absolute_expires_at), sqlc.arg(now))
RETURNING id;

-- name: ResolveSession :one
SELECT s.id, s.account_id, s.client, s.last_used_at, s.absolute_expires_at,
       a.suspended_at, a.deleted_at,
       ARRAY(SELECT r.role FROM identity.account_roles r WHERE r.account_id = a.id ORDER BY r.role)::text[] AS roles
FROM identity.sessions s
JOIN identity.accounts a ON a.id = s.account_id
WHERE s.token_hash = $1;

-- name: RenewSession :execrows
UPDATE identity.sessions
SET last_used_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND last_used_at <= sqlc.arg(renew_before)::timestamptz;

-- name: DeleteSessionByTokenHash :execrows
DELETE FROM identity.sessions WHERE token_hash = $1;

-- name: DeleteSessionOfAccount :execrows
DELETE FROM identity.sessions WHERE id = $1 AND account_id = $2;

-- name: DeleteSessionsOfAccount :execrows
DELETE FROM identity.sessions WHERE account_id = $1;

-- name: DeleteExpiredSessionsOfAccount :execrows
DELETE FROM identity.sessions
WHERE account_id = sqlc.arg(account_id)
  AND (absolute_expires_at <= sqlc.arg(now)::timestamptz
       OR (client = 'WEB' AND last_used_at <= sqlc.arg(web_idle_before)::timestamptz)
       OR (client = 'APP' AND last_used_at <= sqlc.arg(app_idle_before)::timestamptz));

-- name: ListSessionsOfAccount :many
SELECT id, account_id, client, created_at, last_used_at, absolute_expires_at, auth_time
FROM identity.sessions
WHERE account_id = $1
ORDER BY created_at DESC, id DESC;

-- name: GetSessionOfAccount :one
SELECT id, account_id, client, created_at, last_used_at, absolute_expires_at, auth_time
FROM identity.sessions
WHERE id = $1 AND account_id = $2;
