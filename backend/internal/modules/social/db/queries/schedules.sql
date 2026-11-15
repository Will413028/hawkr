-- name: InsertSchedule :one
-- 新排程（PENDING）。同一篇貼文已有 PENDING 時撞 post_schedules_post_pending；呼叫方已鎖貼文列並先把舊的轉 REPLACED。
INSERT INTO social.post_schedules (id, post_id, store_id, publish_at, scheduled_by_kind, scheduled_by_ref, state, created_at)
VALUES (sqlc.arg(id), sqlc.arg(post_id), sqlc.arg(store_id), sqlc.arg(publish_at), sqlc.arg(scheduled_by_kind), sqlc.arg(scheduled_by_ref), 'PENDING', sqlc.arg(now))
RETURNING *;

-- name: GetSchedule :one
-- 不鎖的讀取：worker 用它取得 post_id，再依「貼文列 → 排程列」的順序加鎖。
SELECT * FROM social.post_schedules WHERE id = sqlc.arg(id);

-- name: LockSchedule :one
SELECT * FROM social.post_schedules WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: GetPendingScheduleOfPost :one
SELECT * FROM social.post_schedules WHERE post_id = sqlc.arg(post_id) AND state = 'PENDING';

-- name: ListPendingSchedulesOfStore :many
-- 店家待發布的排程：發布時間早的在前。keyset：(publish_at, id) 由小到大。
SELECT * FROM social.post_schedules
WHERE store_id = sqlc.arg(store_id) AND state = 'PENDING'
  AND (publish_at, id) > (sqlc.arg(after_publish_at)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY publish_at, id
LIMIT sqlc.arg(max_rows);

-- name: ResolveSchedule :one
-- 把 PENDING 的排程轉成終態（PUBLISHED 或 CANCELED）；已經不是 PENDING 回 0 列。
UPDATE social.post_schedules
SET state = sqlc.arg(state), cancel_reason = sqlc.narg(cancel_reason), resolved_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'PENDING'
RETURNING *;

-- name: LockPostsByID :many
-- 依 ID 由小到大鎖貼文列（含已軟刪的）：多篇貼文的取消照這個順序加鎖。
SELECT id, store_id FROM social.posts WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR UPDATE;

-- name: CancelPendingSchedulesOfPosts :many
-- 呼叫方已鎖這些貼文列。回傳被取消的排程。
UPDATE social.post_schedules
SET state = 'CANCELED', cancel_reason = sqlc.arg(reason), resolved_at = sqlc.arg(now)
WHERE post_id = ANY(sqlc.arg(post_ids)::uuid[]) AND state = 'PENDING'
RETURNING *;

-- name: PendingPostIDsOfScheduler :many
-- 不鎖的讀取：某個發起者（種類加一批 ref）待發布的排程所在的貼文，ID 由小到大。
SELECT DISTINCT post_id FROM social.post_schedules
WHERE scheduled_by_kind = sqlc.arg(kind) AND scheduled_by_ref = ANY(sqlc.arg(refs)::text[]) AND state = 'PENDING'
ORDER BY post_id;

-- name: CancelPendingSchedulesOfScheduler :many
-- 呼叫方已鎖 post_ids 的貼文列；只取消仍是 PENDING 而且屬於該發起者的排程。
UPDATE social.post_schedules
SET state = 'CANCELED', cancel_reason = sqlc.arg(reason), resolved_at = sqlc.arg(now)
WHERE post_id = ANY(sqlc.arg(post_ids)::uuid[]) AND scheduled_by_kind = sqlc.arg(kind) AND scheduled_by_ref = ANY(sqlc.arg(refs)::text[]) AND state = 'PENDING'
RETURNING *;

-- name: PendingPostIDsOfStoreByKind :many
-- 不鎖的讀取：某家店某種發起者待發布的排程所在的貼文，ID 由小到大。
SELECT DISTINCT post_id FROM social.post_schedules
WHERE store_id = sqlc.arg(store_id) AND scheduled_by_kind = sqlc.arg(kind) AND state = 'PENDING'
ORDER BY post_id;

-- name: CancelPendingSchedulesOfStoreByKind :many
-- 呼叫方已鎖 post_ids 的貼文列。
UPDATE social.post_schedules
SET state = 'CANCELED', cancel_reason = sqlc.arg(reason), resolved_at = sqlc.arg(now)
WHERE post_id = ANY(sqlc.arg(post_ids)::uuid[]) AND store_id = sqlc.arg(store_id) AND scheduled_by_kind = sqlc.arg(kind) AND state = 'PENDING'
RETURNING *;

-- name: PendingSchedulerRefs :many
-- 某種發起者目前有待發布排程的 ref（去重）：筆數以待發布的排程為界，不隨發起者的歷史增長。
SELECT DISTINCT scheduled_by_ref FROM social.post_schedules
WHERE scheduled_by_kind = sqlc.arg(kind) AND state = 'PENDING'
ORDER BY scheduled_by_ref;

-- name: PostIsTakenDown :one
-- 貼文有沒有被內容下架措施下架（複本 content_takedowns）。
SELECT EXISTS (SELECT 1 FROM social.content_takedowns WHERE post_id = sqlc.arg(post_id));
