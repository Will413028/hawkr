-- 訊息列：寫入、讀取、抹除。

-- name: InsertMessage :one
-- 撞 (對話, 寄件方, request_id) 的唯一鍵時不寫、不回列（pgx.ErrNoRows）：呼叫端把整個 transaction 回滾（連同 AllocateSeq 推進的 seq）再重來，重讀第一則。
INSERT INTO im.messages (id, conversation_id, seq, sender_kind, request_id, message_type, content, image_asset_id, image_object_key, created_at)
VALUES (sqlc.arg(id), sqlc.arg(conversation_id), sqlc.arg(seq), sqlc.arg(sender_kind), sqlc.arg(request_id), sqlc.arg(message_type), sqlc.narg(content), sqlc.narg(image_asset_id), sqlc.narg(image_object_key), sqlc.arg(now))
ON CONFLICT ON CONSTRAINT messages_request_key DO NOTHING
RETURNING *;

-- name: MessageByRequest :one
-- 送出冪等的重送判定：這個對話、這一方、這個 request_id 第一次寫入的訊息；對話還不存在或沒有這個 request_id 時沒有列。隱藏的以 hidden 標出（同列表查詢的述詞），內容由呼叫端換成墓碑；不鎖列。
SELECT m.id, m.conversation_id, m.seq, m.sender_kind, m.message_type, m.content, m.image_asset_id, m.image_object_key, m.created_at, m.erased_at,
       (h.id IS NOT NULL OR EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'CHAT_MESSAGE' AND td.content_id = m.id))::boolean AS hidden
FROM im.messages AS m
JOIN im.conversations AS c ON c.id = m.conversation_id
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE c.buyer_id = sqlc.arg(buyer_id) AND c.store_id = sqlc.arg(store_id)
  AND m.sender_kind = sqlc.arg(sender_kind) AND m.request_id = sqlc.arg(request_id);

-- name: MessageParties :one
-- 訊息所屬對話的雙方（歸屬）：不套可見性（隱藏、抹除的訊息照樣回），不鎖列。
SELECT c.id AS conversation_id, c.buyer_id, c.store_id
FROM im.messages AS m JOIN im.conversations AS c ON c.id = m.conversation_id
WHERE m.id = sqlc.arg(message_id);

-- name: ListMessagesBefore :many
-- 以 seq 做 keyset（新的在前）；before_seq 為 0 表示從最新開始。隱藏的以 hidden 標出，內容由呼叫端換成墓碑。
SELECT m.id, m.conversation_id, m.seq, m.sender_kind, m.message_type, m.content, m.image_asset_id, m.image_object_key, m.created_at, m.erased_at,
       (h.id IS NOT NULL OR EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'CHAT_MESSAGE' AND td.content_id = m.id))::boolean AS hidden
FROM im.messages AS m
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE m.conversation_id = sqlc.arg(conversation_id)
  AND (sqlc.arg(before_seq)::bigint = 0 OR m.seq < sqlc.arg(before_seq)::bigint)
ORDER BY m.seq DESC
LIMIT sqlc.arg(max_rows);

-- name: ListMessagesAfter :many
-- 斷線補洞：seq 大於 after_seq 的訊息，舊的在前。
SELECT m.id, m.conversation_id, m.seq, m.sender_kind, m.message_type, m.content, m.image_asset_id, m.image_object_key, m.created_at, m.erased_at,
       (h.id IS NOT NULL OR EXISTS (SELECT 1 FROM seller.pub_content_takedowns() AS td WHERE td.content_kind = 'CHAT_MESSAGE' AND td.content_id = m.id))::boolean AS hidden
FROM im.messages AS m
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE m.conversation_id = sqlc.arg(conversation_id) AND m.seq > sqlc.arg(after_seq)::bigint
ORDER BY m.seq
LIMIT sqlc.arg(max_rows);

-- name: GetMessageForRelay :one
-- relay 讀單則訊息與它的對話：不套可見性，Hidden、erased_at 與收件人是否已刪除讓呼叫端決定要不要送。
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
-- 抹除的一批：依 id 升序鎖住這個買家身分寄出、還沒抹除的訊息（重跑時只剩沒抹的）。
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
-- 抹除的保底掃描：已關閉的買家（buyer.pub_closed_buyers()）與已退場的店家（seller.pub_retired_stores()）中，仍有未抹除訊息的身分，依 (kind, party_id) 的 keyset 分頁，最多 max_rows 個；第一頁的游標是 ('', 0)。
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
-- 一方是否已關閉（買家）或已退場（店家）：不取鎖的讀取（G4 的提早檢查與在線狀態）。Send 的正確性由 transaction 內的 LockNotClosed、LockNotRetired 保證。
SELECT CASE WHEN sqlc.arg(kind)::text = 'BUYER'
            THEN EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = sqlc.arg(party_id)::bigint)
            ELSE EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = sqlc.arg(party_id)::bigint)
       END::boolean AS inactive;

-- name: MessagesForReport :many
-- 檢舉用的證據讀取：被檢舉的訊息與同一對話 seq 前後各 window 則，一句 SQL 保證同一時點；不鎖列。is_target 標出被檢舉的那則，授權（呼叫者是對話的一方）與
-- 「被檢舉的訊息已隱藏或已抹除」由呼叫端依回來的列判斷。隱藏的以 hidden 標出，內容由呼叫端換成墓碑。
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
