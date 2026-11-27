
-- name: LockReviewsForTakedown :many
SELECT id, store_id FROM review.reviews
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id
FOR NO KEY UPDATE;
