-- name: CreateModerationAction :one
INSERT INTO moderation_actions (id, report_id, admin_id, affected_user_id, action, note, attempted_content, previous_state, new_state)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListModerationActionsByReport :many
SELECT * FROM moderation_actions
WHERE report_id = $1
ORDER BY create_time DESC;

-- name: ListModerationActionsByUser :many
SELECT * FROM moderation_actions
WHERE affected_user_id = $1
ORDER BY create_time DESC
LIMIT $2 OFFSET $3;
