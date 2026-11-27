-- name: GetIdentity :one
SELECT * FROM identity.account_identities WHERE provider = $1 AND subject = $2;

-- name: InsertIdentity :one
INSERT INTO identity.account_identities (provider, subject, account_id, email, email_verified, auto_link_eligible, created_at, updated_at)
VALUES (sqlc.arg(provider), sqlc.arg(subject), sqlc.arg(account_id), sqlc.narg(email), sqlc.arg(email_verified), sqlc.arg(auto_link_eligible), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (provider, subject) DO NOTHING
RETURNING account_id;

-- name: UpdateIdentityClaims :exec
UPDATE identity.account_identities
SET email = sqlc.narg(email), email_verified = sqlc.arg(email_verified), auto_link_eligible = sqlc.arg(auto_link_eligible), updated_at = sqlc.arg(now)::timestamptz
WHERE provider = sqlc.arg(provider) AND subject = sqlc.arg(subject);

-- name: ScrubIdentitiesOfAccount :exec
UPDATE identity.account_identities
SET email = NULL, email_verified = FALSE, auto_link_eligible = FALSE, updated_at = sqlc.arg(now)::timestamptz
WHERE account_id = sqlc.arg(account_id);

-- name: FindAutoLinkAccountIDsByEmail :many
SELECT DISTINCT i.account_id FROM identity.account_identities i
JOIN identity.accounts a ON a.id = i.account_id
WHERE a.deleted_at IS NULL AND i.provider = 'GOOGLE' AND i.email_verified AND i.auto_link_eligible
  AND lower(i.email) = lower(sqlc.arg(email)::text)
ORDER BY i.account_id
LIMIT 2;

-- name: ListIdentitiesOfAccount :many
SELECT provider, created_at FROM identity.account_identities
WHERE account_id = $1
ORDER BY created_at, provider, subject;
