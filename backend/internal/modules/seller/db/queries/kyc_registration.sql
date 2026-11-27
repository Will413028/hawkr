-- name: InsertKYCRegistrationCheck :one
INSERT INTO seller.kyc_registration_checks (check_id, application_id, seq, request_kind, requested_by_admin_id, requested_at, deadline_at, submission_seq)
VALUES (
    sqlc.arg(check_id), sqlc.arg(application_id),
    (SELECT COALESCE(MAX(seq), 0) + 1 FROM seller.kyc_registration_checks WHERE application_id = sqlc.arg(application_id)),
    sqlc.arg(request_kind), sqlc.narg(requested_by_admin_id), sqlc.arg(now), sqlc.arg(deadline_at),
    (SELECT submission_seq FROM seller.kyc_applications WHERE application_id = sqlc.arg(application_id))
)
RETURNING *;

-- name: ClaimKYCRegistrationCheck :one
SELECT c.check_id, c.application_id, c.outcome, c.deadline_at, c.submission_seq AS check_submission_seq,
       a.status AS application_status, a.submission_seq AS application_submission_seq,
       EXISTS (
           SELECT 1 FROM seller.kyc_registration_checks newer
           WHERE newer.application_id = c.application_id AND newer.submission_seq = c.submission_seq AND newer.seq > c.seq
       ) AS superseded,
       a.seller_type, a.tax_id, a.company_name, a.company_town_code, a.company_address_detail,
       a.company_register_date
FROM seller.kyc_registration_checks c
JOIN seller.kyc_applications a ON a.application_id = c.application_id
WHERE c.check_id = $1
FOR UPDATE OF c;

-- name: CompleteKYCRegistrationCheck :one
UPDATE seller.kyc_registration_checks
SET outcome = sqlc.arg(outcome),
    queried_at = sqlc.arg(queried_at),
    tax_id = sqlc.narg(tax_id),
    name = sqlc.narg(name),
    address_text = sqlc.narg(address_text),
    town_code = sqlc.narg(town_code),
    address_detail = sqlc.narg(address_detail),
    setup_date = sqlc.narg(setup_date),
    organization_type = sqlc.narg(organization_type),
    status = sqlc.narg(status),
    status_text = sqlc.narg(status_text),
    suspended_from = sqlc.narg(suspended_from),
    suspended_until = sqlc.narg(suspended_until),
    mismatched_fields = sqlc.arg(mismatched_fields)::text[],
    unverifiable_fields = sqlc.arg(unverifiable_fields)::text[]
WHERE check_id = sqlc.arg(check_id) AND outcome = 'PENDING'
RETURNING *;

-- name: LatestCompletedKYCRegistrationCheck :one
SELECT c.*
FROM seller.kyc_registration_checks c
JOIN seller.kyc_applications a ON a.application_id = c.application_id
WHERE c.application_id = $1
  AND a.status IN ('SUBMITTED', 'APPROVED', 'REJECTED')
  AND c.submission_seq = a.submission_seq
  AND c.outcome NOT IN ('PENDING', 'CANCELLED')
ORDER BY c.seq DESC
LIMIT 1;

-- name: PendingKYCRegistrationCheck :one
SELECT c.*
FROM seller.kyc_registration_checks c
JOIN seller.kyc_applications a ON a.application_id = c.application_id
WHERE c.application_id = $1 AND a.status = 'SUBMITTED' AND c.outcome = 'PENDING' AND c.submission_seq = a.submission_seq
ORDER BY c.seq DESC
LIMIT 1;

-- name: SupersedePendingKYCRegistrationChecks :execrows
UPDATE seller.kyc_registration_checks c
SET outcome = 'CANCELLED', queried_at = sqlc.arg(now)
FROM seller.kyc_applications a
WHERE c.application_id = sqlc.arg(application_id) AND a.application_id = c.application_id
  AND c.outcome = 'PENDING' AND c.submission_seq = a.submission_seq;

-- name: ExpireKYCRegistrationChecks :execrows
UPDATE seller.kyc_registration_checks c
SET outcome = CASE
        WHEN a.status = 'SUBMITTED' AND a.submission_seq = c.submission_seq AND NOT EXISTS (
            SELECT 1 FROM seller.kyc_registration_checks newer
            WHERE newer.application_id = c.application_id AND newer.submission_seq = c.submission_seq AND newer.seq > c.seq
        ) THEN 'UNAVAILABLE'
        ELSE 'CANCELLED'
    END,
    queried_at = sqlc.arg(now)
FROM seller.kyc_applications a
WHERE a.application_id = c.application_id
  AND c.outcome = 'PENDING' AND c.deadline_at <= sqlc.arg(deadline_before)::timestamptz;

-- name: KYCRegistrationChecksToResync :many
SELECT c.check_id
FROM seller.kyc_registration_checks c
WHERE c.outcome = 'PENDING' AND c.requested_at <= sqlc.arg(idle_before)::timestamptz
ORDER BY c.deadline_at, c.check_id
LIMIT sqlc.arg(batch_size);

-- name: ListKYCRegistrationChecks :many
SELECT *
FROM seller.kyc_registration_checks
WHERE application_id = $1
ORDER BY seq;

-- name: KYCReviewRegistrationChecks :many
SELECT r.review_id, c.check_id, c.seq, c.queried_at
FROM seller.kyc_reviews r
JOIN seller.kyc_registration_checks c ON c.check_id = r.registration_check_id
WHERE r.application_id = $1;

-- name: PurgeKYCRegistrationChecks :execrows
DELETE FROM seller.kyc_registration_checks
WHERE application_id = ANY(sqlc.arg(application_ids)::uuid[]);
