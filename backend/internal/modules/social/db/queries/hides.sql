-- 買家寫的留言與討論區留言的隱藏紀錄，與買家的社群處分紀錄。計數由 hides 的 trigger 維護。

-- name: InsertCommentHide :one
-- 同一則留言已經有未還原的隱藏時回既有的那一列（冪等）；回傳的 id 等於傳入的 id 才是新增。
INSERT INTO social.hides (id, comment_id, reason_category, reason_text, source_ref, admin_id, hidden_at)
VALUES (sqlc.arg(id), sqlc.arg(comment_id), sqlc.arg(reason_category), sqlc.arg(reason_text), sqlc.narg(source_ref), sqlc.arg(admin_id), sqlc.arg(now))
ON CONFLICT (comment_id) WHERE restored_at IS NULL AND comment_id IS NOT NULL
DO UPDATE SET comment_id = EXCLUDED.comment_id
RETURNING *;

-- name: InsertForumEntryHide :one
INSERT INTO social.hides (id, forum_entry_id, reason_category, reason_text, source_ref, admin_id, hidden_at)
VALUES (sqlc.arg(id), sqlc.arg(forum_entry_id), sqlc.arg(reason_category), sqlc.arg(reason_text), sqlc.narg(source_ref), sqlc.arg(admin_id), sqlc.arg(now))
ON CONFLICT (forum_entry_id) WHERE restored_at IS NULL AND forum_entry_id IS NOT NULL
DO UPDATE SET forum_entry_id = EXCLUDED.forum_entry_id
RETURNING *;

-- name: RestoreCommentHide :one
-- 還原留言目前未還原的隱藏（留列不刪，之後可以再隱藏）；沒有未還原的隱藏沒有列。
UPDATE social.hides
SET restored_at = sqlc.arg(now), restored_by = sqlc.arg(admin_id), restore_reason = sqlc.arg(reason)
WHERE comment_id = sqlc.arg(comment_id) AND restored_at IS NULL
RETURNING *;

-- name: RestoreForumEntryHide :one
UPDATE social.hides
SET restored_at = sqlc.arg(now), restored_by = sqlc.arg(admin_id), restore_reason = sqlc.arg(reason)
WHERE forum_entry_id = sqlc.arg(forum_entry_id) AND restored_at IS NULL
RETURNING *;

-- name: InsertSanction :one
INSERT INTO social.sanctions (id, buyer_id, kind, reason_category, reason_text, source_ref, admin_id, created_at, ends_at)
VALUES (sqlc.arg(id), sqlc.arg(buyer_id), sqlc.arg(kind), sqlc.arg(reason_category), sqlc.arg(reason_text), sqlc.narg(source_ref), sqlc.arg(admin_id), sqlc.arg(now), sqlc.narg(ends_at))
RETURNING *;

-- name: ListSanctions :many
-- 買家自己的處分紀錄，新的在前。keyset：(created_at, id) 由大到小。
SELECT * FROM social.sanctions
WHERE buyer_id = sqlc.arg(buyer_id)
  AND (created_at, id) < (sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(max_rows);

-- name: ListHiddenContentOf :many
-- 買家自己被隱藏（未還原）的留言與討論區留言與理由：作者讀得到理由與自己的內容，別人讀不到。不含已刪除的內容（body 為 NULL 的列一定已刪除，
-- comments_scrubbed_check，所以這裡的 COALESCE 不會產生空字串）。
-- 新的在前，keyset：(hidden_at, id) 由大到小。留言沒有 store_id（0）、討論區留言沒有 post_id（零值 UUID）：sqlc 以第一段的型別推導欄位，不能用 NULL。
SELECT h.id AS hide_id, c.id AS content_id, 'COMMENT'::text AS content_kind, c.post_id, 0::bigint AS store_id,
       COALESCE(c.body, '')::text AS body, c.created_at, h.reason_category, h.reason_text, h.hidden_at
FROM social.hides h
JOIN social.comments c ON c.id = h.comment_id
WHERE h.restored_at IS NULL AND c.deleted_at IS NULL AND c.actor_kind = 'BUYER' AND c.actor_id = sqlc.arg(buyer_id)
  AND (h.hidden_at, h.id) < (sqlc.arg(after_hidden)::timestamptz, sqlc.arg(after_id)::uuid)
UNION ALL
SELECT h.id, e.id, 'FORUM_ENTRY'::text, '00000000-0000-0000-0000-000000000000'::uuid, e.store_id,
       COALESCE(e.body, '')::text, e.created_at, h.reason_category, h.reason_text, h.hidden_at
FROM social.hides h
JOIN social.forum_entries e ON e.id = h.forum_entry_id
WHERE h.restored_at IS NULL AND e.deleted_at IS NULL AND e.actor_kind = 'BUYER' AND e.actor_id = sqlc.arg(buyer_id)
  AND (h.hidden_at, h.id) < (sqlc.arg(after_hidden)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY hidden_at DESC, hide_id DESC
LIMIT sqlc.arg(max_rows);

