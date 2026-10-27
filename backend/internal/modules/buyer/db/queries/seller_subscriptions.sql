-- name: AddFollow :execrows
-- 插入追蹤關聯，重複追蹤不產生新列。
INSERT INTO buyer.seller_subscriptions (buyer_id, store_id, created_at)
VALUES ($1, $2, $3)
ON CONFLICT (buyer_id, store_id) DO NOTHING;

-- name: RemoveFollow :execrows
-- 移除指定追蹤關聯。
DELETE
FROM buyer.seller_subscriptions
WHERE buyer_id = $1 AND store_id = $2;

-- name: ClearFollow :many
-- 刪除買家的全部追蹤關聯。
DELETE
FROM buyer.seller_subscriptions
WHERE buyer_id = $1
RETURNING store_id;

-- name: ListFollow :many
-- 以時間及店家 ID 進行追蹤 keyset 分頁。
SELECT *
FROM buyer.seller_subscriptions
WHERE buyer_id = @buyer_id AND (NOT @has_after::boolean OR (created_at, store_id) < (@after_time::timestamptz, @after_id::bigint))
ORDER BY created_at DESC, store_id DESC
LIMIT @page_limit;

-- name: FollowIDs :many
-- 列出買家追蹤的店家 ID。
SELECT store_id
FROM buyer.seller_subscriptions
WHERE buyer_id = $1
ORDER BY store_id;

-- name: FollowCount :one
-- 計算買家追蹤的店家數。
SELECT count(*)::bigint FROM buyer.seller_subscriptions WHERE buyer_id = $1;

-- name: CheckFollow :one
-- 查詢是否追蹤指定店家。
SELECT EXISTS(SELECT 1
FROM buyer.seller_subscriptions
WHERE buyer_id = $1 AND store_id = $2);

-- name: BatchFollow :many
-- 回傳輸入清單中已追蹤的店家 ID。
SELECT store_id
FROM buyer.seller_subscriptions
WHERE buyer_id = $1 AND store_id = ANY($2::bigint[]);

-- name: Subscribers :many
-- 以追蹤時間及買家 ID 分頁列出粉絲。
SELECT *
FROM buyer.seller_subscriptions
WHERE store_id = @store_id AND (NOT @has_after::boolean OR (created_at, buyer_id)<(@after_time::timestamptz, @after_id::bigint))
ORDER BY created_at DESC, buyer_id DESC
LIMIT @page_limit;

-- name: SubscriberCounts :many
-- 批次計算店家的粉絲數。
SELECT store_id, count(*)::bigint AS count
FROM buyer.seller_subscriptions
WHERE store_id = ANY($1::bigint[]) GROUP BY store_id;

-- name: SubscribersOffset :many
SELECT buyer_id, created_at
FROM buyer.seller_subscriptions
WHERE store_id = @store_id
ORDER BY created_at DESC, buyer_id DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: SubscriberStats :many
-- 店家的粉絲總數，以及每個時間窗 [window_start, window_end) 內新增的追蹤數（以追蹤時間計）。一句查詢：總數與各窗來自同一個
-- snapshot，結果依窗的順序（window_index 從 1 起）。
SELECT w.idx::int AS window_index, c.total::bigint AS total, c.new_count::bigint AS new_count
FROM generate_series(1, cardinality(@window_starts::timestamptz[])) AS w(idx)
CROSS JOIN LATERAL (
    SELECT count(*) AS total,
           count(*) FILTER (WHERE s.created_at >= (@window_starts::timestamptz[])[w.idx] AND s.created_at < (@window_ends::timestamptz[])[w.idx]) AS new_count
    FROM buyer.seller_subscriptions s
    WHERE s.store_id = @store_id
) c
ORDER BY w.idx;
