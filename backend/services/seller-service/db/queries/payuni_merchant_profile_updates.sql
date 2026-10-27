-- issue #2806 Phase 2 idempotency-key claim + audit ledger.
--
-- ON CONFLICT DO NOTHING means a concurrent/replayed request with the same
-- (store_id, idempotency_key) returns zero rows (:one -> pgx.ErrNoRows in the
-- repository) instead of an error; the repository then calls
-- GetPayuniMerchantProfileUpdateByStoreAndKey to inspect what's already
-- there and decide replay / reject / retry.
-- name: InsertPayuniMerchantProfileUpdate :one
INSERT INTO payuni_merchant_profile_updates (
  id, store_id, account_id, idempotency_key, payload_hash, status,
  changed_fields, reason, trace_id
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (store_id, idempotency_key) DO NOTHING
RETURNING *;

-- name: GetPayuniMerchantProfileUpdateByStoreAndKey :one
SELECT * FROM payuni_merchant_profile_updates
WHERE store_id = $1 AND idempotency_key = $2;

-- name: MarkPayuniMerchantProfileUpdateSucceeded :exec
UPDATE payuni_merchant_profile_updates SET
  status = 'SUCCEEDED',
  provider_status = $2,
  provider_message = $3,
  completed_at = $4
WHERE id = $1;

-- name: MarkPayuniMerchantProfileUpdateFailed :exec
UPDATE payuni_merchant_profile_updates SET
  status = 'FAILED',
  provider_status = $2,
  provider_message = $3,
  completed_at = $4
WHERE id = $1;

-- name: DeletePayuniMerchantProfileUpdate :exec
DELETE FROM payuni_merchant_profile_updates WHERE id = $1;
