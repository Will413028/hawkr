-- name: Addresses :many
-- 預設地址優先，接著以更新時間與 ID 降冪分頁。
SELECT *
FROM buyer.shipping_addresses
WHERE buyer_id = @buyer_id
  AND (NOT @has_cursor::boolean OR
       (is_default, updated_at, address_id) < (@cursor_default::boolean, @cursor_time::timestamptz, @cursor_id::bigint))
ORDER BY is_default DESC, updated_at DESC, address_id DESC
LIMIT @page_limit;

-- name: AllAddresses :many
-- 寫入預設地址時檢查買家的現有地址。
SELECT * FROM buyer.shipping_addresses WHERE buyer_id = $1
ORDER BY is_default DESC, updated_at DESC, address_id DESC;

-- name: Address :one
-- 以買家歸屬查單筆地址。
SELECT *
FROM buyer.shipping_addresses
WHERE buyer_id = $1 AND address_id = $2;

-- name: ClearDefault :exec
-- 移除現有預設標記，呼叫方隨後指定新預設。
UPDATE buyer.shipping_addresses
SET is_default = false
  , version = version + 1
  , updated_at = $3
WHERE buyer_id = $1 AND is_default AND address_id <> $2;

-- name: AddAddress :one
-- 建立地址及預設標記。
INSERT INTO buyer.shipping_addresses (buyer_id, receiver_name, receiver_phone, town_code, address_detail, is_default, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
RETURNING *;

-- name: UpdateAddress :one
-- 修改買家擁有的地址。
UPDATE buyer.shipping_addresses
SET receiver_name = $3,
    receiver_phone = $4,
    town_code = $5,
    address_detail = $6,
    is_default = $7,
    updated_at = $8,
    version = version + 1
WHERE buyer_id = $1 AND address_id = $2 AND version = $9
RETURNING *;

-- name: DeleteAddress :one
-- 永久刪除買家擁有的地址。
DELETE
FROM buyer.shipping_addresses
WHERE buyer_id = $1 AND address_id = $2 AND (NOT @check_version::boolean OR version = @expected_version::bigint)
RETURNING address_id;

-- name: PromoteDefault :exec
-- 沒有預設地址時，以最近更新的剩餘地址遞補。
UPDATE buyer.shipping_addresses
SET is_default = true
  , version = version + 1
  , updated_at = $2
WHERE address_id = (SELECT a.address_id
FROM buyer.shipping_addresses a
WHERE a.buyer_id = $1
ORDER BY a.updated_at DESC, a.address_id DESC
LIMIT 1) AND NOT EXISTS (SELECT 1
FROM buyer.shipping_addresses d
WHERE d.buyer_id = $1 AND d.is_default);

-- name: DeleteAddresses :exec
-- 關閉 stint 時永久刪除全部地址。
DELETE
FROM buyer.shipping_addresses
WHERE buyer_id = $1;
