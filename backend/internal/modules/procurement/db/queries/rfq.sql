-- name: InsertPricePreview :execrows
INSERT INTO procurement.price_previews (
    id, buyer_id, request_id, request_revision_id, range_lower_amount, range_upper_amount, basis_quantity, basis_unit,
    total_lower_amount, total_upper_amount, total_quantity, total_unit, client_request_id, expires_at, created_at)
VALUES (
    @id, @buyer_id, @request_id, @request_revision_id, @range_lower_amount, @range_upper_amount, @basis_quantity, @basis_unit,
    @total_lower_amount, @total_upper_amount, @total_quantity, @total_unit, @client_request_id, @expires_at, @now)
ON CONFLICT (buyer_id, client_request_id) DO NOTHING;

-- name: GetPricePreview :one
SELECT * FROM procurement.price_previews WHERE id = @id AND request_id = @request_id AND buyer_id = @buyer_id;

-- name: GetPricePreviewByClientRequest :one
SELECT * FROM procurement.price_previews WHERE buyer_id = @buyer_id AND client_request_id = @client_request_id;

-- name: InsertConfirmedSnapshot :execrows
INSERT INTO procurement.confirmed_rfq_snapshots (
    id, buyer_id, request_id, request_revision_id, price_preview_id, buyer_note, client_request_id, expires_at, created_at)
VALUES (@id, @buyer_id, @request_id, @request_revision_id, @price_preview_id, @buyer_note, @client_request_id, @expires_at, @now)
ON CONFLICT (buyer_id, client_request_id) DO NOTHING;

-- name: GetConfirmedSnapshot :one
SELECT * FROM procurement.confirmed_rfq_snapshots WHERE id = @id AND request_id = @request_id AND buyer_id = @buyer_id;

-- name: GetConfirmedSnapshotByClientRequest :one
SELECT * FROM procurement.confirmed_rfq_snapshots WHERE buyer_id = @buyer_id AND client_request_id = @client_request_id;

-- name: InsertApproval :execrows
INSERT INTO procurement.rfq_publication_approvals (
    id, buyer_id, request_id, request_revision_id, confirmed_rfq_snapshot_id, response_deadline, audience_mode,
    audience_policy_id, audience_policy_version, state, schema_version, client_request_id, generated_at, expires_at, created_at, updated_at)
VALUES (
    @id, @buyer_id, @request_id, @request_revision_id, @confirmed_rfq_snapshot_id, @response_deadline, @audience_mode,
    @audience_policy_id, @audience_policy_version, 'PENDING', @schema_version, @client_request_id, @now, @expires_at, @now, @now)
ON CONFLICT (buyer_id, client_request_id) DO NOTHING;

-- name: GetApproval :one
SELECT * FROM procurement.rfq_publication_approvals WHERE id = @id AND request_id = @request_id AND buyer_id = @buyer_id;

-- name: GetApprovalByClientRequest :one
SELECT * FROM procurement.rfq_publication_approvals WHERE buyer_id = @buyer_id AND client_request_id = @client_request_id;

-- name: ClaimApprovalForExecute :one
-- PENDING 推進到 EXECUTING；(id, version, buyer_id, state) 四項條件沒命中任何列就是 CAS 失敗（過期的 version 也要擋）。
UPDATE procurement.rfq_publication_approvals
SET state = 'EXECUTING', version = version + 1, updated_at = @now
WHERE id = @id AND version = @version AND buyer_id = @buyer_id AND state = 'PENDING'
RETURNING *;

-- name: MarkApprovalSucceeded :execrows
UPDATE procurement.rfq_publication_approvals
SET state = 'SUCCEEDED', version = version + 1, updated_at = @now
WHERE id = @id AND version = @version AND state = 'EXECUTING';

-- name: InsertApprovalRecipient :exec
INSERT INTO procurement.rfq_publication_approval_recipients (approval_id, position, store_id)
VALUES (@approval_id, @position, @store_id);

-- name: ListApprovalRecipients :many
SELECT * FROM procurement.rfq_publication_approval_recipients WHERE approval_id = @approval_id ORDER BY position;

