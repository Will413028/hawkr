-- name: InsertAnnouncement :one
-- 建立公告。(admin_id, request_id) 已存在時什麼都不寫（回 0 列，pgx.ErrNoRows），呼叫端讀回第一次的那一列並比對內容；沒有 request_id（NULL）不衝突。
INSERT INTO notification.announcements (id, audiences, title, body, admin_id, request_id, created_at)
VALUES (sqlc.arg(id), sqlc.arg(audiences)::text[], sqlc.arg(title), sqlc.arg(body), sqlc.arg(admin_id), sqlc.narg(request_id), sqlc.arg(created_at))
ON CONFLICT ON CONSTRAINT announcements_request_key DO NOTHING
RETURNING id;

-- name: AnnouncementByRequest :one
SELECT * FROM notification.announcements WHERE admin_id = sqlc.arg(admin_id) AND request_id = sqlc.arg(request_id);

-- name: AnnouncementByID :one
SELECT * FROM notification.announcements WHERE id = sqlc.arg(id);

-- name: ListAnnouncements :many
-- 新到舊（created_at，再依 id），keyset 分頁；after_created_at 為 NULL 是第一頁。呼叫端要 limit + 1 筆來判斷有沒有下一頁。
SELECT * FROM notification.announcements
WHERE sqlc.narg(after_created_at)::timestamptz IS NULL OR (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: InsertAnnouncementInbox :execrows
-- 把公告寫進一個收件人的收件匣；同一個 (公告, 收件人) 已有列時什麼都不寫（回 0 列，job 重試安全）。參數只有公告 ID，文案讀取時由公告型別解析。
INSERT INTO notification.inbox_items (id, account_id, audience, store_id, type, params, actor_account_ids, occurred_at, created_at, updated_at, announcement_id)
VALUES (sqlc.arg(id), sqlc.arg(account_id), sqlc.arg(audience), sqlc.narg(store_id), 'announcement', jsonb_build_object('announcement_id', sqlc.arg(announcement_id)::uuid), '{}', sqlc.arg(occurred_at), sqlc.arg(now), sqlc.arg(now), sqlc.arg(announcement_id)::uuid)
ON CONFLICT (announcement_id, account_id, audience, store_id) WHERE announcement_id IS NOT NULL DO NOTHING;

-- name: AddAnnouncementInboxCount :exec
UPDATE notification.announcements SET inbox_count = inbox_count + sqlc.arg(delta) WHERE id = sqlc.arg(id);

-- name: MarkAnnouncementInboxDone :exec
UPDATE notification.announcements SET inbox_done_at = sqlc.arg(now) WHERE id = sqlc.arg(id) AND inbox_done_at IS NULL;

-- name: MarkAnnouncementPushDone :exec
UPDATE notification.announcements SET push_done_at = sqlc.arg(now) WHERE id = sqlc.arg(id) AND push_done_at IS NULL;
