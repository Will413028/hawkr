-- name: InsertSellerSuspension :one
INSERT INTO seller_suspensions (
  id, seller_id, reason_code, reason_detail, status,
  suspended_by_admin_id, suspended_at, created_at, updated_at
) VALUES (
  $1, $2, $3, $4, 'active', $5, $6, $7, $7
) RETURNING *;

-- name: GetActiveSellerSuspensionByAccountID :one
SELECT * FROM seller_suspensions
WHERE seller_id = $1 AND status = 'active';

-- name: HasActiveSellerSuspensionByAccountID :one
SELECT EXISTS(
  SELECT 1 FROM seller_suspensions WHERE seller_id = $1 AND status = 'active'
);

-- name: GetSellerSuspensionByID :one
SELECT * FROM seller_suspensions WHERE id = $1;

-- name: LiftSellerSuspension :one
-- only-if condition status='active' — 重複 lift 呼叫 RowsAffected=0（pgx.ErrNoRows），
-- repository 層轉成 nil,nil 讓 usecase 走冪等「已解除」路徑。
UPDATE seller_suspensions SET
  status              = 'lifted',
  lifted_by_admin_id  = $2,
  lifted_at           = $3,
  updated_at          = $3
WHERE id = $1
  AND status = 'active'
RETURNING *;

-- name: ListSellerSuspensionsByAccountID :many
SELECT * FROM seller_suspensions
WHERE seller_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
