
-- name: UpsertProcurementSellerResponseIdentityPlatform :one
INSERT INTO procurement_seller_responses (
    id, publication_id, buyer_id, request_id, lane, store_id
) VALUES (
    $1, $2, $3, $4, 'PLATFORM', $5
)
ON CONFLICT (publication_id, store_id) WHERE store_id IS NOT NULL
DO UPDATE SET publication_id = EXCLUDED.publication_id
RETURNING *;

-- name: UpsertProcurementSellerResponseIdentityExternal :one
INSERT INTO procurement_seller_responses (
    id, publication_id, buyer_id, request_id, lane, prospect_id
) VALUES (
    $1, $2, $3, $4, 'EXTERNAL', $5
)
ON CONFLICT (publication_id, prospect_id) WHERE prospect_id IS NOT NULL
DO UPDATE SET publication_id = EXCLUDED.publication_id
RETURNING *;

-- name: FindProcurementSellerResponseByID :one
SELECT * FROM procurement_seller_responses WHERE id = $1;

-- name: FindProcurementSellerResponseByIDForUpdate :one
-- ⛔ acceptance 的 CAS 讀必須在寫入的同一個 transaction 內、且鎖住 identity 列。
-- 在交易外讀 latest_version／withdrawn_version，賣家的改版或撤回可以在「讀完」與
-- 「寫入」之間 commit，acceptance 就落在一個已被 supersede 的版本上（doc 64 §2-3）。
-- 賣家側 submit／withdraw 走的是同一列的樂觀版本 CAS，鎖序一致。
SELECT * FROM procurement_seller_responses WHERE id = $1 FOR UPDATE;

-- name: FindProcurementSellerResponseByPlatformIdentity :one
SELECT * FROM procurement_seller_responses
WHERE publication_id = $1 AND store_id = $2;

-- name: FindProcurementSellerResponseByExternalIdentity :one
SELECT * FROM procurement_seller_responses
WHERE publication_id = $1 AND prospect_id = $2;

-- name: UpsertProcurementSellerResponseDraft :one
INSERT INTO procurement_seller_response_drafts (
    seller_response_id, payload, version
) VALUES (
    $1, $2, 1
)
ON CONFLICT (seller_response_id) DO UPDATE
   SET payload    = EXCLUDED.payload,
       version    = procurement_seller_response_drafts.version + 1,
       updated_at = now()
 WHERE procurement_seller_response_drafts.version = $3
RETURNING *;

-- name: FindProcurementSellerResponseDraft :one
SELECT * FROM procurement_seller_response_drafts WHERE seller_response_id = $1;

-- name: InsertProcurementSellerResponseVersion :one
INSERT INTO procurement_seller_response_versions (
    id, seller_response_id, version, response_type,
    offered_quality_grade, offered_quality_inspection_notes,
    unit_basis_quantity, unit_basis_unit,
    offered_quantity, offered_quantity_unit, conversion_basis,
    currency, goods_amount, shipping_amount, tax_amount,
    tax_inclusion_basis, total_amount,
    lead_time_days, promised_delivery_at, valid_until,
    terms_note, deviation_disclosure, outside_buyer_requested_range,
    payload_hash, publication_version, seller_facing_payload_hash,
    client_request_id, submitted_at,
    verification_method, verification_email, verified_at, verification_session_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22, $23, $24, $25, $26, $27, $28,
    $29, $30, $31, $32
)
ON CONFLICT (seller_response_id, client_request_id)
DO UPDATE SET client_request_id = EXCLUDED.client_request_id
RETURNING *;

-- name: FindProcurementSellerResponseVersion :one
SELECT * FROM procurement_seller_response_versions
WHERE seller_response_id = $1 AND version = $2;

-- name: ListProcurementSellerResponseVersions :many
SELECT * FROM procurement_seller_response_versions
WHERE seller_response_id = $1
ORDER BY version;

-- name: BumpProcurementSellerResponseLatestVersion :one
UPDATE procurement_seller_responses
   SET latest_version = $3, version = version + 1, updated_at = now()
 WHERE id = $1 AND version = $2 AND write_closed_at IS NULL
RETURNING *;

-- name: WithdrawProcurementSellerResponse :one
UPDATE procurement_seller_responses
   SET withdrawn_version = $3, withdrawn_at = now(),
       version = version + 1, updated_at = now()
 WHERE id = $1 AND version = $2 AND latest_version = $3
   AND latest_version > 0 AND write_closed_at IS NULL
RETURNING *;

-- name: CloseProcurementSellerResponseWrite :one
UPDATE procurement_seller_responses
   SET write_closed_at = now(), write_closed_reason = $3,
       version = version + 1, updated_at = now()
 WHERE id = $1 AND version = $2 AND write_closed_at IS NULL
RETURNING *;

-- name: ListProcurementSellerResponseVersionsForRequest :many
-- 買家比較清單：這張 request 底下所有賣家的所有 immutable 版本。
-- 舊版本刻意留在清單裡（V.11.5：withdrawn／expired／superseded 留在 audit 但不可接受），
-- 可否接受由讀取端推導成 typed acceptability，不在這裡過濾。
SELECT sqlc.embed(sr), sqlc.embed(v)
FROM procurement_seller_responses sr
JOIN procurement_seller_response_versions v ON v.seller_response_id = sr.id
WHERE sr.request_id = sqlc.arg(request_id) AND sr.buyer_id = sqlc.arg(buyer_id)
ORDER BY v.submitted_at DESC, sr.id DESC, v.version DESC;

-- name: FindProcurementSellerResponseVersionByIdempotencyKey :one
-- 回放查找。⛔ 呼叫端必須在 CAS 守門之前問，否則 ACK 遺失後的重試會被判成「已提交過」。
SELECT * FROM procurement_seller_response_versions
WHERE seller_response_id = $1 AND client_request_id = $2;

-- name: InsertProcurementSellerResponseCommand :one
-- 只追加：這張表沒有任何 UPDATE／DELETE 查詢。撞到冪等鍵不寫入、不回列，呼叫端改查既有那列比對內容。
INSERT INTO procurement_seller_response_commands (
    id, seller_response_id, kind, withdrawn_version, client_request_id, reason_code,
    verification_method, verification_email, verified_at, verification_session_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (seller_response_id, client_request_id) DO NOTHING
RETURNING *;

-- name: FindProcurementSellerResponseCommandByIdempotencyKey :one
SELECT * FROM procurement_seller_response_commands
WHERE seller_response_id = $1 AND client_request_id = $2;

-- name: ListProcurementSellerResponseCommands :many
SELECT * FROM procurement_seller_response_commands
WHERE seller_response_id = $1
ORDER BY created_at, id;
