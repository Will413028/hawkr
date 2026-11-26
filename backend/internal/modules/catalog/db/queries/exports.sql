-- 提供給別的模組與 legacy 轉接層的批次讀取與計數（B4-4a）。每個讀取是一句 SQL（一份快照；會被別的模組呼叫，規則 3 不讓開 InReadSnapshot）。

-- name: GetProductCards :many
-- 批次商品卡片：沒刪除的商品；include_inactive 為假時只回 ACTIVE 且店家在 at 可曝光的（公開可見），為真時任何狀態都回（直接連結仍可瀏覽）。
-- 欄位同 ListPublicProducts 的一列（sort_key 固定 0），讓列表的轉換共用。
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
-- 批次商品所屬店家：沒刪除的商品，任何狀態、不看店家曝光（草稿與下架的商品也回）。
SELECT p.id, p.store_id FROM catalog.products p
WHERE p.id = ANY(sqlc.arg(ids)::uuid[]) AND p.deleted_at IS NULL
ORDER BY p.id;

-- name: ListStoresWithPublicProducts :many
-- 至少有一件公開商品（ACTIVE、沒刪除）且店家在 at 可曝光的店家，ID 升序。
SELECT DISTINCT p.store_id FROM catalog.products p
WHERE p.status = 'ACTIVE' AND p.deleted_at IS NULL
  AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)
ORDER BY p.store_id;

-- name: GetSkuSnapshots :many
-- 批次 SKU 與它的商品此刻的內容：不過濾狀態與刪除（訂單的快照仍指向已刪除或下架的 SKU），由呼叫端依回傳的狀態判斷；不存在的 SKU 沒有列。
SELECT s.id AS sku_id, s.product_id, p.store_id, p.title, p.status,
       (p.deleted_at IS NOT NULL)::boolean AS product_deleted,
       (s.deleted_at IS NOT NULL)::boolean AS sku_deleted,
       s.option1_value, s.option2_value, s.price, s.compare_at_price, s.stock_qty,
       COALESCE((SELECT i.object_key FROM catalog.product_images i WHERE i.product_id = p.id AND i.kind = 'GALLERY' ORDER BY i.position LIMIT 1), '')::text AS cover_key
FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id
WHERE s.id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY s.id;

-- name: IncrementProductFavorites :execrows
-- 收藏數加一（買家在自己的 transaction 內呼叫，D3）；已刪除的商品不計（無列）。上限夾在 INTEGER 最大值。不改 content_version。
UPDATE catalog.products
SET favorite_count = LEAST(favorite_count::bigint + 1, 2147483647)::integer, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: DecrementProductFavorites :execrows
-- 收藏數減一，下限 0；刪除的商品也減（取消收藏時計數要跟著收藏列走）。商品不存在才無列。
UPDATE catalog.products
SET favorite_count = GREATEST(favorite_count - 1, 0), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);
