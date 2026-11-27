
-- name: LockOrderNet :one
INSERT INTO crm.order_nets (order_no, store_id, buyer_id, completed, full_refunded, net_amount, updated_at)
VALUES (sqlc.arg(order_no), sqlc.arg(store_id), sqlc.arg(buyer_id), false, false, 0, sqlc.arg(now))
ON CONFLICT (order_no) DO UPDATE SET updated_at = crm.order_nets.updated_at
RETURNING order_no, store_id, buyer_id, completed, full_refunded, net_amount;

-- name: InsertLedger :execrows
INSERT INTO crm.ledger (dedup_key, order_no)
VALUES (sqlc.arg(dedup_key), sqlc.arg(order_no))
ON CONFLICT (dedup_key) DO NOTHING;

-- name: SetOrderNet :exec
UPDATE crm.order_nets
SET completed = sqlc.arg(completed), full_refunded = sqlc.arg(full_refunded), net_amount = sqlc.arg(net_amount), updated_at = sqlc.arg(now)
WHERE order_no = sqlc.arg(order_no);

-- name: UpsertCustomerOnOrder :exec
INSERT INTO crm.customers (store_id, buyer_id, customer_type, total_spent, order_count, first_order_at, updated_at)
VALUES (sqlc.arg(store_id), sqlc.arg(buyer_id), 'NORMAL', sqlc.arg(delta), 1, sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (store_id, buyer_id) DO UPDATE
SET total_spent = crm.customers.total_spent + EXCLUDED.total_spent,
    order_count = crm.customers.order_count + 1,
    updated_at = EXCLUDED.updated_at;

-- name: AddCustomerSpent :execrows
UPDATE crm.customers
SET total_spent = total_spent + sqlc.arg(delta), updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND buyer_id = sqlc.arg(buyer_id);

-- name: BuyerClosed :one
SELECT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = sqlc.arg(buyer_id)::bigint)::boolean AS closed;

-- name: StoreRetired :one
SELECT EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = sqlc.arg(store_id)::bigint)::boolean AS retired;
