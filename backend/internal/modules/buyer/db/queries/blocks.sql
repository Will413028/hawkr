-- name: AddBlockedBuyer :execrows
INSERT INTO buyer.blocked_buyers (blocker_buyer_id, blocked_buyer_id, created_at)
VALUES ($1, $2, $3)
ON CONFLICT (blocker_buyer_id, blocked_buyer_id) DO NOTHING;

-- name: RemoveBlockedBuyer :execrows
DELETE
FROM buyer.blocked_buyers
WHERE blocker_buyer_id = $1 AND blocked_buyer_id = $2;

-- name: ListBlockedBuyers :many
SELECT blocked_buyer_id, created_at
FROM buyer.blocked_buyers
WHERE blocker_buyer_id = @blocker_buyer_id
  AND (NOT @has_after::boolean OR (created_at, blocked_buyer_id) < (@after_time::timestamptz, @after_id::bigint))
ORDER BY created_at DESC, blocked_buyer_id DESC
LIMIT @page_limit;

-- name: IsBlockedBetweenBuyers :one
SELECT EXISTS(SELECT 1
FROM buyer.blocked_buyers
WHERE (blocker_buyer_id = @buyer_a AND blocked_buyer_id = @buyer_b) OR (blocker_buyer_id = @buyer_b AND blocked_buyer_id = @buyer_a));

-- name: AddBlockedStore :execrows
INSERT INTO buyer.blocked_stores (blocker_buyer_id, store_id, created_at)
VALUES ($1, $2, $3)
ON CONFLICT (blocker_buyer_id, store_id) DO NOTHING;

-- name: RemoveBlockedStore :execrows
DELETE
FROM buyer.blocked_stores
WHERE blocker_buyer_id = $1 AND store_id = $2;

-- name: ListBlockedStores :many
SELECT store_id, created_at
FROM buyer.blocked_stores
WHERE blocker_buyer_id = @blocker_buyer_id
  AND (NOT @has_after::boolean OR (created_at, store_id) < (@after_time::timestamptz, @after_id::bigint))
ORDER BY created_at DESC, store_id DESC
LIMIT @page_limit;

-- name: IsBlockingStore :one
SELECT EXISTS(SELECT 1
FROM buyer.blocked_stores
WHERE blocker_buyer_id = $1 AND store_id = $2);

-- name: ClearBlockedStores :exec
DELETE
FROM buyer.blocked_stores
WHERE blocker_buyer_id = $1;

-- name: ClearBlockedBuyers :exec
DELETE
FROM buyer.blocked_buyers
WHERE blocker_buyer_id = $1 OR blocked_buyer_id = $1;
