-- name: InsertAdmin :one
-- 第一次看到 Access subject 時建立。同一位管理員同時的第一個請求由 UNIQUE 收斂：沒有列回傳時呼叫方重讀。
INSERT INTO identity.admins (access_subject, email, created_at, updated_at)
VALUES (sqlc.arg(access_subject), sqlc.arg(email), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (access_subject) DO NOTHING
RETURNING id;

-- name: ResolveAdmin :one
-- 認證：Access subject 對應的管理員與目前的角色（依字母排序）；每個請求都讀，角色不快取。
SELECT a.id,
       ARRAY(SELECT r.role FROM identity.admin_roles r WHERE r.admin_id = a.id ORDER BY r.role)::text[] AS roles
FROM identity.admins a
WHERE a.access_subject = $1;

-- name: GetAdmin :one
SELECT * FROM identity.admins WHERE id = $1;

-- name: AdminsByIDs :many
SELECT id, email FROM identity.admins WHERE id = ANY(sqlc.arg(ids)::bigint[]);
