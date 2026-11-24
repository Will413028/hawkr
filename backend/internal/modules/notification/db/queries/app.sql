-- name: ListInboxItems :many
-- 收件人（帳號、受眾、店）的收件匣，新到舊（updated_at：聚合列是最後一次合併的時間，再依 id）。types 是呼叫端依受眾與分類從註冊表算出的型別名稱：
-- 沒有註冊的型別（已退場）不顯示也不計數。BUYER 列的 store_id 一定是 NULL（CHECK）：店家條件寫成「等於店家 ID，或兩邊都是 NULL」，參數是常數時只剩一邊，
-- 讓 (account_id, audience, store_id, updated_at, id) 的索引從 store_id 到排序都用得上（IS NOT DISTINCT FROM 不能用索引）。after_* 都是 NULL 表示第一頁。
SELECT id, type, params, read_at, updated_at FROM notification.inbox_items
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience)
  AND (store_id = sqlc.narg(store_id)::bigint OR (sqlc.narg(store_id)::bigint IS NULL AND store_id IS NULL))
  AND type = ANY(sqlc.arg(types)::text[])
  AND (sqlc.narg(after_updated_at)::timestamptz IS NULL OR (updated_at, id) < (sqlc.narg(after_updated_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(max_rows);

-- name: CountInboxItems :one
-- 與 ListInboxItems 同一組篩選（受眾、店、型別）的全部筆數。
SELECT count(*) FROM notification.inbox_items
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience)
  AND (store_id = sqlc.narg(store_id)::bigint OR (sqlc.narg(store_id)::bigint IS NULL AND store_id IS NULL))
  AND type = ANY(sqlc.arg(types)::text[]);

-- name: CountUnreadInboxItems :one
SELECT count(*) FROM notification.inbox_items
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience)
  AND (store_id = sqlc.narg(store_id)::bigint OR (sqlc.narg(store_id)::bigint IS NULL AND store_id IS NULL))
  AND type = ANY(sqlc.arg(types)::text[])
  AND read_at IS NULL;

-- name: LatestInboxItemID :one
-- 串流訊號的最新一列：與 CountUnreadInboxItems 同一組篩選（受眾、店、型別），updated_at 最新（再依 id）的列；沒有列是 pgx.ErrNoRows。
SELECT id FROM notification.inbox_items
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience)
  AND (store_id = sqlc.narg(store_id)::bigint OR (sqlc.narg(store_id)::bigint IS NULL AND store_id IS NULL))
  AND type = ANY(sqlc.arg(types)::text[])
ORDER BY updated_at DESC, id DESC LIMIT 1;

-- name: MarkInboxItemRead :one
-- 只有擁有者（帳號）能標；已讀的列保留原本的已讀時間。不存在與不是擁有者都是 0 列（pgx.ErrNoRows），呼叫端不區分。
UPDATE notification.inbox_items SET read_at = COALESCE(read_at, sqlc.arg(now)::timestamptz)
WHERE id = sqlc.arg(id) AND account_id = sqlc.arg(account_id)
RETURNING id, audience, store_id;

-- name: MarkAllInboxItemsRead :execrows
UPDATE notification.inbox_items SET read_at = sqlc.arg(now)::timestamptz
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience)
  AND (store_id = sqlc.narg(store_id)::bigint OR (sqlc.narg(store_id)::bigint IS NULL AND store_id IS NULL))
  AND type = ANY(sqlc.arg(types)::text[])
  AND read_at IS NULL;

-- name: DeleteInboxItems :many
-- 只刪擁有者（帳號）的列，回傳被刪列的受眾與店（串流訊號的收件人）；呼叫端比較刪除筆數與 ids 的數量，不符就回滾（全有或全無）。
DELETE FROM notification.inbox_items WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND account_id = sqlc.arg(account_id)
RETURNING audience, store_id;

-- name: PreferenceOverridesForRecipient :many
SELECT category, channel, enabled FROM notification.preference_overrides
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience);

-- name: UpsertPreferenceOverride :exec
INSERT INTO notification.preference_overrides (account_id, audience, category, channel, enabled, updated_at)
VALUES (sqlc.arg(account_id), sqlc.arg(audience), sqlc.arg(category), sqlc.arg(channel), sqlc.arg(enabled), sqlc.arg(now))
ON CONFLICT (account_id, audience, category, channel) DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = EXCLUDED.updated_at;

-- name: DeletePreferenceOverride :exec
-- 設回預設值時不留覆寫列（只存與預設不同的值）。
DELETE FROM notification.preference_overrides
WHERE account_id = sqlc.arg(account_id) AND audience = sqlc.arg(audience) AND category = sqlc.arg(category) AND channel = sqlc.arg(channel);

-- name: UpsertDeviceToken :exec
-- token 是主鍵：同一個 token 換帳號登入時改綁給呼叫者，並更新最後註冊時間。
INSERT INTO notification.device_tokens (token, account_id, app, last_registered_at)
VALUES (sqlc.arg(token), sqlc.arg(account_id), sqlc.arg(app), sqlc.arg(now))
ON CONFLICT (token) DO UPDATE SET account_id = EXCLUDED.account_id, app = EXCLUDED.app, last_registered_at = EXCLUDED.last_registered_at;

-- name: DeleteDeviceToken :exec
-- 只刪呼叫者自己的 token；不存在或屬於別的帳號都是 0 列，呼叫端一律回成功。
DELETE FROM notification.device_tokens WHERE token = sqlc.arg(token) AND account_id = sqlc.arg(account_id);
