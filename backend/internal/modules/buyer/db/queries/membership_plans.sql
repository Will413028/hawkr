-- name: Plans :many
-- 列出全部會員方案。
SELECT *
FROM buyer.membership_plans
ORDER BY plan_code;

-- name: Plan :one
-- 以自然鍵查詢會員方案。
SELECT *
FROM buyer.membership_plans
WHERE plan_code = $1;

-- name: UpdatePlan :one
-- 只修改有提供的方案欄位。
UPDATE buyer.membership_plans
SET monthly_answer_limit = coalesce(sqlc.narg(answer_limit)::integer, monthly_answer_limit),
    monthly_price_twd = coalesce(sqlc.narg(price)::bigint, monthly_price_twd),
    is_purchasable = coalesce(sqlc.narg(purchasable)::boolean, is_purchasable),
    updated_at = @now_at,
    version = version + 1
WHERE plan_code = @plan_code AND version = @expected_version
RETURNING *;
