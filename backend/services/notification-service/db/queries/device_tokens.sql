-- name: UpsertDeviceToken :exec
INSERT INTO device_tokens (id, user_id, user_type, token, platform, is_active, updated_at)
VALUES ($1, $2, $3, $4, $5, true, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, user_type, token)
DO UPDATE SET
  is_active = true,
  platform = EXCLUDED.platform,
  updated_at = CURRENT_TIMESTAMP;

-- name: DeactivateDeviceToken :exec
UPDATE device_tokens
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1 AND user_type = $2 AND token = $3;

-- name: DeactivateTokenByValue :exec
UPDATE device_tokens
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE token = $1;

-- DeactivateDeviceTokensByUser deactivates every device_tokens row registered
-- under (user_id, user_type), regardless of token value — unlike
-- DeactivateDeviceToken above, which targets exactly one known token. Used when a
-- seller incarnation retires (issue #2707 follow-up, worker/seller_deleted.go): a
-- device that re-registers under a NEW store_id after the seller closes/reopens
-- their store must stop receiving pushes through the OLD store_id's row, which
-- UpsertDeviceToken's ON CONFLICT (user_id, user_type, token) otherwise leaves
-- is_active=true forever (not self-healing, unlike a pre-deploy stale account_id
-- token, which naturally self-heals on the device's next token rotation).
-- :execrows so the caller can log how many rows were deactivated, mirroring
-- notifications.sql's SoftDeleteByHistoricalStoreIDs in the same handler.
-- name: DeactivateDeviceTokensByUser :execrows
UPDATE device_tokens
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1 AND user_type = $2;

-- name: GetActiveTokensByUser :many
SELECT id, user_id, user_type, token, platform, is_active, created_at, updated_at
FROM device_tokens
WHERE user_id = $1 AND user_type = $2 AND is_active = true;

-- name: GetActiveTokensByUserType :many
SELECT id, user_id, user_type, token, platform, is_active, created_at, updated_at
FROM device_tokens
WHERE user_type = $1 AND is_active = true
ORDER BY id
LIMIT $2 OFFSET $3;
