-- name: CreateProcurementRequest :one
INSERT INTO procurement_requests (id, buyer_id)
VALUES ($1, $2)
RETURNING *;

-- name: CreateProcurementRevision :one
INSERT INTO procurement_request_revisions (
  id, request_id, revision_no, parent_revision_id, branch_id, slots_json, slots_hash, status
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: SetProcurementRequestActiveRevision :exec
UPDATE procurement_requests
SET active_revision_id = $2, updated_at = now()
WHERE id = $1;

-- name: GetProcurementRequestForUpdate :one
-- 交易內 row lock：序列化同一 request_id 的併發 AppendRevision，避免兩者讀到同一個
-- parent revision、算出同一個 revision_no（撞 uq_procurement_request_revisions_no）。
-- 只用來取得鎖，回傳值本身不需要讀取任何欄位。
SELECT * FROM procurement_requests
WHERE id = $1 AND buyer_id = $2
FOR UPDATE;

-- name: GetActiveProcurementRevision :one
SELECT * FROM procurement_request_revisions
WHERE request_id = $1 AND status = 'ACTIVE';

-- name: SupersedeProcurementRevision :exec
UPDATE procurement_request_revisions
SET status = 'SUPERSEDED'
WHERE id = $1;

-- name: CreateProcurementAnswerPatch :one
INSERT INTO procurement_answer_patches (
  id, request_id, buyer_id, base_revision_id, result_revision_id, source, payload_json, client_request_id
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
)
-- 撞冪等鍵不拋錯（PostgreSQL 規範第十二章）。用 DO UPDATE 而不是 DO NOTHING：DO NOTHING
-- 遇到並發「已插入但未提交」的列不等待、直接跳過，之後的復原查詢也看不到那一列；
-- DO UPDATE 會等對方提交，復原因此保證查得到——這是原本靠 23505 阻塞取得的語意。
ON CONFLICT (buyer_id, client_request_id)
DO UPDATE SET client_request_id = EXCLUDED.client_request_id
RETURNING *;

-- name: BumpProcurementRequestRevision :exec
UPDATE procurement_requests
SET active_revision_id = $2, version = version + 1, updated_at = now()
WHERE id = $1;

-- name: FindProcurementAnswerPatchByIdempotencyKey :one
SELECT * FROM procurement_answer_patches
WHERE buyer_id = $1 AND client_request_id = $2;

-- name: GetProcurementRevisionByID :one
SELECT * FROM procurement_request_revisions
WHERE id = $1;

-- name: GetProcurementRequestSnapshot :one
SELECT * FROM procurement_requests
WHERE id = $1 AND buyer_id = $2;

-- name: SetProcurementRequestStatus :exec
UPDATE procurement_requests
SET status = $2, updated_at = now()
WHERE id = $1;

-- name: ListProcurementRevisionsByRequestID :many
SELECT * FROM procurement_request_revisions
WHERE request_id = $1
ORDER BY revision_no ASC;
