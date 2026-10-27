-- name: UpsertStoreVisitor :exec
INSERT INTO seller_store_visitors (store_id, visitor_buyer_id)
VALUES ($1, $2)
ON CONFLICT (store_id, visitor_buyer_id)
DO UPDATE SET
  visit_count = seller_store_visitors.visit_count + 1,
  last_visited_at = NOW();

-- name: ListStoreVisitors :many
SELECT visitor_buyer_id, visit_count, last_visited_at
FROM seller_store_visitors
WHERE store_id = $1
ORDER BY last_visited_at DESC
LIMIT $2 OFFSET $3;

-- name: CountStoreVisitors :one
SELECT COUNT(*) FROM seller_store_visitors
WHERE store_id = $1;

-- name: CountVisitorsByDateRange :one
SELECT COUNT(*) FROM seller_store_visitors
WHERE store_id = $1
  AND last_visited_at >= $2
  AND last_visited_at < $3;
