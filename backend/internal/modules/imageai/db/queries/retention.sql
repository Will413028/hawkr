-- name: DeleteStoreUsage :execrows
DELETE FROM imageai.usage WHERE store_id = @store_id;

-- name: DeleteStoreQuota :execrows
DELETE FROM imageai.quotas WHERE store_id = @store_id;
