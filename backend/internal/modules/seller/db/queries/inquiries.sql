-- name: InsertCaseInquiry :one
INSERT INTO seller.case_inquiries (id, case_id, store_id, kind, message, created_at, created_by_admin_id, due_at, request_id, request_hash)
VALUES (sqlc.arg(id), sqlc.arg(case_id), sqlc.arg(store_id), sqlc.arg(kind), sqlc.arg(message), sqlc.arg(now), sqlc.arg(admin_id), sqlc.arg(due_at), sqlc.narg(request_id), sqlc.narg(request_hash))
ON CONFLICT (created_by_admin_id, request_id) DO NOTHING
RETURNING *;

-- name: CaseInquiryByRequest :one
SELECT *
FROM seller.case_inquiries
WHERE created_by_admin_id = sqlc.arg(admin_id) AND request_id = sqlc.arg(request_id);

-- name: CaseInquiryByID :one
SELECT *
FROM seller.case_inquiries
WHERE id = $1;

-- name: LockCaseInquiry :one
SELECT *
FROM seller.case_inquiries
WHERE id = $1
FOR UPDATE;

-- name: CaseInquiries :many
SELECT *
FROM seller.case_inquiries
WHERE case_id = sqlc.arg(case_id)
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (created_at, id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY created_at, id
LIMIT sqlc.arg(page_size);

-- name: OpenCaseInquiries :many
SELECT case_id, id
FROM seller.case_inquiries
WHERE case_id = ANY(sqlc.arg(case_ids)::uuid[]) AND reply_received_at IS NULL AND withdrawn_at IS NULL;

-- name: RecordCaseInquiryReply :one
UPDATE seller.case_inquiries
SET reply_body = sqlc.arg(body)::text,
    reply_channel = sqlc.arg(channel)::text,
    reply_received_at = sqlc.arg(received_at)::timestamptz,
    reply_recorded_at = sqlc.arg(now)::timestamptz,
    reply_recorded_by_admin_id = sqlc.arg(admin_id)::bigint
WHERE id = sqlc.arg(id) AND reply_received_at IS NULL AND withdrawn_at IS NULL
RETURNING *;

-- name: WithdrawCaseInquiry :one
UPDATE seller.case_inquiries
SET withdrawn_at = sqlc.arg(now)::timestamptz,
    withdrawn_by_admin_id = sqlc.arg(admin_id)::bigint,
    withdraw_cause = 'ADMIN'
WHERE id = sqlc.arg(id) AND reply_received_at IS NULL AND withdrawn_at IS NULL
RETURNING *;

-- name: WithdrawOpenCaseInquiry :many
UPDATE seller.case_inquiries
SET withdrawn_at = sqlc.arg(now)::timestamptz,
    withdrawn_by_admin_id = sqlc.arg(admin_id)::bigint,
    withdraw_cause = 'CASE_CLOSED'
WHERE case_id = sqlc.arg(case_id) AND reply_received_at IS NULL AND withdrawn_at IS NULL
RETURNING id;

-- name: InsertCaseInquiryEvidence :exec
INSERT INTO seller.case_inquiry_reply_evidence (inquiry_id, position, asset_id, object_key)
VALUES (sqlc.arg(inquiry_id), sqlc.arg(position), sqlc.arg(asset_id), sqlc.arg(object_key));

-- name: CaseInquiryEvidence :many
SELECT inquiry_id, position, asset_id, object_key
FROM seller.case_inquiry_reply_evidence
WHERE inquiry_id = ANY(sqlc.arg(inquiry_ids)::uuid[])
ORDER BY inquiry_id, position;

-- name: InsertInquiryEvidenceAccess :exec
INSERT INTO seller.evidence_accesses (access_id, inquiry_id, position, admin_id, accessed_at)
VALUES (sqlc.arg(access_id), sqlc.arg(inquiry_id), sqlc.arg(position), sqlc.arg(admin_id), sqlc.arg(now));

-- name: AccountCaseInquiries :many
SELECT i.*
FROM seller.case_inquiries i
JOIN seller.stores s ON s.store_id = i.store_id
WHERE s.account_id = sqlc.arg(account_id)
  AND (
    sqlc.narg(state)::text IS NULL OR
    (sqlc.narg(state)::text = 'OPEN' AND i.reply_received_at IS NULL AND i.withdrawn_at IS NULL) OR
    (sqlc.narg(state)::text = 'REPLIED' AND i.reply_received_at IS NOT NULL) OR
    (sqlc.narg(state)::text = 'WITHDRAWN' AND i.withdrawn_at IS NOT NULL)
  )
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (i.created_at, i.id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY i.created_at DESC, i.id DESC
LIMIT sqlc.arg(page_size);

-- name: CountAccountOpenCaseInquiries :one
SELECT count(*)::int
FROM seller.case_inquiries i
JOIN seller.stores s ON s.store_id = i.store_id
WHERE s.account_id = sqlc.arg(account_id) AND i.reply_received_at IS NULL AND i.withdrawn_at IS NULL;

-- name: ReplyCaseInquiryBySeller :one
UPDATE seller.case_inquiries
SET reply_body = sqlc.arg(body)::text,
    reply_channel = 'SELLER_CENTER',
    reply_received_at = sqlc.arg(now)::timestamptz,
    reply_recorded_at = sqlc.arg(now)::timestamptz,
    replied_by_account_id = sqlc.arg(account_id)::bigint
WHERE id = sqlc.arg(id) AND reply_received_at IS NULL AND withdrawn_at IS NULL
RETURNING *;
