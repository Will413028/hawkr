-- name: InsertAdmin :one
INSERT INTO identity.admins (access_subject, email, created_at, updated_at)
VALUES (sqlc.arg(access_subject), sqlc.arg(email), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (access_subject) DO NOTHING
RETURNING id;

-- name: ResolveAdmin :one
SELECT a.id,
       ARRAY(SELECT r.role FROM identity.admin_roles r WHERE r.admin_id = a.id ORDER BY r.role)::text[] AS roles
FROM identity.admins a
WHERE a.access_subject = $1;

-- name: GetAdmin :one
SELECT * FROM identity.admins WHERE id = $1;

-- name: AdminsByIDs :many
SELECT id, email FROM identity.admins WHERE id = ANY(sqlc.arg(ids)::bigint[]);
