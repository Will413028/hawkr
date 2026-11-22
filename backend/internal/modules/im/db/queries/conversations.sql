-- 對話列：建立、配發 seq、已讀游標、列表與未讀。

-- name: AllocateSeq :one
-- 同一句 SQL 建立對話（還不存在時）、配發 seq，並把寄件人那一方的已讀游標推到新的 seq。並行送出在 ON CONFLICT 的列鎖上排隊，
-- 所以 seq 連續而且不重複；SET 右側讀的是更新前的列。last_message_at 只由資料庫的 clock_timestamp() 寫入（取更新前的值與它的較大者，
-- 所以不會倒退），並回傳給呼叫者當這則訊息的時間：監聽重連的補洞以資料庫時間比較，訊息的 created_at 與事件時間用同一個值。
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
-- 對話的雙方（歸屬）：不鎖列。
SELECT id AS conversation_id, buyer_id, store_id FROM im.conversations WHERE id = sqlc.arg(id);

-- name: MarkReadBuyer :one
-- 只前進不後退，也不超過 last_seq；沒有這個買家的對話沒有列。advanced 表示游標真的前進（冪等重送為假）：先 FOR UPDATE 鎖住並讀出
-- 更新前的值（並行的 MarkRead 在這裡排隊，拿到的是對方提交後的值），再與它比較，所以同一次前進只會有一個呼叫者得到 advanced。
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
-- 預覽在讀取時由最後一則訊息推導（隱藏與抹除不必另外更新對話列）：以 (id, last_seq) 取最後一則，LEFT JOIN 未還原的隱藏。
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
-- 店家依買家取對話：欄位與預覽同 ListConversationsStore；(buyer_id, store_id) 唯一，最多一列。
SELECT c.id, c.buyer_id, c.store_id, c.last_seq, c.store_last_read_seq AS read_seq, c.buyer_last_read_seq AS peer_read_seq, c.last_message_at,
       m.id AS message_id, m.sender_kind, m.message_type, m.content, m.erased_at,
       (h.id IS NOT NULL OR EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'CHAT_MESSAGE' AND td.content_id = m.id))::boolean AS hidden
FROM im.conversations AS c
JOIN im.messages AS m ON m.conversation_id = c.id AND m.seq = c.last_seq
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE c.store_id = sqlc.arg(store_id) AND c.buyer_id = sqlc.arg(buyer_id);

-- name: ActiveConversationIDsBuyer :many
-- 監聽重連後的補洞：這個買家在 since（資料庫時間）之後有新訊息的對話，最多 max_rows 筆。
SELECT id FROM im.conversations
WHERE buyer_id = sqlc.arg(buyer_id) AND last_message_at > sqlc.arg(since)::timestamptz
LIMIT sqlc.arg(max_rows)::bigint;

-- name: ActiveConversationIDsStore :many
SELECT id FROM im.conversations
WHERE store_id = sqlc.arg(store_id) AND last_message_at > sqlc.arg(since)::timestamptz
LIMIT sqlc.arg(max_rows)::bigint;

-- name: DatabaseNow :one
-- 監聽迴圈啟動時讀一次資料庫時間當作第一個補洞起點。用 clock_timestamp()：存活確認的 SELECT now() 是測試與維運辨認監聽連線的字首。
SELECT clock_timestamp()::timestamptz;
