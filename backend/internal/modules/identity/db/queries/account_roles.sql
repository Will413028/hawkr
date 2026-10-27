-- name: GrantAccountRole :execrows
-- 授予角色；已經有這個角色時改指到新的來源（呼叫方永遠帶當下的來源，重新入駐後遲到的舊撤銷因此擋得下）。
-- 回傳 0 表示角色與來源都沒有變。
INSERT INTO identity.account_roles (account_id, role, source_id, granted_at)
VALUES (sqlc.arg(account_id), sqlc.arg(role), sqlc.narg(source_id), sqlc.arg(now))
ON CONFLICT (account_id, role) DO UPDATE
SET source_id = EXCLUDED.source_id
WHERE identity.account_roles.source_id IS DISTINCT FROM EXCLUDED.source_id;

-- name: RevokeAccountRole :one
-- 移除角色，只在目前的授予沒有來源、或來源就是 source_id 時生效（#3189 的柵欄）。
-- had_role 是移除之前帳號有沒有這個角色：had_role 而且沒有 revoked，表示被柵欄擋下（角色屬於較新的來源）。
-- 兩個 CTE 讀同一個 snapshot，had_role 描述的是這次移除面對的狀態。
WITH current_grant AS (
    SELECT 1 FROM identity.account_roles cg
    WHERE cg.account_id = sqlc.arg(account_id) AND cg.role = sqlc.arg(role)
),
revoked_grant AS (
    DELETE FROM identity.account_roles rg
    WHERE rg.account_id = sqlc.arg(account_id)
      AND rg.role = sqlc.arg(role)
      AND (rg.source_id IS NULL OR rg.source_id = sqlc.narg(source_id))
    RETURNING 1
)
SELECT
    EXISTS (SELECT 1 FROM revoked_grant) AS revoked,
    EXISTS (SELECT 1 FROM current_grant) AS had_role;

-- name: DeleteAccountRoles :exec
DELETE FROM identity.account_roles WHERE account_id = $1;

-- name: ListRolesOfAccounts :many
SELECT account_id, role FROM identity.account_roles
WHERE account_id = ANY(sqlc.arg(account_ids)::bigint[])
ORDER BY account_id, role;
