-- name: AdminDelegationEligible :one
SELECT EXISTS (SELECT 1 FROM delegation.admin_eligibilities WHERE admin_id = $1) AS eligible;

-- name: LockAdminDelegationEligibility :one
SELECT admin_id FROM delegation.admin_eligibilities WHERE admin_id = $1 FOR UPDATE;

-- name: EnableAdminDelegation :exec
INSERT INTO delegation.admin_eligibilities (admin_id) VALUES ($1) ON CONFLICT DO NOTHING;

-- name: DisableAdminDelegation :exec
DELETE FROM delegation.admin_eligibilities WHERE admin_id = $1;
