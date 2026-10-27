-- name: CreateBlock :one
-- Idempotent block: ON CONFLICT returns the existing row's id.
INSERT INTO user_blocks (id, blocker_id, blocked_id)
VALUES ($1, $2, $3)
ON CONFLICT (blocker_id, blocked_id) DO UPDATE SET id = user_blocks.id
RETURNING *;

-- name: DeleteBlock :exec
DELETE FROM user_blocks
WHERE blocker_id = $1 AND blocked_id = $2;

-- name: ListBlocksByBlocker :many
SELECT * FROM user_blocks
WHERE blocker_id = $1
ORDER BY create_time DESC
LIMIT $2 OFFSET $3;

-- name: GetBlock :one
SELECT * FROM user_blocks
WHERE blocker_id = $1 AND blocked_id = $2
LIMIT 1;
