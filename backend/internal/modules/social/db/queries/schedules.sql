-- name: InsertSchedule :one
INSERT INTO social.post_schedules (id, post_id, store_id, publish_at, scheduled_by_kind, scheduled_by_ref, state, created_at)
VALUES (sqlc.arg(id), sqlc.arg(post_id), sqlc.arg(store_id), sqlc.arg(publish_at), sqlc.arg(scheduled_by_kind), sqlc.arg(scheduled_by_ref), 'PENDING', sqlc.arg(now))
RETURNING *;

-- name: GetSchedule :one
SELECT * FROM social.post_schedules WHERE id = sqlc.arg(id);

-- name: LockSchedule :one
SELECT * FROM social.post_schedules WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: GetPendingScheduleOfPost :one
SELECT * FROM social.post_schedules WHERE post_id = sqlc.arg(post_id) AND state = 'PENDING';

-- name: ListPendingSchedulesOfStore :many
SELECT * FROM social.post_schedules
WHERE store_id = sqlc.arg(store_id) AND state = 'PENDING'
  AND (publish_at, id) > (sqlc.arg(after_publish_at)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY publish_at, id
LIMIT sqlc.arg(max_rows);

-- name: ResolveSchedule :one
UPDATE social.post_schedules
SET state = sqlc.arg(state), cancel_reason = sqlc.narg(cancel_reason), resolved_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'PENDING'
RETURNING *;

-- name: LockPostsByID :many
SELECT id, store_id FROM social.posts WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR UPDATE;

-- name: CancelPendingSchedulesOfPosts :many
UPDATE social.post_schedules
SET state = 'CANCELED', cancel_reason = sqlc.arg(reason), resolved_at = sqlc.arg(now)
WHERE post_id = ANY(sqlc.arg(post_ids)::uuid[]) AND state = 'PENDING'
RETURNING *;

-- name: PendingPostIDsOfScheduler :many
SELECT DISTINCT post_id FROM social.post_schedules
WHERE scheduled_by_kind = sqlc.arg(kind) AND scheduled_by_ref = ANY(sqlc.arg(refs)::text[]) AND state = 'PENDING'
ORDER BY post_id;

-- name: CancelPendingSchedulesOfScheduler :many
UPDATE social.post_schedules
SET state = 'CANCELED', cancel_reason = sqlc.arg(reason), resolved_at = sqlc.arg(now)
WHERE post_id = ANY(sqlc.arg(post_ids)::uuid[]) AND scheduled_by_kind = sqlc.arg(kind) AND scheduled_by_ref = ANY(sqlc.arg(refs)::text[]) AND state = 'PENDING'
RETURNING *;

-- name: PendingPostIDsOfStoreByKind :many
SELECT DISTINCT post_id FROM social.post_schedules
WHERE store_id = sqlc.arg(store_id) AND scheduled_by_kind = sqlc.arg(kind) AND state = 'PENDING'
ORDER BY post_id;

-- name: CancelPendingSchedulesOfStoreByKind :many
UPDATE social.post_schedules
SET state = 'CANCELED', cancel_reason = sqlc.arg(reason), resolved_at = sqlc.arg(now)
WHERE post_id = ANY(sqlc.arg(post_ids)::uuid[]) AND store_id = sqlc.arg(store_id) AND scheduled_by_kind = sqlc.arg(kind) AND state = 'PENDING'
RETURNING *;

-- name: PendingSchedulerRefs :many
SELECT DISTINCT scheduled_by_ref FROM social.post_schedules
WHERE scheduled_by_kind = sqlc.arg(kind) AND state = 'PENDING'
ORDER BY scheduled_by_ref;

-- name: PostIsTakenDown :one
SELECT EXISTS (SELECT 1 FROM social.content_takedowns WHERE post_id = sqlc.arg(post_id));

-- name: CountPendingSchedulesOfStore :one
SELECT count(*)::bigint FROM social.post_schedules WHERE store_id = sqlc.arg(store_id) AND state = 'PENDING';
