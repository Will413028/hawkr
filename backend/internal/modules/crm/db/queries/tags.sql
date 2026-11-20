-- 標籤與客戶的標籤。

-- name: LockStoreTags :exec
-- 同一家店的標籤變更（CreateTag、AddCustomerTags、RenameTag、DeleteTag）串行：每店標籤數上限的檢查與插入之間不能有人插入，
-- AddCustomerTags 以名稱查到的標籤 ID 在它寫入客戶之前也不能被改名或刪除。選 advisory lock 而不是對查到的列 FOR SHARE：
-- AddCustomerTags 要先判斷「名稱不存在」再建立，列鎖鎖不住不存在的名稱，仍需要這把店家層級的鎖。crm 自己的命名空間，不進鎖序。
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
-- 已退場的店家沒有標籤（可見性在清除 job 跑之前就已經改好）。
SELECT t.id, t.name FROM crm.tags AS t
WHERE t.store_id = sqlc.arg(store_id)
  AND NOT EXISTS (SELECT 1 FROM crm.inactive_parties AS ip WHERE ip.kind = 'STORE' AND ip.party_id = t.store_id)
ORDER BY t.name, t.id;

-- name: IsStoreInactive :one
SELECT EXISTS (SELECT 1 FROM crm.inactive_parties WHERE kind = 'STORE' AND party_id = sqlc.arg(store_id));

-- name: ListTagIDsByNames :many
SELECT id, name FROM crm.tags WHERE store_id = sqlc.arg(store_id) AND name = ANY(sqlc.arg(names)::text[]);

-- name: LockCustomer :one
-- 鎖住客戶列（沒有列回 ErrNoRows）：同一位客戶的標籤上限檢查與插入串行，並讓只有客戶才能貼標在 FK 之前就得到 NotFound。
-- inactive 的買家或店家也回 ErrNoRows。
SELECT c.buyer_id FROM crm.customers AS c
WHERE c.store_id = sqlc.arg(store_id) AND c.buyer_id = sqlc.arg(buyer_id)
  AND NOT EXISTS (
        SELECT 1 FROM crm.inactive_parties AS ip
        WHERE (ip.kind = 'STORE' AND ip.party_id = c.store_id) OR (ip.kind = 'BUYER' AND ip.party_id = c.buyer_id))
FOR UPDATE OF c;

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
