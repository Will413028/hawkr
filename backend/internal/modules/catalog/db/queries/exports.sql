
-- name: GetProductCards :many
SELECT p.id, p.store_id, p.category_id, p.title, p.status, p.first_published_at, p.min_price, p.max_price, p.sales_count, p.favorite_count, p.rating_average, p.review_count, p.low_stock_threshold, p.content_version, p.created_at,
       COALESCE((SELECT i.object_key FROM catalog.product_images i WHERE i.product_id = p.id AND i.kind = 'GALLERY' ORDER BY i.position LIMIT 1), '')::text AS cover_key,
       COALESCE((SELECT LEAST(sum(k.stock_qty), 2147483647) FROM catalog.product_skus k WHERE k.product_id = p.id AND k.deleted_at IS NULL), 0)::integer AS total_stock,
       0::bigint AS sort_key
FROM catalog.products p
WHERE p.id = ANY(sqlc.arg(ids)::uuid[]) AND p.deleted_at IS NULL
  AND (sqlc.arg(include_inactive)::boolean
       OR (p.status = 'ACTIVE' AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)))
ORDER BY p.id;

-- name: GetProductStoreIDs :many
SELECT p.id, p.store_id FROM catalog.products p
WHERE p.id = ANY(sqlc.arg(ids)::uuid[]) AND p.deleted_at IS NULL
ORDER BY p.id;

-- name: ListStoresWithPublicProducts :many
SELECT DISTINCT p.store_id FROM catalog.products p
WHERE p.status = 'ACTIVE' AND p.deleted_at IS NULL
  AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)
ORDER BY p.store_id;

-- name: GetSkuSnapshots :many
SELECT s.id AS sku_id, s.product_id, p.store_id, p.title, p.status,
       (p.deleted_at IS NOT NULL)::boolean AS product_deleted,
       (s.deleted_at IS NOT NULL)::boolean AS sku_deleted,
       s.option1_value, s.option2_value, s.price, s.compare_at_price, s.stock_qty,
       COALESCE((SELECT i.object_key FROM catalog.product_images i WHERE i.product_id = p.id AND i.kind = 'GALLERY' ORDER BY i.position LIMIT 1), '')::text AS cover_key
FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id
WHERE s.id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY s.id;

-- name: IncrementProductFavorites :execrows
UPDATE catalog.products
SET favorite_count = LEAST(favorite_count::bigint + 1, 2147483647)::integer, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: DecrementProductFavorites :execrows
UPDATE catalog.products
SET favorite_count = GREATEST(favorite_count - 1, 0), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);
