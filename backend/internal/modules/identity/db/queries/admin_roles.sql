-- name: AdminIDsByEmail :many
-- 以 email 找管理員（不分大小寫）；email 不是唯一的，多於一筆時由呼叫方要求改用 ID。
SELECT id FROM identity.admins WHERE lower(email) = lower($1) ORDER BY id;

-- name: InsertAdminRole :execrows
-- 授予角色；已經有時回傳 0。
INSERT INTO identity.admin_roles (admin_id, role, granted_at)
VALUES (sqlc.arg(admin_id), sqlc.arg(role), sqlc.arg(now))
ON CONFLICT (admin_id, role) DO NOTHING;

-- name: DeleteAdminRole :execrows
-- 撤銷角色；沒有這個角色時回傳 0。刪除會等待持有同一列 FOR SHARE 的 transaction（代行的開始）結束。
DELETE FROM identity.admin_roles WHERE admin_id = $1 AND role = $2;

-- name: InsertAdminRoleEvent :exec
-- 追加一筆授予或撤銷的紀錄。db_user 是連線的 session_user，未經驗證，只供參考。
INSERT INTO identity.admin_role_events (admin_id, role, action, reason, source, db_user, os_user, occurred_at)
VALUES (sqlc.arg(admin_id), sqlc.arg(role), sqlc.arg(action), sqlc.arg(reason), sqlc.arg(source), session_user, sqlc.narg(os_user), sqlc.arg(now));

-- name: ListAdminRolesForShare :many
-- 在呼叫方的 transaction 內讀管理員的角色並鎖住這些列（FOR SHARE）：撤銷的 DELETE 與它互相等待，讀到的角色在 transaction
-- 結束之前不會被撤銷。
SELECT role FROM identity.admin_roles WHERE admin_id = $1 ORDER BY role FOR SHARE;

-- name: AdminRoleHolderExists :one
-- 有沒有任何管理員持有這個角色。
SELECT EXISTS (SELECT 1 FROM identity.admin_roles WHERE role = $1) AS held;
