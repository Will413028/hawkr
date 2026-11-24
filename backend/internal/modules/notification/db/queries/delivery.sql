-- name: DeliveryByID :one
-- 渠道 job 讀自己的投遞紀錄（不加鎖，用來知道收件人是誰；鎖收件人之後再以 DeliveryForUpdate 重讀狀態）。
SELECT id, intent_id, account_id, audience, store_id, channel, status
FROM notification.deliveries WHERE id = sqlc.arg(id);

-- name: DeliveryForUpdate :one
-- 鎖收件人之後重讀並鎖住投遞紀錄（notification 的列在鎖順序最後）。
SELECT id, intent_id, account_id, audience, store_id, channel, status
FROM notification.deliveries WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: MarkDeliverySent :execrows
-- 記下實際送出的文字與語系（送出之後收件人才被刪除時三者為 NULL，不留文字）；只改 PENDING 的列，已是終態的不動（job 重複執行時回 0 列）。
UPDATE notification.deliveries
SET status = 'SENT', locale = sqlc.narg(locale), sent_title = sqlc.narg(title), sent_body = sqlc.narg(body), sent_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND status = 'PENDING';

-- name: MarkDeliveryFailed :execrows
-- 永久失敗：不寫任何送出文字；只改 PENDING 的列。
UPDATE notification.deliveries
SET status = 'FAILED_PERMANENT', failure_reason = sqlc.arg(reason)::text, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND status = 'PENDING';

-- name: DeviceTokensForRecipient :many
-- 推播目標：帳號在指定 App 的 token。BUYER 受眾只查 BUYER App、STORE 受眾只查 SELLER App（呼叫端依受眾決定 app），不跨 App。
SELECT token FROM notification.device_tokens
WHERE account_id = sqlc.arg(account_id) AND app = sqlc.arg(app)
ORDER BY token;

-- name: DeleteDeviceTokensByValue :execrows
-- FCM 回 UNREGISTERED 或 SENDER_ID_MISMATCH 的 token：失效，直接刪（token 是主鍵，換帳號登入時由登入改綁，不留停用列）。
DELETE FROM notification.device_tokens WHERE token = ANY(sqlc.arg(tokens)::text[]);

-- name: DeleteDeviceTokensByAccount :execrows
-- 帳號停權或刪除時，同一個 transaction 刪掉它的全部 token（兩支 App 都刪）。
DELETE FROM notification.device_tokens WHERE account_id = sqlc.arg(account_id);

-- name: DeleteStaleDeviceTokens :execrows
-- 每日清除：last_registered_at 早於 cutoff（App 超過 60 天沒有啟動註冊）的 token，依 last_registered_at 分批。
DELETE FROM notification.device_tokens
WHERE token IN (
    SELECT t.token FROM notification.device_tokens t
    WHERE t.last_registered_at < sqlc.arg(cutoff)
    ORDER BY t.last_registered_at, t.token
    LIMIT sqlc.arg(max_rows)
);

-- name: EmailSuppressed :one
SELECT EXISTS (SELECT 1 FROM notification.email_suppressions WHERE address = sqlc.arg(address)) AS suppressed;

-- name: InsertEmailSuppression :exec
-- 已在名單內的位址不動（保留最早的原因與時間），所以 webhook 重複處理無害。
INSERT INTO notification.email_suppressions (address, reason, created_at)
VALUES (sqlc.arg(address), sqlc.arg(reason), sqlc.arg(now))
ON CONFLICT (address) DO NOTHING;

-- name: InboxItemForDelivery :one
-- 推播 data 帶的 inbox_item_id：這筆投遞的收件人在這個意圖下的收件匣列（意圖連結，或公告自己的列）；沒有收件匣列（只推播的型別）回 0 列。
SELECT x.id FROM notification.inbox_items x
WHERE x.account_id = sqlc.arg(account_id) AND x.audience = sqlc.arg(audience) AND x.store_id IS NOT DISTINCT FROM sqlc.narg(store_id)
  AND (x.announcement_id = sqlc.arg(intent_id)::uuid
       OR EXISTS (SELECT 1 FROM notification.inbox_item_intents l WHERE l.inbox_item_id = x.id AND l.intent_id = sqlc.arg(intent_id)::uuid))
LIMIT 1;
