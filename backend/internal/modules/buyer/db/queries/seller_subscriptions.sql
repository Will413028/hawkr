-- name: AddFollow :execrows
INSERT INTO buyer.seller_subscriptions (buyer_id, store_id, created_at)
VALUES ($1, $2, $3)
ON CONFLICT (buyer_id, store_id) DO NOTHING;

-- name: RemoveFollow :execrows
DELETE
FROM buyer.seller_subscriptions
WHERE buyer_id = $1 AND store_id = $2;

-- name: ClearFollow :many
DELETE
FROM buyer.seller_subscriptions
WHERE buyer_id = $1
RETURNING store_id;

-- name: ListFollow :many
SELECT *
FROM buyer.seller_subscriptions
WHERE buyer_id = @buyer_id AND (NOT @has_after::boolean OR (created_at, store_id) < (@after_time::timestamptz, @after_id::bigint))
ORDER BY created_at DESC, store_id DESC
LIMIT @page_limit;

-- name: FollowIDs :many
SELECT store_id
FROM buyer.seller_subscriptions
WHERE buyer_id = $1
ORDER BY store_id;

-- name: FollowCount :one
SELECT count(*)::bigint FROM buyer.seller_subscriptions WHERE buyer_id = $1;

-- name: CheckFollow :one
SELECT EXISTS(SELECT 1
FROM buyer.seller_subscriptions
WHERE buyer_id = $1 AND store_id = $2);

-- name: BatchFollow :many
SELECT store_id
FROM buyer.seller_subscriptions
WHERE buyer_id = $1 AND store_id = ANY($2::bigint[]);

-- name: Subscribers :many
SELECT *
FROM buyer.seller_subscriptions
WHERE store_id = @store_id AND (NOT @has_after::boolean OR (created_at, buyer_id)<(@after_time::timestamptz, @after_id::bigint))
ORDER BY created_at DESC, buyer_id DESC
LIMIT @page_limit;

-- name: SubscriberCounts :many
SELECT store_id, count(*)::bigint AS count
FROM buyer.seller_subscriptions
WHERE store_id = ANY($1::bigint[]) GROUP BY store_id;

-- name: SubscriberStats :many
SELECT w.idx::int AS window_index, c.total::bigint AS total, c.new_count::bigint AS new_count
FROM generate_series(1, cardinality(@window_starts::timestamptz[])) AS w(idx)
CROSS JOIN LATERAL (
    SELECT count(*) AS total,
           count(*) FILTER (WHERE s.created_at >= (@window_starts::timestamptz[])[w.idx] AND s.created_at < (@window_ends::timestamptz[])[w.idx]) AS new_count
    FROM buyer.seller_subscriptions s
    WHERE s.store_id = @store_id
) c
ORDER BY w.idx;

-- name: ListLiveFollowerAccounts :many
SELECT b.buyer_id, b.account_id
FROM buyer.seller_subscriptions s
JOIN buyer.buyers b ON b.buyer_id = s.buyer_id
WHERE s.store_id = sqlc.arg(store_id) AND s.buyer_id > sqlc.arg(after_buyer_id) AND s.created_at <= sqlc.arg(followed_by) AND b.status <> 'CLOSED'
ORDER BY s.buyer_id
LIMIT sqlc.arg(page_size);
