-- name: InsertIntent :one
-- 意圖冪等：同一個事件來源身分（idempotency_key）只會有一列。已存在時回傳 0 列（pgx.ErrNoRows），呼叫端不再排 job。
INSERT INTO notification.intents (id, type, idempotency_key, params, created_at)
VALUES (sqlc.arg(id), sqlc.arg(type), sqlc.arg(idempotency_key), sqlc.arg(params), sqlc.arg(created_at))
ON CONFLICT ON CONSTRAINT intents_idempotency_key_key DO NOTHING
RETURNING id;

-- name: IntentByID :one
SELECT id, type, idempotency_key, params, created_at FROM notification.intents WHERE id = sqlc.arg(id);

-- name: PreferenceOverrides :many
-- 收件人在一個分類下各渠道的覆寫列（沒有列就是註冊表的預設）。
SELECT channel, enabled FROM notification.preference_overrides
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience) AND category = sqlc.arg(category);

-- name: InboxItemExistsForIntent :one
-- 同一個意圖對同一個收件人已經寫過收件匣列（worker 重試時略過）。聚合合併進既有列的意圖不帶 intent_id，重試時再合併一次是冪等的（actor 取聯集）。
SELECT EXISTS (
    SELECT 1 FROM notification.inbox_items
    WHERE intent_id = sqlc.arg(intent_id) AND account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience)
      AND store_id IS NOT DISTINCT FROM sqlc.narg(store_id)::bigint
) AS written;

-- name: UpsertInboxItem :exec
-- 寫收件匣列。有聚合鍵時，同一個收件人（含 store_id）已有未讀的同鍵列就合併進去（參數換成最新、actor 取聯集、updated_at 往前推）；
-- 沒有聚合鍵（NULL）或已讀的列不會衝突，另開新列。
INSERT INTO notification.inbox_items (id, account_id, audience, store_id, type, params, aggregation_key, actor_account_ids, intent_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(account_id), sqlc.arg(audience), sqlc.narg(store_id), sqlc.arg(type), sqlc.arg(params), sqlc.narg(aggregation_key),
        sqlc.arg(actor_account_ids)::bigint[], sqlc.arg(intent_id), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (account_id, audience, store_id, aggregation_key) WHERE aggregation_key IS NOT NULL AND read_at IS NULL
DO UPDATE SET
    params = EXCLUDED.params,
    actor_account_ids = ARRAY(SELECT DISTINCT a FROM unnest(notification.inbox_items.actor_account_ids || EXCLUDED.actor_account_ids) AS a ORDER BY a),
    updated_at = GREATEST(notification.inbox_items.updated_at, EXCLUDED.updated_at);

-- name: InsertDelivery :exec
-- 投遞紀錄是推播與 email 的唯一帳本；(意圖, 收件人, 渠道) 已存在時不重複建立。status 一律從 PENDING 開始。
INSERT INTO notification.deliveries (id, intent_id, account_id, audience, store_id, channel, status, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(intent_id), sqlc.arg(account_id), sqlc.arg(audience), sqlc.narg(store_id), sqlc.arg(channel), 'PENDING', sqlc.arg(now), sqlc.arg(now))
ON CONFLICT ON CONSTRAINT deliveries_recipient_channel_key DO NOTHING;
