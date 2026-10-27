-- name: GetReviewsByApplication :many
SELECT * FROM certification_reviews
WHERE application_id = $1
ORDER BY created_at DESC;

-- name: GetLatestReview :one
SELECT * FROM certification_reviews
WHERE application_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateReview :one
INSERT INTO certification_reviews (
  application_id, reviewer_id, reviewer_name,
  action, result, comment, rejection_reason, required_fields,
  created_at
) VALUES (
  $1, $2, $3,
  $4, $5, $6, $7, $8,
  $9
) RETURNING *;
