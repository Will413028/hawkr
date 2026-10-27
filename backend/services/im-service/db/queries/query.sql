-- name: UpsertConversation :one
INSERT INTO conversations (
    id,
    room_id,
    buyer_id,
    store_id,
    last_message_id,
    last_message_content,
    last_message_type,
    last_message_preview,
    updated_at
) VALUES (
    @id, @room_id, @buyer_id, @store_id,
    @last_message_id, @last_message_content, @last_message_type, @last_message_preview, @updated_at
)
ON CONFLICT (room_id) DO UPDATE SET
    last_message_id = EXCLUDED.last_message_id,
    last_message_content = EXCLUDED.last_message_content,
    last_message_type = EXCLUDED.last_message_type,
    last_message_preview = EXCLUDED.last_message_preview,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: GetConversationByRoomID :one
SELECT * FROM conversations
WHERE room_id = $1 LIMIT 1;

-- name: ListConversationsByBuyer :many
SELECT * FROM conversations
WHERE buyer_id = $1
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: ListConversationsBySeller :many
SELECT * FROM conversations
WHERE store_id = $1
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: IncrementBuyerUnread :exec
UPDATE conversations SET buyer_unread_count = buyer_unread_count + 1
WHERE room_id = $1;

-- name: IncrementSellerUnread :exec
UPDATE conversations SET seller_unread_count = seller_unread_count + 1
WHERE room_id = $1;

-- name: ResetBuyerUnread :exec
UPDATE conversations SET buyer_unread_count = 0
WHERE room_id = $1;

-- name: ResetSellerUnread :exec
UPDATE conversations SET seller_unread_count = 0
WHERE room_id = $1;

-- name: ResetAllBuyerUnread :exec
UPDATE conversations SET buyer_unread_count = 0
WHERE buyer_id = $1;

-- name: ResetAllSellerUnread :exec
UPDATE conversations SET seller_unread_count = 0
WHERE store_id = $1;

-- name: AcquireRoomLock :exec
-- 取得 room-scoped advisory lock，確保同 room 的 seq_id 產生是 atomic 的。
-- 必須在 transaction 內呼叫，transaction 結束時自動釋放。
SELECT pg_advisory_xact_lock(hashtext($1));

-- name: CreateMessage :one
INSERT INTO messages (
    id,
    room_id,
    sender_id,
    receiver_id,
    type,
    content,
    preview_text,
    created_at,
    is_read,
    seq_id
) VALUES (
    @id, @room_id, @sender_id, @receiver_id, @type, @content, @preview_text, @created_at, @is_read,
    (SELECT COALESCE(MAX(seq_id), 0) + 1 FROM messages m WHERE m.room_id = sqlc.arg(seq_room))
) RETURNING *;

-- name: ListMessages :many
SELECT * FROM messages
WHERE room_id = $1
ORDER BY id DESC
LIMIT $2 OFFSET $3;

-- name: ListMessagesBefore :many
-- Cursor-based pagination using message ID
SELECT * FROM messages
WHERE room_id = $1 AND id < $2
ORDER BY id DESC
LIMIT $3;

-- name: MarkMessagesAsRead :exec
UPDATE messages
SET is_read = TRUE
WHERE room_id = $1 AND receiver_id = $2 AND is_read = FALSE;

-- name: GetMessageByID :one
SELECT * FROM messages
WHERE id = $1 LIMIT 1;

-- name: SumBuyerUnreadCount :one
SELECT COALESCE(SUM(buyer_unread_count), 0)::bigint FROM conversations WHERE buyer_id = $1;

-- name: SumSellerUnreadCount :one
SELECT COALESCE(SUM(seller_unread_count), 0)::bigint FROM conversations WHERE store_id = $1;

-- name: MarkAllAsReadByBuyer :exec
UPDATE messages SET is_read = TRUE
WHERE receiver_id = $1 AND is_read = FALSE
AND room_id IN (SELECT room_id FROM conversations WHERE buyer_id = $1);

