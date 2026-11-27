
-- name: ListCustomersBySpentDesc :many
SELECT c.store_id, c.buyer_id, c.customer_type, c.total_spent, c.order_count, c.first_order_at
FROM crm.customers AS c
WHERE c.store_id = sqlc.arg(store_id)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id)
  AND (sqlc.narg(customer_type)::text IS NULL OR c.customer_type = sqlc.narg(customer_type)::text)
  AND (sqlc.narg(tag_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM crm.customer_tags AS ct
        WHERE ct.store_id = c.store_id AND ct.buyer_id = c.buyer_id AND ct.tag_id = sqlc.narg(tag_id)::uuid))
  AND (sqlc.narg(after_key)::bigint IS NULL OR (c.total_spent, c.buyer_id) < (sqlc.narg(after_key)::bigint, sqlc.narg(after_buyer)::bigint))
ORDER BY c.total_spent DESC, c.buyer_id DESC
LIMIT sqlc.arg(max_rows);

-- name: ListCustomersBySpentAsc :many
SELECT c.store_id, c.buyer_id, c.customer_type, c.total_spent, c.order_count, c.first_order_at
FROM crm.customers AS c
WHERE c.store_id = sqlc.arg(store_id)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id)
  AND (sqlc.narg(customer_type)::text IS NULL OR c.customer_type = sqlc.narg(customer_type)::text)
  AND (sqlc.narg(tag_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM crm.customer_tags AS ct
        WHERE ct.store_id = c.store_id AND ct.buyer_id = c.buyer_id AND ct.tag_id = sqlc.narg(tag_id)::uuid))
  AND (sqlc.narg(after_key)::bigint IS NULL OR (c.total_spent, c.buyer_id) > (sqlc.narg(after_key)::bigint, sqlc.narg(after_buyer)::bigint))
ORDER BY c.total_spent ASC, c.buyer_id ASC
LIMIT sqlc.arg(max_rows);

-- name: ListCustomersByOrdersDesc :many
SELECT c.store_id, c.buyer_id, c.customer_type, c.total_spent, c.order_count, c.first_order_at
FROM crm.customers AS c
WHERE c.store_id = sqlc.arg(store_id)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id)
  AND (sqlc.narg(customer_type)::text IS NULL OR c.customer_type = sqlc.narg(customer_type)::text)
  AND (sqlc.narg(tag_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM crm.customer_tags AS ct
        WHERE ct.store_id = c.store_id AND ct.buyer_id = c.buyer_id AND ct.tag_id = sqlc.narg(tag_id)::uuid))
  AND (sqlc.narg(after_key)::bigint IS NULL OR (c.order_count, c.buyer_id) < (sqlc.narg(after_key)::bigint, sqlc.narg(after_buyer)::bigint))
ORDER BY c.order_count DESC, c.buyer_id DESC
LIMIT sqlc.arg(max_rows);

-- name: ListCustomersByOrdersAsc :many
SELECT c.store_id, c.buyer_id, c.customer_type, c.total_spent, c.order_count, c.first_order_at
FROM crm.customers AS c
WHERE c.store_id = sqlc.arg(store_id)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id)
  AND (sqlc.narg(customer_type)::text IS NULL OR c.customer_type = sqlc.narg(customer_type)::text)
  AND (sqlc.narg(tag_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM crm.customer_tags AS ct
        WHERE ct.store_id = c.store_id AND ct.buyer_id = c.buyer_id AND ct.tag_id = sqlc.narg(tag_id)::uuid))
  AND (sqlc.narg(after_key)::bigint IS NULL OR (c.order_count, c.buyer_id) > (sqlc.narg(after_key)::bigint, sqlc.narg(after_buyer)::bigint))
ORDER BY c.order_count ASC, c.buyer_id ASC
LIMIT sqlc.arg(max_rows);

-- name: ListCustomersByFirstOrderDesc :many
SELECT c.store_id, c.buyer_id, c.customer_type, c.total_spent, c.order_count, c.first_order_at
FROM crm.customers AS c
WHERE c.store_id = sqlc.arg(store_id)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id)
  AND (sqlc.narg(customer_type)::text IS NULL OR c.customer_type = sqlc.narg(customer_type)::text)
  AND (sqlc.narg(tag_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM crm.customer_tags AS ct
        WHERE ct.store_id = c.store_id AND ct.buyer_id = c.buyer_id AND ct.tag_id = sqlc.narg(tag_id)::uuid))
  AND (sqlc.narg(after_key)::timestamptz IS NULL OR (c.first_order_at, c.buyer_id) < (sqlc.narg(after_key)::timestamptz, sqlc.narg(after_buyer)::bigint))
ORDER BY c.first_order_at DESC, c.buyer_id DESC
LIMIT sqlc.arg(max_rows);

-- name: ListCustomersByFirstOrderAsc :many
SELECT c.store_id, c.buyer_id, c.customer_type, c.total_spent, c.order_count, c.first_order_at
FROM crm.customers AS c
WHERE c.store_id = sqlc.arg(store_id)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id)
  AND (sqlc.narg(customer_type)::text IS NULL OR c.customer_type = sqlc.narg(customer_type)::text)
  AND (sqlc.narg(tag_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM crm.customer_tags AS ct
        WHERE ct.store_id = c.store_id AND ct.buyer_id = c.buyer_id AND ct.tag_id = sqlc.narg(tag_id)::uuid))
  AND (sqlc.narg(after_key)::timestamptz IS NULL OR (c.first_order_at, c.buyer_id) > (sqlc.narg(after_key)::timestamptz, sqlc.narg(after_buyer)::bigint))
ORDER BY c.first_order_at ASC, c.buyer_id ASC
LIMIT sqlc.arg(max_rows);

-- name: GetCustomer :one
SELECT c.store_id, c.buyer_id, c.customer_type, c.total_spent, c.order_count, c.first_order_at
FROM crm.customers AS c
WHERE c.store_id = sqlc.arg(store_id) AND c.buyer_id = sqlc.arg(buyer_id)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id);

-- name: SetCustomerType :one
UPDATE crm.customers AS c SET customer_type = sqlc.arg(customer_type), updated_at = sqlc.arg(now)
WHERE c.store_id = sqlc.arg(store_id) AND c.buyer_id = sqlc.arg(buyer_id)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id)
RETURNING c.store_id, c.buyer_id, c.customer_type, c.total_spent, c.order_count, c.first_order_at;

-- name: ListVIPStores :many
SELECT c.store_id FROM crm.customers AS c
WHERE c.buyer_id = sqlc.arg(buyer_id) AND c.store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND c.customer_type = 'VIP'
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id);

-- name: ListCustomerBuyerIDs :many
SELECT c.buyer_id FROM crm.customers AS c
WHERE c.store_id = sqlc.arg(store_id) AND c.buyer_id = ANY(sqlc.arg(buyer_ids)::bigint[])
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id);
