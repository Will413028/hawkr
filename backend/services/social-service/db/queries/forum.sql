-- name: ListTopLevelForum :many
SELECT * FROM cms_seller_forum
WHERE seller_id = $1 AND parent_id = 0 AND deleted_at IS NULL
ORDER BY create_time DESC
LIMIT $2 OFFSET $3;

-- name: CountTopLevelForum :one
SELECT COUNT(*) FROM cms_seller_forum
WHERE seller_id = $1 AND parent_id = 0 AND deleted_at IS NULL;

-- name: ListForumReplies :many
SELECT * FROM cms_seller_forum
WHERE parent_id = $1 AND deleted_at IS NULL
ORDER BY create_time ASC
LIMIT $2 OFFSET $3;

-- name: CountForumReplies :one
SELECT COUNT(*) FROM cms_seller_forum
WHERE parent_id = $1 AND deleted_at IS NULL;

-- name: CreateForumEntry :one
INSERT INTO cms_seller_forum
    (id, seller_id, user_id, user_name, user_avatar, content, parent_id, reply_to_id, reply_to_name, role)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetForumEntry :one
SELECT * FROM cms_seller_forum WHERE id = $1 LIMIT 1;

-- name: SoftDeleteForumEntry :exec
UPDATE cms_seller_forum SET deleted_at = NOW(), update_time = NOW() WHERE id = $1;

-- name: IncrForumReplyCount :exec
UPDATE cms_seller_forum SET reply_count = reply_count + 1, update_time = NOW() WHERE id = $1;

-- name: UpdateForumContent :one
UPDATE cms_seller_forum SET content = $2, update_time = NOW() WHERE id = $1
RETURNING *;

-- name: DecrForumReplyCount :exec
UPDATE cms_seller_forum SET reply_count = GREATEST(0, reply_count - 1), update_time = NOW() WHERE id = $1;

-- name: SoftDeleteForumByUserID :exec
UPDATE cms_seller_forum SET deleted_at = NOW(), update_time = NOW()
WHERE role = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: HideForumEntry :exec
UPDATE cms_seller_forum
SET hidden_at = NOW(), hidden_reason = $2, update_time = NOW()
WHERE id = $1 AND deleted_at IS NULL AND hidden_at IS NULL;
