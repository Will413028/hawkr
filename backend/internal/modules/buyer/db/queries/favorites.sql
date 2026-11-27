-- name: AddFavorite :execrows
INSERT INTO buyer.favorites (buyer_id, product_id, created_at)
VALUES ($1, $2, $3)
ON CONFLICT (buyer_id, product_id) DO NOTHING;

-- name: RemoveFavorite :execrows
DELETE
FROM buyer.favorites
WHERE buyer_id = $1 AND product_id = $2;

-- name: ClearFavorite :many
DELETE
FROM buyer.favorites
WHERE buyer_id = $1
RETURNING product_id;

-- name: ListFavorite :many
SELECT *
FROM buyer.favorites
WHERE buyer_id = @buyer_id AND (NOT @has_after::boolean OR (created_at, product_id) < (@after_time::timestamptz, @after_id::uuid))
ORDER BY created_at DESC, product_id DESC
LIMIT @page_limit;

-- name: FavoriteIDs :many
SELECT product_id
FROM buyer.favorites
WHERE buyer_id = $1
ORDER BY product_id;

-- name: CheckFavorite :one
SELECT EXISTS(SELECT 1
FROM buyer.favorites
WHERE buyer_id = $1 AND product_id = $2);

-- name: BatchFavorite :many
SELECT product_id
FROM buyer.favorites
WHERE buyer_id = $1 AND product_id = ANY($2::uuid[]);

-- name: CountFavorite :one
SELECT count(*)
FROM buyer.favorites
WHERE buyer_id = $1;
