-- 客戶列表：三個排序欄位 × 兩個方向，各自一句（索引 customers_by_spent、customers_by_orders、customers_by_first_order 正反向都能掃）。

-- name: ListCustomersBySpentDesc :many
-- keyset：(total_spent, buyer_id) < 游標，排序鍵加 buyer_id 次序鍵讓同值的列順序確定（CD9）；索引方向與排序相同。type、tag_id 為空表示不篩。
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
-- keyset：(total_spent, buyer_id) > 游標，排序鍵加 buyer_id 次序鍵讓同值的列順序確定（CD9）；索引方向與排序相同。type、tag_id 為空表示不篩。
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
-- keyset：(order_count, buyer_id) < 游標，排序鍵加 buyer_id 次序鍵讓同值的列順序確定（CD9）；索引方向與排序相同。type、tag_id 為空表示不篩。
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
-- keyset：(order_count, buyer_id) > 游標，排序鍵加 buyer_id 次序鍵讓同值的列順序確定（CD9）；索引方向與排序相同。type、tag_id 為空表示不篩。
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
-- keyset：(first_order_at, buyer_id) < 游標，排序鍵加 buyer_id 次序鍵讓同值的列順序確定（CD9）；索引方向與排序相同。type、tag_id 為空表示不篩。
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
-- keyset：(first_order_at, buyer_id) > 游標，排序鍵加 buyer_id 次序鍵讓同值的列順序確定（CD9）；索引方向與排序相同。type、tag_id 為空表示不篩。
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
-- 已關閉的買家與已退場的店家（buyer.pub_closed_buyers()、seller.pub_retired_stores()）視為不存在：可見性在清除 job 跑之前就已經改好。
SELECT c.store_id, c.buyer_id, c.customer_type, c.total_spent, c.order_count, c.first_order_at
FROM crm.customers AS c
WHERE c.store_id = sqlc.arg(store_id) AND c.buyer_id = sqlc.arg(buyer_id)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id);

-- name: SetCustomerType :one
-- inactive 的買家或店家沒有列（ErrNoRows，呼叫者回 NotFound）；回傳寫入當下的列。
UPDATE crm.customers AS c SET customer_type = sqlc.arg(customer_type), updated_at = sqlc.arg(now)
WHERE c.store_id = sqlc.arg(store_id) AND c.buyer_id = sqlc.arg(buyer_id)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id)
RETURNING c.store_id, c.buyer_id, c.customer_type, c.total_spent, c.order_count, c.first_order_at;

-- name: ListVIPStores :many
-- 買家在這些店裡是 vip 的店（主鍵點查；沒有客戶列的店不回）。
-- inactive 的買家或店家不回。
SELECT c.store_id FROM crm.customers AS c
WHERE c.buyer_id = sqlc.arg(buyer_id) AND c.store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND c.customer_type = 'VIP'
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id);

-- name: ListCustomerBuyerIDs :many
-- 這些買家裡，是這家店客戶的買家（主鍵點查）。inactive 的買家或店家不回。
SELECT c.buyer_id FROM crm.customers AS c
WHERE c.store_id = sqlc.arg(store_id) AND c.buyer_id = ANY(sqlc.arg(buyer_ids)::bigint[])
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id);
