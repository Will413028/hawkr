
-- name: SetProductRating :execrows
UPDATE catalog.products
SET rating_average = sqlc.arg(rating_average), review_count = sqlc.arg(review_count), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: LockProduct :one
SELECT store_id, rating_average, review_count FROM catalog.products WHERE id = sqlc.arg(id) FOR NO KEY UPDATE;

-- name: ClaimSalesOrder :execrows
INSERT INTO catalog.sales_orders (order_no, recorded_at) VALUES (sqlc.arg(order_no), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: AddProductSales :execrows
UPDATE catalog.products
SET sales_count = LEAST(sales_count::bigint + sqlc.arg(quantity)::bigint, 2147483647)::integer, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);
