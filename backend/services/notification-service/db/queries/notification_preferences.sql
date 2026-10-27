-- name: UpsertNotificationPreference :exec
INSERT INTO notification_preferences (user_id, user_type, category, enabled, updated_at)
VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, user_type, category)
DO UPDATE SET enabled = $4, updated_at = CURRENT_TIMESTAMP;

-- name: GetNotificationPreferencesByUser :many
SELECT * FROM notification_preferences
WHERE user_id = $1 AND user_type = $2;

-- name: GetNotificationPreference :one
SELECT * FROM notification_preferences
WHERE user_id = $1 AND user_type = $2 AND category = $3;
