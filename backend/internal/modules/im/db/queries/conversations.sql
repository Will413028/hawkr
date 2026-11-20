-- 對話列：建立、配發 seq、已讀游標、列表與未讀。

-- name: AllocateSeq :one
-- 同一句 SQL 建立對話（還不存在時）、配發 seq，並把寄件人那一方的已讀游標推到新的 seq。並行送出在 ON CONFLICT 的列鎖上排隊，
-- 所以 seq 連續而且不重複；SET 右側讀的是更新前的列。
INSERT INTO im.conversations (id, buyer_id, store_id, last_seq, buyer_last_read_seq, store_last_read_seq, last_message_at, created_at)
VALUES (
    sqlc.arg(new_id), sqlc.arg(buyer_id), sqlc.arg(store_id), 1,
    CASE WHEN sqlc.arg(sender_kind)::text = 'BUYER' THEN 1 ELSE 0 END,
    CASE WHEN sqlc.arg(sender_kind)::text = 'STORE' THEN 1 ELSE 0 END,
    sqlc.arg(now), sqlc.arg(now)
)
ON CONFLICT (buyer_id, store_id) DO UPDATE
SET last_seq = im.conversations.last_seq + 1,
    buyer_last_read_seq = CASE WHEN sqlc.arg(sender_kind)::text = 'BUYER' THEN im.conversations.last_seq + 1 ELSE im.conversations.buyer_last_read_seq END,
    store_last_read_seq = CASE WHEN sqlc.arg(sender_kind)::text = 'STORE' THEN im.conversations.last_seq + 1 ELSE im.conversations.store_last_read_seq END,
    last_message_at = EXCLUDED.last_message_at
RETURNING id, last_seq, (xmax = 0) AS created;

-- name: GetConversation :one
SELECT * FROM im.conversations WHERE id = sqlc.arg(id);

-- name: MarkReadBuyer :one
-- 只前進不後退，也不超過 last_seq；沒有這個買家的對話沒有列。
UPDATE im.conversations
SET buyer_last_read_seq = GREATEST(buyer_last_read_seq, LEAST(sqlc.arg(upto_seq)::bigint, last_seq))
WHERE id = sqlc.arg(id) AND buyer_id = sqlc.arg(buyer_id)
RETURNING buyer_last_read_seq AS read_seq, last_seq;

-- name: MarkReadStore :one
UPDATE im.conversations
SET store_last_read_seq = GREATEST(store_last_read_seq, LEAST(sqlc.arg(upto_seq)::bigint, last_seq))
WHERE id = sqlc.arg(id) AND store_id = sqlc.arg(store_id)
RETURNING store_last_read_seq AS read_seq, last_seq;

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
-- 預覽在讀取時由最後一則訊息推導（隱藏與抹除不必另外更新對話列）：以 (id, last_seq) 取最後一則，LEFT JOIN 未還原的隱藏。
SELECT c.id, c.buyer_id, c.store_id, c.last_seq, c.buyer_last_read_seq AS read_seq, c.last_message_at,
       m.id AS message_id, m.sender_kind, m.message_type, m.content, m.erased_at,
       (h.id IS NOT NULL)::boolean AS hidden
FROM im.conversations AS c
JOIN im.messages AS m ON m.conversation_id = c.id AND m.seq = c.last_seq
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE c.buyer_id = sqlc.arg(buyer_id)
  AND (sqlc.narg(after_at)::timestamptz IS NULL OR (c.last_message_at, c.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY c.last_message_at DESC, c.id DESC
LIMIT sqlc.arg(max_rows);

-- name: ListConversationsStore :many
SELECT c.id, c.buyer_id, c.store_id, c.last_seq, c.store_last_read_seq AS read_seq, c.last_message_at,
       m.id AS message_id, m.sender_kind, m.message_type, m.content, m.erased_at,
       (h.id IS NOT NULL)::boolean AS hidden
FROM im.conversations AS c
JOIN im.messages AS m ON m.conversation_id = c.id AND m.seq = c.last_seq
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE c.store_id = sqlc.arg(store_id)
  AND (sqlc.narg(after_at)::timestamptz IS NULL OR (c.last_message_at, c.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY c.last_message_at DESC, c.id DESC
LIMIT sqlc.arg(max_rows);
