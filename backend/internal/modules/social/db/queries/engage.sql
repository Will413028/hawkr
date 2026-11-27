
-- name: InsertPostLike :execrows
INSERT INTO social.post_likes (post_id, buyer_id, created_at)
VALUES (sqlc.arg(post_id), sqlc.arg(buyer_id), sqlc.arg(now))
ON CONFLICT (post_id, buyer_id) DO NOTHING;

-- name: DeletePostLike :execrows
DELETE FROM social.post_likes WHERE post_id = sqlc.arg(post_id) AND buyer_id = sqlc.arg(buyer_id);

-- name: PostsLikedBy :many
SELECT post_id FROM social.post_likes WHERE buyer_id = sqlc.arg(buyer_id) AND post_id = ANY(sqlc.arg(post_ids)::uuid[]);

-- name: InsertCommentLike :execrows
INSERT INTO social.comment_likes (comment_id, actor_kind, actor_id, created_at)
VALUES (sqlc.arg(comment_id), sqlc.arg(actor_kind), sqlc.arg(actor_id), sqlc.arg(now))
ON CONFLICT (comment_id, actor_kind, actor_id) DO NOTHING;

-- name: DeleteCommentLike :execrows
DELETE FROM social.comment_likes
WHERE comment_id = sqlc.arg(comment_id) AND actor_kind = sqlc.arg(actor_kind) AND actor_id = sqlc.arg(actor_id);

-- name: CommentsLikedBy :many
SELECT comment_id FROM social.comment_likes
WHERE actor_kind = sqlc.arg(actor_kind) AND actor_id = sqlc.arg(actor_id) AND comment_id = ANY(sqlc.arg(comment_ids)::uuid[]);

-- name: InsertPostShare :execrows
INSERT INTO social.post_shares (post_id, buyer_id, created_at)
VALUES (sqlc.arg(post_id), sqlc.arg(buyer_id), sqlc.arg(now))
ON CONFLICT (post_id, buyer_id) DO NOTHING;

-- name: InsertViewSalt :exec
INSERT INTO social.view_salts (view_day, salt) VALUES (sqlc.arg(view_day), sqlc.arg(salt))
ON CONFLICT (view_day) DO NOTHING;

-- name: GetViewSalt :one
SELECT salt FROM social.view_salts WHERE view_day = sqlc.arg(view_day);

-- name: InsertPostView :execrows
INSERT INTO social.post_views (post_id, view_day, viewer_key)
VALUES (sqlc.arg(post_id), sqlc.arg(view_day), sqlc.arg(viewer_key))
ON CONFLICT (post_id, view_day, viewer_key) DO NOTHING;

-- name: DeleteOldViewSalts :execrows
DELETE FROM social.view_salts WHERE view_day < sqlc.arg(before_day);

-- name: DeleteOldPostViews :execrows
DELETE FROM social.post_views WHERE view_day < sqlc.arg(before_day);
