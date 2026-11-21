-- 店家回覆的內容下架（seller 的措施，A6-D21）：review 不存複本，讀取直接 join seller.pub_content_takedowns()；訂閱者只鎖評價列並核對歸屬。

-- name: LockReviewsForTakedown :many
-- 依 id 升序鎖住存在的評價列（不存在的列不回）。
SELECT id, store_id FROM review.reviews
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id
FOR NO KEY UPDATE;
