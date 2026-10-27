-- name: InsertSellerDeletionAudit :exec
INSERT INTO seller_deletion_audit (
  id, account_id, operator_id, active_order_count, reason, created_at
) VALUES ($1, $2, $3, $4, $5, $6);
