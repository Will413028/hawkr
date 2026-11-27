-- name: GetQuotaLimit :one
SELECT weekly_limit FROM imageai.quotas WHERE store_id = @store_id;

-- name: GetQuota :one
SELECT * FROM imageai.quotas WHERE store_id = @store_id;

-- name: LockQuota :one
SELECT * FROM imageai.quotas WHERE store_id = @store_id FOR UPDATE;

-- name: InsertQuota :one
INSERT INTO imageai.quotas (store_id, weekly_limit, memo, created_at, updated_at)
VALUES (@store_id, @weekly_limit, @memo, @now, @now)
ON CONFLICT (store_id) DO NOTHING
RETURNING *;

-- name: UpdateQuota :one
UPDATE imageai.quotas SET weekly_limit = @weekly_limit, memo = @memo, version = version + 1, updated_at = @now
WHERE store_id = @store_id AND version = @expected_version
RETURNING *;

-- name: DeleteQuota :execrows
DELETE FROM imageai.quotas WHERE store_id = @store_id;
