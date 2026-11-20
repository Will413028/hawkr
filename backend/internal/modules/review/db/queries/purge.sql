-- 買家刪帳的清除與 relay 用的讀取。不是公開讀取：作者已在 inactive_authors，公開述詞必定排除他的評價。
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

-- name: ListBuyerReviewTargets :many
-- 買家評過的 (店, 商品)，含已隱藏的：這些聚合都要重送。
SELECT DISTINCT store_id, product_id FROM review.reviews
WHERE buyer_id = sqlc.arg(buyer_id)
ORDER BY store_id, product_id;

-- name: GetReviewByID :one
-- relay 讀單則評價（建立事件的 payload）：不過濾可見性，不存在才是沒有列。
SELECT * FROM review.reviews WHERE id = sqlc.arg(id);
