
-- name: LockMessagesForTakedown :many
SELECT m.id, m.conversation_id, c.buyer_id, c.store_id, m.seq, m.sender_kind FROM im.messages AS m
JOIN im.conversations AS c ON c.id = m.conversation_id
WHERE m.id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY m.id
FOR NO KEY UPDATE OF m;
