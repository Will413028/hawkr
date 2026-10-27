-- name: GetAiQuotaByStoreID :one
SELECT * FROM ai_quotas WHERE store_id = $1;

-- name: GetAiUsageByStoreIDAndWeek :one
SELECT * FROM ai_usage WHERE store_id = $1 AND week_start = $2;

-- name: UpsertAiQuota :exec
INSERT INTO ai_quotas (store_id, weekly_limit, memo)
VALUES ($1, $2, $3)
ON CONFLICT (store_id) DO UPDATE SET
  weekly_limit = EXCLUDED.weekly_limit,
  memo = EXCLUDED.memo,
  updated_at = NOW();

-- name: DeleteAiQuota :exec
DELETE FROM ai_quotas WHERE store_id = $1;
