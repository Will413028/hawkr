
-- name: InsertProduct :execrows
INSERT INTO catalog.products (id, store_id, category_id, title, description, status, first_published_at, min_price, max_price, searchable_text, low_stock_threshold, request_id, request_hash, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(category_id), sqlc.arg(title), sqlc.arg(description), sqlc.arg(status), sqlc.narg(first_published_at),
        sqlc.arg(min_price), sqlc.arg(max_price), sqlc.arg(searchable_text), sqlc.narg(low_stock_threshold), sqlc.arg(request_id), sqlc.arg(request_hash), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: GetProductByRequest :one
SELECT id, request_hash, deleted_at FROM catalog.products WHERE store_id = sqlc.arg(store_id) AND request_id = sqlc.arg(request_id);

-- name: LockOwnProduct :one
SELECT id, status, first_published_at, content_version FROM catalog.products
WHERE id = sqlc.arg(id) AND store_id = sqlc.arg(store_id) AND deleted_at IS NULL
FOR UPDATE;

-- name: OwnProductDeleted :one
SELECT EXISTS (SELECT 1 FROM catalog.products WHERE id = sqlc.arg(id) AND store_id = sqlc.arg(store_id) AND deleted_at IS NOT NULL);

-- name: UpdateProductContent :exec
UPDATE catalog.products
SET category_id = sqlc.arg(category_id), title = sqlc.arg(title), description = sqlc.arg(description),
    searchable_text = sqlc.arg(searchable_text), low_stock_threshold = sqlc.narg(low_stock_threshold),
    content_version = content_version + sqlc.arg(version_bump)::bigint, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: SetProductStatus :exec
UPDATE catalog.products
SET status = sqlc.arg(status)::text, content_version = content_version + 1,
    first_published_at = CASE WHEN sqlc.arg(status)::text = 'ACTIVE' THEN COALESCE(first_published_at, sqlc.arg(now)::timestamptz) ELSE first_published_at END,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: SoftDeleteProduct :exec
UPDATE catalog.products SET deleted_at = sqlc.arg(now), updated_at = sqlc.arg(now) WHERE id = sqlc.arg(id);

-- name: RecomputePriceRange :execrows
UPDATE catalog.products p
SET min_price = r.lo, max_price = r.hi, updated_at = sqlc.arg(now)
FROM (SELECT min(price)::bigint AS lo, max(price)::bigint AS hi FROM catalog.product_skus WHERE product_id = sqlc.arg(id) AND deleted_at IS NULL) r
WHERE p.id = sqlc.arg(id) AND r.lo IS NOT NULL;

-- name: InsertSku :exec
INSERT INTO catalog.product_skus (id, product_id, option1_value, option2_value, price, compare_at_price, stock_qty, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(product_id), sqlc.narg(option1_value), sqlc.narg(option2_value), sqlc.arg(price), sqlc.narg(compare_at_price), sqlc.arg(stock_qty), sqlc.arg(now), sqlc.arg(now));

-- name: LockLiveSkus :many
SELECT id, product_id, option1_value, option2_value, price, compare_at_price, stock_qty
FROM catalog.product_skus WHERE product_id = sqlc.arg(product_id) AND deleted_at IS NULL
ORDER BY id
FOR UPDATE;

-- name: SoftDeleteLiveSkus :exec
UPDATE catalog.product_skus SET deleted_at = sqlc.arg(now), updated_at = sqlc.arg(now) WHERE product_id = sqlc.arg(product_id) AND deleted_at IS NULL;

-- name: UpdateSkuPrices :exec
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
SELECT p.id, p.store_id, p.category_id, p.title, p.description, p.status, p.first_published_at, p.min_price, p.max_price, p.sales_count, p.favorite_count, p.rating_average, p.review_count, p.low_stock_threshold, p.content_version, p.created_at, p.updated_at,
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
SELECT p.id, p.store_id, p.category_id, p.title, p.description, p.status, p.first_published_at, p.min_price, p.max_price, p.sales_count, p.favorite_count, p.rating_average, p.review_count, p.low_stock_threshold, p.content_version, p.created_at, p.updated_at,
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
WHERE p.id = sqlc.arg(id) AND p.status IN ('ACTIVE', 'ARCHIVED') AND p.deleted_at IS NULL
  AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e);

-- name: GetAdminProduct :one
SELECT p.id, p.store_id, p.category_id, p.title, p.description, p.status, p.first_published_at, p.min_price, p.max_price, p.sales_count, p.favorite_count, p.rating_average, p.review_count, p.low_stock_threshold, p.content_version, p.created_at, p.updated_at,
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
FROM catalog.products p WHERE p.id = sqlc.arg(id) AND p.deleted_at IS NULL;

