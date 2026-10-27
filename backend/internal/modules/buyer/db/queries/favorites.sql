-- name: AddFavorite :execrows
-- 僅插入不存在的收藏，回傳受影響列數供事件判定。
INSERT INTO buyer.favorites (buyer_id, product_id, created_at)
VALUES ($1, $2, $3)
ON CONFLICT (buyer_id, product_id) DO NOTHING;

-- name: RemoveFavorite :execrows
-- 僅實際刪除收藏時回傳一列。
DELETE
FROM buyer.favorites
WHERE buyer_id = $1 AND product_id = $2;

-- name: ClearFavorite :many
-- 刪除全部收藏並回傳商品 ID 供事件發布。
DELETE
FROM buyer.favorites
WHERE buyer_id = $1
RETURNING product_id;

-- name: ListFavorite :many
-- 以建立時間及商品 ID 進行收藏 keyset 分頁。
SELECT *
FROM buyer.favorites
WHERE buyer_id = @buyer_id AND (NOT @has_after::boolean OR (created_at, product_id) < (@after_time::timestamptz, @after_id::bigint))
ORDER BY created_at DESC, product_id DESC
LIMIT @page_limit;

-- name: FavoriteIDs :many
-- 列出買家的全部收藏商品 ID。
SELECT product_id
FROM buyer.favorites
WHERE buyer_id = $1
ORDER BY product_id;

-- name: CheckFavorite :one
-- 查詢單一商品的收藏狀態。
SELECT EXISTS(SELECT 1
FROM buyer.favorites
WHERE buyer_id = $1 AND product_id = $2);

-- name: BatchFavorite :many
-- 回傳輸入清單中已收藏的商品 ID。
SELECT product_id
FROM buyer.favorites
WHERE buyer_id = $1 AND product_id = ANY($2::bigint[]);

-- name: CountFavorite :one
-- 計算買家收藏數量。
SELECT count(*)
FROM buyer.favorites
WHERE buyer_id = $1;
