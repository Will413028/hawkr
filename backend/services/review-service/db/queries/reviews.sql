-- name: InsertReview :one
INSERT INTO order_item_reviews (
  id, order_no, order_item_id, buyer_id, store_id,
  product_id, rating, content, image_keys, sku_name, item_image_key, price
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetReviewByOrderItemID :one
SELECT * FROM order_item_reviews
WHERE order_item_id = $1
LIMIT 1;

-- name: ListReviewedItemIDsByOrderNo :many
SELECT order_item_id FROM order_item_reviews
WHERE order_no = $1 AND buyer_id = $2;

-- name: ListSellerReviews :many
SELECT * FROM order_item_reviews
WHERE store_id = $1
  AND ($2::smallint = 0 OR rating = $2)
  AND deleted_at IS NULL AND hidden_at IS NULL
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountSellerReviews :one
SELECT COUNT(*) FROM order_item_reviews
WHERE store_id = $1
  AND ($2::smallint = 0 OR rating = $2)
  AND deleted_at IS NULL AND hidden_at IS NULL;

-- name: GetSellerStats :one
SELECT
    COUNT(*)                                        AS total_count,
    ROUND(COALESCE(AVG(rating), 0)::numeric, 2)::float8 AS avg_rating,
    COUNT(*) FILTER (WHERE rating = 5)              AS rating_5_count,
    COUNT(*) FILTER (WHERE rating = 4)              AS rating_4_count,
    COUNT(*) FILTER (WHERE rating = 3)              AS rating_3_count,
    COUNT(*) FILTER (WHERE rating = 2)              AS rating_2_count,
    COUNT(*) FILTER (WHERE rating = 1)              AS rating_1_count
FROM order_item_reviews
WHERE store_id = $1
  AND deleted_at IS NULL AND hidden_at IS NULL;

-- name: ListStoreRatingStats :many
-- 批次版的 GetSellerStats（total_count 與 avg_rating 的定義相同：排除軟刪除與隱藏的評論，平均四捨五入到小數兩位）。
-- 只回有評論的店（沒有評論的店沒有列），依 store_id 升冪。
SELECT
    store_id,
    COUNT(*)                                            AS total_count,
    ROUND(AVG(rating)::numeric, 2)::float8              AS avg_rating
FROM order_item_reviews
WHERE store_id = ANY($1::bigint[])
  AND deleted_at IS NULL AND hidden_at IS NULL
GROUP BY store_id
ORDER BY store_id;

-- name: ListBuyerReviews :many
-- 隱藏（平台審核）/ 軟刪除評論一律排除，與全站「hidden = 不可見」語意一致（#1156 regression）。
SELECT * FROM order_item_reviews
WHERE buyer_id = $1
  AND deleted_at IS NULL AND hidden_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountBuyerReviews :one
SELECT COUNT(*) FROM order_item_reviews
WHERE buyer_id = $1
  AND deleted_at IS NULL AND hidden_at IS NULL;

-- name: ListProductReviews :many
SELECT * FROM order_item_reviews
WHERE product_id = $1
  AND ($2::smallint = 0 OR rating = $2)
  AND ($3::boolean = false OR array_length(image_keys, 1) > 0)
  AND deleted_at IS NULL AND hidden_at IS NULL
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountProductReviews :one
SELECT COUNT(*) FROM order_item_reviews
WHERE product_id = $1
  AND ($2::smallint = 0 OR rating = $2)
  AND ($3::boolean = false OR array_length(image_keys, 1) > 0)
  AND deleted_at IS NULL AND hidden_at IS NULL;

-- name: GetReviewByID :one
SELECT * FROM order_item_reviews
WHERE id = $1
LIMIT 1;

-- name: GetProductStats :one
SELECT
    COUNT(*)                                        AS total_count,
    ROUND(COALESCE(AVG(rating), 0)::numeric, 2)::float8 AS avg_rating
FROM order_item_reviews
WHERE product_id = $1
  AND deleted_at IS NULL AND hidden_at IS NULL;

-- name: HideReview :exec
UPDATE order_item_reviews
SET hidden_at = NOW(), hidden_reason = $2
WHERE id = $1 AND deleted_at IS NULL AND hidden_at IS NULL;

-- name: ReplyToReview :one
-- 賣家回覆評論（#1174）。WHERE 含擁有權（store_id）+ 可見性（非 hidden/deleted），
-- 0 rows 由 usecase 補查分流 404/403/409。可編輯覆蓋（replied_at 每次更新為 NOW）。
UPDATE order_item_reviews
SET reply_content = $2, replied_at = NOW()
WHERE id = $1
  AND store_id = $3
  AND deleted_at IS NULL
  AND hidden_at IS NULL
RETURNING *;
