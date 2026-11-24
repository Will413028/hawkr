-- name: AdminListPayments :many
-- 後台列付款，新的在前（主鍵是 UUIDv7，依 id 由大到小就是建立時間由新到舊；keyset：id < after_id）；status、order_no 空字串不篩。
SELECT *
FROM payment.payments
WHERE (sqlc.arg(status)::text = '' OR status = sqlc.arg(status)::text)
  AND (sqlc.arg(order_no)::text = '' OR EXISTS (SELECT 1 FROM payment.payment_orders o WHERE o.payment_id = payments.id AND o.order_no = sqlc.arg(order_no)::text))
  AND id < sqlc.arg(after_id)::uuid
ORDER BY id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: AdminPaymentOrders :many
-- 一頁付款的全部訂單應收（後台付款列表一筆付款對 N 張訂單），依付款再依 ID 升序。
SELECT id, payment_id, order_no, store_id, amount
FROM payment.payment_orders
WHERE payment_id = ANY(sqlc.arg(payment_ids)::uuid[])
ORDER BY payment_id, id;

-- name: AdminListTransfers :many
-- 後台列轉帳，新的在前（keyset 同 AdminListPayments）；order_no 是所屬訂單應收的訂單編號，status、order_no 空字串不篩。
SELECT t.id, o.payment_id, t.payment_order_id, o.order_no, t.store_id, t.amount, t.reversed_amount, t.status, t.external_transfer_id, t.failure_reason, t.created_at
FROM payment.transfers t
JOIN payment.payment_orders o ON o.id = t.payment_order_id
WHERE (sqlc.arg(status)::text = '' OR t.status = sqlc.arg(status)::text)
  AND (sqlc.arg(order_no)::text = '' OR o.order_no = sqlc.arg(order_no)::text)
  AND t.id < sqlc.arg(after_id)::uuid
ORDER BY t.id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: AdminListRefunds :many
-- 後台列退款，新的在前（keyset 同 AdminListPayments）；order_no 是所屬訂單應收的訂單編號；status 篩退款本身的狀態，
-- anomaly_only 為真時只列退款、收回或補差額任一個在 ANOMALY 的退款（收回與補差額的人工結案要從這裡找到列）。
SELECT r.id, o.payment_id, r.payment_order_id, o.order_no, r.request_key, r.amount, r.auto, r.status, r.reversal_status, r.reversal_amount, r.topup_status,
       r.external_refund_id, r.failure_reason, r.created_at
FROM payment.refunds r
JOIN payment.payment_orders o ON o.id = r.payment_order_id
WHERE (sqlc.arg(status)::text = '' OR r.status = sqlc.arg(status)::text)
  AND (sqlc.arg(order_no)::text = '' OR o.order_no = sqlc.arg(order_no)::text)
  AND (NOT sqlc.arg(anomaly_only)::boolean OR 'ANOMALY' IN (r.status, r.reversal_status, r.topup_status))
  AND r.id < sqlc.arg(after_id)::uuid
ORDER BY r.id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: AdminRefundReversals :many
-- 一頁退款的全部撤回列（後台人工結案撤回要用撤回列 ID）。
SELECT id, refund_id, seq, amount, status, reason, external_id, created_at
FROM payment.refund_reversals
WHERE refund_id = ANY(sqlc.arg(refund_ids)::uuid[])
ORDER BY refund_id, seq;
