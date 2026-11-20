-- 標籤與客戶的標籤。

-- name: LockStoreTags :exec
-- 同一家店的建立標籤（CreateTag、AddCustomerTags）串行：每店標籤數上限的檢查與插入之間不能有人插入。crm 自己的命名空間，不進鎖序。
SELECT pg_advisory_xact_lock(hashtextextended('crm.tags:' || sqlc.arg(store_id)::bigint::text, 0));

-- name: CountStoreTags :one
SELECT count(*) FROM crm.tags WHERE store_id = sqlc.arg(store_id);

-- name: InsertTag :execrows
-- 撞 UNIQUE (store_id, name) 回 0 列（名稱已被用）。
INSERT INTO crm.tags (id, store_id, name, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(name), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (store_id, name) DO NOTHING;

-- name: RenameTag :execrows
-- 撞名會是 UNIQUE 違反（呼叫者轉成 AlreadyExists，transaction 隨即結束）；0 列表示標籤不存在（或屬於別家店）。
UPDATE crm.tags SET name = sqlc.arg(name), updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND id = sqlc.arg(id);

-- name: DeleteTag :execrows
-- 關聯由 customer_tags 的 FK cascade 刪除。
DELETE FROM crm.tags WHERE store_id = sqlc.arg(store_id) AND id = sqlc.arg(id);

-- name: ListTags :many
SELECT id, name FROM crm.tags WHERE store_id = sqlc.arg(store_id) ORDER BY name, id;

-- name: ListTagIDsByNames :many
SELECT id, name FROM crm.tags WHERE store_id = sqlc.arg(store_id) AND name = ANY(sqlc.arg(names)::text[]);

-- name: LockCustomer :one
-- 鎖住客戶列（沒有列回 ErrNoRows）：同一位客戶的標籤上限檢查與插入串行，並讓只有客戶才能貼標在 FK 之前就得到 NotFound。
SELECT buyer_id FROM crm.customers WHERE store_id = sqlc.arg(store_id) AND buyer_id = sqlc.arg(buyer_id) FOR UPDATE;

-- name: CountCustomerTags :one
SELECT count(*) FROM crm.customer_tags WHERE store_id = sqlc.arg(store_id) AND buyer_id = sqlc.arg(buyer_id);

-- name: CountCustomerTagsAmong :one
SELECT count(*) FROM crm.customer_tags
WHERE store_id = sqlc.arg(store_id) AND buyer_id = sqlc.arg(buyer_id) AND tag_id = ANY(sqlc.arg(tag_ids)::uuid[]);

-- name: InsertCustomerTags :execrows
INSERT INTO crm.customer_tags (store_id, buyer_id, tag_id, created_at)
SELECT sqlc.arg(store_id), sqlc.arg(buyer_id), t, sqlc.arg(now) FROM unnest(sqlc.arg(tag_ids)::uuid[]) AS t
ON CONFLICT (store_id, buyer_id, tag_id) DO NOTHING;

-- name: DeleteCustomerTag :execrows
DELETE FROM crm.customer_tags WHERE store_id = sqlc.arg(store_id) AND buyer_id = sqlc.arg(buyer_id) AND tag_id = sqlc.arg(tag_id);

-- name: ListCustomerTags :many
-- 一頁客戶的標籤（名稱升序）。
SELECT ct.buyer_id, t.id, t.name
FROM crm.customer_tags AS ct JOIN crm.tags AS t ON t.store_id = ct.store_id AND t.id = ct.tag_id
WHERE ct.store_id = sqlc.arg(store_id) AND ct.buyer_id = ANY(sqlc.arg(buyer_ids)::bigint[])
ORDER BY ct.buyer_id, t.name, t.id;
