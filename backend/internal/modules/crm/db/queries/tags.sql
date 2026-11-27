
-- name: LockStoreTags :exec
SELECT pg_advisory_xact_lock(hashtextextended('crm.tags:' || sqlc.arg(store_id)::bigint::text, 0));

-- name: CountStoreTags :one
SELECT count(*) FROM crm.tags WHERE store_id = sqlc.arg(store_id);

-- name: InsertTag :execrows
INSERT INTO crm.tags (id, store_id, name, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(name), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (store_id, name) DO NOTHING;

-- name: RenameTag :one
UPDATE crm.tags SET name = sqlc.arg(name), updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND id = sqlc.arg(id)
RETURNING id, name;

-- name: DeleteTag :execrows
DELETE FROM crm.tags WHERE store_id = sqlc.arg(store_id) AND id = sqlc.arg(id);

-- name: ListTags :many
SELECT t.id, t.name FROM crm.tags AS t
WHERE t.store_id = sqlc.arg(store_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = t.store_id)
ORDER BY t.name, t.id;

-- name: ListTagIDsByNames :many
SELECT id, name FROM crm.tags WHERE store_id = sqlc.arg(store_id) AND name = ANY(sqlc.arg(names)::text[]);

-- name: LockCustomer :one
SELECT c.buyer_id FROM crm.customers AS c
WHERE c.store_id = sqlc.arg(store_id) AND c.buyer_id = sqlc.arg(buyer_id)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id)
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
SELECT ct.buyer_id, t.id, t.name
FROM crm.customer_tags AS ct JOIN crm.tags AS t ON t.store_id = ct.store_id AND t.id = ct.tag_id
WHERE ct.store_id = sqlc.arg(store_id) AND ct.buyer_id = ANY(sqlc.arg(buyer_ids)::bigint[])
ORDER BY ct.buyer_id, t.name, t.id;
