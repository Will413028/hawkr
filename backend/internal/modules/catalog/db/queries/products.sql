-- 商品、選項、SKU、圖片引用與 hashtag 的寫入與讀取。

-- name: InsertProduct :execrows
-- 建立的冪等：(store_id, request_id) 唯一索引是判重點。同鍵併發時後到的交易在這裡等到先到的提交，然後 ON CONFLICT DO NOTHING 回 0 列
-- （呼叫端改讀回先到的那筆、比對 request_hash）；先到的回滾時後到的插入成功。
INSERT INTO catalog.products (id, store_id, category_id, title, description, status, first_published_at, min_price, max_price, searchable_text, low_stock_threshold, request_id, request_hash, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(category_id), sqlc.arg(title), sqlc.arg(description), sqlc.arg(status), sqlc.narg(first_published_at),
        sqlc.arg(min_price), sqlc.arg(max_price), sqlc.arg(searchable_text), sqlc.narg(low_stock_threshold), sqlc.arg(request_id), sqlc.arg(request_hash), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: GetProductByRequest :one
-- 同一家店以同一個 request_id 建立過的商品（含已刪除）；重送時用它讀回並比對 request_hash。
SELECT id, request_hash, deleted_at FROM catalog.products WHERE store_id = sqlc.arg(store_id) AND request_id = sqlc.arg(request_id);

-- name: LockOwnProduct :one
-- 店家自己的、沒刪除的商品，鎖列；同一個商品的寫入都從這裡開始，所以 SKU、選項與價格區間的重算不會並行交錯。
SELECT id, status, first_published_at, content_version FROM catalog.products
WHERE id = sqlc.arg(id) AND store_id = sqlc.arg(store_id) AND deleted_at IS NULL
FOR UPDATE;

-- name: OwnProductDeleted :one
-- 店家自己的商品是否已刪除（Delete 重送冪等：已刪除回成功；別家的或不存在的查不到，仍是找不到）。
SELECT EXISTS (SELECT 1 FROM catalog.products WHERE id = sqlc.arg(id) AND store_id = sqlc.arg(store_id) AND deleted_at IS NOT NULL);

-- name: UpdateProductContent :exec
-- 賣家可編輯的內容；content_version 加 version_bump（etag 的來源；修改是 1，建立時補寫內容是 0，建立的版本是 1）。呼叫前已用 LockOwnProduct 鎖列並比對過版本（同一把列鎖之下，版本不會在比對與寫入之間改變）。
UPDATE catalog.products
SET category_id = sqlc.arg(category_id), title = sqlc.arg(title), description = sqlc.arg(description),
    searchable_text = sqlc.arg(searchable_text), low_stock_threshold = sqlc.narg(low_stock_threshold),
    content_version = content_version + sqlc.arg(version_bump)::bigint, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: SetProductStatus :exec
-- 設成 ACTIVE 時 first_published_at 只在還是空的時候填（之後的下架再上架不改它）。狀態是 etag 涵蓋的範圍，content_version 加一。
UPDATE catalog.products
SET status = sqlc.arg(status)::text, content_version = content_version + 1,
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
-- 公開可讀：ACTIVE 或 ARCHIVED（下架的商品頁保留、不可購買）、沒刪除、店家在 at 這一刻可曝光；草稿不可讀。子表的 JSON 欄位同 GetOwnProduct：公開讀取會被別的模組呼叫，不開快照 transaction（backend/AGENTS.md 規則 3），靠單一語句取得一致的讀取。
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
-- 後台讀任何店家的商品（任何狀態，沒刪除）；唯讀。圖片、選項、SKU 與 hashtag 以 JSON 欄位在同一句 SQL 內讀出：一句就是一份快照，不會讀到並行寫入的一半。
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
-- 賣家清單：店家自己的所有沒刪除的商品，不看狀態也不看曝光；最新建立的在前。status 空字串是全部；stock_status 空字串是不篩選，LOW 與 OUT_OF_STOCK
-- 只比對 ACTIVE 的商品（LOW：有 SKU 庫存大於 0 且小於或等於商品的門檻，沒設用 default_threshold；OUT_OF_STOCK：有 SKU 庫存是 0）。
-- query_pattern 是已跳脫的 ILIKE 樣式（空字串不篩選），比對 searchable_text。游標是 (created_at, id)；sort_key 是 created_at 的微秒數，給呼叫端組游標。
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
-- 條件同 ListSellerProducts（沒有游標）。
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
-- 後台列表：全站沒刪除的商品（任何狀態），最新建立的在前；store_id 與 query_pattern 選填。
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
-- 公開列表與搜尋：只含 ACTIVE、沒刪除、店家在 at 這一刻可曝光（seller.pub_exposable_stores）的商品。排序鍵 sort_key 依 sort 決定、一律由大到小
-- （價格低到高取負值），同鍵依 id 遞減；游標是 (sort_key, id)。sort 是 LATEST、PRICE_ASC、PRICE_DESC、SALES、RATING 或 RELEVANCE（RELEVANCE 要有 query）。
-- query_pattern 是已跳脫的 ILIKE 樣式（空字串不篩選），query 是原字串（相關度用 word_similarity）。category_ids 可給頂層（含底下的葉節點）或葉節點。
-- 空的陣列是不篩選；hashtags 是正規化名稱，hashtag_all 為真時要全部都有。
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
-- 搜尋框的建議：可見商品（ACTIVE、店家可曝光）的標題含 query_pattern，相關度高的在前，已去重。
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
-- 標籤建議：名稱符合 name_pattern（已跳脫的 ILIKE 樣式）的標籤與它的可見商品數（ACTIVE、店家可曝光），商品數多的在前。display 是最常用的輸入樣子。
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
-- 熱門標籤：各標籤的可見商品數，多的在前。
SELECT h.name::text AS name, mode() WITHIN GROUP (ORDER BY h.display_name)::text AS display, count(*)::bigint AS product_count
FROM catalog.product_hashtags h
JOIN catalog.products p ON p.id = h.product_id
WHERE p.status = 'ACTIVE' AND p.deleted_at IS NULL
  AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)
GROUP BY h.name
ORDER BY count(*) DESC, h.name
LIMIT sqlc.arg(row_limit);
