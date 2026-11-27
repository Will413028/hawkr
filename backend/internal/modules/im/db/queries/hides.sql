
-- name: LockMessage :one
SELECT m.id, m.conversation_id, c.buyer_id, c.store_id, m.seq, m.sender_kind FROM im.messages AS m
JOIN im.conversations AS c ON c.id = m.conversation_id
WHERE m.id = sqlc.arg(id) FOR NO KEY UPDATE OF m;

-- name: InsertHide :one
INSERT INTO im.hides (id, message_id, reason_category, reason_text, source_ref, admin_id, hidden_at)
VALUES (sqlc.arg(id), sqlc.arg(message_id), sqlc.arg(reason_category), sqlc.arg(reason_text), sqlc.narg(source_ref), sqlc.arg(admin_id), sqlc.arg(now))
ON CONFLICT (message_id) WHERE restored_at IS NULL
DO UPDATE SET message_id = EXCLUDED.message_id
RETURNING *;

-- name: RestoreHide :one
UPDATE im.hides
SET restored_at = GREATEST(sqlc.arg(now)::timestamptz, hidden_at), restored_by = sqlc.arg(admin_id), restore_reason = sqlc.arg(reason)
WHERE message_id = sqlc.arg(message_id) AND restored_at IS NULL AND (sqlc.narg(source_ref)::text IS NULL OR source_ref = sqlc.narg(source_ref)::text)
RETURNING *;

-- name: HasActiveHide :one
SELECT EXISTS (SELECT 1 FROM im.hides WHERE message_id = sqlc.arg(message_id) AND restored_at IS NULL);
