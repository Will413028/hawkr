-- name: SubscribeRestock :exec
INSERT INTO buyer.restock_subscriptions (buyer_id, sku_id, product_id, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (buyer_id, sku_id) DO NOTHING;

-- name: UnsubscribeRestock :exec
DELETE FROM buyer.restock_subscriptions
WHERE buyer_id = $1 AND sku_id = $2;

-- name: CancelRestock :exec
DELETE FROM buyer.restock_subscriptions WHERE buyer_id = $1;

-- name: FanOut :many
WITH targets AS (
    SELECT pending.buyer_id, pending.sku_id FROM buyer.restock_subscriptions AS pending
    WHERE pending.sku_id = @sku_id
    ORDER BY pending.buyer_id
    LIMIT @batch_limit
    FOR UPDATE SKIP LOCKED
)
DELETE FROM buyer.restock_subscriptions AS subscriptions
USING targets
WHERE subscriptions.buyer_id = targets.buyer_id AND subscriptions.sku_id = targets.sku_id
RETURNING subscriptions.buyer_id;

-- name: HasRestock :one
SELECT EXISTS (SELECT 1 FROM buyer.restock_subscriptions WHERE sku_id = $1);

-- name: PurgeRestockSKUs :exec
DELETE FROM buyer.restock_subscriptions WHERE sku_id = ANY(@sku_ids::uuid[]);

-- name: ListRestock :many
SELECT * FROM buyer.restock_subscriptions
WHERE buyer_id = @buyer_id AND (NOT @has_after::boolean OR (created_at, sku_id) < (@after_time::timestamptz, @after_id::uuid))
ORDER BY created_at DESC, sku_id DESC
LIMIT @page_limit;

-- name: BatchRestock :many
SELECT DISTINCT product_id FROM buyer.restock_subscriptions
WHERE buyer_id = $1 AND product_id = ANY($2::uuid[]);
