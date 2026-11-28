
-- name: InsertStockOperation :execrows
INSERT INTO catalog.stock_operations (id, idempotency_key, kind, store_id, deduct_operation_id, request_hash, created_at)
VALUES (sqlc.arg(id), sqlc.arg(idempotency_key), sqlc.arg(kind), sqlc.narg(store_id), sqlc.narg(deduct_operation_id), sqlc.narg(request_hash), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: GetStockOperationByKey :one
SELECT * FROM catalog.stock_operations WHERE idempotency_key = sqlc.arg(idempotency_key) AND store_id IS NULL;

-- name: GetStoreStockOperationByKey :one
SELECT * FROM catalog.stock_operations WHERE store_id = sqlc.arg(store_id)::bigint AND idempotency_key = sqlc.arg(idempotency_key);

-- name: LockStockOperation :one
SELECT * FROM catalog.stock_operations WHERE id = sqlc.arg(id) AND kind = 'DEDUCT' FOR UPDATE;

-- name: InsertStockMovement :exec
INSERT INTO catalog.stock_movements (operation_id, sku_id, delta) VALUES (sqlc.arg(operation_id), sqlc.arg(sku_id), sqlc.arg(delta));

-- name: ListStockMovements :many
SELECT sku_id, delta FROM catalog.stock_movements WHERE operation_id = sqlc.arg(operation_id) ORDER BY sku_id;

-- name: SumRestoredBySku :many
SELECT m.sku_id, SUM(m.delta)::integer AS restored
FROM catalog.stock_operations o
JOIN catalog.stock_movements m ON m.operation_id = o.id
WHERE o.deduct_operation_id = sqlc.arg(deduct_operation_id)
GROUP BY m.sku_id
ORDER BY m.sku_id;

-- name: DeductSkuStock :execrows
UPDATE catalog.product_skus s
SET stock_qty = s.stock_qty - sqlc.arg(quantity)::integer, updated_at = sqlc.arg(now)
FROM catalog.buyer_visible_products(sqlc.arg(now)::timestamptz) AS p
WHERE s.id = sqlc.arg(id) AND p.id = s.product_id AND s.deleted_at IS NULL AND s.stock_qty >= sqlc.arg(quantity)::integer;

-- name: RestoreSkuStock :execrows
UPDATE catalog.product_skus
SET stock_qty = stock_qty + sqlc.arg(quantity)::integer, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: GetLiveSkuStock :one
SELECT s.id, s.stock_qty,
       EXISTS (SELECT 1 FROM catalog.buyer_visible_products(sqlc.arg(now)::timestamptz) v WHERE v.id = s.product_id) AS product_visible
FROM catalog.product_skus s WHERE s.id = sqlc.arg(id) AND s.deleted_at IS NULL;

-- name: AdjustOwnSkuStock :execrows
UPDATE catalog.product_skus s
SET stock_qty = s.stock_qty + sqlc.arg(delta)::integer, updated_at = sqlc.arg(now)
FROM catalog.products p
WHERE s.id = sqlc.arg(id) AND s.deleted_at IS NULL
  AND p.id = s.product_id AND p.store_id = sqlc.arg(store_id) AND p.deleted_at IS NULL
  AND s.stock_qty::bigint + sqlc.arg(delta)::integer BETWEEN 0 AND 2147483647;

-- name: SetOwnSkuStock :execrows
UPDATE catalog.product_skus s
SET stock_qty = sqlc.arg(quantity)::integer, updated_at = sqlc.arg(now)
FROM catalog.products p
WHERE s.id = sqlc.arg(id) AND s.deleted_at IS NULL
  AND p.id = s.product_id AND p.store_id = sqlc.arg(store_id) AND p.deleted_at IS NULL
  AND s.stock_qty = sqlc.arg(expected)::integer;

-- name: GetOwnLiveSku :one
SELECT s.id, s.stock_qty FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id
WHERE s.id = sqlc.arg(id) AND s.deleted_at IS NULL AND p.store_id = sqlc.arg(store_id) AND p.deleted_at IS NULL;

-- name: SkuInStore :one
SELECT EXISTS (
    SELECT 1 FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id
    WHERE s.id = sqlc.arg(id) AND p.store_id = sqlc.arg(store_id)
);

-- name: GetOwnSkuState :one
SELECT s.id, s.option1_value, s.option2_value, s.price, s.compare_at_price, s.stock_qty,
       (s.deleted_at IS NULL AND p.deleted_at IS NULL AND p.status = 'ACTIVE' AND s.stock_qty > 0)::boolean AS sellable
FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id
WHERE s.id = sqlc.arg(id) AND p.store_id = sqlc.arg(store_id);