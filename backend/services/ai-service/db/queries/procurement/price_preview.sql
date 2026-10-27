-- name: InsertPricePreview :one
INSERT INTO procurement_price_previews (
    id, buyer_id, request_id, request_revision_id, pricing_mode,
    quality_knowledge_state, reference_availability, reference_facts_hash,
    reference_payload_json, unavailable_reason, buyer_requested_range_json,
    deal_temperature_json, market_price_basis_json, quote_total_range_json,
    client_request_id, preview_input_hash,
    payload_hash, version, expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
)
-- 撞冪等鍵時不拋錯（拋錯會讓呼叫端的交易整段 aborted，見 PostgreSQL 規範第十二章）。
-- DO UPDATE 把冪等鍵寫回自己：不改任何內容，但保證一定 RETURNING 一列，呼叫端用
-- 「回傳的 id 是不是這次產的」區分插入與既有列。
ON CONFLICT (buyer_id, client_request_id)
DO UPDATE SET client_request_id = EXCLUDED.client_request_id
RETURNING *;

-- name: FindPricePreviewByIdempotencyKey :one
SELECT * FROM procurement_price_previews
WHERE buyer_id = $1 AND client_request_id = $2;

-- name: FindPricePreviewByIDAndBuyer :one
SELECT * FROM procurement_price_previews
WHERE id = $1 AND buyer_id = $2;

-- name: CountPricePreviewsByRequestID :one
SELECT count(*) FROM procurement_price_previews WHERE request_id = $1;
