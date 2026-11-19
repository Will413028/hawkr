-- 評價的隱藏紀錄（可多次隱藏與還原）。可見性只由 review 寫；呼叫者是 moderation 的 transaction，先鎖評價列（LockReview）。

-- name: InsertHide :one
-- 同一則評價已經有未還原的隱藏時回既有的那一列（冪等）；回傳的 id 等於傳入的 id 才是新增。
INSERT INTO review.hides (id, review_id, reason_category, reason_text, source_ref, admin_id, hidden_at)
VALUES (sqlc.arg(id), sqlc.arg(review_id), sqlc.arg(reason_category), sqlc.arg(reason_text), sqlc.narg(source_ref), sqlc.arg(admin_id), sqlc.arg(now))
ON CONFLICT (review_id) WHERE restored_at IS NULL
DO UPDATE SET review_id = EXCLUDED.review_id
RETURNING *;

-- name: RestoreHide :one
-- 還原評價目前未還原的隱藏（留列不刪，之後可以再隱藏）；沒有未還原的隱藏沒有列。
UPDATE review.hides
SET restored_at = GREATEST(sqlc.arg(now)::timestamptz, hidden_at), restored_by = sqlc.arg(admin_id), restore_reason = sqlc.arg(reason)
WHERE review_id = sqlc.arg(review_id) AND restored_at IS NULL
RETURNING *;
