-- name: SellerListEarnings :many
-- 賣家入帳帳本，新的在前（keyset：id < after_id；各列的 ID 都是 UUIDv7，依 ID 由大到小就是記錄時間由新到舊）。店家以訂單應收的 store_id 判定，
-- 三種列都經訂單應收：轉帳（金額為正）、收回（撤回列，金額為負，RELEASED 是沒有收回，不列）、補差額（退款列的 topup_*，金額為退款額，
-- NONE 與 RELEASED 沒有補，不列）。order_no 空字串不篩，from_time 含、to_time 不含，NULL 不限。
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
-- 這個店家有沒有這張訂單的訂單應收（有付款才有；別家店的訂單與不存在的訂單都是 false）。
SELECT EXISTS (
    SELECT 1 FROM payment.payment_orders WHERE store_id = sqlc.arg(store_id)::bigint AND order_no = sqlc.arg(order_no)::text
) AS found;
