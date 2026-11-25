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
-- 商品的價格區間是沒刪除 SKU 價格的最小與最大；每次 SKU 寫入的同 transaction 呼叫。沒有 SKU 時不改（影響 0 列）。
UPDATE catalog.products p
SET min_price = r.lo, max_price = r.hi, updated_at = sqlc.arg(now)
FROM (SELECT min(price)::bigint AS lo, max(price)::bigint AS hi FROM catalog.product_skus WHERE product_id = sqlc.arg(id) AND deleted_at IS NULL) r
WHERE p.id = sqlc.arg(id) AND r.lo IS NOT NULL;

-- name: InsertSku :exec
INSERT INTO catalog.product_skus (id, product_id, option1_value, option2_value, price, compare_at_price, stock_qty, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(product_id), sqlc.narg(option1_value), sqlc.narg(option2_value), sqlc.arg(price), sqlc.narg(compare_at_price), sqlc.arg(stock_qty), sqlc.arg(now), sqlc.arg(now));

-- name: LockLiveSkus :many
-- 商品的未刪除 SKU，依 ID 升序鎖列（FOR UPDATE 在排序之後逐列取鎖）：與 DeductStock 的 SKU 鎖順序一致，商品寫入之後再改這些列不會再等別人。
SELECT id, product_id, option1_value, option2_value, price, compare_at_price, stock_qty
FROM catalog.product_skus WHERE product_id = sqlc.arg(product_id) AND deleted_at IS NULL
ORDER BY id
FOR UPDATE;

-- name: SoftDeleteLiveSkus :exec
-- 商品刪除時，它還在的 SKU 一併標刪除，讓所有庫存路徑對「已刪除的商品」一致（扣減與調整都找不到）；呼叫前先 LockLiveSkus。
UPDATE catalog.product_skus SET deleted_at = sqlc.arg(now), updated_at = sqlc.arg(now) WHERE product_id = sqlc.arg(product_id) AND deleted_at IS NULL;

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

-- name: GetOwnProduct :one
-- 店家自己的商品（任何狀態，沒刪除）。圖片、選項、SKU 與 hashtag 以 JSON 欄位在同一句 SQL 內讀出：一句就是一份快照，不會讀到並行寫入的一半。
SELECT p.id, p.store_id, p.category_id, p.title, p.description, p.status, p.first_published_at, p.min_price, p.max_price, p.sales_count, p.favorite_count, p.rating_average, p.review_count, p.created_at, p.updated_at,
       COALESCE((SELECT jsonb_agg(jsonb_build_object('kind', i.kind, 'asset_id', i.asset_id, 'object_key', i.object_key, 'alt', i.alt) ORDER BY i.kind, i.position)
                 FROM catalog.product_images i WHERE i.product_id = p.id), '[]'::jsonb)::jsonb AS images,
       COALESCE((SELECT jsonb_agg(jsonb_build_object('name', x.name, 'values', COALESCE((SELECT jsonb_agg(v.value ORDER BY v.position)
                                                                                         FROM catalog.product_option_values v
                                                                                         WHERE v.product_id = x.product_id AND v.axis_position = x.position), '[]'::jsonb)) ORDER BY x.position)
                 FROM catalog.product_option_axes x WHERE x.product_id = p.id), '[]'::jsonb)::jsonb AS axes,
       COALESCE((SELECT jsonb_agg(jsonb_build_object('id', k.id, 'option1', k.option1_value, 'option2', k.option2_value, 'price', k.price, 'compare_at_price', k.compare_at_price, 'stock_qty', k.stock_qty) ORDER BY k.created_at, k.id)
                 FROM catalog.product_skus k WHERE k.product_id = p.id AND k.deleted_at IS NULL), '[]'::jsonb)::jsonb AS skus,
       COALESCE((SELECT jsonb_agg(jsonb_build_object('name', t.name, 'display_name', t.display_name) ORDER BY t.position)
                 FROM catalog.product_hashtags t WHERE t.product_id = p.id), '[]'::jsonb)::jsonb AS hashtags
FROM catalog.products p WHERE p.id = sqlc.arg(id) AND p.store_id = sqlc.arg(store_id) AND p.deleted_at IS NULL;

-- name: GetPublicProduct :one
-- 公開可讀：ACTIVE、沒刪除、店家在 at 這一刻可曝光。子表的 JSON 欄位同 GetOwnProduct：公開讀取會被別的模組呼叫，不開快照 transaction（backend/AGENTS.md 規則 3），靠單一語句取得一致的讀取。
SELECT p.id, p.store_id, p.category_id, p.title, p.description, p.status, p.first_published_at, p.min_price, p.max_price, p.sales_count, p.favorite_count, p.rating_average, p.review_count, p.created_at, p.updated_at,
       COALESCE((SELECT jsonb_agg(jsonb_build_object('kind', i.kind, 'asset_id', i.asset_id, 'object_key', i.object_key, 'alt', i.alt) ORDER BY i.kind, i.position)
                 FROM catalog.product_images i WHERE i.product_id = p.id), '[]'::jsonb)::jsonb AS images,
       COALESCE((SELECT jsonb_agg(jsonb_build_object('name', x.name, 'values', COALESCE((SELECT jsonb_agg(v.value ORDER BY v.position)
                                                                                         FROM catalog.product_option_values v
                                                                                         WHERE v.product_id = x.product_id AND v.axis_position = x.position), '[]'::jsonb)) ORDER BY x.position)
                 FROM catalog.product_option_axes x WHERE x.product_id = p.id), '[]'::jsonb)::jsonb AS axes,
       COALESCE((SELECT jsonb_agg(jsonb_build_object('id', k.id, 'option1', k.option1_value, 'option2', k.option2_value, 'price', k.price, 'compare_at_price', k.compare_at_price, 'stock_qty', k.stock_qty) ORDER BY k.created_at, k.id)
                 FROM catalog.product_skus k WHERE k.product_id = p.id AND k.deleted_at IS NULL), '[]'::jsonb)::jsonb AS skus,
       COALESCE((SELECT jsonb_agg(jsonb_build_object('name', t.name, 'display_name', t.display_name) ORDER BY t.position)
                 FROM catalog.product_hashtags t WHERE t.product_id = p.id), '[]'::jsonb)::jsonb AS hashtags

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
