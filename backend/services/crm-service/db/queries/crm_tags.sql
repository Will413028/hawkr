-- name: GetTag :one
SELECT * FROM crm_tags
WHERE id = $1
LIMIT 1;

-- name: GetTagByName :one
SELECT * FROM crm_tags
WHERE store_id = $1 AND name = $2
LIMIT 1;

-- name: ListTags :many
SELECT * FROM crm_tags
WHERE store_id = $1
ORDER BY created_at ASC;

-- name: CreateTag :one
INSERT INTO crm_tags (id, store_id, name)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateTag :exec
UPDATE crm_tags
SET name = $2, updated_at = NOW()
WHERE id = $1 AND store_id = $3;

-- name: DeleteTag :exec
DELETE FROM crm_tags
WHERE id = $1 AND store_id = $2;

-- name: GetCustomerTags :many
SELECT t.* FROM crm_tags t
INNER JOIN crm_customer_tags ct ON ct.tag_id = t.id
WHERE ct.store_id = $1 AND ct.buyer_id = $2
ORDER BY t.created_at ASC;

-- name: AddCustomerTag :exec
INSERT INTO crm_customer_tags (store_id, buyer_id, tag_id)
VALUES ($1, $2, $3)
ON CONFLICT (store_id, buyer_id, tag_id) DO NOTHING;

-- name: RemoveCustomerTag :exec
DELETE FROM crm_customer_tags
WHERE store_id = $1 AND buyer_id = $2 AND tag_id = $3;

-- name: CountCustomersByTag :one
SELECT COUNT(*) FROM crm_customer_tags
WHERE tag_id = $1;
