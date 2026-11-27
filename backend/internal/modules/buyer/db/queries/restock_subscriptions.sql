-- name: SubscribeRestock :exec
-- 建立待通知關聯，同組（買家、SKU）訂閱冪等。
INSERT INTO buyer.restock_subscriptions (buyer_id, sku_id, product_id, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (buyer_id, sku_id) DO NOTHING;

-- name: UnsubscribeRestock :exec
-- 刪除指定 SKU 的訂閱。
DELETE FROM buyer.restock_subscriptions
WHERE buyer_id = $1 AND sku_id = $2;

-- name: CancelRestock :exec
-- 關閉 stint 時刪除全部訂閱。
DELETE FROM buyer.restock_subscriptions WHERE buyer_id = $1;

-- name: FanOut :many
-- 有界鎖定並刪除某個 SKU 的待通知關聯，回傳買家與 SKU 所屬商品；事件與刪除同 transaction。
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
-- 檢查 SKU 是否仍有待通知關聯。
SELECT EXISTS (SELECT 1 FROM buyer.restock_subscriptions WHERE sku_id = $1);

-- name: PurgeRestockSKUs :exec
-- 刪除這些 SKU 的全部訂閱（商品或 SKU 刪除後）；沒有訂閱的 SKU 是 no-op，可重複呼叫。
DELETE FROM buyer.restock_subscriptions WHERE sku_id = ANY(@sku_ids::uuid[]);

-- name: ListRestock :many
-- 以時間與 SKU ID 分頁列出待通知關聯。
SELECT * FROM buyer.restock_subscriptions
WHERE buyer_id = @buyer_id AND (NOT @has_after::boolean OR (created_at, sku_id) < (@after_time::timestamptz, @after_id::uuid))
ORDER BY created_at DESC, sku_id DESC
LIMIT @page_limit;

-- name: BatchRestock :many
-- 回傳輸入清單中至少有一個 SKU 已訂閱補貨的商品 ID。
SELECT DISTINCT product_id FROM buyer.restock_subscriptions
WHERE buyer_id = $1 AND product_id = ANY($2::uuid[]);
