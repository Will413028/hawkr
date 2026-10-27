-- name: ItemsWithSellerName :many
SELECT i.id, s.name
FROM shop.items i
JOIN seller.stores s ON s.id = i.seller_id;
