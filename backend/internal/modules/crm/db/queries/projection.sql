-- 消費統計的投影（過渡，ordering 搬進單體後退場）。一個事件的 transaction：LockOrderNet → InsertLedger → SetOrderNet → 客戶的增量（design §3.3）。

-- name: LockOrderNet :one
-- 取得並鎖住這一單的列（不存在時建立，completed=false、full_refunded=false、net_amount=0）：同單事件在這把列鎖上序列化。DO UPDATE 把 updated_at
-- 設成自己，只為了鎖住既有的列並讓 RETURNING 回傳它。
INSERT INTO crm.order_nets (order_no, store_id, buyer_id, completed, full_refunded, net_amount, updated_at)
VALUES (sqlc.arg(order_no), sqlc.arg(store_id), sqlc.arg(buyer_id), false, false, 0, sqlc.arg(now))
ON CONFLICT (order_no) DO UPDATE SET updated_at = crm.order_nets.updated_at
RETURNING order_no, store_id, buyer_id, completed, full_refunded, net_amount;

-- name: InsertLedger :execrows
-- 冪等的唯一依據是 dedup_key 的 UNIQUE：0 列表示重放。
INSERT INTO crm.ledger (dedup_key, order_no)
VALUES (sqlc.arg(dedup_key), sqlc.arg(order_no))
ON CONFLICT (dedup_key) DO NOTHING;

-- name: SetOrderNet :exec
UPDATE crm.order_nets
SET completed = sqlc.arg(completed), full_refunded = sqlc.arg(full_refunded), net_amount = sqlc.arg(net_amount), updated_at = sqlc.arg(now)
WHERE order_no = sqlc.arg(order_no);

-- name: UpsertCustomerOnOrder :exec
-- 首筆完成訂單建立客戶；之後的訂單累加。total_spent 只加增量（delta 在鎖住的 order_nets 列上算出），不以 SUM 重算：
-- READ COMMITTED 下 ON CONFLICT DO UPDATE 讀到最新的列值，同一客戶的事件在 customers 的列鎖上序列化。
INSERT INTO crm.customers (store_id, buyer_id, customer_type, total_spent, order_count, first_order_at, updated_at)
VALUES (sqlc.arg(store_id), sqlc.arg(buyer_id), 'NORMAL', sqlc.arg(delta), 1, sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (store_id, buyer_id) DO UPDATE
SET total_spent = crm.customers.total_spent + EXCLUDED.total_spent,
    order_count = crm.customers.order_count + 1,
    updated_at = EXCLUDED.updated_at;

-- name: AddCustomerSpent :execrows
-- 退款只動既有的客戶列，不建立（Q11）；0 列表示客戶還不存在（訂單未完成，增量必為 0）。
UPDATE crm.customers
SET total_spent = total_spent + sqlc.arg(delta), updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND buyer_id = sqlc.arg(buyer_id);

-- name: ClosedBuyerIDs :many
-- 守門回 NotFound 之後才呼叫（只在略過路徑）：回傳這個買家的列，有列表示已關閉，沒有表示從未存在。
SELECT cb.buyer_id FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = $1::bigint;

-- name: RetiredStoreIDs :many
-- 守門回 NotFound 之後才呼叫（只在略過路徑）：回傳這個店家的列，有列表示已退場，沒有表示從未存在。
SELECT rs.store_id FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = $1::bigint;
