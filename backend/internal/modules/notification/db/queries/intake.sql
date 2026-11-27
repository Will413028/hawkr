-- name: InsertIntent :one
INSERT INTO notification.intents (id, type, idempotency_key, params, occurred_at, created_at)
VALUES (sqlc.arg(id), sqlc.arg(type), sqlc.arg(idempotency_key), sqlc.arg(params), sqlc.arg(occurred_at), sqlc.arg(created_at))
ON CONFLICT ON CONSTRAINT intents_idempotency_key_key DO NOTHING
RETURNING id;

-- name: IntentByID :one
SELECT id, type, idempotency_key, params, occurred_at, created_at FROM notification.intents WHERE id = sqlc.arg(id);

-- name: PreferenceOverrides :many
SELECT channel, enabled FROM notification.preference_overrides
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience) AND category = sqlc.arg(category);

-- name: InboxItemIntentLinked :one
SELECT EXISTS (
    SELECT 1 FROM notification.inbox_item_intents WHERE intent_id = sqlc.arg(intent_id)
) AS linked;

-- name: IntentHasRecipientRows :one
SELECT (EXISTS (
    SELECT 1 FROM notification.inbox_item_intents l WHERE l.intent_id = sqlc.arg(intent_id)
) OR EXISTS (
    SELECT 1 FROM notification.deliveries d WHERE d.intent_id = sqlc.arg(intent_id)
)) ::boolean AS has_rows;

-- name: UpsertInboxItem :one
INSERT INTO notification.inbox_items (id, account_id, audience, store_id, type, params, aggregation_key, actor_account_ids, occurred_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(account_id), sqlc.arg(audience), sqlc.narg(store_id), sqlc.arg(type), sqlc.arg(params), sqlc.narg(aggregation_key),
        sqlc.arg(actor_account_ids)::bigint[], sqlc.arg(occurred_at), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (account_id, audience, store_id, aggregation_key) WHERE aggregation_key IS NOT NULL AND read_at IS NULL
DO UPDATE SET
    params = CASE WHEN EXCLUDED.occurred_at >= notification.inbox_items.occurred_at THEN EXCLUDED.params ELSE notification.inbox_items.params END,
    occurred_at = GREATEST(notification.inbox_items.occurred_at, EXCLUDED.occurred_at),
    actor_account_ids = ARRAY(SELECT DISTINCT a FROM unnest(notification.inbox_items.actor_account_ids || EXCLUDED.actor_account_ids) AS a ORDER BY a),
    updated_at = GREATEST(notification.inbox_items.updated_at, EXCLUDED.updated_at)
RETURNING id;

-- name: LinkInboxItemIntent :exec
INSERT INTO notification.inbox_item_intents (intent_id, inbox_item_id)
VALUES (sqlc.arg(intent_id), sqlc.arg(inbox_item_id))
ON CONFLICT (intent_id) DO NOTHING;

-- name: InsertDelivery :one
INSERT INTO notification.deliveries (id, intent_id, account_id, audience, store_id, channel, status, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(intent_id), sqlc.arg(account_id), sqlc.arg(audience), sqlc.narg(store_id), sqlc.arg(channel), 'PENDING', sqlc.arg(now), sqlc.arg(now))
ON CONFLICT ON CONSTRAINT deliveries_recipient_channel_key DO NOTHING
RETURNING id;
