
-- name: InsertHide :one
INSERT INTO review.hides (id, review_id, reason_category, reason_text, source_ref, admin_id, hidden_at)
VALUES (sqlc.arg(id), sqlc.arg(review_id), sqlc.arg(reason_category), sqlc.arg(reason_text), sqlc.narg(source_ref), sqlc.arg(admin_id), sqlc.arg(now))
ON CONFLICT (review_id) WHERE restored_at IS NULL
DO UPDATE SET review_id = EXCLUDED.review_id
RETURNING *;

-- name: RestoreHide :one
UPDATE review.hides
SET restored_at = GREATEST(sqlc.arg(now)::timestamptz, hidden_at), restored_by = sqlc.arg(admin_id), restore_reason = sqlc.arg(reason)
WHERE review_id = sqlc.arg(review_id) AND restored_at IS NULL AND (sqlc.narg(source_ref)::text IS NULL OR source_ref = sqlc.narg(source_ref)::text)
RETURNING *;

-- name: HasActiveHide :one
SELECT EXISTS (SELECT 1 FROM review.hides WHERE review_id = sqlc.arg(review_id) AND restored_at IS NULL);

-- name: TakenDownReplyIDs :many
SELECT td.content_id::uuid AS review_id FROM seller.pub_content_takedowns() AS td
WHERE td.content_kind = 'REVIEW_REPLY' AND td.content_id = ANY(sqlc.arg(ids)::uuid[]);
