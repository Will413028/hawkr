-- name: GetQuotaLimit :one
SELECT weekly_limit FROM imageai.quotas WHERE store_id = @store_id;
