-- name: PendingRequest :one
SELECT * FROM accountdeletion.deletion_requests
WHERE account_id = $1 AND status = 'PENDING_ORDERS';

-- name: LatestRequest :one
SELECT * FROM accountdeletion.deletion_requests
WHERE account_id = $1
ORDER BY requested_at DESC, id DESC
LIMIT 1;

-- name: RequestByID :one
SELECT * FROM accountdeletion.deletion_requests
WHERE id = $1;

-- name: PendingBatch :many
SELECT account_id FROM accountdeletion.deletion_requests
WHERE status = 'PENDING_ORDERS' AND account_id > $1
ORDER BY account_id
LIMIT $2;

-- name: UpsertPendingRequest :one
INSERT INTO accountdeletion.deletion_requests
    (id, account_id, status, requested_at)
VALUES ($1, $2, 'PENDING_ORDERS', $3)
ON CONFLICT (account_id) WHERE status = 'PENDING_ORDERS'
DO UPDATE SET account_id = EXCLUDED.account_id
RETURNING *;

-- name: InsertCompletedRequest :one
INSERT INTO accountdeletion.deletion_requests
    (id, account_id, status, requested_at, completed_at)
VALUES ($1, $2, 'COMPLETED', $3, $3)
RETURNING *;

-- name: CompleteRequest :one
UPDATE accountdeletion.deletion_requests
SET status = 'COMPLETED', completed_at = $2
WHERE id = $1 AND status = 'PENDING_ORDERS'
RETURNING *;

-- name: InsertRequestAttempt :exec
INSERT INTO accountdeletion.request_attempts
    (id, request_id, requester, operator_admin_id, reason, attempted_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: AccountsWithOpenRequest :many
SELECT DISTINCT account_id FROM accountdeletion.deletion_requests
WHERE account_id = ANY(sqlc.arg(account_ids)::bigint[]) AND status = 'PENDING_ORDERS'
ORDER BY account_id;
