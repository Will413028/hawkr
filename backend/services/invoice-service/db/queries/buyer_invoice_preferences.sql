
-- name: GetBuyerInvoicePreferenceByID :one
SELECT * FROM buyer_invoice_preferences
WHERE id = sqlc.arg(id)
  AND buyer_id = sqlc.arg(buyer_id)
  AND deleted_at IS NULL
  AND superseded_at IS NULL;

-- name: GetDefaultBuyerInvoicePreference :one
SELECT * FROM buyer_invoice_preferences
WHERE buyer_id = sqlc.arg(buyer_id)
  AND is_default = true
  AND deleted_at IS NULL
  AND superseded_at IS NULL
LIMIT 1;

-- name: ListBuyerInvoicePreferences :many
SELECT * FROM buyer_invoice_preferences
WHERE buyer_id = sqlc.arg(buyer_id)
  AND deleted_at IS NULL
  AND superseded_at IS NULL
ORDER BY created_at DESC
LIMIT sqlc.arg(row_limit);

-- name: InsertBuyerInvoicePreference :one
INSERT INTO buyer_invoice_preferences (
  id, buyer_id, preference_kind, carrier_type,
  carrier_value_ciphertext, carrier_value_hash, carrier_value_mask,
  tax_id, tax_title, donation_code, label, is_default, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(buyer_id), sqlc.arg(preference_kind), sqlc.narg(carrier_type),
  sqlc.narg(carrier_value_ciphertext), sqlc.narg(carrier_value_hash), sqlc.narg(carrier_value_mask),
  sqlc.narg(tax_id), sqlc.narg(tax_title), sqlc.narg(donation_code), sqlc.narg(label),
  sqlc.arg(is_default), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: ClearDefaultBuyerInvoicePreferences :execrows
UPDATE buyer_invoice_preferences
SET is_default = false, updated_at = sqlc.arg(now)
WHERE buyer_id = sqlc.arg(buyer_id)
  AND is_default
  AND deleted_at IS NULL
  AND superseded_at IS NULL;

-- name: SetDefaultBuyerInvoicePreference :execrows
UPDATE buyer_invoice_preferences
SET is_default = true, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND buyer_id = sqlc.arg(buyer_id)
  AND deleted_at IS NULL
  AND superseded_at IS NULL;

-- name: SupersedeBuyerInvoicePreference :execrows
UPDATE buyer_invoice_preferences
SET superseded_at = sqlc.arg(now),
    superseded_by_id = sqlc.arg(superseded_by_id),
    is_default = false,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND buyer_id = sqlc.arg(buyer_id)
  AND deleted_at IS NULL
  AND superseded_at IS NULL;

-- name: SoftDeleteBuyerInvoicePreference :execrows
UPDATE buyer_invoice_preferences
SET deleted_at = sqlc.arg(now),
    is_default = false,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND buyer_id = sqlc.arg(buyer_id)
  AND deleted_at IS NULL
  AND superseded_at IS NULL;
