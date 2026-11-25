-- 商品、選項、SKU、圖片引用與 hashtag 的寫入與讀取。

-- name: InsertProduct :exec
INSERT INTO catalog.products (id, store_id, category_id, title, description, status, first_published_at, min_price, max_price, searchable_text, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(category_id), sqlc.arg(title), sqlc.arg(description), sqlc.arg(status), sqlc.narg(first_published_at),
        sqlc.arg(min_price), sqlc.arg(max_price), sqlc.arg(searchable_text), sqlc.arg(now), sqlc.arg(now));

-- name: LockOwnProduct :one
-- 店家自己的、沒刪除的商品，鎖列；同一個商品的寫入都從這裡開始，所以 SKU、選項與價格區間的重算不會並行交錯。
SELECT id, status, first_published_at FROM catalog.products
WHERE id = sqlc.arg(id) AND store_id = sqlc.arg(store_id) AND deleted_at IS NULL
FOR UPDATE;

-- name: UpdateProductContent :exec
UPDATE catalog.products
SET category_id = sqlc.arg(category_id), title = sqlc.arg(title), description = sqlc.arg(description),
    searchable_text = sqlc.arg(searchable_text), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: SetProductStatus :exec
-- 設成 ACTIVE 時 first_published_at 只在還是空的時候填（之後的下架再上架不改它）。
UPDATE catalog.products
SET status = sqlc.arg(status)::text,
    first_published_at = CASE WHEN sqlc.arg(status)::text = 'ACTIVE' THEN COALESCE(first_published_at, sqlc.arg(now)::timestamptz) ELSE first_published_at END,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: SoftDeleteProduct :exec
UPDATE catalog.products SET deleted_at = sqlc.arg(now), updated_at = sqlc.arg(now) WHERE id = sqlc.arg(id);

-- name: RecomputePriceRange :execrows
-- 商品的價格區間是沒刪除 SKU 價格的最小與最大（D19）；每次 SKU 寫入的同 transaction 呼叫。沒有 SKU 時不改（影響 0 列）。
UPDATE catalog.products p
SET min_price = r.lo, max_price = r.hi, updated_at = sqlc.arg(now)
FROM (SELECT min(price)::bigint AS lo, max(price)::bigint AS hi FROM catalog.product_skus WHERE product_id = sqlc.arg(id) AND deleted_at IS NULL) r
WHERE p.id = sqlc.arg(id) AND r.lo IS NOT NULL;

-- name: InsertSku :exec
INSERT INTO catalog.product_skus (id, product_id, option1_value, option2_value, price, compare_at_price, stock_qty, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(product_id), sqlc.narg(option1_value), sqlc.narg(option2_value), sqlc.arg(price), sqlc.narg(compare_at_price), sqlc.arg(stock_qty), sqlc.arg(now), sqlc.arg(now));

-- name: ListLiveSkus :many
SELECT id, product_id, option1_value, option2_value, price, compare_at_price, stock_qty
FROM catalog.product_skus WHERE product_id = sqlc.arg(product_id) AND deleted_at IS NULL
ORDER BY created_at, id;

-- name: UpdateSkuPrices :exec
-- 庫存不在這裡改（只由 stock 改）。
UPDATE catalog.product_skus SET price = sqlc.arg(price), compare_at_price = sqlc.narg(compare_at_price), updated_at = sqlc.arg(now) WHERE id = sqlc.arg(id);

-- name: SoftDeleteSku :exec
UPDATE catalog.product_skus SET deleted_at = sqlc.arg(now), updated_at = sqlc.arg(now) WHERE id = sqlc.arg(id);

-- name: DeleteOptionValues :exec
DELETE FROM catalog.product_option_values WHERE product_id = sqlc.arg(product_id);

-- name: DeleteOptionAxes :exec
DELETE FROM catalog.product_option_axes WHERE product_id = sqlc.arg(product_id);

-- name: InsertOptionAxis :exec
INSERT INTO catalog.product_option_axes (product_id, position, name) VALUES (sqlc.arg(product_id), sqlc.arg(position), sqlc.arg(name));

-- name: InsertOptionValue :exec
INSERT INTO catalog.product_option_values (product_id, axis_position, position, value) VALUES (sqlc.arg(product_id), sqlc.arg(axis_position), sqlc.arg(position), sqlc.arg(value));

