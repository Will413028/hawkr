-- name: AddVisit :execrows
-- 新增或更新最近瀏覽時間。
INSERT INTO buyer.visit_history (buyer_id, product_id, visited_at)
VALUES ($1, $2, $3)
ON CONFLICT (buyer_id, product_id) DO UPDATE
SET visited_at = excluded.visited_at;

-- name: ClearVisit :many
-- 清空指定買家的瀏覽紀錄。
DELETE
FROM buyer.visit_history
WHERE buyer_id = $1
RETURNING product_id;

-- name: ListVisit :many
-- 以瀏覽時間及商品 ID 分頁。
SELECT *
FROM buyer.visit_history
WHERE buyer_id = @buyer_id AND (NOT @has_after::boolean OR (visited_at, product_id) < (@after_time::timestamptz, @after_id::bigint))
ORDER BY visited_at DESC, product_id DESC
LIMIT @page_limit;
