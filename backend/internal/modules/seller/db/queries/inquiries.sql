-- name: InsertCaseInquiry :one
-- 發一筆詢問：同一位管理員的同一個 request_id 已經有時不寫入、回 0 列（不讓 transaction 失效，呼叫方接著讀原本那一筆）。case_inquiries_one_open 只是保底：
-- 呼叫方已鎖住店家列與案件列並以 Availability 擋下，撞到它整筆失敗。
INSERT INTO seller.case_inquiries (id, case_id, store_id, kind, message, created_at, created_by_admin_id, due_at, request_id, request_hash)
VALUES (sqlc.arg(id), sqlc.arg(case_id), sqlc.arg(store_id), sqlc.arg(kind), sqlc.arg(message), sqlc.arg(now), sqlc.arg(admin_id), sqlc.arg(due_at), sqlc.narg(request_id), sqlc.narg(request_hash))
ON CONFLICT (created_by_admin_id, request_id) DO NOTHING
RETURNING *;

-- name: CaseInquiryByRequest :one
-- 冪等重送的查詢：範圍是 (管理員, request_id)。
SELECT *
FROM seller.case_inquiries
WHERE created_by_admin_id = sqlc.arg(admin_id) AND request_id = sqlc.arg(request_id);

-- name: CaseInquiryByID :one
SELECT *
FROM seller.case_inquiries
WHERE id = $1;

-- name: LockCaseInquiry :one
-- 鎖順序：店家列、通報列、案件列、詢問列、措施列。
SELECT *
FROM seller.case_inquiries
WHERE id = $1
FOR UPDATE;

-- name: CaseInquiries :many
-- 案件的詢問，舊到新（keyset）。
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
-- 一批案件還沒結束的詢問（每個案件最多一筆：case_inquiries_one_open）。「等賣家」的定義（還沒回覆也沒撤回）與 model.CaseInquiry.Open 同一個。
SELECT case_id, id
FROM seller.case_inquiries
WHERE case_id = ANY(sqlc.arg(case_ids)::uuid[]) AND reply_received_at IS NULL AND withdrawn_at IS NULL;

-- name: RecordCaseInquiryReply :one
-- 記下回覆；只在還沒結束時寫入，否則 0 列。
UPDATE seller.case_inquiries
SET reply_body = sqlc.arg(body)::text,
    reply_channel = sqlc.arg(channel)::text,
    reply_received_at = sqlc.arg(received_at)::timestamptz,
    reply_recorded_at = sqlc.arg(now)::timestamptz,
    reply_recorded_by_admin_id = sqlc.arg(admin_id)::bigint
WHERE id = sqlc.arg(id) AND reply_received_at IS NULL AND withdrawn_at IS NULL
RETURNING *;

-- name: WithdrawCaseInquiry :one
-- 管理員撤回；只在還沒結束時寫入，否則 0 列。
UPDATE seller.case_inquiries
SET withdrawn_at = sqlc.arg(now)::timestamptz,
    withdrawn_by_admin_id = sqlc.arg(admin_id)::bigint,
    withdraw_cause = 'ADMIN'
WHERE id = sqlc.arg(id) AND reply_received_at IS NULL AND withdrawn_at IS NULL
RETURNING *;

-- name: WithdrawOpenCaseInquiry :many
-- 案件結案（決定，或通報都移走）時，系統撤回案件還沒結束的詢問（最多一筆）；作者是結案的管理員。呼叫方已鎖住案件列。
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
-- 一批詢問回覆的附件，依詢問與位置。
SELECT inquiry_id, position, asset_id, object_key
FROM seller.case_inquiry_reply_evidence
WHERE inquiry_id = ANY(sqlc.arg(inquiry_ids)::uuid[])
ORDER BY inquiry_id, position;

-- name: InsertInquiryEvidenceAccess :exec
-- 詢問回覆附件的讀取紀錄：在讀取之前寫入，以 (inquiry_id, position) 指向那一張附件。
INSERT INTO seller.evidence_accesses (access_id, inquiry_id, position, admin_id, accessed_at)
VALUES (sqlc.arg(access_id), sqlc.arg(inquiry_id), sqlc.arg(position), sqlc.arg(admin_id), sqlc.arg(now));
