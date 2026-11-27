
-- name: InsertMessage :one
INSERT INTO im.messages (id, conversation_id, seq, sender_kind, request_id, message_type, content, image_asset_id, image_object_key, created_at)
VALUES (sqlc.arg(id), sqlc.arg(conversation_id), sqlc.arg(seq), sqlc.arg(sender_kind), sqlc.arg(request_id), sqlc.arg(message_type), sqlc.narg(content), sqlc.narg(image_asset_id), sqlc.narg(image_object_key), sqlc.arg(now))
RETURNING *;

-- name: LockSendRequest :exec
SELECT pg_advisory_xact_lock(hashtextextended('im.send:' || sqlc.arg(buyer_id)::bigint::text || ':' || sqlc.arg(store_id)::bigint::text || ':' || sqlc.arg(sender_kind)::text || ':' || sqlc.arg(request_id)::uuid::text, 0));

-- name: MessageByRequest :one
SELECT m.id, m.conversation_id, m.seq, m.sender_kind, m.message_type, m.content, m.image_asset_id, m.image_object_key, m.created_at, m.erased_at,
       (h.id IS NOT NULL OR EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'CHAT_MESSAGE' AND td.content_id = m.id))::boolean AS hidden
FROM im.messages AS m
JOIN im.conversations AS c ON c.id = m.conversation_id
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE c.buyer_id = sqlc.arg(buyer_id) AND c.store_id = sqlc.arg(store_id)
  AND m.sender_kind = sqlc.arg(sender_kind) AND m.request_id = sqlc.arg(request_id);

-- name: MessageParties :one
SELECT c.id AS conversation_id, c.buyer_id, c.store_id
FROM im.messages AS m JOIN im.conversations AS c ON c.id = m.conversation_id
WHERE m.id = sqlc.arg(message_id);

-- name: ListMessagesBefore :many
SELECT m.id, m.conversation_id, m.seq, m.sender_kind, m.message_type, m.content, m.image_asset_id, m.image_object_key, m.created_at, m.erased_at,
       (h.id IS NOT NULL OR EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'CHAT_MESSAGE' AND td.content_id = m.id))::boolean AS hidden
FROM im.messages AS m
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE m.conversation_id = sqlc.arg(conversation_id)
  AND (sqlc.arg(before_seq)::bigint = 0 OR m.seq < sqlc.arg(before_seq)::bigint)
ORDER BY m.seq DESC
LIMIT sqlc.arg(max_rows);

-- name: ListMessagesAfter :many
SELECT m.id, m.conversation_id, m.seq, m.sender_kind, m.message_type, m.content, m.image_asset_id, m.image_object_key, m.created_at, m.erased_at,
       (h.id IS NOT NULL OR EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'CHAT_MESSAGE' AND td.content_id = m.id))::boolean AS hidden
FROM im.messages AS m
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE m.conversation_id = sqlc.arg(conversation_id) AND m.seq > sqlc.arg(after_seq)::bigint
ORDER BY m.seq
LIMIT sqlc.arg(max_rows);

-- name: GetMessageForRelay :one
SELECT m.id, m.conversation_id, m.seq, m.sender_kind, m.message_type, m.content, m.image_asset_id, m.image_object_key, m.created_at, m.erased_at,
       c.buyer_id, c.store_id,
       (h.id IS NOT NULL OR EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'CHAT_MESSAGE' AND td.content_id = m.id))::boolean AS hidden,
       CASE WHEN m.sender_kind = 'BUYER'
            THEN EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id)
            ELSE EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
       END::boolean AS receiver_inactive
FROM im.messages AS m
JOIN im.conversations AS c ON c.id = m.conversation_id
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE m.id = sqlc.arg(id);

-- name: LockBuyerMessageBatch :many
SELECT m.id, m.image_asset_id FROM im.messages AS m
JOIN im.conversations AS c ON c.id = m.conversation_id
WHERE c.buyer_id = sqlc.arg(buyer_id) AND m.sender_kind = 'BUYER' AND m.erased_at IS NULL
ORDER BY m.id
LIMIT sqlc.arg(max_rows)
FOR NO KEY UPDATE OF m;

-- name: LockStoreMessageBatch :many
SELECT m.id, m.image_asset_id FROM im.messages AS m
JOIN im.conversations AS c ON c.id = m.conversation_id
WHERE c.store_id = sqlc.arg(store_id) AND m.sender_kind = 'STORE' AND m.erased_at IS NULL
ORDER BY m.id
LIMIT sqlc.arg(max_rows)
FOR NO KEY UPDATE OF m;

-- name: EraseMessages :execrows
UPDATE im.messages
SET content = NULL, image_asset_id = NULL, image_object_key = NULL, erased_at = sqlc.arg(now)
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND erased_at IS NULL;

-- name: ListInactivePartiesWithMessages :many
SELECT kind::text AS kind, party_id::bigint AS party_id FROM (
    SELECT 'BUYER' AS kind, cb.buyer_id AS party_id FROM buyer.pub_closed_buyers() AS cb
    WHERE EXISTS (
        SELECT 1 FROM im.conversations AS c JOIN im.messages AS m ON m.conversation_id = c.id
        WHERE c.buyer_id = cb.buyer_id AND m.sender_kind = 'BUYER' AND m.erased_at IS NULL)
    UNION ALL
    SELECT 'STORE' AS kind, rs.store_id AS party_id FROM seller.pub_retired_stores() AS rs
    WHERE EXISTS (
        SELECT 1 FROM im.conversations AS c JOIN im.messages AS m ON m.conversation_id = c.id
        WHERE c.store_id = rs.store_id AND m.sender_kind = 'STORE' AND m.erased_at IS NULL)
) AS parties
WHERE (kind, party_id) > (sqlc.arg(after_kind)::text, sqlc.arg(after_id)::bigint)
ORDER BY kind, party_id
LIMIT sqlc.arg(max_rows);

-- name: PartyInactive :one
SELECT CASE WHEN sqlc.arg(kind)::text = 'BUYER'
            THEN EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = sqlc.arg(party_id)::bigint)
            ELSE EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = sqlc.arg(party_id)::bigint)
       END::boolean AS inactive;

-- name: MessagesForReport :many
SELECT (m.id = sqlc.arg(message_id)::uuid)::boolean AS is_target,
       m.id, m.conversation_id, m.seq, m.sender_kind, m.message_type, m.content, m.image_asset_id, m.created_at, m.erased_at,
       c.buyer_id, c.store_id,
       (h.id IS NOT NULL OR EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'CHAT_MESSAGE' AND td.content_id = m.id))::boolean AS hidden
FROM im.messages AS t
JOIN im.messages AS m ON m.conversation_id = t.conversation_id AND m.seq BETWEEN t.seq - sqlc.arg(radius)::bigint AND t.seq + sqlc.arg(radius)::bigint
JOIN im.conversations AS c ON c.id = m.conversation_id
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE t.id = sqlc.arg(message_id)::uuid
ORDER BY m.seq;
