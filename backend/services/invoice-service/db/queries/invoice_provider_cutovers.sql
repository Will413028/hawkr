
-- name: CloseOpenProviderCutover :execrows
UPDATE invoice_provider_cutovers
SET deactivated_at = sqlc.arg(now)
WHERE provider_account_id = sqlc.arg(provider_account_id)
  AND deactivated_at IS NULL;

-- name: InsertProviderCutover :one
INSERT INTO invoice_provider_cutovers (
  provider_account_id, issuer_id, account_version, cutover_at,
  mode, created_by_account_id, reason, created_at)
VALUES (
  sqlc.arg(provider_account_id), sqlc.arg(issuer_id), sqlc.arg(account_version),
  sqlc.arg(cutover_at), 'AMEGO_AUTO', sqlc.arg(created_by_account_id),
  sqlc.narg(reason), sqlc.arg(now))
RETURNING *;

-- name: ListProviderCutoversForAdmin :many
SELECT * FROM invoice_provider_cutovers
WHERE provider_account_id = sqlc.arg(provider_account_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountProviderCutoversForAdmin :one
SELECT COUNT(*)::bigint FROM invoice_provider_cutovers
WHERE provider_account_id = sqlc.arg(provider_account_id);
