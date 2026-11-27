
-- name: InsertCancelledOrder :execrows
INSERT INTO invoice_cancelled_orders (order_no, buyer_id, cancel_reason, occurred_at, created_at)
VALUES (sqlc.arg(order_no), sqlc.arg(buyer_id), sqlc.narg(cancel_reason), sqlc.arg(occurred_at), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: IsOrderCancelledForBuyer :one
SELECT EXISTS (
  SELECT 1 FROM invoice_cancelled_orders
  WHERE order_no = sqlc.arg(order_no) AND buyer_id = sqlc.arg(buyer_id)
);
