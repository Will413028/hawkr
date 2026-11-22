-- 公開可讀的評價讀取。這個檔的每一支查詢都以同一段述詞（公開可讀：沒有未還原的隱藏、作者不在 buyer.pub_inactive_buyers()，別名 r）過濾，
-- testdata/public_predicate.sql 是那段述詞的 golden，TestEveryPublicQueryCarriesThePublicPredicate 逐支比對；
-- 沒有述詞的讀取不得放在這個檔。觀看者的封鎖另有一支查詢（匿名與有觀看者分兩支，不用 `$v IS NULL OR …` 開關）：
-- 買家之間任一方向的封鎖與觀看者封鎖店，在 keyset 之前過濾，所以被擋的列不佔頁。統計不過濾觀看者。
-- 觀看者參數寫成 `sqlc.arg(viewer_buyer_id)::bigint`：sqlc 對「pub_* 函式別名的欄位 = 裸 sqlc.arg」會誤報 table alias does not exist，加型別轉換即可。

-- name: ListProductReviewsAnonymous :many
SELECT r.* FROM review.reviews AS r
WHERE r.product_id = sqlc.arg(product_id)
  AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id)
  AND (sqlc.arg(rating)::smallint = 0 OR r.rating = sqlc.arg(rating)::smallint)
  AND (NOT sqlc.arg(with_images)::boolean OR EXISTS (SELECT 1 FROM review.review_images AS ri WHERE ri.review_id = r.id))
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL OR (r.created_at, r.id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(page_size);

-- name: ListProductReviewsForViewer :many
SELECT r.* FROM review.reviews AS r
WHERE r.product_id = sqlc.arg(product_id)
  AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id)
  AND NOT EXISTS (
    SELECT 1 FROM buyer.pub_blocked_buyers() bb
    WHERE (bb.blocker_buyer_id = sqlc.arg(viewer_buyer_id)::bigint AND bb.blocked_buyer_id = r.buyer_id)
       OR (bb.blocker_buyer_id = r.buyer_id AND bb.blocked_buyer_id = sqlc.arg(viewer_buyer_id)::bigint))
  AND NOT EXISTS (
    SELECT 1 FROM buyer.pub_blocked_stores() bs
    WHERE bs.blocker_buyer_id = sqlc.arg(viewer_buyer_id)::bigint AND bs.store_id = r.store_id)
  AND (sqlc.arg(rating)::smallint = 0 OR r.rating = sqlc.arg(rating)::smallint)
  AND (NOT sqlc.arg(with_images)::boolean OR EXISTS (SELECT 1 FROM review.review_images AS ri WHERE ri.review_id = r.id))
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL OR (r.created_at, r.id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(page_size);

-- name: ListStoreReviewsAnonymous :many
SELECT r.* FROM review.reviews AS r
WHERE r.store_id = sqlc.arg(store_id)
  AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id)
  AND (sqlc.arg(rating)::smallint = 0 OR r.rating = sqlc.arg(rating)::smallint)
  AND (NOT sqlc.arg(with_images)::boolean OR EXISTS (SELECT 1 FROM review.review_images AS ri WHERE ri.review_id = r.id))
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL OR (r.created_at, r.id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(page_size);

-- name: ListStoreReviewsForViewer :many
SELECT r.* FROM review.reviews AS r
WHERE r.store_id = sqlc.arg(store_id)
  AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id)
  AND NOT EXISTS (
    SELECT 1 FROM buyer.pub_blocked_buyers() bb
    WHERE (bb.blocker_buyer_id = sqlc.arg(viewer_buyer_id)::bigint AND bb.blocked_buyer_id = r.buyer_id)
       OR (bb.blocker_buyer_id = r.buyer_id AND bb.blocked_buyer_id = sqlc.arg(viewer_buyer_id)::bigint))
  AND NOT EXISTS (
    SELECT 1 FROM buyer.pub_blocked_stores() bs
    WHERE bs.blocker_buyer_id = sqlc.arg(viewer_buyer_id)::bigint AND bs.store_id = r.store_id)
  AND (sqlc.arg(rating)::smallint = 0 OR r.rating = sqlc.arg(rating)::smallint)
  AND (NOT sqlc.arg(with_images)::boolean OR EXISTS (SELECT 1 FROM review.review_images AS ri WHERE ri.review_id = r.id))
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL OR (r.created_at, r.id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(page_size);

-- name: GetPublicReviewAnonymous :one
SELECT r.* FROM review.reviews AS r
WHERE r.id = sqlc.arg(id)
  AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id);

-- name: GetPublicReviewForViewer :one
SELECT r.* FROM review.reviews AS r
WHERE r.id = sqlc.arg(id)
  AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id)
  AND NOT EXISTS (
    SELECT 1 FROM buyer.pub_blocked_buyers() bb
    WHERE (bb.blocker_buyer_id = sqlc.arg(viewer_buyer_id)::bigint AND bb.blocked_buyer_id = r.buyer_id)
       OR (bb.blocker_buyer_id = r.buyer_id AND bb.blocked_buyer_id = sqlc.arg(viewer_buyer_id)::bigint))
  AND NOT EXISTS (
    SELECT 1 FROM buyer.pub_blocked_stores() bs
    WHERE bs.blocker_buyer_id = sqlc.arg(viewer_buyer_id)::bigint AND bs.store_id = r.store_id);

-- name: ProductStats :one
SELECT
    COUNT(*)::bigint                                   AS total_count,
    ROUND(COALESCE(AVG(r.rating), 0)::numeric, 2)::float8 AS avg_rating,
    COUNT(*) FILTER (WHERE r.rating = 5)::bigint       AS rating_5_count,
    COUNT(*) FILTER (WHERE r.rating = 4)::bigint       AS rating_4_count,
    COUNT(*) FILTER (WHERE r.rating = 3)::bigint       AS rating_3_count,
    COUNT(*) FILTER (WHERE r.rating = 2)::bigint       AS rating_2_count,
    COUNT(*) FILTER (WHERE r.rating = 1)::bigint       AS rating_1_count,
    COUNT(*) FILTER (WHERE EXISTS (SELECT 1 FROM review.review_images AS ri WHERE ri.review_id = r.id))::bigint AS with_images_count
FROM review.reviews AS r
WHERE r.product_id = sqlc.arg(product_id)
  AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id);

-- name: StoreStats :one
SELECT
    COUNT(*)::bigint                                   AS total_count,
    ROUND(COALESCE(AVG(r.rating), 0)::numeric, 2)::float8 AS avg_rating,
    COUNT(*) FILTER (WHERE r.rating = 5)::bigint       AS rating_5_count,
    COUNT(*) FILTER (WHERE r.rating = 4)::bigint       AS rating_4_count,
    COUNT(*) FILTER (WHERE r.rating = 3)::bigint       AS rating_3_count,
    COUNT(*) FILTER (WHERE r.rating = 2)::bigint       AS rating_2_count,
    COUNT(*) FILTER (WHERE r.rating = 1)::bigint       AS rating_1_count
FROM review.reviews AS r
WHERE r.store_id = sqlc.arg(store_id)
  AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id);

-- name: StoreRatingStats :many
-- 只回有可見評價的店（沒有評價的店沒有列），依 store_id 升冪。
SELECT
    r.store_id,
    COUNT(*)::bigint                          AS total_count,
    ROUND(AVG(r.rating)::numeric, 2)::float8  AS avg_rating
FROM review.reviews AS r
WHERE r.store_id = ANY(sqlc.arg(store_ids)::bigint[])
  AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id)
GROUP BY r.store_id
ORDER BY r.store_id;
