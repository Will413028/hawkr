-- name: GetSellerMembershipPlanByCode :one
SELECT * FROM seller_membership_plans WHERE plan_code = $1;

-- name: ListSellerMembershipPlans :many
SELECT * FROM seller_membership_plans ORDER BY tier_rank ASC;

-- name: UpdateSellerMembershipPlan :one
-- tier_rank 刻意不在 SET 子句：這輪不開放編輯（決定 AI 排序權重，風險層級不同）。
UPDATE seller_membership_plans
SET
  monthly_order_cap = COALESCE(sqlc.narg(monthly_order_cap), monthly_order_cap),
  monthly_price_twd = COALESCE(sqlc.narg(monthly_price_twd), monthly_price_twd),
  is_purchasable = COALESCE(sqlc.narg(is_purchasable), is_purchasable),
  updated_at = now()
WHERE plan_code = $1
RETURNING *;

-- name: GetActiveSellerMembershipSubscription :one
SELECT * FROM seller_membership_subscriptions WHERE store_id = $1 AND status = 'active';

-- name: FindSellerMembershipSubscriptionByRequestID :one
SELECT * FROM seller_membership_subscriptions WHERE request_id = $1 AND request_id != '';

-- name: SupersedeActiveSellerMembershipSubscription :exec
UPDATE seller_membership_subscriptions SET status = 'superseded' WHERE store_id = $1 AND status = 'active';

-- name: CreateSellerMembershipSubscription :one
INSERT INTO seller_membership_subscriptions (id, store_id, plan_code, effective_at, source, granted_by, reason, request_id, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active', now())
RETURNING *;
