-- name: GetPayuniRegistrationByStoreID :one
SELECT * FROM payuni_registrations WHERE store_id = $1;

-- name: GetPayuniRegistrationByMerID :one
SELECT * FROM payuni_registrations WHERE payuni_mer_id = $1;

-- name: GetPendingPayuniRegistrations :many
SELECT * FROM payuni_registrations
WHERE (
    registration_status = 'pending'
    OR COALESCE(mer_status, '') = '91'
    OR (COALESCE(mer_status, '') = '9' AND COALESCE(member_status, '') NOT IN ('8', '92', '99'))
    OR (COALESCE(mer_status, '') = '1' AND COALESCE(member_status, '') NOT IN ('1', '8', '92', '99'))
    OR (COALESCE(mer_status, '') NOT IN ('1', '2', '8', '9', '91', '92', '99') AND COALESCE(member_status, '') = '5')
  )
  AND (last_claimed_at IS NULL OR last_claimed_at <= sqlc.arg(stale_before))
  AND (last_synced_at IS NULL OR last_synced_at <= sqlc.arg(stale_before))
ORDER BY last_claimed_at ASC NULLS FIRST
LIMIT sqlc.arg(row_limit);

-- name: ClaimPayuniRegistrationForSync :execrows
UPDATE payuni_registrations
SET last_claimed_at = sqlc.arg(claimed_at)
WHERE id = sqlc.arg(id)
  AND (
    registration_status = 'pending'
    OR COALESCE(mer_status, '') = '91'
    OR (COALESCE(mer_status, '') = '9' AND COALESCE(member_status, '') NOT IN ('8', '92', '99'))
    OR (COALESCE(mer_status, '') = '1' AND COALESCE(member_status, '') NOT IN ('1', '8', '92', '99'))
    OR (COALESCE(mer_status, '') NOT IN ('1', '2', '8', '9', '91', '92', '99') AND COALESCE(member_status, '') = '5')
  )
  AND (last_claimed_at IS NULL OR last_claimed_at <= sqlc.arg(stale_before))
  AND (last_synced_at IS NULL OR last_synced_at <= sqlc.arg(stale_before));

-- name: CreatePayuniRegistration :one
INSERT INTO payuni_registrations (
  id, account_id, application_id, payuni_mer_id, registration_status,
  payment_enabled, updated_at, store_id
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: UpdatePayuniRegistration :exec
UPDATE payuni_registrations SET
  payuni_mer_id = $2,
  registration_status = $3,
  payment_enabled = $4,
  mer_status = $5,
  member_status = $6,
  credit_status = $7,
  applepay_status = $8,
  googlepay_status = $9,
  atm_status = $10,
  reason = $11,
  error_message = $12,
  raw_request = $13,
  raw_response = $14,
  registered_at = $15,
  approved_at = $16,
  last_synced_at = $17,
  updated_at = $18,
  error_code = $19
WHERE id = $1;

-- name: UpsertPayuniRegistration :one
INSERT INTO payuni_registrations (
  id, account_id, application_id, payuni_mer_id, registration_status,
  payment_enabled, mer_status, member_status, credit_status,
  applepay_status, googlepay_status, atm_status,
  reason, error_message, raw_request, raw_response,
  registered_at, approved_at, last_synced_at, updated_at, store_id, error_code
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
)
ON CONFLICT (store_id) DO UPDATE SET
  application_id = EXCLUDED.application_id,
  payuni_mer_id = COALESCE(EXCLUDED.payuni_mer_id, payuni_registrations.payuni_mer_id),
  registration_status = EXCLUDED.registration_status,
  payment_enabled = EXCLUDED.payment_enabled,
  mer_status = COALESCE(NULLIF(EXCLUDED.mer_status, ''), payuni_registrations.mer_status),
  member_status = COALESCE(NULLIF(EXCLUDED.member_status, ''), payuni_registrations.member_status),
  credit_status = COALESCE(NULLIF(EXCLUDED.credit_status, ''), payuni_registrations.credit_status),
  applepay_status = COALESCE(NULLIF(EXCLUDED.applepay_status, ''), payuni_registrations.applepay_status),
  googlepay_status = COALESCE(NULLIF(EXCLUDED.googlepay_status, ''), payuni_registrations.googlepay_status),
  atm_status = COALESCE(NULLIF(EXCLUDED.atm_status, ''), payuni_registrations.atm_status),
  reason = COALESCE(NULLIF(EXCLUDED.reason, ''), payuni_registrations.reason),
  error_message = EXCLUDED.error_message,
  error_code = EXCLUDED.error_code,
  raw_request = COALESCE(EXCLUDED.raw_request, payuni_registrations.raw_request),
  raw_response = EXCLUDED.raw_response,
  registered_at = COALESCE(EXCLUDED.registered_at, payuni_registrations.registered_at),
  approved_at = COALESCE(EXCLUDED.approved_at, payuni_registrations.approved_at),
  last_synced_at = COALESCE(EXCLUDED.last_synced_at, payuni_registrations.last_synced_at),
  updated_at = EXCLUDED.updated_at
WHERE payuni_registrations.payuni_mer_id IS NULL
   OR EXCLUDED.payuni_mer_id IS NULL
   OR payuni_registrations.payuni_mer_id = EXCLUDED.payuni_mer_id
RETURNING *;

-- name: UpdatePayuniMerchantProfile :one
UPDATE payuni_registrations SET
  contact_name = COALESCE(NULLIF(sqlc.arg(contact_name)::varchar, ''), contact_name),
  contact_mobile = COALESCE(NULLIF(sqlc.arg(contact_mobile)::varchar, ''), contact_mobile),
  contact_tel = COALESCE(NULLIF(sqlc.arg(contact_tel)::varchar, ''), contact_tel),
  mer_email = COALESCE(NULLIF(sqlc.arg(mer_email)::varchar, ''), mer_email),
  mer_addr = COALESCE(NULLIF(sqlc.arg(mer_addr)::varchar, ''), mer_addr),
  mer_addr_post = COALESCE(NULLIF(sqlc.arg(mer_addr_post)::varchar, ''), mer_addr_post),
  mer_tel = COALESCE(NULLIF(sqlc.arg(mer_tel)::varchar, ''), mer_tel),
  merchant_profile_version = merchant_profile_version + 1
WHERE store_id = $1 AND merchant_profile_version = $2
RETURNING *;

-- name: AcquireMerchantProfileUpdateLock :exec
SELECT pg_advisory_xact_lock(hashtextextended('payuni_merchant_profile_update', $1::bigint));
