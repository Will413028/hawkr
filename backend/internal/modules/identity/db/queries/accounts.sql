-- name: InsertAccount :one
-- 第一個認證過的請求時建立帳號。同一個 Clerk 使用者同時的第一個請求由 UNIQUE 收斂：沒有列回傳時呼叫方重讀。
INSERT INTO identity.accounts (clerk_user_id, email, created_at, updated_at)
VALUES (sqlc.arg(clerk_user_id), sqlc.narg(email), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (clerk_user_id) DO NOTHING
RETURNING id;

-- name: ResolveAccount :one
-- 認證：Clerk 使用者對應的帳號、狀態與目前的角色（依字母排序）。含墓碑：刪除的帳號不得再建一個。
SELECT a.id, a.suspended_at, a.deleted_at,
       ARRAY(SELECT r.role FROM identity.account_roles r WHERE r.account_id = a.id ORDER BY r.role)::text[] AS roles
FROM identity.accounts a
WHERE a.clerk_user_id = $1;

-- name: GetAccount :one
-- 含已刪除的帳號（墓碑）：舊服務要查得到「已刪除」。
SELECT * FROM identity.accounts WHERE id = $1;

-- name: GetAccounts :many
SELECT * FROM identity.accounts WHERE id = ANY(sqlc.arg(ids)::bigint[]) ORDER BY id;

-- name: LockLiveAccount :one
-- 鎖住存活的帳號，讓同一個帳號的角色變更、停權與刪除依序執行。
SELECT * FROM identity.accounts WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: SuspendAccount :execrows
-- 已經停權時沒有列被更新（冪等）。
UPDATE identity.accounts
SET suspended_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND suspended_at IS NULL;

-- name: UnsuspendAccount :execrows
-- 沒有停權時沒有列被更新（冪等）。
UPDATE identity.accounts
SET suspended_at = NULL, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND suspended_at IS NOT NULL;

-- name: DeleteAccount :execrows
-- 留下沒有個資的墓碑（accounts_deleted_check），clerk_user_id 保留。已經刪除時沒有列被更新（冪等）。
UPDATE identity.accounts
SET deleted_at = sqlc.arg(now)::timestamptz, email = NULL, suspended_at = NULL, deletion_pending_at = NULL,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: MarkDeletionPending :execrows
-- 帳號進入等待刪除；已經在等待時沒有列被更新（冪等）。
UPDATE identity.accounts
SET deletion_pending_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND deletion_pending_at IS NULL;

-- name: ListAccounts :many
-- 後台的帳號清單：存活的帳號，新到舊。role 空字串不篩選角色；pattern 空字串不篩選，否則比對 email 包含這段文字
-- （呼叫方已跳脫 % 與 _），或帳號 ID 等於 account_id（搜尋的文字不是數字時是 0，不會命中）；email 空字串不篩選，否則
-- 要 lower(email) 等於它（呼叫方已去空白並轉小寫）。keyset 分頁：
-- has_cursor 時從 (after_created, after_id) 之後開始。
SELECT a.* FROM identity.accounts a
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(role)::text = ''
       OR EXISTS (SELECT 1 FROM identity.account_roles r WHERE r.account_id = a.id AND r.role = sqlc.arg(role)::text))
  AND (sqlc.arg(pattern)::text = '' OR a.id = sqlc.arg(account_id)::bigint OR a.email ILIKE sqlc.arg(pattern)::text)
  AND (sqlc.arg(email)::text = '' OR lower(a.email) = sqlc.arg(email)::text)
  AND (NOT sqlc.arg(has_cursor)::bool
       OR (a.created_at, a.id) < (sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::bigint))
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(max_rows);

-- name: CountAccounts :one
-- 與 ListAccounts 相同的篩選，給後台顯示頁數。
SELECT count(*) FROM identity.accounts a
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(role)::text = ''
       OR EXISTS (SELECT 1 FROM identity.account_roles r WHERE r.account_id = a.id AND r.role = sqlc.arg(role)::text))
  AND (sqlc.arg(pattern)::text = '' OR a.id = sqlc.arg(account_id)::bigint OR a.email ILIKE sqlc.arg(pattern)::text)
  AND (sqlc.arg(email)::text = '' OR lower(a.email) = sqlc.arg(email)::text);
