-- name: AdminIDsByEmail :many
SELECT id FROM identity.admins WHERE lower(email) = lower($1) ORDER BY id;

-- name: InsertAdminRole :execrows
INSERT INTO identity.admin_roles (admin_id, role, granted_at)
VALUES (sqlc.arg(admin_id), sqlc.arg(role), sqlc.arg(now))
ON CONFLICT (admin_id, role) DO NOTHING;

-- name: DeleteAdminRole :execrows
DELETE FROM identity.admin_roles WHERE admin_id = $1 AND role = $2;

-- name: InsertAdminRoleEvent :exec
INSERT INTO identity.admin_role_events (admin_id, role, action, reason, source, db_user, os_user, occurred_at)
VALUES (sqlc.arg(admin_id), sqlc.arg(role), sqlc.arg(action), sqlc.arg(reason), sqlc.arg(source), session_user, sqlc.narg(os_user), sqlc.arg(now));

-- name: ListAdminRolesForShare :many
SELECT role FROM identity.admin_roles WHERE admin_id = $1 ORDER BY role FOR SHARE;

-- name: ListAdminRoles :many
SELECT role FROM identity.admin_roles WHERE admin_id = $1 ORDER BY role;

-- name: AdminRoleHolderExists :one
SELECT EXISTS (SELECT 1 FROM identity.admin_roles WHERE role = $1) AS held;
