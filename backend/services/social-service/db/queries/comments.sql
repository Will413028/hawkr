-- name: GetComment :one
SELECT * FROM cms_comment WHERE id = $1 LIMIT 1;

-- name: ListTopLevelComments :many
SELECT * FROM cms_comment
WHERE post_id = $1 AND parent_id = 0 AND deleted_at IS NULL
ORDER BY create_time DESC
LIMIT $2 OFFSET $3;

-- name: CountTopLevelComments :one
SELECT COUNT(*) FROM cms_comment
WHERE post_id = $1 AND parent_id = 0 AND deleted_at IS NULL;

-- name: ListReplies :many
SELECT * FROM cms_comment
WHERE parent_id = $1 AND deleted_at IS NULL
ORDER BY create_time DESC
LIMIT $2 OFFSET $3;

-- name: CountReplies :one
SELECT COUNT(*) FROM cms_comment
WHERE parent_id = $1 AND deleted_at IS NULL;

-- name: GetLatestReplies :many
SELECT * FROM cms_comment
WHERE parent_id = $1 AND deleted_at IS NULL
ORDER BY create_time DESC
LIMIT $2;

-- name: BatchGetLatestReplies :many
-- Get latest N replies for multiple parent IDs in one query.
SELECT r.*
FROM unnest(@parent_ids::bigint[]) AS pid(id)
CROSS JOIN LATERAL (
  SELECT * FROM cms_comment
  WHERE parent_id = pid.id AND deleted_at IS NULL
  ORDER BY create_time DESC
  LIMIT sqlc.arg(reply_limit)
) r
ORDER BY r.parent_id, r.create_time DESC;

-- name: CreateComment :one
INSERT INTO cms_comment
    (id, post_id, user_id, user_name, user_avatar, content, parent_id, reply_to_id, reply_to_name, role)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateCommentContent :one
UPDATE cms_comment SET content = $2, update_time = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteComment :exec
UPDATE cms_comment SET deleted_at = NOW(), update_time = NOW() WHERE id = $1;

-- name: IncrCommentReplyCount :exec
UPDATE cms_comment SET reply_count = reply_count + 1, update_time = NOW() WHERE id = $1;

-- name: DecrCommentReplyCount :exec
UPDATE cms_comment SET reply_count = GREATEST(0, reply_count - 1), update_time = NOW() WHERE id = $1;

-- name: IncrCommentLikeCount :exec
UPDATE cms_comment SET like_count = like_count + 1, update_time = NOW() WHERE id = $1;

-- name: DecrCommentLikeCount :exec
UPDATE cms_comment SET like_count = GREATEST(0, like_count - 1), update_time = NOW() WHERE id = $1;

-- name: GetCommentLike :one
SELECT * FROM cms_comment_like WHERE comment_id = $1 AND actor_kind = $2 AND user_id = $3 LIMIT 1;

-- name: CreateCommentLike :exec
INSERT INTO cms_comment_like (comment_id, actor_kind, user_id) VALUES ($1, $2, $3)
ON CONFLICT (comment_id, actor_kind, user_id) DO NOTHING;

-- name: DeleteCommentLike :exec
DELETE FROM cms_comment_like WHERE comment_id = $1 AND actor_kind = $2 AND user_id = $3;

-- name: ReconcileCommentLikeCounts :exec
-- LEFT JOIN so comments whose likes all dropped to 0 reconcile back to 0 instead
-- of staying stuck at a stale value (#1171). GROUP BY on cms_comment_like alone
-- omits zero-like comments entirely.
UPDATE cms_comment c SET like_count = sub.cnt
FROM (
    SELECT c2.id AS comment_id, COUNT(l.comment_id)::int AS cnt
    FROM cms_comment c2
    LEFT JOIN cms_comment_like l ON l.comment_id = c2.id
    WHERE c2.deleted_at IS NULL
    GROUP BY c2.id
) sub
WHERE c.id = sub.comment_id AND c.like_count != sub.cnt;

-- name: ReconcileCommentReplyCounts :exec
-- reply_count lives on top-level comments (parent_id = 0). LEFT JOIN from those
-- parents so a comment whose replies all got deleted reconciles back to 0 (#1171);
-- a GROUP BY on the replies alone can never emit a 0 row for a now-empty parent.
UPDATE cms_comment c SET reply_count = sub.cnt
FROM (
    SELECT parent.id AS parent_id, COUNT(r.id)::int AS cnt
    FROM cms_comment parent
    LEFT JOIN cms_comment r ON r.parent_id = parent.id AND r.deleted_at IS NULL
    WHERE parent.parent_id = 0 AND parent.deleted_at IS NULL
    GROUP BY parent.id
) sub
WHERE c.id = sub.parent_id AND c.reply_count != sub.cnt;

-- name: BatchGetCommentLikes :many
SELECT comment_id FROM cms_comment_like
WHERE comment_id = ANY($1::bigint[]) AND actor_kind = $2 AND user_id = $3;

-- name: DeleteCommentLikesByUserID :exec
DELETE FROM cms_comment_like WHERE actor_kind = $1 AND user_id = $2;

-- name: SoftDeleteCommentsByUserID :exec
UPDATE cms_comment SET deleted_at = NOW(), update_time = NOW()
WHERE role = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: HideComment :exec
UPDATE cms_comment
SET hidden_at = NOW(), hidden_reason = $2, update_time = NOW()
WHERE id = $1 AND deleted_at IS NULL AND hidden_at IS NULL;