-- name: ListSellerProducts :many
SELECT t.id, t.store_id, t.category_id, t.title, t.status, t.first_published_at, t.min_price, t.max_price, t.sales_count, t.favorite_count, t.rating_average, t.review_count, t.low_stock_threshold, t.content_version, t.created_at, t.cover_key, t.total_stock, t.sort_key
FROM (
    SELECT p.id, p.store_id, p.category_id, p.title, p.status, p.first_published_at, p.min_price, p.max_price, p.sales_count, p.favorite_count, p.rating_average, p.review_count, p.low_stock_threshold, p.content_version, p.created_at,
       COALESCE((SELECT i.object_key FROM catalog.product_images i WHERE i.product_id = p.id AND i.kind = 'GALLERY' ORDER BY i.position LIMIT 1), '')::text AS cover_key,
       COALESCE((SELECT LEAST(sum(k.stock_qty), 2147483647) FROM catalog.product_skus k WHERE k.product_id = p.id AND k.deleted_at IS NULL), 0)::integer AS total_stock,
           (extract(epoch FROM p.created_at) * 1000000)::bigint AS sort_key
    FROM catalog.products p
    WHERE p.store_id = sqlc.arg(store_id) AND p.deleted_at IS NULL
      AND (sqlc.arg(status)::text = '' OR p.status = sqlc.arg(status)::text)
  AND (sqlc.arg(stock_status)::text = ''
       OR (p.status = 'ACTIVE' AND EXISTS (
            SELECT 1 FROM catalog.product_skus k
            WHERE k.product_id = p.id AND k.deleted_at IS NULL
              AND CASE sqlc.arg(stock_status)::text
                    WHEN 'LOW' THEN k.stock_qty > 0 AND k.stock_qty <= COALESCE(p.low_stock_threshold, sqlc.arg(default_threshold)::integer)
                    ELSE k.stock_qty = 0
                  END)))
      AND (sqlc.arg(query_pattern)::text = '' OR p.searchable_text ILIKE sqlc.arg(query_pattern)::text)
      AND (sqlc.narg(after_at)::timestamptz IS NULL OR (p.created_at, p.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
) t
ORDER BY t.created_at DESC, t.id DESC
LIMIT sqlc.arg(row_limit);

-- name: CountSellerProducts :one
SELECT count(*)::integer
FROM catalog.products p
WHERE p.store_id = sqlc.arg(store_id) AND p.deleted_at IS NULL
  AND (sqlc.arg(status)::text = '' OR p.status = sqlc.arg(status)::text)
  AND (sqlc.arg(stock_status)::text = ''
       OR (p.status = 'ACTIVE' AND EXISTS (
            SELECT 1 FROM catalog.product_skus k
            WHERE k.product_id = p.id AND k.deleted_at IS NULL
              AND CASE sqlc.arg(stock_status)::text
                    WHEN 'LOW' THEN k.stock_qty > 0 AND k.stock_qty <= COALESCE(p.low_stock_threshold, sqlc.arg(default_threshold)::integer)
                    ELSE k.stock_qty = 0
                  END)))
  AND (sqlc.arg(query_pattern)::text = '' OR p.searchable_text ILIKE sqlc.arg(query_pattern)::text);

-- name: ListAdminProducts :many
SELECT t.id, t.store_id, t.category_id, t.title, t.status, t.first_published_at, t.min_price, t.max_price, t.sales_count, t.favorite_count, t.rating_average, t.review_count, t.low_stock_threshold, t.content_version, t.created_at, t.cover_key, t.total_stock, t.sort_key
FROM (
    SELECT p.id, p.store_id, p.category_id, p.title, p.status, p.first_published_at, p.min_price, p.max_price, p.sales_count, p.favorite_count, p.rating_average, p.review_count, p.low_stock_threshold, p.content_version, p.created_at,
       COALESCE((SELECT i.object_key FROM catalog.product_images i WHERE i.product_id = p.id AND i.kind = 'GALLERY' ORDER BY i.position LIMIT 1), '')::text AS cover_key,
       COALESCE((SELECT LEAST(sum(k.stock_qty), 2147483647) FROM catalog.product_skus k WHERE k.product_id = p.id AND k.deleted_at IS NULL), 0)::integer AS total_stock,
           (extract(epoch FROM p.created_at) * 1000000)::bigint AS sort_key
    FROM catalog.products p
    WHERE p.deleted_at IS NULL
      AND (sqlc.narg(store_id)::bigint IS NULL OR p.store_id = sqlc.narg(store_id)::bigint)
      AND (sqlc.arg(query_pattern)::text = '' OR p.searchable_text ILIKE sqlc.arg(query_pattern)::text)
      AND (sqlc.narg(after_at)::timestamptz IS NULL OR (p.created_at, p.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
) t
ORDER BY t.created_at DESC, t.id DESC
LIMIT sqlc.arg(row_limit);

-- name: CountAdminProducts :one
SELECT count(*)::integer
FROM catalog.products p
WHERE p.deleted_at IS NULL
  AND (sqlc.narg(store_id)::bigint IS NULL OR p.store_id = sqlc.narg(store_id)::bigint)
  AND (sqlc.arg(query_pattern)::text = '' OR p.searchable_text ILIKE sqlc.arg(query_pattern)::text);

-- name: ListPublicProducts :many
SELECT t.id, t.store_id, t.category_id, t.title, t.status, t.first_published_at, t.min_price, t.max_price, t.sales_count, t.favorite_count, t.rating_average, t.review_count, t.low_stock_threshold, t.content_version, t.created_at, t.cover_key, t.total_stock, t.sort_key
FROM (
    SELECT p.id, p.store_id, p.category_id, p.title, p.status, p.first_published_at, p.min_price, p.max_price, p.sales_count, p.favorite_count, p.rating_average, p.review_count, p.low_stock_threshold, p.content_version, p.created_at,
       COALESCE((SELECT i.object_key FROM catalog.product_images i WHERE i.product_id = p.id AND i.kind = 'GALLERY' ORDER BY i.position LIMIT 1), '')::text AS cover_key,
       COALESCE((SELECT LEAST(sum(k.stock_qty), 2147483647) FROM catalog.product_skus k WHERE k.product_id = p.id AND k.deleted_at IS NULL), 0)::integer AS total_stock,
           CASE sqlc.arg(sort)::text
              WHEN 'LATEST' THEN (extract(epoch FROM p.first_published_at) * 1000000)::bigint
              WHEN 'PRICE_ASC' THEN -p.min_price
              WHEN 'PRICE_DESC' THEN p.max_price
              WHEN 'SALES' THEN p.sales_count::bigint
              WHEN 'RATING' THEN round(p.rating_average * 1000)::bigint
              ELSE round(word_similarity(sqlc.arg(query)::text, p.searchable_text) * 1000000)::bigint
          END AS sort_key
    FROM catalog.products p
    WHERE p.status = 'ACTIVE' AND p.deleted_at IS NULL
      AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)
      AND (cardinality(sqlc.arg(category_ids)::uuid[]) = 0
           OR p.category_id IN (SELECT c.id FROM catalog.categories c WHERE c.id = ANY(sqlc.arg(category_ids)::uuid[]) OR c.parent_id = ANY(sqlc.arg(category_ids)::uuid[])))
      AND (sqlc.narg(price_min)::bigint IS NULL OR p.max_price >= sqlc.narg(price_min)::bigint)
      AND (sqlc.narg(price_max)::bigint IS NULL OR p.min_price <= sqlc.narg(price_max)::bigint)
      AND (cardinality(sqlc.arg(hashtags)::text[]) = 0
           OR (SELECT count(*) FROM catalog.product_hashtags h WHERE h.product_id = p.id AND h.name = ANY(sqlc.arg(hashtags)::text[]))
              >= CASE WHEN sqlc.arg(hashtag_all)::boolean THEN cardinality(sqlc.arg(hashtags)::text[]) ELSE 1 END)
      AND (cardinality(sqlc.arg(store_ids)::bigint[]) = 0 OR p.store_id = ANY(sqlc.arg(store_ids)::bigint[]))
      AND (sqlc.arg(query_pattern)::text = '' OR p.searchable_text ILIKE sqlc.arg(query_pattern)::text)
) t
WHERE sqlc.narg(after_key)::bigint IS NULL OR (t.sort_key, t.id) < (sqlc.narg(after_key)::bigint, sqlc.narg(after_id)::uuid)
ORDER BY t.sort_key DESC, t.id DESC
LIMIT sqlc.arg(row_limit);

-- name: SuggestProductNames :many
SELECT t.title::text AS title
FROM (
    SELECT p.title, max(word_similarity(sqlc.arg(query)::text, p.title)) AS score
    FROM catalog.products p
    WHERE p.status = 'ACTIVE' AND p.deleted_at IS NULL
      AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)
      AND p.title ILIKE sqlc.arg(query_pattern)::text
    GROUP BY p.title
) t
ORDER BY t.score DESC, t.title
LIMIT sqlc.arg(row_limit);

-- name: SuggestHashtags :many
SELECT h.name::text AS name, mode() WITHIN GROUP (ORDER BY h.display_name)::text AS display, count(*)::bigint AS product_count
FROM catalog.product_hashtags h
JOIN catalog.products p ON p.id = h.product_id
WHERE p.status = 'ACTIVE' AND p.deleted_at IS NULL
  AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)
  AND h.name ILIKE sqlc.arg(name_pattern)::text
GROUP BY h.name
ORDER BY count(*) DESC, h.name
LIMIT sqlc.arg(row_limit);

-- name: TrendingHashtags :many
SELECT h.name::text AS name, mode() WITHIN GROUP (ORDER BY h.display_name)::text AS display, count(*)::bigint AS product_count
FROM catalog.product_hashtags h
JOIN catalog.products p ON p.id = h.product_id
WHERE p.status = 'ACTIVE' AND p.deleted_at IS NULL
  AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)
GROUP BY h.name
ORDER BY count(*) DESC, h.name
LIMIT sqlc.arg(row_limit);
