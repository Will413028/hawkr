-- name: LockPayoutCapability :one
-- 鎖定店家的開通閘門副本（PayoutAccountChanged 的訂閱者；payment 的帳戶列已在同一個 transaction 先鎖，全域鎖順序 payment 先於 seller）。
SELECT transfers_active
FROM seller.payout_capabilities
WHERE store_id = $1
FOR UPDATE;

-- name: UpsertPayoutCapability :exec
-- 寫入 payment 發布的轉帳能力；沒有列時建立。
INSERT INTO seller.payout_capabilities (store_id, provider, transfers_active, updated_at)
VALUES (sqlc.arg(store_id), sqlc.arg(provider), sqlc.arg(transfers_active), sqlc.arg(now))
ON CONFLICT (store_id) DO UPDATE
SET provider = EXCLUDED.provider,
    transfers_active = EXCLUDED.transfers_active,
    updated_at = EXCLUDED.updated_at;
