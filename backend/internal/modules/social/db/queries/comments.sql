-- 貼文的留言的寫入與店主、作者自己的讀取。帶「公開可讀、可見」規則的讀取（列表、佔位項目、回覆的對象檢查、隱藏旗標）是 go-jet 建的，
-- 在 internal/reads，規則在 internal/visibility。

-- name: InsertComment :one
INSERT INTO social.comments (id, post_id, parent_id, reply_to_id, actor_kind, actor_id, body, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(post_id), sqlc.narg(parent_id), sqlc.narg(reply_to_id), sqlc.arg(actor_kind), sqlc.arg(actor_id),
    sqlc.arg(body)::text, sqlc.arg(now), sqlc.arg(now)
)
RETURNING *;

-- name: LockComment :one
-- 鎖住留言（任何狀態）並帶回所屬貼文的店家：編輯與刪除要先確認作者與歸屬。
SELECT sqlc.embed(c), p.store_id AS post_store_id
FROM social.comments c
JOIN social.posts p ON p.id = c.post_id
WHERE c.id = sqlc.arg(id)
FOR UPDATE OF c;

-- name: GetComment :one
SELECT sqlc.embed(c), p.store_id AS post_store_id
FROM social.comments c
JOIN social.posts p ON p.id = c.post_id
WHERE c.id = sqlc.arg(id);

-- name: UpdateCommentBody :one
UPDATE social.comments SET body = sqlc.arg(body)::text, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteComment :execrows
-- 軟刪；已經刪除的 0 列（呼叫方不做副作用）。
UPDATE social.comments SET deleted_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: GetCommentCounts :one
SELECT like_count, reply_count FROM social.comment_stats WHERE comment_id = sqlc.arg(id);