-- name: InsertPublication :execrows
INSERT INTO procurement.rfq_publications (
    id, buyer_id, request_id, request_revision_id, approval_id, schema_version, client_request_id, published_at, created_at, updated_at)
VALUES (@id, @buyer_id, @request_id, @request_revision_id, @approval_id, @schema_version, @client_request_id, @now, @now, @now)
ON CONFLICT DO NOTHING;

-- name: GetPublication :one
SELECT * FROM procurement.rfq_publications WHERE id = @id;

-- name: GetPublicationByClientRequest :one
SELECT * FROM procurement.rfq_publications WHERE buyer_id = @buyer_id AND client_request_id = @client_request_id;

-- name: InsertInvite :exec
INSERT INTO procurement.rfq_publication_invites (id, publication_id, position, store_id, delivery_state, delivery_state_at, created_at, updated_at)
VALUES (@id, @publication_id, @position, @store_id, 'QUEUED', @now, @now, @now);

-- name: ListInvites :many
SELECT * FROM procurement.rfq_publication_invites WHERE publication_id = @publication_id ORDER BY position;

-- name: MarkInviteSent :execrows
UPDATE procurement.rfq_publication_invites SET delivery_state = 'SENT', delivery_state_at = @now, updated_at = @now
WHERE id = @id AND delivery_state = 'QUEUED';

-- name: MarkInviteFailed :execrows
UPDATE procurement.rfq_publication_invites SET delivery_state = 'FAILED', delivery_state_at = @now, updated_at = @now
WHERE id = @id AND delivery_state = 'QUEUED';

-- name: InsertSellerResponse :exec
INSERT INTO procurement.seller_responses (id, publication_id, request_id, store_id, created_at, updated_at)
VALUES (@id, @publication_id, @request_id, @store_id, @now, @now);

-- name: GetSellerResponse :one
SELECT * FROM procurement.seller_responses WHERE publication_id = @publication_id AND store_id = @store_id;

-- name: InsertAcceptance :execrows
-- live 鎖（buyer_quote_acceptances_live）只擋 state = 'ACTIVE'：同一張需求已有 ACTIVE 的接受單時回 0 列。
-- 冪等鍵相同也回 0 列，呼叫端用 GetAcceptanceByClientRequest 區分。
INSERT INTO procurement.buyer_quote_acceptances (
    id, buyer_id, request_id, publication_id, seller_response_id, seller_response_version, seller_response_payload_hash,
    confirmed_rfq_payload_hash, store_id, currency, goods_amount, shipping_amount, tax_amount, accepted_deal_price,
    client_request_id, accepted_at, created_at, updated_at)
VALUES (
    @id, @buyer_id, @request_id, @publication_id, @seller_response_id, @seller_response_version, @seller_response_payload_hash,
    @confirmed_rfq_payload_hash, @store_id, @currency, @goods_amount, @shipping_amount, @tax_amount, @accepted_deal_price,
    @client_request_id, @now, @now, @now)
ON CONFLICT DO NOTHING;

-- name: GetAcceptanceByClientRequest :one
SELECT * FROM procurement.buyer_quote_acceptances WHERE buyer_id = @buyer_id AND client_request_id = @client_request_id;

-- name: GetLiveAcceptance :one
SELECT * FROM procurement.buyer_quote_acceptances WHERE request_id = @request_id AND state = 'ACTIVE';

-- name: MarkOrderCreated :execrows
UPDATE procurement.buyer_quote_acceptances
SET order_provisioning_state = 'ORDER_CREATED', order_no = @order_no, payment_deadline = @payment_deadline, updated_at = @now
WHERE id = @id AND order_provisioning_state = 'CREATING_ORDER';

-- name: MarkOrderFailed :execrows
-- 建單的 River job 重試用盡時呼叫（A9-D19 S6 修訂）：同時轉成 ORDER_FAILED 並解除 live 鎖。
UPDATE procurement.buyer_quote_acceptances
SET state = 'ORDER_FAILED', order_provisioning_state = 'ORDER_FAILED', order_failure_reason = @reason, updated_at = @now
WHERE id = @id AND order_provisioning_state = 'CREATING_ORDER';
