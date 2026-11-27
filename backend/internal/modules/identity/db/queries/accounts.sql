-- name: GetAccount :one
SELECT * FROM identity.accounts WHERE id = $1;

-- name: GetAccounts :many
SELECT * FROM identity.accounts WHERE id = ANY(sqlc.arg(ids)::bigint[]) ORDER BY id;

-- name: LockLiveAccount :one
SELECT * FROM identity.accounts WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: ShareLockLiveAccount :one
SELECT id FROM identity.accounts WHERE id = $1 AND deleted_at IS NULL FOR SHARE;

-- name: SuspendAccount :execrows
UPDATE identity.accounts
SET suspended_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND suspended_at IS NULL;

-- name: UnsuspendAccount :execrows
UPDATE identity.accounts
SET suspended_at = NULL, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND suspended_at IS NOT NULL;

-- name: DeleteAccount :execrows
UPDATE identity.accounts
SET deleted_at = sqlc.arg(now)::timestamptz, email = NULL, self_entered_email = NULL, suspended_at = NULL, deletion_pending_at = NULL,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: MarkDeletionPending :execrows
UPDATE identity.accounts
SET deletion_pending_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND deletion_pending_at IS NULL;

-- name: ListAccounts :many
SELECT a.* FROM identity.accounts a
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(role)::text = ''
       OR EXISTS (SELECT 1 FROM identity.account_roles r WHERE r.account_id = a.id AND r.role = sqlc.arg(role)::text))
  AND (sqlc.arg(pattern)::text = '' OR a.id = sqlc.arg(account_id)::bigint OR a.email ILIKE sqlc.arg(pattern)::text)
  AND (sqlc.arg(email)::text = '' OR lower(a.email) = sqlc.arg(email)::text)
  AND (NOT sqlc.arg(has_cursor)::bool
       OR (a.created_at, a.id) < (sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::bigint))
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(max_rows);

-- name: CountAccounts :one
SELECT count(*) FROM identity.accounts a
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(role)::text = ''
       OR EXISTS (SELECT 1 FROM identity.account_roles r WHERE r.account_id = a.id AND r.role = sqlc.arg(role)::text))
  AND (sqlc.arg(pattern)::text = '' OR a.id = sqlc.arg(account_id)::bigint OR a.email ILIKE sqlc.arg(pattern)::text)
  AND (sqlc.arg(email)::text = '' OR lower(a.email) = sqlc.arg(email)::text);

-- name: InsertFederatedAccount :one
INSERT INTO identity.accounts (email, created_at, updated_at)
VALUES (sqlc.narg(email), sqlc.arg(now), sqlc.arg(now))
RETURNING id;

-- name: UpdateAccountEmail :execrows
UPDATE identity.accounts
SET email = sqlc.arg(email), updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND email IS DISTINCT FROM sqlc.arg(email);


-- name: SetSelfEnteredEmail :execrows
UPDATE identity.accounts
SET self_entered_email = sqlc.arg(self_entered_email), updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SetPreferredLocale :execrows
UPDATE identity.accounts
SET preferred_locale = sqlc.arg(locale), updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND preferred_locale <> sqlc.arg(locale);
