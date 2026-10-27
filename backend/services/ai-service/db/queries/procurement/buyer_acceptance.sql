
-- name: UpsertProcurementBuyerShortlistEntry :one
INSERT INTO procurement_buyer_shortlist_entries (
    id, buyer_id, request_id, seller_response_id, seller_response_version
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (request_id, seller_response_id) DO UPDATE
   SET seller_response_version = EXCLUDED.seller_response_version,
       updated_at              = now()
RETURNING *;

-- name: DeleteProcurementBuyerShortlistEntry :execrows
DELETE FROM procurement_buyer_shortlist_entries
WHERE request_id = $1 AND seller_response_id = $2 AND buyer_id = $3;

-- name: ListProcurementBuyerShortlistEntries :many
SELECT * FROM procurement_buyer_shortlist_entries
WHERE request_id = $1 AND buyer_id = $2
ORDER BY created_at, id;

-- name: InsertProcurementBuyerQuoteAcceptance :one
INSERT INTO procurement_buyer_quote_acceptances (
    id, buyer_id, request_id, publication_id,
    seller_response_id, seller_response_version, seller_response_payload_hash,
    confirmed_rfq_snapshot_id, confirmed_rfq_payload_hash,
    lane, store_id, prospect_id,
    accepted_deal_price, currency, goods_amount, shipping_amount, tax_amount,
    state, order_provisioning_state,
    client_request_id, request_hash, accepted_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17,
    'ACTIVE', 'CREATING_ORDER',
    $18, $19, $20
)
ON CONFLICT (buyer_id, client_request_id)
DO UPDATE SET client_request_id = EXCLUDED.client_request_id
RETURNING *;

-- name: FindProcurementBuyerQuoteAcceptanceByIdempotencyKey :one
SELECT * FROM procurement_buyer_quote_acceptances
WHERE buyer_id = $1 AND client_request_id = $2;

-- name: FindProcurementBuyerQuoteAcceptanceByIDAndBuyer :one
SELECT * FROM procurement_buyer_quote_acceptances
WHERE id = $1 AND buyer_id = $2;

-- name: FindLiveProcurementBuyerQuoteAcceptanceForRequest :one
SELECT * FROM procurement_buyer_quote_acceptances
WHERE request_id = $1 AND buyer_id = $2
  AND state IN ('ACTIVE', 'PAYMENT_FINALIZED');

-- name: AttachOrderToProcurementBuyerQuoteAcceptance :one
UPDATE procurement_buyer_quote_acceptances
   SET order_provisioning_state = 'ORDER_CREATED',
       order_no                 = $3,
       payment_deadline         = $4,
       version                  = version + 1,
       updated_at               = now()
 WHERE id = $1 AND version = $2
   AND order_provisioning_state = 'CREATING_ORDER'
RETURNING *;

-- name: MarkProcurementBuyerQuoteAcceptanceOrderFailed :one
-- 終態拒絕：worker 拿到 order-service 的終態錯誤時，在**知道 version** 的路徑上 CAS 回寫。
-- order_failure_reason 是粗粒度成因（doc 64 §6.4.14 的五個值），細節留在
-- procurement_order_provisioning_outbox.last_error——同一個 DB、acceptance_id 可直接 join。
UPDATE procurement_buyer_quote_acceptances
   SET order_provisioning_state = 'ORDER_FAILED',
       order_failure_reason     = $3,
       version                  = version + 1,
       updated_at               = now()
 WHERE id = $1 AND version = $2
   AND order_provisioning_state = 'CREATING_ORDER'
RETURNING *;

-- name: MarkProcurementBuyerQuoteAcceptanceOrderFailedByOutboxID :exec
-- 重試耗盡：outbox 列轉 'failed' 的**同一個交易**內把 acceptance 轉 ORDER_FAILED。
-- ⛔ 不可改用 pkg/outbox.Hooks：OnAttemptFailed 每次嘗試失敗都觸發（第一次暫時性失敗就會
--    把 acceptance 打成終態，而它其實還會被重試）；OnTerminalFailed 雖只在終態觸發，但簽名
--    帶 markFailedErr ⇒ 它在 MarkFailed 回來**之後**才跑，中間留下「outbox 已 failed、
--    acceptance 仍 CREATING_ORDER」的窗口。與 publication outbox 的 invite 是同一個理由。
-- ⚠️ 這裡用 order_provisioning_state 當 CAS 述詞而不是 version：本路徑手上只有 outbox 列的
--    id，而 CREATING_ORDER 是唯一的非終態 ⇒ 述詞本身就是「還沒有人寫過」。影響 0 列代表
--    別的副本先寫了，視為成功（同 §6.4.13 對 CAS 影響 0 列的處置）。
UPDATE procurement_buyer_quote_acceptances
   SET order_provisioning_state = 'ORDER_FAILED',
       order_failure_reason     = $2,
       version                  = version + 1,
       updated_at               = now()
 WHERE id = (SELECT sub.acceptance_id FROM procurement_order_provisioning_outbox AS sub WHERE sub.id = $1)
   AND order_provisioning_state = 'CREATING_ORDER';

-- name: ReleaseProcurementBuyerQuoteAcceptance :one
UPDATE procurement_buyer_quote_acceptances
   SET state      = $3,
       version    = version + 1,
       updated_at = now()
 WHERE id = $1 AND version = $2 AND state = 'ACTIVE'
RETURNING *;

-- name: MarkProcurementBuyerQuoteAcceptancePaymentFinalized :one
UPDATE procurement_buyer_quote_acceptances
   SET state      = 'PAYMENT_FINALIZED',
       version    = version + 1,
       updated_at = now()
 WHERE id = $1 AND version = $2 AND state = 'ACTIVE'
   AND order_provisioning_state = 'ORDER_CREATED'
RETURNING *;
