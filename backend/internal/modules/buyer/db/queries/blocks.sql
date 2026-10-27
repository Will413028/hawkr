-- name: AddBlockedBuyer :execrows
-- 買家封鎖買家，重複封鎖不產生新列（回 0）。
INSERT INTO buyer.blocked_buyers (blocker_buyer_id, blocked_buyer_id, created_at)
VALUES ($1, $2, $3)
ON CONFLICT (blocker_buyer_id, blocked_buyer_id) DO NOTHING;

-- name: RemoveBlockedBuyer :execrows
-- 解除買家封鎖買家，沒有這個封鎖時回 0。
DELETE
FROM buyer.blocked_buyers
WHERE blocker_buyer_id = $1 AND blocked_buyer_id = $2;

-- name: ListBlockedBuyers :many
-- 以時間與被封鎖的買家 ID 進行封鎖名單 keyset 分頁（降冪）。
SELECT blocked_buyer_id, created_at
FROM buyer.blocked_buyers
WHERE blocker_buyer_id = @blocker_buyer_id
  AND (NOT @has_after::boolean OR (created_at, blocked_buyer_id) < (@after_time::timestamptz, @after_id::bigint))
ORDER BY created_at DESC, blocked_buyer_id DESC
LIMIT @page_limit;

-- name: IsBlockedBetweenBuyers :one
-- 兩位買家之間任一方向是否有封鎖。
SELECT EXISTS(SELECT 1
FROM buyer.blocked_buyers
WHERE (blocker_buyer_id = @buyer_a AND blocked_buyer_id = @buyer_b) OR (blocker_buyer_id = @buyer_b AND blocked_buyer_id = @buyer_a));

-- name: AddBlockedStore :execrows
-- 買家封鎖店，重複封鎖不產生新列（回 0）。
INSERT INTO buyer.blocked_stores (blocker_buyer_id, store_id, created_at)
VALUES ($1, $2, $3)
ON CONFLICT (blocker_buyer_id, store_id) DO NOTHING;

-- name: RemoveBlockedStore :execrows
-- 解除買家封鎖店，沒有這個封鎖時回 0。
DELETE
FROM buyer.blocked_stores
WHERE blocker_buyer_id = $1 AND store_id = $2;

-- name: ListBlockedStores :many
-- 以時間與店家 ID 進行封鎖名單 keyset 分頁（降冪）。
SELECT store_id, created_at
FROM buyer.blocked_stores
WHERE blocker_buyer_id = @blocker_buyer_id
  AND (NOT @has_after::boolean OR (created_at, store_id) < (@after_time::timestamptz, @after_id::bigint))
ORDER BY created_at DESC, store_id DESC
LIMIT @page_limit;

-- name: IsBlockingStore :one
-- 買家是否封鎖了指定的店。
SELECT EXISTS(SELECT 1
FROM buyer.blocked_stores
WHERE blocker_buyer_id = $1 AND store_id = $2);

-- name: ClearBlockedStores :exec
-- 買家刪帳：刪除他發起的買家封鎖店。
DELETE
FROM buyer.blocked_stores
WHERE blocker_buyer_id = $1;

-- name: ClearBlockedBuyers :exec
-- 買家刪帳：刪除他發起的買家封鎖買家，以及以他為對象的列。
DELETE
FROM buyer.blocked_buyers
WHERE blocker_buyer_id = $1 OR blocked_buyer_id = $1;
