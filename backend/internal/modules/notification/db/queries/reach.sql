-- name: RequeueFailedDeliveries :many
-- 重送：把永久失敗、原因在 reasons 內、失敗時間（updated_at）落在 [start, end) 的投遞改回待送，一次最多 max_rows 筆，依失敗時間由舊到新。
-- 只動 FAILED_PERMANENT 的列（SENT 與 PENDING 不會被選中，所以重複呼叫不會讓同一筆送出兩次）；SKIP LOCKED 讓並行的重送各拿不重疊的列。
-- failure_reason 清空以滿足 deliveries_failure_check；updated_at 改成 now，所以再失敗時一定晚於窗的終點。types 為 NULL 表示不限型別。
WITH picked AS (
    SELECT d.id
    FROM notification.deliveries d
    JOIN notification.intents i ON i.id = d.intent_id
    WHERE d.status = 'FAILED_PERMANENT'
      AND d.failure_reason = ANY(sqlc.arg(reasons)::text[])
      AND d.updated_at >= sqlc.arg(start_time)::timestamptz AND d.updated_at < sqlc.arg(end_time)::timestamptz
      AND (sqlc.narg(channel)::text IS NULL OR d.channel = sqlc.narg(channel)::text)
      AND (sqlc.narg(account_id)::bigint IS NULL OR d.account_id = sqlc.narg(account_id)::bigint)
      AND (sqlc.narg(types)::text[] IS NULL OR i.type = ANY(sqlc.narg(types)::text[]))
    ORDER BY d.updated_at, d.id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE OF d SKIP LOCKED
)
UPDATE notification.deliveries d
SET status = 'PENDING', failure_reason = NULL, updated_at = sqlc.arg(now)::timestamptz
FROM picked
WHERE d.id = picked.id AND d.status = 'FAILED_PERMANENT'
RETURNING d.id;

-- name: CountRequeueableDeliveries :one
-- 與 RequeueFailedDeliveries 同一組條件，最多數到 max_rows 筆：dry run 與「還有沒有更多」用。
SELECT count(*) FROM (
    SELECT 1
    FROM notification.deliveries d
    JOIN notification.intents i ON i.id = d.intent_id
    WHERE d.status = 'FAILED_PERMANENT'
      AND d.failure_reason = ANY(sqlc.arg(reasons)::text[])
      AND d.updated_at >= sqlc.arg(start_time)::timestamptz AND d.updated_at < sqlc.arg(end_time)::timestamptz
      AND (sqlc.narg(channel)::text IS NULL OR d.channel = sqlc.narg(channel)::text)
      AND (sqlc.narg(account_id)::bigint IS NULL OR d.account_id = sqlc.narg(account_id)::bigint)
      AND (sqlc.narg(types)::text[] IS NULL OR i.type = ANY(sqlc.narg(types)::text[]))
    LIMIT sqlc.arg(max_rows)
) c;

-- name: CountUnreachedNotifications :one
-- 未觸達：窗內（意圖建立時間 >= since）、型別在 types（交易必要）的意圖，某個收件人的所有投遞紀錄都是永久失敗，且沒有一筆的原因在 excluded_reasons
-- （刻意不送、收件人已刪除）。只看投遞紀錄：收件匣列不是觸達；還在待送的不算；沒有投遞紀錄的意圖不算。
SELECT count(*) FROM (
    SELECT 1
    FROM notification.deliveries d
    JOIN notification.intents i ON i.id = d.intent_id
    WHERE i.created_at >= sqlc.arg(since)::timestamptz AND i.type = ANY(sqlc.arg(types)::text[])
    GROUP BY i.id, d.account_id, d.audience, d.store_id
    HAVING bool_and(d.status = 'FAILED_PERMANENT' AND d.failure_reason <> ALL(sqlc.arg(excluded_reasons)::text[]))
) g;

-- name: ListUnreachedNotifications :many
-- 與 CountUnreachedNotifications 同一組條件；新到舊（意圖建立時間，再依代表投遞 id），keyset 分頁，呼叫端要 limit + 1 筆來判斷有沒有下一頁。
-- ref_id 是該收件人這則通知的投遞 id 最小者（一個收件人一則通知最多兩列：推播與 email）。
SELECT i.id AS intent_id, i.type, i.created_at AS intent_created_at, d.account_id, d.audience, d.store_id,
       (array_agg(d.id ORDER BY d.id))[1]::uuid AS ref_id,
       array_agg(d.channel ORDER BY d.channel)::text[] AS channels,
       array_agg(d.failure_reason ORDER BY d.channel)::text[] AS reasons
FROM notification.deliveries d
JOIN notification.intents i ON i.id = d.intent_id
WHERE i.created_at >= sqlc.arg(since)::timestamptz AND i.type = ANY(sqlc.arg(types)::text[])
GROUP BY i.id, d.account_id, d.audience, d.store_id
HAVING bool_and(d.status = 'FAILED_PERMANENT' AND d.failure_reason <> ALL(sqlc.arg(excluded_reasons)::text[]))
   AND (sqlc.narg(after_created_at)::timestamptz IS NULL
        OR (i.created_at, (array_agg(d.id ORDER BY d.id))[1]::uuid) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY i.created_at DESC, ref_id DESC
LIMIT sqlc.arg(page_size);
