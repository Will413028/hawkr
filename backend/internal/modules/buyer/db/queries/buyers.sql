-- name: Current :one
SELECT *
FROM buyer.buyers
WHERE account_id = $1 AND status <> 'CLOSED';

-- name: CurrentBuyerID :one
SELECT buyer_id
FROM buyer.buyers
WHERE account_id = $1 AND status <> 'CLOSED';

-- name: ByID :one
SELECT *
FROM buyer.buyers
WHERE buyer_id = $1;

-- name: LockBuyer :one
SELECT *
FROM buyer.buyers
WHERE buyer_id = $1
FOR NO KEY UPDATE;

-- name: LockBuyerForTransition :one
SELECT *
FROM buyer.buyers
WHERE buyer_id = $1
FOR UPDATE;

-- name: LockBuyerShared :one
SELECT *
FROM buyer.buyers
WHERE buyer_id = $1
FOR KEY SHARE;

-- name: LockBuyerSharedByAccount :one
SELECT status, pii_cleared_at
FROM buyer.buyers
WHERE account_id = $1
FOR KEY SHARE;

-- name: CreateBuyer :one
INSERT INTO buyer.buyers (account_id, display_name, created_at, updated_at)
VALUES ($1, $2, $3, $3)
RETURNING *;

-- name: UpdateProfile :one
UPDATE buyer.buyers
SET display_name = $2,
    phone = $3,
    gender = $4,
    birth_date = $5,
    city = $6,
    job = $7,
    updated_at = $8,
    version = version + 1
WHERE buyer_id = $1 AND version = $9
RETURNING *;

-- name: UpdateAvatar :one
UPDATE buyer.buyers
SET avatar = sqlc.arg(avatar),
    avatar_asset_id = sqlc.arg(avatar_asset_id),
    updated_at = sqlc.arg(updated_at),
    version = version + 1
WHERE buyer_id = sqlc.arg(buyer_id) AND version = sqlc.arg(version)
RETURNING *;

-- name: BatchProfiles :many
SELECT *
FROM buyer.buyers
WHERE buyer_id = ANY($1::bigint[]) AND status <> 'CLOSED' AND pii_cleared_at IS NULL
ORDER BY buyer_id;

-- name: BatchDisplay :many
SELECT buyer_id, display_name, avatar, avatar_asset_id, (status = 'CLOSED')::boolean AS deleted
FROM buyer.buyers
WHERE buyer_id = ANY($1::bigint[])
ORDER BY buyer_id;

-- name: Principals :many
SELECT buyer_id, account_id
FROM buyer.buyers
WHERE buyer_id = ANY($1::bigint[]) AND status <> 'CLOSED'
ORDER BY buyer_id;

-- name: BuyerIDs :many
SELECT buyer_id
FROM buyer.buyers
WHERE account_id = $1
ORDER BY buyer_id;

-- name: Stats :one
SELECT count(*)
FROM buyer.buyers
WHERE status <> 'CLOSED';

-- name: Pending :exec
UPDATE buyer.buyers
SET status = 'PENDING_DELETION',
    updated_at = $2,
    version = version + 1
WHERE buyer_id = $1;

-- name: Close :exec
UPDATE buyer.buyers
SET status = 'CLOSED',
    deleted_at = $2,
    updated_at = $2,
    version = version + 1
WHERE buyer_id = $1;

-- name: CleanupBatch :many
SELECT buyer_id
FROM buyer.buyers
WHERE status = 'CLOSED' AND deleted_at < $1 AND pii_cleared_at IS NULL
ORDER BY buyer_id
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: ClearPII :exec
UPDATE buyer.buyers
SET display_name = NULL,
    phone = NULL,
    gender = NULL,
    avatar = NULL,
    avatar_asset_id = NULL,
    birth_date = NULL,
    city = NULL,
    job = NULL,
    pii_cleared_at = $2,
    updated_at = $2,
    version = version + 1
WHERE buyer_id = $1 AND status = 'CLOSED' AND pii_cleared_at IS NULL;

-- name: ListLiveAccounts :many
SELECT buyer_id, account_id
FROM buyer.buyers
WHERE status <> 'CLOSED' AND buyer_id > sqlc.arg(after_buyer_id)
ORDER BY buyer_id
LIMIT sqlc.arg(page_size);
