-- name: StoreExists :one
-- 店家是否存在（封鎖寫入前確認，不存在回 NotFound 而不是外鍵錯誤）。
SELECT EXISTS(SELECT 1 FROM seller.stores WHERE store_id = $1);

-- name: AddStoreBlock :execrows
-- 店家封鎖買家，重複封鎖不產生新列（回 0）。
INSERT INTO seller.store_blocks (store_id, buyer_id, created_at)
VALUES ($1, $2, $3)
ON CONFLICT (store_id, buyer_id) DO NOTHING;

-- name: RemoveStoreBlock :execrows
-- 店家解除封鎖，沒有這個封鎖時回 0。
DELETE
FROM seller.store_blocks
WHERE store_id = $1 AND buyer_id = $2;

-- name: ListStoreBlocks :many
-- 以時間與買家 ID 進行某店的封鎖名單 keyset 分頁（降冪）。
SELECT buyer_id, created_at
FROM seller.store_blocks
WHERE store_id = @store_id
  AND (NOT @has_after::boolean OR (created_at, buyer_id) < (@after_time::timestamptz, @after_id::bigint))
ORDER BY created_at DESC, buyer_id DESC
LIMIT @page_limit;

-- name: StoreBlocksBuyer :one
-- 店家是否封鎖了指定買家。
SELECT EXISTS(SELECT 1
FROM seller.store_blocks
WHERE store_id = $1 AND buyer_id = $2);

-- name: StoresBlockingBuyer :many
-- 輸入的店家中封鎖了指定買家的 store_id。
SELECT store_id
FROM seller.store_blocks
WHERE buyer_id = $1 AND store_id = ANY($2::bigint[])
ORDER BY store_id;

-- name: ClearStoreBlocksAgainstBuyer :exec
-- 買家刪帳：刪除各店對這位買家的封鎖。
DELETE
FROM seller.store_blocks
WHERE buyer_id = $1;
