-- 寫入路徑與作者本人的讀取。公開讀取在 public.sql（那裡每支都帶公開可讀述詞）；這個檔的查詢都不是公開讀取，
-- TestNonPublicReviewReadsAreListed 要求每一支讀 review.reviews 的查詢不是在 public.sql，就是在這裡明列的名單裡。

-- name: InsertReview :one
-- 同一個訂單項目已有評價時沒有列（ON CONFLICT DO NOTHING，不丟 23505：撞唯一鍵後同一個 transaction 不再下 SQL）。
INSERT INTO review.reviews (id, order_no, order_item_id, buyer_id, store_id, product_id, rating, content, sku_name, item_image_key, price, created_at)
VALUES (sqlc.arg(id), sqlc.arg(order_no), sqlc.arg(order_item_id), sqlc.arg(buyer_id), sqlc.arg(store_id), sqlc.arg(product_id), sqlc.arg(rating), sqlc.narg(content), sqlc.arg(sku_name), sqlc.narg(item_image_key), sqlc.arg(price), sqlc.arg(created_at))
ON CONFLICT (order_item_id) DO NOTHING
RETURNING *;

-- name: InsertReviewImage :exec
INSERT INTO review.review_images (review_id, position, asset_id, object_key)
VALUES (sqlc.arg(review_id), sqlc.arg(position), sqlc.arg(asset_id), sqlc.arg(object_key));

-- name: ListReviewImages :many
-- 已經過可讀述詞的評價的附圖，依評價與位置。
SELECT review_id, position, asset_id, object_key FROM review.review_images
WHERE review_id = ANY(sqlc.arg(review_ids)::uuid[])
ORDER BY review_id, position;

-- name: ListMyReviews :many
-- 作者本人：隱藏的評價不給作者（舊行為）。keyset：(created_at, id) 由大到小。
SELECT r.* FROM review.reviews AS r
WHERE r.buyer_id = sqlc.arg(buyer_id)
  AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL OR (r.created_at, r.id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(page_size);

-- name: ReplyToReview :one
-- 店家回覆（可覆蓋）：擁有權與公開可讀都在 WHERE，0 列由呼叫端補查分流（不存在、不是這家店、不可回覆）。
UPDATE review.reviews AS r
SET reply_content = sqlc.arg(content), replied_at = sqlc.arg(now)
WHERE r.id = sqlc.arg(id)
  AND r.store_id = sqlc.arg(store_id)
  AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM review.inactive_authors AS ia WHERE ia.buyer_id = r.buyer_id)
RETURNING r.*;

-- name: GetReviewForDiagnosis :one
-- 回覆被拒時分流用：不過濾可見性。
SELECT r.*, EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL) AS hidden
FROM review.reviews AS r
WHERE r.id = sqlc.arg(id);

-- name: LockReview :one
-- 隱藏與還原先鎖評價列（只鎖自己的列）。
SELECT * FROM review.reviews WHERE id = sqlc.arg(id) FOR NO KEY UPDATE;
