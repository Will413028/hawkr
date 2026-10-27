-- name: CreateInboxNotification :execrows
INSERT INTO notifications (
  id, user_id, user_type, event_type, reference_id, title, body, data
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (user_id, user_type, event_type, reference_id) WHERE reference_id IS NOT NULL
DO NOTHING;

-- name: ListByUser :many
-- Notifications are addressed by the current domain surrogate in user_id:
-- buyer rows use buyer_id and seller rows use store_id. The gateway resolves
-- the login principal to that current surrogate before calling this service.
-- Rows belonging to retired buyer/seller stints are intentionally not rewritten;
-- their existing lifecycle cleanup soft-deletes them. Keeping this predicate
-- simple also preserves idx_notifications_inbox ordered pagination.
SELECT * FROM notifications
WHERE notifications.user_type = @user_type
  AND notifications.user_id = @user_id
  AND notifications.deleted_at IS NULL
  AND (@cursor_id::bigint = 0 OR notifications.id < @cursor_id::bigint)
  AND (@category::text = '' OR notifications.data->>'category' = @category::text)
ORDER BY notifications.id DESC
LIMIT @limit_count;

-- name: CountByUser :one
SELECT count(*) FROM notifications
WHERE notifications.user_type = @user_type
  AND notifications.user_id = @user_id
  AND notifications.deleted_at IS NULL;

-- name: CountUnread :one
SELECT count(*) FROM notifications
WHERE notifications.user_type = @user_type
  AND notifications.user_id = @user_id
  AND notifications.deleted_at IS NULL AND notifications.is_read = false;

-- name: MarkAsRead :execrows
UPDATE notifications
SET is_read = true, read_at = NOW()
WHERE notifications.id = @id
  AND notifications.user_type = @user_type
  AND notifications.user_id = @user_id
  AND notifications.is_read = false;

-- name: MarkAllAsRead :execrows
UPDATE notifications
SET is_read = true, read_at = NOW()
WHERE notifications.user_type = @user_type
  AND notifications.user_id = @user_id
  AND notifications.deleted_at IS NULL AND notifications.is_read = false;

-- name: SoftDeleteByIDs :execrows
UPDATE notifications
SET deleted_at = NOW()
WHERE notifications.id = ANY(@ids::bigint[])
  AND notifications.user_type = @user_type
  AND notifications.user_id = @user_id
  AND notifications.deleted_at IS NULL;

-- name: RedactChatNotificationsBySender :execrows
-- Account deletion (個資法 right-to-erasure): a deleted account's display name +
-- message preview are denormalized into every chat-notification inbox row they
-- triggered (title=senderName, body=contentPreview, and possibly data keys). Purge
-- those PII columns AND soft-delete the row (a notification from a gone account is
-- noise). senderId is stored as a string in data->>'senderId'. Idempotent.
-- Batch match: a deleted account can have diverged across its historical
-- buyer_id/store_id surrogates, so callers pass the full historical id set for
-- that account, not a single sender id.
UPDATE notifications
SET title = '',
    body = '',
    data = (data - 'senderName' - 'contentPreview'),
    deleted_at = NOW()
WHERE event_type = 'chat.message.sent'
  AND data->>'senderId' = ANY(@sender_ids::text[])
  AND deleted_at IS NULL;

-- name: SoftDeleteByHistoricalBuyerIDs :execrows
-- 退休 buyer stint 的收件匣歸宿（44 號設計書 §10 D6 / §7.6 第 6 步）：買家角色關閉後，
-- 那些列在新身分下永遠讀不到（gateway 以現任 buyer_id 查收件匣），留著等於零價值 +
-- 純個資責任。soft delete 讓既有的 cleanup job（worker/cleanup.go，deleted_at 滿 90 天
-- 實刪）接手，不新增任何排程或表。
-- user_type 寫死 'buyer' 不是可省的細節：buyer_id 與 store_id 是兩個獨立的身分空間，
-- 而首任帳號 grandfather 恆等（buyer_id == store_id == account_id）使兩者的數值大量重疊，
-- 少了這個條件就會把同一帳號**still-active 賣家 stint** 的收件匣一起清掉。
UPDATE notifications
SET deleted_at = NOW()
WHERE user_type = 'buyer'
  AND user_id = ANY(@buyer_ids::bigint[])
  AND deleted_at IS NULL;

-- name: SoftDeleteByHistoricalStoreIDs :execrows
-- 退休 seller incarnation 的收件匣歸宿——鏡射 SoftDeleteByHistoricalBuyerIDs（issue
-- #2707）：賣家關店再重開後，gateway 已改用現任 store_id 查收件匣（見 gateway
-- resolveNotificationUserID 修復），舊 incarnation 寫入的那些列在新 store_id 下永遠
-- 讀不到，留著等於零價值 + 純個資責任。soft delete 讓既有的 cleanup job
-- （worker/cleanup.go，deleted_at 滿 90 天實刪）接手，不新增任何排程或表。
-- user_type 寫死 'seller' 不是可省的細節：buyer_id 與 store_id 是兩個獨立的身分空間，
-- 而首任帳號 grandfather 恆等（buyer_id == store_id == account_id）使兩者的數值大量重疊，
-- 少了這個條件就會把同一帳號**still-active 買家 stint** 的收件匣一起清掉。
UPDATE notifications
SET deleted_at = NOW()
WHERE user_type = 'seller'
  AND user_id = ANY(@store_ids::bigint[])
  AND deleted_at IS NULL;
