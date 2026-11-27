
-- name: InsertForumEntry :one
INSERT INTO social.forum_entries (id, store_id, parent_id, reply_to_id, actor_kind, actor_id, body, created_at, updated_at, request_id, request_hash)
VALUES (
    sqlc.arg(id), sqlc.arg(store_id), sqlc.narg(parent_id), sqlc.narg(reply_to_id), sqlc.arg(actor_kind), sqlc.arg(actor_id),
    sqlc.arg(body)::text, sqlc.arg(now), sqlc.arg(now), sqlc.narg(request_id), sqlc.narg(request_hash)
)
ON CONFLICT (actor_kind, actor_id, request_id) DO NOTHING
RETURNING *;

-- name: ForumEntryByRequest :one
SELECT * FROM social.forum_entries
WHERE actor_kind = sqlc.arg(actor_kind) AND actor_id = sqlc.arg(actor_id) AND request_id = sqlc.arg(request_id);

-- name: LockForumEntry :one
SELECT * FROM social.forum_entries WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: GetForumEntry :one
SELECT * FROM social.forum_entries WHERE id = sqlc.arg(id);

-- name: UpdateForumEntryBody :one
UPDATE social.forum_entries SET body = sqlc.arg(body)::text, version = version + 1, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteForumEntry :execrows
UPDATE social.forum_entries SET deleted_at = sqlc.arg(now), version = version + 1, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;


-- name: GetForumEntryReplyCount :one
SELECT reply_count FROM social.forum_entry_stats WHERE forum_entry_id = sqlc.arg(id);
