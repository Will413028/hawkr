-- name: ListInboxItems :many
SELECT id, type, params, read_at, updated_at FROM notification.inbox_items
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience)
  AND (store_id = sqlc.narg(store_id)::bigint OR (sqlc.narg(store_id)::bigint IS NULL AND store_id IS NULL))
  AND type = ANY(sqlc.arg(types)::text[])
  AND (sqlc.narg(after_updated_at)::timestamptz IS NULL OR (updated_at, id) < (sqlc.narg(after_updated_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(max_rows);

-- name: CountInboxItems :one
SELECT count(*) FROM notification.inbox_items
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience)
  AND (store_id = sqlc.narg(store_id)::bigint OR (sqlc.narg(store_id)::bigint IS NULL AND store_id IS NULL))
  AND type = ANY(sqlc.arg(types)::text[]);

-- name: CountUnreadInboxItems :one
SELECT count(*) FROM notification.inbox_items
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience)
  AND (store_id = sqlc.narg(store_id)::bigint OR (sqlc.narg(store_id)::bigint IS NULL AND store_id IS NULL))
  AND type = ANY(sqlc.arg(types)::text[])
  AND read_at IS NULL;

-- name: LatestInboxItemID :one
SELECT id FROM notification.inbox_items
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience)
  AND (store_id = sqlc.narg(store_id)::bigint OR (sqlc.narg(store_id)::bigint IS NULL AND store_id IS NULL))
  AND type = ANY(sqlc.arg(types)::text[])
ORDER BY updated_at DESC, id DESC LIMIT 1;

-- name: MarkInboxItemRead :one
UPDATE notification.inbox_items SET read_at = COALESCE(read_at, sqlc.arg(now)::timestamptz)
WHERE id = sqlc.arg(id) AND account_id = sqlc.arg(account_id)
RETURNING id, audience, store_id;

-- name: MarkAllInboxItemsRead :execrows
UPDATE notification.inbox_items SET read_at = sqlc.arg(now)::timestamptz
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience)
  AND (store_id = sqlc.narg(store_id)::bigint OR (sqlc.narg(store_id)::bigint IS NULL AND store_id IS NULL))
  AND type = ANY(sqlc.arg(types)::text[])
  AND read_at IS NULL;

-- name: DeleteInboxItems :many
DELETE FROM notification.inbox_items WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND account_id = sqlc.arg(account_id)
RETURNING audience, store_id;

-- name: PreferenceOverridesForRecipient :many
SELECT category, channel, enabled FROM notification.preference_overrides
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience);

-- name: UpsertPreferenceOverride :exec
INSERT INTO notification.preference_overrides (account_id, audience, category, channel, enabled, updated_at)
VALUES (sqlc.arg(account_id), sqlc.arg(audience), sqlc.arg(category), sqlc.arg(channel), sqlc.arg(enabled), sqlc.arg(now))
ON CONFLICT (account_id, audience, category, channel) DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = EXCLUDED.updated_at;

-- name: DeletePreferenceOverride :exec
DELETE FROM notification.preference_overrides
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience) AND category = sqlc.arg(category) AND channel = sqlc.arg(channel);

-- name: UpsertDeviceToken :exec
INSERT INTO notification.device_tokens (token, account_id, app, last_registered_at)
VALUES (sqlc.arg(token), sqlc.arg(account_id), sqlc.arg(app), sqlc.arg(now))
ON CONFLICT (token) DO UPDATE SET account_id = EXCLUDED.account_id, app = EXCLUDED.app, last_registered_at = EXCLUDED.last_registered_at;

-- name: DeleteDeviceToken :exec
DELETE FROM notification.device_tokens WHERE token = sqlc.arg(token) AND account_id = sqlc.arg(account_id);