-- name: ListOptionAxes :many
SELECT product_id, position, name FROM catalog.product_option_axes WHERE product_id = sqlc.arg(product_id) ORDER BY position;

-- name: ListOptionValues :many
SELECT product_id, axis_position, position, value FROM catalog.product_option_values WHERE product_id = sqlc.arg(product_id) ORDER BY axis_position, position;

-- name: ListProductImages :many
SELECT product_id, kind, position, asset_id, object_key, alt FROM catalog.product_images WHERE product_id = sqlc.arg(product_id) ORDER BY kind, position;

-- name: DeleteProductImages :exec
DELETE FROM catalog.product_images WHERE product_id = sqlc.arg(product_id);

-- name: InsertProductImage :exec
INSERT INTO catalog.product_images (product_id, kind, position, asset_id, object_key, alt)
VALUES (sqlc.arg(product_id), sqlc.arg(kind), sqlc.arg(position), sqlc.arg(asset_id), sqlc.arg(object_key), sqlc.arg(alt));

-- name: DeleteProductHashtags :exec
DELETE FROM catalog.product_hashtags WHERE product_id = sqlc.arg(product_id);

-- name: InsertProductHashtag :exec
INSERT INTO catalog.product_hashtags (product_id, name, display_name, position) VALUES (sqlc.arg(product_id), sqlc.arg(name), sqlc.arg(display_name), sqlc.arg(position));

-- name: ListProductHashtags :many
SELECT product_id, name, display_name, position FROM catalog.product_hashtags WHERE product_id = sqlc.arg(product_id) ORDER BY position;

-- name: GetOwnProduct :one
-- 店家自己的商品（任何狀態，沒刪除）。
SELECT id, store_id, category_id, title, description, status, first_published_at, min_price, max_price, sales_count, favorite_count, rating_average, review_count, created_at, updated_at
FROM catalog.products WHERE id = sqlc.arg(id) AND store_id = sqlc.arg(store_id) AND deleted_at IS NULL;

-- name: GetPublicProduct :one
-- 公開可讀：ACTIVE、沒刪除、店家在 at 這一刻可曝光。
SELECT p.id, p.store_id, p.category_id, p.title, p.description, p.status, p.first_published_at, p.min_price, p.max_price, p.sales_count, p.favorite_count, p.rating_average, p.review_count, p.created_at, p.updated_at
FROM catalog.products p
WHERE p.id = sqlc.arg(id) AND p.status = 'ACTIVE' AND p.deleted_at IS NULL
  AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e);

-- name: ListSellerProducts :many
-- 賣家清單：店家自己的所有沒刪除的商品，不看狀態也不看曝光；最新建立的在前。
SELECT p.id, p.store_id, p.category_id, p.title, p.status, p.first_published_at, p.min_price, p.max_price, p.sales_count, p.favorite_count, p.rating_average, p.review_count, p.created_at,
       COALESCE((SELECT i.object_key FROM catalog.product_images i WHERE i.product_id = p.id AND i.kind = 'GALLERY' ORDER BY i.position LIMIT 1), '')::text AS cover_key
FROM catalog.products p
WHERE p.store_id = sqlc.arg(store_id) AND p.deleted_at IS NULL
  AND (sqlc.narg(after_at)::timestamptz IS NULL OR (p.created_at, p.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg(row_limit);

-- name: ListPublicProducts :many
-- 公開列表：只含 ACTIVE、沒刪除、店家在 at 這一刻可曝光的商品；最新上架的在前。
SELECT p.id, p.store_id, p.category_id, p.title, p.status, p.first_published_at, p.min_price, p.max_price, p.sales_count, p.favorite_count, p.rating_average, p.review_count, p.created_at,
       COALESCE((SELECT i.object_key FROM catalog.product_images i WHERE i.product_id = p.id AND i.kind = 'GALLERY' ORDER BY i.position LIMIT 1), '')::text AS cover_key
FROM catalog.products p
WHERE p.status = 'ACTIVE' AND p.deleted_at IS NULL
  AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)
  AND (sqlc.narg(category_id)::uuid IS NULL OR p.category_id = sqlc.narg(category_id)::uuid)
  AND (sqlc.narg(after_at)::timestamptz IS NULL OR (p.first_published_at, p.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY p.first_published_at DESC, p.id DESC
LIMIT sqlc.arg(row_limit);
