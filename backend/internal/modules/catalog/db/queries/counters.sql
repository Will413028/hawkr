-- 衍生計數：由事實擁有者在自己的 transaction 內傳入，不用 ±1（D3、D26）。不改 content_version（etag 只涵蓋賣家可編輯的內容）。

-- name: SetProductRating :execrows
-- 評分與則數的絕對值；商品刪除後評價仍在，計數仍要更新，所以不看 deleted_at。products_rating_check 是後備。
UPDATE catalog.products
SET rating_average = sqlc.arg(rating_average), review_count = sqlc.arg(review_count), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: ClaimSalesOrder :execrows
-- 銷量去重：同一個 order_no 只有第一次插入成功（回 1 列），重送回 0 列，呼叫端不再累計。
INSERT INTO catalog.sales_orders (order_no, recorded_at) VALUES (sqlc.arg(order_no), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: AddProductSales :execrows
-- 同 transaction 內以資料庫端的加法累計（不先讀再寫）；上限夾在 INTEGER 最大值。
UPDATE catalog.products
SET sales_count = LEAST(sales_count::bigint + sqlc.arg(quantity)::bigint, 2147483647)::integer, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);
