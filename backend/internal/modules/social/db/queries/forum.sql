-- 店家的討論區留言的寫入與作者自己的讀取。帶「公開可讀、可見」規則的讀取在 internal/reads（go-jet），規則在 internal/visibility。

-- name: InsertForumEntry :one
INSERT INTO social.forum_entries (id, store_id, parent_id, reply_to_id, actor_kind, actor_id, body, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(store_id), sqlc.narg(parent_id), sqlc.narg(reply_to_id), sqlc.arg(actor_kind), sqlc.arg(actor_id),
    sqlc.arg(body)::text, sqlc.arg(now), sqlc.arg(now)
)
RETURNING *;

-- name: LockForumEntry :one
SELECT * FROM social.forum_entries WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: GetForumEntry :one
SELECT * FROM social.forum_entries WHERE id = sqlc.arg(id);

-- name: UpdateForumEntryBody :one
UPDATE social.forum_entries SET body = sqlc.arg(body)::text, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteForumEntry :execrows
UPDATE social.forum_entries SET deleted_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

