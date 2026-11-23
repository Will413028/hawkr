-- name: DeleteExpiredDeliveries :execrows
-- 保留期清除：created_at 早於 cutoff 的投遞紀錄，依 created_at 分批。先刪投遞，intents 才刪得掉（deliveries.intent_id 是 RESTRICT）。
DELETE FROM notification.deliveries
WHERE id IN (
    SELECT d.id FROM notification.deliveries d
    WHERE d.created_at < sqlc.arg(cutoff)
    ORDER BY d.created_at, d.id
    LIMIT sqlc.arg(max_rows)
);

-- name: DeleteExpiredIntents :execrows
-- 保留期清除：created_at 早於 cutoff 且已沒有投遞紀錄的意圖（投遞紀錄還在保留期內的意圖留著）；inbox_item_intents 的連結隨意圖 CASCADE 刪除。
DELETE FROM notification.intents
WHERE id IN (
    SELECT i.id FROM notification.intents i
    WHERE i.created_at < sqlc.arg(cutoff)
      AND NOT EXISTS (SELECT 1 FROM notification.deliveries d WHERE d.intent_id = i.id)
    ORDER BY i.created_at, i.id
    LIMIT sqlc.arg(max_rows)
);

-- name: DeleteExpiredInboxItems :execrows
-- 保留期清除：updated_at（最後一次合併）早於 cutoff 的收件匣列，依 updated_at 分批；連結隨列 CASCADE 刪除。
DELETE FROM notification.inbox_items
WHERE id IN (
    SELECT x.id FROM notification.inbox_items x
    WHERE x.updated_at < sqlc.arg(cutoff)
    ORDER BY x.updated_at, x.id
    LIMIT sqlc.arg(max_rows)
);

-- name: ScrubRecipientIntentParams :execrows
-- 個資清除：把該收件人的意圖參數換成 '{}'（意圖經 inbox_item_intents 與 deliveries 找到），分批；已是 '{}' 的不再處理，所以可重跑。
-- 收件人範圍：audience 為 NULL 是帳號的全部列；store_id 為 NULL 不限店。
UPDATE notification.intents
SET params = '{}'::jsonb
WHERE id IN (
    SELECT s.id FROM notification.intents s
    WHERE s.params <> '{}'::jsonb
      AND s.id IN (
        SELECT l.intent_id
        FROM notification.inbox_item_intents l
        JOIN notification.inbox_items x ON x.id = l.inbox_item_id
        WHERE x.account_id = sqlc.arg(account_id)
          AND (sqlc.narg(audience)::text IS NULL OR x.audience = sqlc.narg(audience)::text)
          AND (sqlc.narg(store_id)::bigint IS NULL OR x.store_id = sqlc.narg(store_id)::bigint)
        UNION
        SELECT d.intent_id
        FROM notification.deliveries d
        WHERE d.account_id = sqlc.arg(account_id)
          AND (sqlc.narg(audience)::text IS NULL OR d.audience = sqlc.narg(audience)::text)
          AND (sqlc.narg(store_id)::bigint IS NULL OR d.store_id = sqlc.narg(store_id)::bigint)
      )
    LIMIT sqlc.arg(max_rows)
);

-- name: ScrubRecipientDeliveryText :execrows
-- 個資清除：該收件人的投遞紀錄清掉送出文字與語系（列留著，D16），分批；已清過的不再處理。
UPDATE notification.deliveries
SET locale = NULL, sent_title = NULL, sent_body = NULL
WHERE id IN (
    SELECT d.id FROM notification.deliveries d
    WHERE d.account_id = sqlc.arg(account_id)
      AND (sqlc.narg(audience)::text IS NULL OR d.audience = sqlc.narg(audience)::text)
      AND (sqlc.narg(store_id)::bigint IS NULL OR d.store_id = sqlc.narg(store_id)::bigint)
      AND (d.locale IS NOT NULL OR d.sent_title IS NOT NULL OR d.sent_body IS NOT NULL)
    LIMIT sqlc.arg(max_rows)
);

-- name: DeleteRecipientInboxItems :execrows
-- 個資清除：刪該收件人的收件匣列（連結隨列 CASCADE），分批。
DELETE FROM notification.inbox_items
WHERE id IN (
    SELECT x.id FROM notification.inbox_items x
    WHERE x.account_id = sqlc.arg(account_id)
      AND (sqlc.narg(audience)::text IS NULL OR x.audience = sqlc.narg(audience)::text)
      AND (sqlc.narg(store_id)::bigint IS NULL OR x.store_id = sqlc.narg(store_id)::bigint)
    LIMIT sqlc.arg(max_rows)
);

-- name: DeleteAccountPreferenceOverrides :exec
-- 個資清除：刪帳號的偏好覆寫；偏好是 (帳號, 受眾) 的，沒有店別，audience 為 NULL 是全部受眾。每個帳號最多 14 分類乘 3 渠道乘 2 受眾列，一次刪完。
DELETE FROM notification.preference_overrides
WHERE account_id = sqlc.arg(account_id)
  AND (sqlc.narg(audience)::text IS NULL OR audience = sqlc.narg(audience)::text);
