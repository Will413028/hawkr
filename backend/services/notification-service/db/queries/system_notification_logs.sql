-- name: InsertSystemNotificationLog :one
INSERT INTO system_notification_logs (
  id, notification_type, title, body, image_key, deep_link_url,
  content_hash, triggered_by_account_id, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending')
RETURNING id, triggered_at;

-- name: FindRecentLogByContentHash :one
SELECT id, notification_type, title, body, image_key, deep_link_url,
       content_hash, triggered_by_account_id, triggered_at, fcm_message_id,
       status, failed_reason, completed_at
FROM system_notification_logs
WHERE content_hash = $1 AND triggered_at > $2
ORDER BY triggered_at DESC
LIMIT 1;

-- name: ListSystemNotificationLogs :many
SELECT id, notification_type, title, body, image_key, deep_link_url,
       content_hash, triggered_by_account_id, triggered_at, fcm_message_id,
       status, failed_reason, completed_at
FROM system_notification_logs
WHERE (sqlc.narg('type')::varchar IS NULL OR notification_type = sqlc.narg('type'))
ORDER BY triggered_at DESC
LIMIT $1 OFFSET $2;

-- name: CountSystemNotificationLogs :one
SELECT COUNT(*)
FROM system_notification_logs
WHERE (sqlc.narg('type')::varchar IS NULL OR notification_type = sqlc.narg('type'));

-- name: UpdateSystemNotificationLogStatus :exec
UPDATE system_notification_logs
SET status = $2,
    fcm_message_id = $3,
    failed_reason = $4,
    completed_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: DeleteOldSystemNotificationLogs :execrows
DELETE FROM system_notification_logs
WHERE triggered_at < $1;
