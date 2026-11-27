-- name: SellerListEarnings :many
SELECT e.id, e.kind, e.order_no, e.amount, e.status, e.created_at
FROM (
    SELECT t.id, 'TRANSFER'::text AS kind, o.order_no, t.amount::bigint AS amount, t.status::text AS status, t.created_at
    FROM payment.transfers t
    JOIN payment.payment_orders o ON o.id = t.payment_order_id
    WHERE o.store_id = sqlc.arg(store_id)::bigint
    UNION ALL
    SELECT v.id, 'REVERSAL'::text, o.order_no, (-v.amount)::bigint, v.status::text, v.created_at
    FROM payment.refund_reversals v
    JOIN payment.payment_orders o ON o.id = v.payment_order_id
    WHERE o.store_id = sqlc.arg(store_id)::bigint AND v.status <> 'RELEASED'
    UNION ALL
    SELECT r.id, 'TOP_UP'::text, o.order_no, r.amount::bigint, r.topup_status::text, r.created_at
    FROM payment.refunds r
    JOIN payment.payment_orders o ON o.id = r.payment_order_id
    WHERE o.store_id = sqlc.arg(store_id)::bigint AND r.topup_status IN ('WAITING', 'SENDING', 'DONE', 'ANOMALY')
) e
WHERE (sqlc.arg(order_no)::text = '' OR e.order_no = sqlc.arg(order_no)::text)
  AND (sqlc.narg(from_time)::timestamptz IS NULL OR e.created_at >= sqlc.narg(from_time)::timestamptz)
  AND (sqlc.narg(to_time)::timestamptz IS NULL OR e.created_at < sqlc.narg(to_time)::timestamptz)
  AND e.id < sqlc.arg(after_id)::uuid
ORDER BY e.id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: SellerHasOrderPayment :one
SELECT EXISTS (
    SELECT 1 FROM payment.payment_orders WHERE store_id = sqlc.arg(store_id)::bigint AND order_no = sqlc.arg(order_no)::text
) AS found;
