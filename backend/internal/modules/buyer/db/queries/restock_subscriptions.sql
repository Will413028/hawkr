-- name: SubscribeRestock :exec
-- 建立待通知關聯，同組訂閱冪等。
INSERT INTO buyer.restock_subscriptions (buyer_id, product_id, created_at)
VALUES ($1, $2, $3)
ON CONFLICT (buyer_id, product_id) DO NOTHING;

-- name: UnsubscribeRestock :exec
-- 刪除指定商品的訂閱。
DELETE FROM buyer.restock_subscriptions
WHERE buyer_id = $1 AND product_id = $2;

-- name: CancelRestock :exec
-- 關閉 stint 時刪除全部訂閱。
DELETE FROM buyer.restock_subscriptions WHERE buyer_id = $1;

-- name: FanOut :many
-- 有界鎖定並刪除待通知關聯；事件與刪除同 transaction。
WITH targets AS (
    SELECT pending.buyer_id, pending.product_id FROM buyer.restock_subscriptions AS pending
    WHERE pending.product_id = @product_id
    ORDER BY pending.buyer_id
    LIMIT @batch_limit
    FOR UPDATE SKIP LOCKED
)
DELETE FROM buyer.restock_subscriptions AS subscriptions
USING targets
WHERE subscriptions.buyer_id = targets.buyer_id AND subscriptions.product_id = targets.product_id
RETURNING subscriptions.buyer_id;

-- name: HasRestock :one
-- 檢查商品是否仍有待通知關聯。
SELECT EXISTS (SELECT 1 FROM buyer.restock_subscriptions WHERE product_id = $1);

-- name: ListRestock :many
-- 以時間與商品 ID 分頁列出待通知關聯。
SELECT * FROM buyer.restock_subscriptions
WHERE buyer_id = @buyer_id AND (NOT @has_after::boolean OR (created_at, product_id) < (@after_time::timestamptz, @after_id::bigint))
ORDER BY created_at DESC, product_id DESC
LIMIT @page_limit;

-- name: BatchRestock :many
-- 回傳輸入清單中已訂閱補貨的商品 ID。
SELECT product_id FROM buyer.restock_subscriptions
WHERE buyer_id = $1 AND product_id = ANY($2::bigint[]);
