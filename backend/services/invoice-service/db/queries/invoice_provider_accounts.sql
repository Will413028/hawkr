
-- name: FetchDueHealthChecks :many
SELECT * FROM invoice_provider_accounts
WHERE (
    status IN ('ACTIVE', 'DEGRADED')
    OR (
      status = 'PROVIDER_UNAVAILABLE'
      AND breaker_opened_at IS NOT NULL
      AND breaker_opened_at > sqlc.arg(unavailable_since_floor)
    )
  )
  AND health_check_next_at <= sqlc.arg(now)
ORDER BY health_check_next_at
LIMIT sqlc.arg(row_limit);

-- name: ClaimHealthCheck :execrows
UPDATE invoice_provider_accounts
SET health_check_next_at = sqlc.arg(lease_until), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND (
    status IN ('ACTIVE', 'DEGRADED')
    OR (
      status = 'PROVIDER_UNAVAILABLE'
      AND breaker_opened_at IS NOT NULL
      AND breaker_opened_at > sqlc.arg(unavailable_since_floor)
    )
  )
  AND health_check_next_at <= sqlc.arg(claimable_before);

-- name: RecordHealthCheckResult :execrows
UPDATE invoice_provider_accounts
SET last_health_checked_at = sqlc.arg(now),
    health_check_next_at = sqlc.arg(next_check_at),
    status = sqlc.arg(next_status),
    status_reason = sqlc.narg(status_reason),
    breaker_opened_at = sqlc.narg(breaker_opened_at),
    breaker_probe_success = sqlc.arg(breaker_probe_success),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id);

-- name: ListActiveProviderAccounts :many
SELECT * FROM invoice_provider_accounts
WHERE status = 'ACTIVE'
ORDER BY id;

-- name: GetProviderAccountByIssuer :one
SELECT * FROM invoice_provider_accounts
WHERE issuer_id = sqlc.arg(issuer_id) AND provider = sqlc.arg(provider);

-- name: GetProviderAccountByID :one
SELECT * FROM invoice_provider_accounts WHERE id = sqlc.arg(id);