-- name: MarkAllAsReadBySeller :exec
UPDATE messages SET is_read = TRUE
WHERE receiver_id = $1 AND is_read = FALSE
AND room_id IN (SELECT room_id FROM conversations WHERE store_id = $1);

-- name: MarkMessagesByIDsAsRead :exec
UPDATE messages
SET is_read = TRUE
WHERE id = ANY($1::bigint[]) AND receiver_id = $2 AND is_read = FALSE;

-- name: RecalcUnreadForMessages :exec
-- 根據 message IDs 找到受影響的 conversations，重算 unread count。
UPDATE conversations c SET
    buyer_unread_count = (SELECT COUNT(*) FROM messages m WHERE m.room_id = c.room_id AND m.receiver_id = c.buyer_id AND m.is_read = FALSE),
    seller_unread_count = (SELECT COUNT(*) FROM messages m WHERE m.room_id = c.room_id AND m.receiver_id = c.store_id AND m.is_read = FALSE)
WHERE c.room_id IN (SELECT DISTINCT room_id FROM messages WHERE id = ANY($1::bigint[]));

-- name: ListMessagesAfterSeq :many
-- Fetch messages in a room with seq_id greater than the given value (for gap-fill sync).
SELECT * FROM messages
WHERE room_id = $1 AND seq_id > $2
ORDER BY seq_id ASC
LIMIT $3;

-- name: HideMessage :exec
UPDATE messages
SET hidden_at = NOW(), hidden_reason = $2
WHERE id = $1 AND deleted_at IS NULL AND hidden_at IS NULL;

-- name: RedactMessagesBySender :execrows
-- Account deletion (Apple 5.1.1(v) + 個資法 right-to-erasure): purge the deleted
-- account's message bodies in place. deleted_at flips the read-path tombstone
-- (TombstoneContent); content is blanked so the message text/URL no longer
-- survives at rest — chat has no at-rest encryption, so purge IS the compliance
-- lever. NOTE: for image messages this blanks only the CDN URL; the OSS blob +
-- media-service reference are NOT erased here (see media-orphan deletion gap).
-- sender_id is already indexed (idx_messages_sender_id). Idempotent. D8: senders
-- is every historical buyer_id/store_id for the deleted account, not one id.
UPDATE messages
SET deleted_at = NOW(), content = '', preview_text = NULL
WHERE sender_id = ANY(@sender_ids::bigint[]) AND deleted_at IS NULL;

-- name: RedactConversationLastMessageBySender :exec
-- Blank the denormalized preview on conversations whose last message was any of
-- the deleted account's historical identities (D8). GetConversations returns
-- last_message_content raw (not via TombstoneContent), so the preview must be
-- overwritten here too. @placeholder is the neutral tombstone text (a constant,
-- not PII), so this both purges and shows a tombstone. Idempotent.
UPDATE conversations
SET last_message_content = @placeholder, last_message_preview = @placeholder
WHERE last_message_id IN (
    SELECT id FROM messages WHERE sender_id = ANY(@sender_ids::bigint[]) AND deleted_at IS NOT NULL
);

-- name: ListImageMessageIDsBySender :many
-- Image message IDs (type=2) any of senders sent, used to release their
-- media-service asset_reference rows on account deletion so the OSS blob can be
-- GC'd. Returns all such IDs regardless of deleted_at (idempotent across
-- consumer retries). D8: senders is every historical buyer_id/store_id.
SELECT id FROM messages WHERE sender_id = ANY(@sender_ids::bigint[]) AND type = 2;

-- name: BindConversationOrder :exec
-- 賣家停權訂單關聯綁定（14 號 §6.5.1，僅伺服器端兩條路徑寫入，client 無 API 可直寫）。
-- order_binding_verified_at 使用呼叫端傳入的 now（與其餘寫入時間戳一致，非 DB NOW()）。
UPDATE conversations
SET order_no = @order_no, order_binding_verified_at = @verified_at
WHERE room_id = @room_id;
