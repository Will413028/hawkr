
-- name: LockBuyerReviewBatch :many
SELECT id, store_id, product_id FROM review.reviews
WHERE buyer_id = sqlc.arg(buyer_id)
ORDER BY id
LIMIT sqlc.arg(max_rows)
FOR NO KEY UPDATE;

-- name: DeleteReviewsByID :execrows
DELETE FROM review.reviews WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ListInactiveBuyersWithReviews :many
SELECT ib.buyer_id FROM buyer.pub_inactive_buyers() AS ib
WHERE ib.buyer_id > sqlc.arg(after_id)::bigint
  AND EXISTS (SELECT 1 FROM review.reviews AS r WHERE r.buyer_id = ib.buyer_id)
ORDER BY ib.buyer_id
LIMIT sqlc.arg(max_rows);

-- name: GetReviewByID :one
SELECT * FROM review.reviews WHERE id = sqlc.arg(id);

-- name: IsReviewPublic :one
SELECT EXISTS (
  SELECT 1 FROM review.reviews AS r
  WHERE r.id = sqlc.arg(id)
    AND NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
    AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id)
);
