-- name: RequeueFailedDeliveries :many
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
SELECT count(*) FROM (
    SELECT 1
    FROM notification.deliveries d
    JOIN notification.intents i ON i.id = d.intent_id
    WHERE i.created_at >= sqlc.arg(since)::timestamptz AND i.type = ANY(sqlc.arg(types)::text[])
    GROUP BY i.id, d.account_id, d.audience, d.store_id
    HAVING bool_and(d.status = 'FAILED_PERMANENT' AND d.failure_reason <> ALL(sqlc.arg(excluded_reasons)::text[]))
) g;

-- name: ListUnreachedNotifications :many
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
