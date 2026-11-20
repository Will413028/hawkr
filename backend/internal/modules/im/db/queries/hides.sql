-- 訊息的隱藏紀錄（可多次隱藏與還原）。可見性只由 im 寫；呼叫者是 moderation 的 transaction，先鎖訊息列（LockMessage）。

-- name: LockMessage :one
SELECT m.id, m.conversation_id, c.buyer_id, c.store_id, m.seq FROM im.messages AS m
JOIN im.conversations AS c ON c.id = m.conversation_id
WHERE m.id = sqlc.arg(id) FOR NO KEY UPDATE OF m;

-- name: InsertHide :one
-- 同一則訊息已經有未還原的隱藏時回既有的那一列（冪等）；回傳的 id 等於傳入的 id 才是新增。
INSERT INTO im.hides (id, message_id, reason_category, reason_text, source_ref, admin_id, hidden_at)
VALUES (sqlc.arg(id), sqlc.arg(message_id), sqlc.arg(reason_category), sqlc.arg(reason_text), sqlc.narg(source_ref), sqlc.arg(admin_id), sqlc.arg(now))
ON CONFLICT (message_id) WHERE restored_at IS NULL
DO UPDATE SET message_id = EXCLUDED.message_id
RETURNING *;

-- name: RestoreHide :one
-- 還原訊息目前未還原的隱藏（留列不刪，之後可以再隱藏）；沒有未還原的隱藏沒有列。
UPDATE im.hides
SET restored_at = GREATEST(sqlc.arg(now)::timestamptz, hidden_at), restored_by = sqlc.arg(admin_id), restore_reason = sqlc.arg(reason)
WHERE message_id = sqlc.arg(message_id) AND restored_at IS NULL
RETURNING *;
