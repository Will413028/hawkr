-- name: DeleteExpiredDeliveries :execrows
DELETE FROM notification.deliveries
WHERE id IN (
    SELECT d.id FROM notification.deliveries d
    WHERE d.created_at < sqlc.arg(cutoff)
    ORDER BY d.created_at, d.id
    LIMIT sqlc.arg(max_rows)
);

-- name: DeleteExpiredIntents :execrows
DELETE FROM notification.intents
WHERE id IN (
    SELECT i.id FROM notification.intents i
    WHERE i.created_at < sqlc.arg(cutoff)
      AND NOT EXISTS (SELECT 1 FROM notification.deliveries d WHERE d.intent_id = i.id)
    ORDER BY i.created_at, i.id
    LIMIT sqlc.arg(max_rows)
);

-- name: DeleteExpiredInboxItems :execrows
DELETE FROM notification.inbox_items
WHERE id IN (
    SELECT x.id FROM notification.inbox_items x
    WHERE x.updated_at < sqlc.arg(cutoff)
    ORDER BY x.updated_at, x.id
    LIMIT sqlc.arg(max_rows)
);

-- name: ScrubIntentParams :exec
UPDATE notification.intents
SET params = '{}'::jsonb
WHERE id = sqlc.arg(id) AND params <> '{}'::jsonb;

-- name: ScrubRecipientIntentParams :execrows
UPDATE notification.intents
SET params = '{}'::jsonb
WHERE id IN (
    SELECT s.id FROM notification.intents s
    WHERE s.params <> '{}'::jsonb
      AND NOT EXISTS (SELECT 1 FROM notification.announcements an WHERE an.id = s.id)
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
DELETE FROM notification.inbox_items
WHERE id IN (
    SELECT x.id FROM notification.inbox_items x
    WHERE x.account_id = sqlc.arg(account_id)
      AND (sqlc.narg(audience)::text IS NULL OR x.audience = sqlc.narg(audience)::text)
      AND (sqlc.narg(store_id)::bigint IS NULL OR x.store_id = sqlc.narg(store_id)::bigint)
    LIMIT sqlc.arg(max_rows)
);

-- name: DeleteAccountPreferenceOverrides :exec
DELETE FROM notification.preference_overrides
WHERE account_id = sqlc.arg(account_id)
  AND (sqlc.narg(audience)::text IS NULL OR audience = sqlc.narg(audience)::text);
