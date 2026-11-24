-- 商品、選項與 SKU 的寫入與讀取（B4-2b 擴充；這裡先放庫存與分類的測試與讀取需要的最小集合）。

-- name: InsertProduct :exec
INSERT INTO catalog.products (id, store_id, category_id, title, description, status, first_published_at, min_price, max_price, searchable_text, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(category_id), sqlc.arg(title), sqlc.arg(description), sqlc.arg(status), sqlc.narg(first_published_at),
        sqlc.arg(min_price), sqlc.arg(max_price), sqlc.arg(searchable_text), sqlc.arg(now), sqlc.arg(now));

-- name: InsertSku :exec
INSERT INTO catalog.product_skus (id, product_id, option1_value, option2_value, price, compare_at_price, stock_qty, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(product_id), sqlc.narg(option1_value), sqlc.narg(option2_value), sqlc.arg(price), sqlc.narg(compare_at_price), sqlc.arg(stock_qty), sqlc.arg(now), sqlc.arg(now));