-- name: InsertProviderAccount :execrows
INSERT INTO invoice_provider_accounts (
  id, issuer_id, provider, provider_tax_id,
  credential_ciphertext, credential_key_version, credential_fingerprint, credential_last4,
  status, health_check_next_at, credential_updated_at, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(issuer_id), sqlc.arg(provider), sqlc.arg(provider_tax_id),
  sqlc.arg(credential_ciphertext), sqlc.arg(credential_key_version),
  sqlc.arg(credential_fingerprint), sqlc.arg(credential_last4),
  sqlc.arg(status), sqlc.arg(health_check_next_at),
  sqlc.arg(now)::timestamptz, sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: UpdateProviderAccountStatus :execrows
UPDATE invoice_provider_accounts
SET status = sqlc.arg(next_status),
    status_reason = sqlc.narg(status_reason),
    verified_at = sqlc.narg(verified_at),
    health_check_next_at = sqlc.arg(health_check_next_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version);

-- name: UpdateProviderAccountCredential :execrows
UPDATE invoice_provider_accounts
SET credential_ciphertext = sqlc.arg(credential_ciphertext),
    credential_key_version = sqlc.arg(credential_key_version),
    credential_fingerprint = sqlc.arg(credential_fingerprint),
    credential_last4 = sqlc.arg(credential_last4),
    pending_ciphertext = NULL,
    pending_fingerprint = NULL,
    pending_last4 = NULL,
    rotation_started_at = NULL,
    status = sqlc.arg(next_status),
    credential_updated_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status = 'ROTATION_PENDING';

-- name: StartProviderCredentialRotation :execrows
UPDATE invoice_provider_accounts
SET pending_ciphertext = sqlc.arg(pending_ciphertext),
    pending_fingerprint = sqlc.arg(pending_fingerprint),
    pending_last4 = sqlc.arg(pending_last4),
    rotation_started_at = sqlc.arg(now),
    status = 'ROTATION_PENDING',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status = 'ACTIVE';

-- name: GetProviderAccountByStoreID :one
SELECT a.* FROM invoice_provider_accounts a
JOIN invoice_issuers i ON i.id = a.issuer_id
WHERE i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint
  AND a.provider = sqlc.arg(provider);

-- name: ListProviderAccountsForAdmin :many
SELECT sqlc.embed(i),
       a.id                     AS account_id,
       a.provider               AS provider,
       a.provider_tax_id        AS provider_tax_id,
       a.status                 AS account_status,
       a.status_reason          AS status_reason,
       a.credential_last4       AS credential_last4,
       a.verified_at            AS verified_at,
       a.automation_cutover_at  AS automation_cutover_at,
       a.last_health_checked_at AS last_health_checked_at,
       a.version                AS account_version,
       a.updated_at             AS account_updated_at
FROM invoice_issuers i
LEFT JOIN invoice_provider_accounts a
       ON a.issuer_id = i.id AND a.provider = sqlc.arg(provider)::text
WHERE i.issuer_type = 'SELLER'
  AND (sqlc.arg(status_filter)::text = ''
       OR a.status = sqlc.arg(status_filter)::text
       OR (sqlc.arg(match_not_started)::boolean AND a.id IS NULL))
  AND (sqlc.arg(keyword)::text = ''
       OR a.provider_tax_id = sqlc.arg(keyword)::text
       OR i.display_name ILIKE '%' || sqlc.arg(keyword)::text || '%')
ORDER BY i.created_at DESC, i.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountProviderAccountsForAdmin :one
SELECT COUNT(*)::bigint
FROM invoice_issuers i
LEFT JOIN invoice_provider_accounts a
       ON a.issuer_id = i.id AND a.provider = sqlc.arg(provider)::text
WHERE i.issuer_type = 'SELLER'
  AND (sqlc.arg(status_filter)::text = ''
       OR a.status = sqlc.arg(status_filter)::text
       OR (sqlc.arg(match_not_started)::boolean AND a.id IS NULL))
  AND (sqlc.arg(keyword)::text = ''
       OR a.provider_tax_id = sqlc.arg(keyword)::text
       OR i.display_name ILIKE '%' || sqlc.arg(keyword)::text || '%');


-- name: ResubmitProviderCredential :execrows
UPDATE invoice_provider_accounts
SET credential_ciphertext = sqlc.arg(credential_ciphertext),
    credential_key_version = sqlc.arg(credential_key_version),
    credential_fingerprint = sqlc.arg(credential_fingerprint),
    credential_last4 = sqlc.arg(credential_last4),
    provider_tax_id = sqlc.arg(provider_tax_id),
    status = 'CREDENTIAL_VERIFYING',
    status_reason = NULL,
    health_check_next_at = sqlc.arg(health_check_next_at),
    credential_updated_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('NOT_CONFIGURED','CREDENTIAL_SUBMITTED','CREDENTIAL_VERIFYING',
                 'INVALID_CREDENTIAL','PROVIDER_REJECTED','PROVIDER_UNAVAILABLE');

-- name: StartProviderVerification :execrows
UPDATE invoice_provider_accounts
SET status = 'CREDENTIAL_VERIFYING',
    status_reason = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('CREDENTIAL_SUBMITTED','CREDENTIAL_VERIFYING',
                 'INVALID_CREDENTIAL','PROVIDER_REJECTED','PROVIDER_UNAVAILABLE','SUSPENDED');

-- name: LandProviderVerification :execrows
UPDATE invoice_provider_accounts
SET status = sqlc.arg(next_status),
    status_reason = sqlc.narg(status_reason),
    verified_at = sqlc.narg(verified_at),
    breaker_opened_at = sqlc.narg(breaker_opened_at),
    last_health_checked_at = sqlc.arg(now),
    health_check_next_at = sqlc.arg(health_check_next_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status = 'CREDENTIAL_VERIFYING';

-- name: FailProviderVerificationFromActive :execrows
UPDATE invoice_provider_accounts
SET status = sqlc.arg(next_status),
    status_reason = sqlc.narg(status_reason),
    last_health_checked_at = sqlc.arg(now),
    health_check_next_at = sqlc.arg(health_check_next_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('ACTIVE','DEGRADED');

-- name: TouchProviderHealthCheck :execrows
UPDATE invoice_provider_accounts
SET last_health_checked_at = sqlc.arg(now),
    health_check_next_at = sqlc.arg(health_check_next_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('ACTIVE','DEGRADED');

-- name: AbortProviderCredentialRotation :execrows
UPDATE invoice_provider_accounts
SET pending_ciphertext = NULL,
    pending_fingerprint = NULL,
    pending_last4 = NULL,
    rotation_started_at = NULL,
    status = 'ACTIVE',
    status_reason = sqlc.narg(status_reason),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status = 'ROTATION_PENDING';

-- name: SetProviderAutomationCutover :execrows
UPDATE invoice_provider_accounts
SET automation_cutover_at = sqlc.arg(cutover_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status = 'ACTIVE'
  AND verified_at IS NOT NULL;

-- name: SuspendProviderAccount :execrows
UPDATE invoice_provider_accounts
SET status = 'SUSPENDED',
    status_reason = sqlc.narg(status_reason),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('ACTIVE','DEGRADED');

-- name: RevokeProviderAccount :execrows
UPDATE invoice_provider_accounts
SET status = 'REVOKED',
    status_reason = sqlc.narg(status_reason),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('ACTIVE','DEGRADED','SUSPENDED');
