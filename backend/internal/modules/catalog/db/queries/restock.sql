-- 補貨：SKU 由不可買變可買（有庫存、商品 ACTIVE 且未刪、SKU 未刪、店家在 at 可曝光）的偵測與 job 執行時重新確認用的讀取。

-- name: GetSkuRestockState :one
-- 一個 SKU 此刻的庫存與可買與否，加上通知用的最新快照（商品名稱、主圖 key）。SKU 已刪除也回列（歸還不看刪除），purchasable 為 false。
SELECT s.product_id, p.store_id, p.title, s.stock_qty,
       COALESCE((SELECT i.object_key FROM catalog.product_images i WHERE i.product_id = p.id AND i.kind = 'GALLERY' ORDER BY i.position LIMIT 1), '')::text AS cover_key,
       (s.deleted_at IS NULL AND s.stock_qty > 0 AND p.deleted_at IS NULL AND p.status = 'ACTIVE'
        AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e))::boolean AS purchasable
FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id
WHERE s.id = sqlc.arg(id);

-- name: ListPurchasableSkusOfProduct :many
-- 商品切成 ACTIVE 之後可買的 SKU。
SELECT s.id FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id
WHERE p.id = sqlc.arg(product_id) AND s.deleted_at IS NULL AND s.stock_qty > 0 AND p.deleted_at IS NULL AND p.status = 'ACTIVE'
  AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)
ORDER BY s.id;

-- name: ListPurchasableSkusOfStore :many
-- 店家目前可買的 SKU（店家要在 at 可曝光），依 ID 升序的 keyset 分頁；after 是上一頁最後一個 ID（第一頁用 nil UUID）。
SELECT s.id FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id
WHERE p.store_id = sqlc.arg(store_id) AND s.id > sqlc.arg(after)::uuid
  AND s.deleted_at IS NULL AND s.stock_qty > 0 AND p.deleted_at IS NULL AND p.status = 'ACTIVE'
  AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)
ORDER BY s.id
LIMIT sqlc.arg(page_size);
