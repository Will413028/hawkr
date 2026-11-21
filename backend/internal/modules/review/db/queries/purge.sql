-- 買家刪帳的清除與 relay 用的讀取。不是公開讀取：作者已不是營運中（pub_inactive_buyers），公開述詞必定排除他的評價。
-- 這個檔的查詢都列在 readpolicy_test.go 的 nonPublicReads。

-- name: LockBuyerReviewBatch :many
-- 清除的一批：依 id 升序鎖住買家還在的評價（重跑時只剩沒刪的）。
SELECT id, store_id, product_id FROM review.reviews
WHERE buyer_id = sqlc.arg(buyer_id)
ORDER BY id
LIMIT sqlc.arg(max_rows)
FOR NO KEY UPDATE;

-- name: DeleteReviewsByID :execrows
-- 附圖列由 review_images 的 ON DELETE CASCADE 一起刪。
DELETE FROM review.reviews WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ListInactiveBuyersWithReviews :many
-- 清除的保底掃描：買家不再營運中（buyer.pub_inactive_buyers()）但仍有評價列，依 buyer_id 的 keyset 由小到大，最多 max_rows 位；第一頁的游標是 0。
SELECT ib.buyer_id FROM buyer.pub_inactive_buyers() AS ib
WHERE ib.buyer_id > sqlc.arg(after_id)::bigint
  AND EXISTS (SELECT 1 FROM review.reviews AS r WHERE r.buyer_id = ib.buyer_id)
ORDER BY ib.buyer_id
LIMIT sqlc.arg(max_rows);

-- name: GetReviewByID :one
-- relay 讀單則評價（建立事件的 payload）：不過濾可見性，不存在才是沒有列。
SELECT * FROM review.reviews WHERE id = sqlc.arg(id);

-- name: IsReviewPublic :one
-- relay 在 job 執行當下判斷這則評價現在是否公開可讀（沒有未還原的隱藏、作者不在 buyer.pub_inactive_buyers()，與 testdata/public_predicate.sql 同一段述詞）。
SELECT EXISTS (
  SELECT 1 FROM review.reviews AS r
  WHERE r.id = sqlc.arg(id)
    AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
    AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id)
);
