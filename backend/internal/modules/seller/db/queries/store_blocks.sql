-- name: StoreExists :one
SELECT EXISTS(SELECT 1 FROM seller.stores WHERE store_id = $1);

-- name: AddStoreBlock :execrows
INSERT INTO seller.store_blocks (store_id, buyer_id, created_at)
VALUES ($1, $2, $3)
ON CONFLICT (store_id, buyer_id) DO NOTHING;

-- name: RemoveStoreBlock :execrows
DELETE
FROM seller.store_blocks
WHERE store_id = $1 AND buyer_id = $2;

-- name: ListStoreBlocks :many
SELECT buyer_id, created_at
FROM seller.store_blocks
WHERE store_id = @store_id
  AND (NOT @has_after::boolean OR (created_at, buyer_id) < (@after_time::timestamptz, @after_id::bigint))
ORDER BY created_at DESC, buyer_id DESC
LIMIT @page_limit;

-- name: StoreBlocksBuyer :one
SELECT EXISTS(SELECT 1
FROM seller.store_blocks
WHERE store_id = $1 AND buyer_id = $2);

-- name: StoresBlockingBuyer :many
SELECT store_id
FROM seller.store_blocks
WHERE buyer_id = $1 AND store_id = ANY($2::bigint[])
ORDER BY store_id;

-- name: BuyersBlockedByStore :many
SELECT buyer_id
FROM seller.store_blocks
WHERE store_id = $1 AND buyer_id = ANY($2::bigint[])
ORDER BY buyer_id;

-- name: ClearStoreBlocksAgainstBuyer :exec
DELETE
FROM seller.store_blocks
WHERE buyer_id = $1;
