-- name: DeliveryByID :one
SELECT id, intent_id, account_id, audience, store_id, channel, status
FROM notification.deliveries WHERE id = sqlc.arg(id);

-- name: DeliveryForUpdate :one
SELECT id, intent_id, account_id, audience, store_id, channel, status
FROM notification.deliveries WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: MarkDeliverySent :execrows
UPDATE notification.deliveries
SET status = 'SENT', locale = sqlc.narg(locale), sent_title = sqlc.narg(title), sent_body = sqlc.narg(body), sent_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND status = 'PENDING';

-- name: MarkDeliveryFailed :execrows
UPDATE notification.deliveries
SET status = 'FAILED_PERMANENT', failure_reason = sqlc.arg(reason)::text, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND status = 'PENDING';

-- name: DeviceTokensForRecipient :many
SELECT token FROM notification.device_tokens
WHERE account_id = sqlc.arg(account_id) AND app = sqlc.arg(app)
ORDER BY token;

-- name: DeleteDeviceTokensByValue :execrows
DELETE FROM notification.device_tokens WHERE token = ANY(sqlc.arg(tokens)::text[]);

-- name: DeleteDeviceTokensByAccount :execrows
DELETE FROM notification.device_tokens WHERE account_id = sqlc.arg(account_id);

-- name: DeleteStaleDeviceTokens :execrows
DELETE FROM notification.device_tokens
WHERE token IN (
    SELECT t.token FROM notification.device_tokens t
    WHERE t.last_registered_at < sqlc.arg(cutoff)
    ORDER BY t.last_registered_at, t.token
    LIMIT sqlc.arg(max_rows)
);

-- name: EmailSuppressed :one
SELECT EXISTS (SELECT 1 FROM notification.email_suppressions WHERE address_hash = ANY(sqlc.arg(hashes)::bytea[])) AS suppressed;

-- name: InsertEmailSuppression :exec
INSERT INTO notification.email_suppressions (address_hash, reason, created_at)
VALUES (sqlc.arg(address_hash), sqlc.arg(reason), sqlc.arg(now))
ON CONFLICT (address_hash) DO UPDATE SET reason = 'COMPLAINT'
WHERE excluded.reason = 'COMPLAINT' AND notification.email_suppressions.reason <> 'COMPLAINT';

-- name: DeleteHardBounceSuppressions :execrows
DELETE FROM notification.email_suppressions WHERE address_hash = ANY(sqlc.arg(hashes)::bytea[]) AND reason = 'HARD_BOUNCE';

-- name: EmailSuppressionReasons :many
SELECT reason FROM notification.email_suppressions WHERE address_hash = ANY(sqlc.arg(hashes)::bytea[]);

-- name: InboxItemForDelivery :one
SELECT x.id FROM notification.inbox_items x
WHERE x.account_id = sqlc.arg(account_id) AND x.audience = sqlc.arg(audience) AND x.store_id IS NOT DISTINCT FROM sqlc.narg(store_id)
  AND (x.announcement_id = sqlc.arg(intent_id)::uuid
       OR EXISTS (SELECT 1 FROM notification.inbox_item_intents l WHERE l.inbox_item_id = x.id AND l.intent_id = sqlc.arg(intent_id)::uuid))
LIMIT 1;
