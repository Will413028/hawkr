
-- name: InsertReview :one
INSERT INTO review.reviews (id, order_no, order_item_id, buyer_id, store_id, product_id, rating, content, sku_name, item_image_key, price, created_at)
VALUES (sqlc.arg(id), sqlc.arg(order_no), sqlc.arg(order_item_id), sqlc.arg(buyer_id), sqlc.arg(store_id), sqlc.arg(product_id), sqlc.arg(rating), sqlc.narg(content), sqlc.arg(sku_name), sqlc.narg(item_image_key), sqlc.arg(price), sqlc.arg(created_at))
ON CONFLICT (order_item_id) DO NOTHING
RETURNING *;

-- name: InsertReviewImage :exec
INSERT INTO review.review_images (review_id, position, asset_id, object_key)
VALUES (sqlc.arg(review_id), sqlc.arg(position), sqlc.arg(asset_id), sqlc.arg(object_key));

-- name: ListReviewImages :many
SELECT review_id, position, asset_id, object_key FROM review.review_images
WHERE review_id = ANY(sqlc.arg(review_ids)::uuid[])
ORDER BY review_id, position;

-- name: ListMyReviews :many
SELECT r.* FROM review.reviews AS r
WHERE r.buyer_id = sqlc.arg(buyer_id)
  AND (sqlc.arg(order_item_id)::bigint = 0 OR r.order_item_id = sqlc.arg(order_item_id)::bigint)
  AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL OR (r.created_at, r.id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(page_size);

-- name: ReplyToReview :one
UPDATE review.reviews AS r
SET reply_content = sqlc.arg(content), replied_at = sqlc.arg(now)
WHERE r.id = sqlc.arg(id)
  AND r.store_id = sqlc.arg(store_id)
  AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'REVIEW_REPLY' AND td.content_id = r.id)
RETURNING r.*;

-- name: GetReviewForDiagnosis :one
SELECT r.*, EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL) AS hidden
FROM review.reviews AS r
WHERE r.id = sqlc.arg(id);

-- name: GetReviewStore :one
SELECT r.store_id,
  (NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
   AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id))::boolean AS public
FROM review.reviews AS r
WHERE r.id = sqlc.arg(id);

-- name: LockReview :one
SELECT * FROM review.reviews WHERE id = sqlc.arg(id) FOR NO KEY UPDATE;

-- name: AuthorInactive :one
SELECT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = sqlc.arg(buyer_id)::bigint);
