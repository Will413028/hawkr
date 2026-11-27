-- name: GrantAccountRole :exec
INSERT INTO identity.account_roles (account_id, role, granted_at)
VALUES (sqlc.arg(account_id), sqlc.arg(role), sqlc.arg(now))
ON CONFLICT (account_id, role) DO NOTHING;

-- name: RevokeAccountRole :exec
DELETE FROM identity.account_roles
WHERE account_id = sqlc.arg(account_id) AND role = sqlc.arg(role);

-- name: DeleteAccountRoles :exec
DELETE FROM identity.account_roles WHERE account_id = $1;

-- name: ListRolesOfAccounts :many
SELECT account_id, role FROM identity.account_roles
WHERE account_id = ANY(sqlc.arg(account_ids)::bigint[])
ORDER BY account_id, role;
