
-- name: GetInvoiceIssuerByID :one
SELECT * FROM invoice_issuers WHERE id = sqlc.arg(id);

-- name: GetInvoiceIssuerByRef :one
SELECT * FROM invoice_issuers
WHERE issuer_type = sqlc.arg(issuer_type) AND issuer_ref_id = sqlc.arg(issuer_ref_id);

-- name: InsertInvoiceIssuer :one
INSERT INTO invoice_issuers (
  id, issuer_type, issuer_ref_id, display_name, status, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(issuer_type), sqlc.arg(issuer_ref_id), sqlc.arg(display_name),
  'active', sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: RetireInvoiceIssuer :execrows
UPDATE invoice_issuers
SET status = 'retired',
    retired_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'active'
  AND version = sqlc.arg(expected_version);

-- name: UpdateInvoiceIssuerDisplayName :execrows
UPDATE invoice_issuers
SET display_name = sqlc.arg(display_name),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'active'
  AND display_name <> sqlc.arg(display_name);
