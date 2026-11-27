-- name: AdminListPayments :many
SELECT *
FROM payment.payments
WHERE (sqlc.arg(status)::text = '' OR status = sqlc.arg(status)::text)
  AND (sqlc.arg(order_no)::text = '' OR EXISTS (SELECT 1 FROM payment.payment_orders o WHERE o.payment_id = payments.id AND o.order_no = sqlc.arg(order_no)::text))
  AND id < sqlc.arg(after_id)::uuid
ORDER BY id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: AdminPaymentOrders :many
SELECT id, payment_id, order_no, store_id, amount
FROM payment.payment_orders
WHERE payment_id = ANY(sqlc.arg(payment_ids)::uuid[])
ORDER BY payment_id, id;

-- name: AdminListTransfers :many
SELECT t.id, o.payment_id, t.payment_order_id, o.order_no, t.store_id, t.amount, t.reversed_amount, t.status, t.external_transfer_id, t.failure_reason, t.created_at
FROM payment.transfers t
JOIN payment.payment_orders o ON o.id = t.payment_order_id
WHERE (sqlc.arg(status)::text = '' OR t.status = sqlc.arg(status)::text)
  AND (sqlc.arg(order_no)::text = '' OR o.order_no = sqlc.arg(order_no)::text)
  AND t.id < sqlc.arg(after_id)::uuid
ORDER BY t.id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: AdminListRefunds :many
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
SELECT id, refund_id, seq, amount, status, reason, external_id, created_at
FROM payment.refund_reversals
WHERE refund_id = ANY(sqlc.arg(refund_ids)::uuid[])
ORDER BY refund_id, seq;
