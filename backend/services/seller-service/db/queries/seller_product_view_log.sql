-- name: InsertProductViewLog :exec
INSERT INTO seller_product_view_log (id, seller_id, product_id, source, viewed_at)
VALUES ($1, $2, $3, $4, NOW());

-- name: CountProductViewsBySeller :one
SELECT COUNT(*) FROM seller_product_view_log
WHERE seller_id = $1
  AND viewed_at >= $2
  AND viewed_at < $3;
