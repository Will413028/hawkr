-- 庫存：冪等的扣減與歸還（D2）。

-- name: InsertStockOperation :execrows
-- 冪等鍵 UNIQUE 是唯一的判重點：同一個鍵的併發或重送，後到的交易在這裡等到先到的提交，然後回 0 列（呼叫端改走重送路徑）；
-- 先到的回滾時，後到的插入成功。沒有「先查再扣」。
INSERT INTO catalog.stock_operations (id, idempotency_key, kind, deduct_operation_id, created_at)
VALUES (sqlc.arg(id), sqlc.arg(idempotency_key), sqlc.arg(kind), sqlc.narg(deduct_operation_id), sqlc.arg(now))
ON CONFLICT (idempotency_key) DO NOTHING;

-- name: GetStockOperationByKey :one
SELECT * FROM catalog.stock_operations WHERE idempotency_key = sqlc.arg(idempotency_key);

-- name: LockStockOperation :one
-- 歸還前鎖住原扣減：同一次扣減的多筆歸還（不同鍵）排隊，累計檢查看到前一筆已提交的結果。
SELECT * FROM catalog.stock_operations WHERE id = sqlc.arg(id) AND kind = 'DEDUCT' FOR UPDATE;

-- name: InsertStockMovement :exec
INSERT INTO catalog.stock_movements (operation_id, sku_id, delta) VALUES (sqlc.arg(operation_id), sqlc.arg(sku_id), sqlc.arg(delta));

-- name: ListStockMovements :many
SELECT sku_id, delta FROM catalog.stock_movements WHERE operation_id = sqlc.arg(operation_id) ORDER BY sku_id;

-- name: SumRestoredBySku :many
-- 一次扣減已歸還的累計量（正數），依 SKU。
SELECT m.sku_id, SUM(m.delta)::integer AS restored
FROM catalog.stock_operations o
JOIN catalog.stock_movements m ON m.operation_id = o.id
WHERE o.deduct_operation_id = sqlc.arg(deduct_operation_id)
GROUP BY m.sku_id
ORDER BY m.sku_id;

-- name: DeductSkuStock :execrows
-- 條件式扣減：庫存不足或 SKU 已刪除回 0 列，不會變負（skus_stock_check 是後備）。
UPDATE catalog.product_skus
SET stock_qty = stock_qty - sqlc.arg(quantity)::integer, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND stock_qty >= sqlc.arg(quantity)::integer;

-- name: RestoreSkuStock :execrows
-- 歸還不檢查 SKU 是否已刪除：訂單取消時商品可能已被賣家移除，庫存仍要記回（量由原扣減決定）。
UPDATE catalog.product_skus
SET stock_qty = stock_qty + sqlc.arg(quantity)::integer, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: GetLiveSkuStock :one
-- 未刪除的 SKU 的庫存；扣減失敗時用來區分「不存在或已刪除」與「庫存不足」。
SELECT id, stock_qty FROM catalog.product_skus WHERE id = sqlc.arg(id) AND deleted_at IS NULL;
