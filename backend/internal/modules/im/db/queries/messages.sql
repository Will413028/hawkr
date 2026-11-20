-- 訊息列：寫入、讀取、抹除。

-- name: InsertMessage :one
INSERT INTO im.messages (id, conversation_id, seq, sender_kind, message_type, content, image_asset_id, image_object_key, created_at)
VALUES (sqlc.arg(id), sqlc.arg(conversation_id), sqlc.arg(seq), sqlc.arg(sender_kind), sqlc.arg(message_type), sqlc.narg(content), sqlc.narg(image_asset_id), sqlc.narg(image_object_key), sqlc.arg(now))
RETURNING *;

-- name: ListMessagesBefore :many
-- 以 seq 做 keyset（新的在前）；before_seq 為 0 表示從最新開始。隱藏的以 hidden 標出，內容由呼叫端換成墓碑。
SELECT m.id, m.conversation_id, m.seq, m.sender_kind, m.message_type, m.content, m.image_asset_id, m.image_object_key, m.created_at, m.erased_at,
       (h.id IS NOT NULL)::boolean AS hidden
FROM im.messages AS m
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE m.conversation_id = sqlc.arg(conversation_id)
  AND (sqlc.arg(before_seq)::bigint = 0 OR m.seq < sqlc.arg(before_seq)::bigint)
ORDER BY m.seq DESC
LIMIT sqlc.arg(max_rows);

-- name: ListMessagesAfter :many
-- 斷線補洞：seq 大於 after_seq 的訊息，舊的在前。
SELECT m.id, m.conversation_id, m.seq, m.sender_kind, m.message_type, m.content, m.image_asset_id, m.image_object_key, m.created_at, m.erased_at,
       (h.id IS NOT NULL)::boolean AS hidden
FROM im.messages AS m
LEFT JOIN im.hides AS h ON h.message_id = m.id AND h.restored_at IS NULL
WHERE m.conversation_id = sqlc.arg(conversation_id) AND m.seq > sqlc.arg(after_seq)::bigint
ORDER BY m.seq
LIMIT sqlc.arg(max_rows);

-- name: GetMessageForRelay :one
-- relay 讀單則訊息與它的對話：不套可見性，Hidden、erased_at 與收件人是否已刪除讓呼叫端決定要不要送。
SELECT m.id, m.conversation_id, m.seq, m.sender_kind, m.message_type, m.content, m.image_asset_id, m.image_object_key, m.created_at, m.erased_at,
       c.buyer_id, c.store_id,
       (h.id IS NOT NULL)::boolean AS hidden,
       EXISTS (
           SELECT 1 FROM im.inactive_parties AS ip
           WHERE (m.sender_kind = 'BUYER' AND ip.kind = 'STORE' AND ip.party_id = c.store_id)
              OR (m.sender_kind = 'STORE' AND ip.kind = 'BUYER' AND ip.party_id = c.buyer_id)
       ) AS receiver_inactive
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
-- 抹除的保底掃描：已在 inactive_parties 但仍有未抹除訊息的身分，最多 max_rows 個。
SELECT ip.kind, ip.party_id FROM im.inactive_parties AS ip
WHERE (ip.kind = 'BUYER' AND EXISTS (
          SELECT 1 FROM im.conversations AS c JOIN im.messages AS m ON m.conversation_id = c.id
          WHERE c.buyer_id = ip.party_id AND m.sender_kind = 'BUYER' AND m.erased_at IS NULL))
   OR (ip.kind = 'STORE' AND EXISTS (
          SELECT 1 FROM im.conversations AS c JOIN im.messages AS m ON m.conversation_id = c.id
          WHERE c.store_id = ip.party_id AND m.sender_kind = 'STORE' AND m.erased_at IS NULL))
ORDER BY ip.kind, ip.party_id
LIMIT sqlc.arg(max_rows);
