-- 庫存：冪等的扣減與歸還。

-- name: InsertStockOperation :execrows
-- 冪等鍵 UNIQUE 是唯一的判重點：同一個鍵的併發或重送，後到的交易在這裡等到先到的提交，然後回 0 列（呼叫端改走重送路徑）；
-- 先到的回滾時，後到的插入成功。沒有「先查再扣」。
-- 鍵的範圍：賣家調整帶 store_id，鍵只在該店家內唯一；結帳的扣減與歸還 store_id 為空，鍵全域唯一（兩個部分唯一索引，ON CONFLICT 不指定目標）。
INSERT INTO catalog.stock_operations (id, idempotency_key, kind, store_id, deduct_operation_id, request_hash, created_at)
VALUES (sqlc.arg(id), sqlc.arg(idempotency_key), sqlc.arg(kind), sqlc.narg(store_id), sqlc.narg(deduct_operation_id), sqlc.narg(request_hash), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: GetStockOperationByKey :one
-- 結帳扣減與歸還的操作（沒有店家）。
SELECT * FROM catalog.stock_operations WHERE idempotency_key = sqlc.arg(idempotency_key) AND store_id IS NULL;

-- name: GetStoreStockOperationByKey :one
-- 某家店的賣家調整操作；別家店的同名鍵不可見。
SELECT * FROM catalog.stock_operations WHERE store_id = sqlc.arg(store_id)::bigint AND idempotency_key = sqlc.arg(idempotency_key);

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

-- name: AdjustOwnSkuStock :execrows
-- 賣家調整：只動自己店家、未刪除商品的未刪除 SKU；結果必須落在 0 到 INTEGER 上限之間（以 bigint 算，不溢位），否則回 0 列，庫存不變。
UPDATE catalog.product_skus s
SET stock_qty = s.stock_qty + sqlc.arg(delta)::integer, updated_at = sqlc.arg(now)
FROM catalog.products p
WHERE s.id = sqlc.arg(id) AND s.deleted_at IS NULL
  AND p.id = s.product_id AND p.store_id = sqlc.arg(store_id) AND p.deleted_at IS NULL
  AND s.stock_qty::bigint + sqlc.arg(delta)::integer BETWEEN 0 AND 2147483647;

-- name: SetOwnSkuStock :execrows
-- 賣家設定：只動自己店家、未刪除商品的未刪除 SKU，而且現值要等於 expected（比對與寫入是同一句 UPDATE，沒有「先查再設」）；不符回 0 列，庫存不變。
UPDATE catalog.product_skus s
SET stock_qty = sqlc.arg(quantity)::integer, updated_at = sqlc.arg(now)
FROM catalog.products p
WHERE s.id = sqlc.arg(id) AND s.deleted_at IS NULL
  AND p.id = s.product_id AND p.store_id = sqlc.arg(store_id) AND p.deleted_at IS NULL
  AND s.stock_qty = sqlc.arg(expected)::integer;

-- name: GetOwnLiveSku :one
-- 店家自己的未刪除 SKU；調整失敗時用來區分「不存在、不是自己的或已刪除」與「結果超出範圍」。
SELECT s.id, s.stock_qty FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id
WHERE s.id = sqlc.arg(id) AND s.deleted_at IS NULL AND p.store_id = sqlc.arg(store_id) AND p.deleted_at IS NULL;

-- name: SkuInStore :one
-- SKU 是否屬於這家店（不看 SKU 或商品是否已刪除）；調整的冪等重送用它確認重送的人就是原本的店家。
SELECT EXISTS (
    SELECT 1 FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id
    WHERE s.id = sqlc.arg(id) AND p.store_id = sqlc.arg(store_id)
);

-- name: GetOwnLiveSkuProduct :one
-- 店家自己的未刪除 SKU 所屬的商品；庫存調整成功後要回該 SKU 的完整內容（價格、選項與可購買性在商品上），用它找商品。
SELECT s.product_id FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id
WHERE s.id = sqlc.arg(id) AND s.deleted_at IS NULL AND p.store_id = sqlc.arg(store_id) AND p.deleted_at IS NULL;
