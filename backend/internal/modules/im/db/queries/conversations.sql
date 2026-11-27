
-- name: AllocateSeq :one
INSERT INTO im.conversations (id, buyer_id, store_id, last_seq, buyer_last_read_seq, store_last_read_seq, last_message_at, created_at)
VALUES (
    sqlc.arg(new_id), sqlc.arg(buyer_id), sqlc.arg(store_id), 1,
    CASE WHEN sqlc.arg(sender_kind)::text = 'BUYER' THEN 1 ELSE 0 END,
    CASE WHEN sqlc.arg(sender_kind)::text = 'STORE' THEN 1 ELSE 0 END,
    clock_timestamp(), now()
)
ON CONFLICT (buyer_id, store_id) DO UPDATE
SET last_seq = im.conversations.last_seq + 1,
    buyer_last_read_seq = CASE WHEN sqlc.arg(sender_kind)::text = 'BUYER' THEN im.conversations.last_seq + 1 ELSE im.conversations.buyer_last_read_seq END,
    store_last_read_seq = CASE WHEN sqlc.arg(sender_kind)::text = 'STORE' THEN im.conversations.last_seq + 1 ELSE im.conversations.store_last_read_seq END,
    last_message_at = GREATEST(im.conversations.last_message_at, clock_timestamp())
RETURNING id, last_seq, (xmax = 0) AS created, last_message_at;

-- name: GetConversation :one
SELECT * FROM im.conversations WHERE id = sqlc.arg(id);

-- name: ConversationParties :one
SELECT id AS conversation_id, buyer_id, store_id FROM im.conversations WHERE id = sqlc.arg(id);

-- name: MarkReadBuyer :one
WITH prev AS (
    SELECT o.id, o.buyer_last_read_seq AS read_seq FROM im.conversations AS o
    WHERE o.id = sqlc.arg(id) AND o.buyer_id = sqlc.arg(buyer_id)
    FOR UPDATE
)
UPDATE im.conversations AS c
SET buyer_last_read_seq = GREATEST(c.buyer_last_read_seq, LEAST(sqlc.arg(upto_seq)::bigint, c.last_seq))
FROM prev
WHERE c.id = prev.id
RETURNING c.buyer_last_read_seq AS read_seq, c.last_seq, c.buyer_id, c.store_id,
          (c.buyer_last_read_seq > prev.read_seq)::boolean AS advanced;

-- name: MarkReadStore :one
WITH prev AS (
    SELECT o.id, o.store_last_read_seq AS read_seq FROM im.conversations AS o
    WHERE o.id = sqlc.arg(id) AND o.store_id = sqlc.arg(store_id)
    FOR UPDATE
)
UPDATE im.conversations AS c
SET store_last_read_seq = GREATEST(c.store_last_read_seq, LEAST(sqlc.arg(upto_seq)::bigint, c.last_seq))
FROM prev
WHERE c.id = prev.id
RETURNING c.store_last_read_seq AS read_seq, c.last_seq, c.buyer_id, c.store_id,
          (c.store_last_read_seq > prev.read_seq)::boolean AS advanced;

-- name: MarkAllReadBuyer :execrows
UPDATE im.conversations SET buyer_last_read_seq = last_seq
WHERE buyer_id = sqlc.arg(buyer_id) AND buyer_last_read_seq < last_seq;

-- name: MarkAllReadStore :execrows
UPDATE im.conversations SET store_last_read_seq = last_seq
WHERE store_id = sqlc.arg(store_id) AND store_last_read_seq < last_seq;

-- name: UnreadTotalBuyer :one
SELECT COALESCE(SUM(last_seq - buyer_last_read_seq), 0)::bigint FROM im.conversations WHERE buyer_id = sqlc.arg(buyer_id);

-- name: UnreadTotalStore :one
SELECT COALESCE(SUM(last_seq - store_last_read_seq), 0)::bigint FROM im.conversations WHERE store_id = sqlc.arg(store_id);

-- name: ListConversationsBuyer :many
SELECT c.id, c.buyer_id, c.store_id, c.last_seq, c.buyer_last_read_seq AS read_seq, c.store_last_read_seq AS peer_read_seq, c.last_message_at,
       m.id AS message_id, m.sender_kind, m.message_type, m.content, m.erased_at,
       (h.id IS NOT NULL OR EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'CHAT_MESSAGE' AND td.content_id = m.id))::boolean AS hidden
FROM im.conversations AS c
JOIN im.messages AS m ON m.conversation_id = c.id AND m.seq = c.last_seq
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE c.buyer_id = sqlc.arg(buyer_id)
  AND (sqlc.narg(after_at)::timestamptz IS NULL OR (c.last_message_at, c.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY c.last_message_at DESC, c.id DESC
LIMIT sqlc.arg(max_rows);

-- name: ListConversationsStore :many
SELECT c.id, c.buyer_id, c.store_id, c.last_seq, c.store_last_read_seq AS read_seq, c.buyer_last_read_seq AS peer_read_seq, c.last_message_at,
       m.id AS message_id, m.sender_kind, m.message_type, m.content, m.erased_at,
       (h.id IS NOT NULL OR EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'CHAT_MESSAGE' AND td.content_id = m.id))::boolean AS hidden
FROM im.conversations AS c
JOIN im.messages AS m ON m.conversation_id = c.id AND m.seq = c.last_seq
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE c.store_id = sqlc.arg(store_id)
  AND (sqlc.narg(after_at)::timestamptz IS NULL OR (c.last_message_at, c.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY c.last_message_at DESC, c.id DESC
LIMIT sqlc.arg(max_rows);

-- name: GetConversationStoreBuyer :one
SELECT c.id, c.buyer_id, c.store_id, c.last_seq, c.store_last_read_seq AS read_seq, c.buyer_last_read_seq AS peer_read_seq, c.last_message_at,
       m.id AS message_id, m.sender_kind, m.message_type, m.content, m.erased_at,
       (h.id IS NOT NULL OR EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'CHAT_MESSAGE' AND td.content_id = m.id))::boolean AS hidden
FROM im.conversations AS c
JOIN im.messages AS m ON m.conversation_id = c.id AND m.seq = c.last_seq
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE c.store_id = sqlc.arg(store_id) AND c.buyer_id = sqlc.arg(buyer_id);

-- name: ActiveConversationIDsBuyer :many
SELECT id FROM im.conversations
WHERE buyer_id = sqlc.arg(buyer_id) AND last_message_at > sqlc.arg(since)::timestamptz
LIMIT sqlc.arg(max_rows)::bigint;

-- name: ActiveConversationIDsStore :many
SELECT id FROM im.conversations
WHERE store_id = sqlc.arg(store_id) AND last_message_at > sqlc.arg(since)::timestamptz
LIMIT sqlc.arg(max_rows)::bigint;

-- name: DatabaseNow :one
SELECT clock_timestamp()::timestamptz;
