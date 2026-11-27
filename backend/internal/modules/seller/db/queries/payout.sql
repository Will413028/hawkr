-- name: LockPayoutCapability :one
SELECT transfers_active
FROM seller.payout_capabilities
WHERE store_id = $1
FOR UPDATE;

-- name: UpsertPayoutCapability :exec
INSERT INTO seller.payout_capabilities (store_id, provider, transfers_active, updated_at)
VALUES (sqlc.arg(store_id), sqlc.arg(provider), sqlc.arg(transfers_active), sqlc.arg(now))
ON CONFLICT (store_id) DO UPDATE
SET provider = EXCLUDED.provider,
    transfers_active = EXCLUDED.transfers_active,
    updated_at = EXCLUDED.updated_at;
