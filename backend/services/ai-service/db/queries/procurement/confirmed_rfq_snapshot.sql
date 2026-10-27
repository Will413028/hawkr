-- name: InsertConfirmedRfqSnapshot :one
INSERT INTO procurement_confirmed_rfq_snapshots (
    id, buyer_id, request_id, request_revision_id, price_preview_id,
    price_preview_version, quality_knowledge_state, pricing_mode,
    canonical_payload_json, payload_hash, client_request_id, expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
-- 撞冪等鍵不拋錯（PostgreSQL 規範第十二章）。DO UPDATE 把冪等鍵寫回自己：不改內容，
-- 但保證一定 RETURNING 一列，呼叫端用「回傳的 id 是不是這次產的」區分插入與既有列。
ON CONFLICT (buyer_id, client_request_id)
DO UPDATE SET client_request_id = EXCLUDED.client_request_id
RETURNING *;

-- name: FindConfirmedRfqSnapshotByIdempotencyKey :one
SELECT * FROM procurement_confirmed_rfq_snapshots
WHERE buyer_id = $1 AND client_request_id = $2;

-- name: FindLatestConfirmedRfqSnapshotByRequestID :one
SELECT * FROM procurement_confirmed_rfq_snapshots
WHERE request_id = $1 AND buyer_id = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: FindConfirmedRfqSnapshotByIDAndBuyer :one
SELECT * FROM procurement_confirmed_rfq_snapshots
WHERE id = $1 AND buyer_id = $2;
