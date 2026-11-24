-- name: InsertApproval :execrows
INSERT INTO procurement.rfq_publication_approvals (
    id, buyer_id, request_id, request_revision_id, range_lower_amount, range_upper_amount, buyer_note, candidate_set_id, response_deadline,
    state, schema_version, client_request_id, generated_at, expires_at, created_at, updated_at)
VALUES (
    @id, @buyer_id, @request_id, @request_revision_id, @range_lower_amount, @range_upper_amount, @buyer_note, @candidate_set_id, @response_deadline,
    'PENDING', @schema_version, @client_request_id, @now, @expires_at, @now, @now)
ON CONFLICT (buyer_id, client_request_id) DO NOTHING;

-- name: GetApproval :one
SELECT * FROM procurement.rfq_publication_approvals WHERE id = @id AND request_id = @request_id AND buyer_id = @buyer_id;

-- name: GetApprovalForBuyer :one
-- 只依擁有者查（不收斂到需求）：核准屬於買家的另一張需求與核准不存在要分得開（前者是過時，後者是 NOT_FOUND）。
SELECT * FROM procurement.rfq_publication_approvals WHERE id = @id AND buyer_id = @buyer_id;

-- name: GetApprovalByClientRequest :one
SELECT * FROM procurement.rfq_publication_approvals WHERE buyer_id = @buyer_id AND client_request_id = @client_request_id;

-- name: ClaimApprovalForExecute :one
-- PENDING 推進到 EXECUTING；(id, version, buyer_id, state, expires_at) 沒命中任何列就是 CAS 失敗（過期的 version 與過期的核准都要擋）。
UPDATE procurement.rfq_publication_approvals
SET state = 'EXECUTING', version = version + 1, updated_at = @now
WHERE id = @id AND version = @version AND buyer_id = @buyer_id AND state = 'PENDING' AND expires_at > @now::timestamptz
RETURNING *;

-- name: MarkApprovalSucceeded :execrows
-- 不帶 buyer_id：(id, version, state) 已是 CAS，擁有者在 ClaimApprovalForExecute 驗過；BuyerDeleted 把 buyer_id 設 NULL 後仍要能收尾。
UPDATE procurement.rfq_publication_approvals
SET state = 'SUCCEEDED', version = version + 1, updated_at = @now
WHERE id = @id AND version = @version AND state = 'EXECUTING';

-- name: InsertApprovalRecipient :exec
INSERT INTO procurement.rfq_publication_approval_recipients (approval_id, position, store_id)
VALUES (@approval_id, @position, @store_id);

-- name: ListApprovalRecipients :many
SELECT r.* FROM procurement.rfq_publication_approval_recipients r
JOIN procurement.rfq_publication_approvals a ON a.id = r.approval_id
WHERE r.approval_id = @approval_id AND a.buyer_id = @buyer_id
ORDER BY r.position;

-- name: InsertPublication :execrows
INSERT INTO procurement.rfq_publications (
    id, buyer_id, request_id, request_revision_id, approval_id, approval_version, schema_version, client_request_id, published_at, created_at, updated_at)
VALUES (@id, @buyer_id, @request_id, @request_revision_id, @approval_id, @approval_version, @schema_version, @client_request_id, @now, @now, @now)
ON CONFLICT DO NOTHING;

-- name: GetPublication :one
SELECT * FROM procurement.rfq_publications WHERE id = @id AND buyer_id = @buyer_id;

-- name: GetPublicationByClientRequest :one
SELECT * FROM procurement.rfq_publications WHERE buyer_id = @buyer_id AND client_request_id = @client_request_id;

-- name: InsertInvite :exec
INSERT INTO procurement.rfq_publication_invites (id, publication_id, position, store_id, delivery_state, delivery_state_at, created_at, updated_at)
VALUES (@id, @publication_id, @position, @store_id, 'QUEUED', @now, @now, @now);

-- name: ListInvites :many
SELECT i.* FROM procurement.rfq_publication_invites i
JOIN procurement.rfq_publications p ON p.id = i.publication_id
WHERE i.publication_id = @publication_id AND p.buyer_id = @buyer_id
ORDER BY i.position;

-- name: MarkInviteSent :execrows
UPDATE procurement.rfq_publication_invites SET delivery_state = 'SENT', delivery_state_at = @now, updated_at = @now
WHERE id = @id AND delivery_state = 'QUEUED';

-- name: MarkInviteFailed :execrows
UPDATE procurement.rfq_publication_invites SET delivery_state = 'FAILED', delivery_state_at = @now, updated_at = @now
WHERE id = @id AND delivery_state = 'QUEUED';

-- name: ListStaleQueuedInvites :many
SELECT id FROM procurement.rfq_publication_invites
WHERE delivery_state = 'QUEUED' AND created_at < @before
ORDER BY created_at, id
LIMIT @max_rows;

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
    seller_facing_hash, store_id, currency, goods_amount, shipping_amount, tax_amount, accepted_deal_price,
    client_request_id, accepted_at, created_at, updated_at)
VALUES (
    @id, @buyer_id, @request_id, @publication_id, @seller_response_id, @seller_response_version, @seller_response_payload_hash,
    @seller_facing_hash, @store_id, @currency, @goods_amount, @shipping_amount, @tax_amount, @accepted_deal_price,
    @client_request_id, @now, @now, @now)
ON CONFLICT DO NOTHING;

-- name: GetAcceptanceByClientRequest :one
SELECT * FROM procurement.buyer_quote_acceptances WHERE buyer_id = @buyer_id AND client_request_id = @client_request_id;

-- name: GetLiveAcceptance :one
SELECT * FROM procurement.buyer_quote_acceptances WHERE request_id = @request_id AND buyer_id = @buyer_id AND state = 'ACTIVE';

-- name: MarkOrderCreated :execrows
-- 建單 worker 專用（系統內部，免 buyer_id）：以 job 引數裡的接受單 id 定位，且買家關閉後接受單的 buyer_id 會被設為 NULL，帶 buyer_id 會讓已建好的單記不回來。
UPDATE procurement.buyer_quote_acceptances
SET order_provisioning_state = 'ORDER_CREATED', order_no = @order_no, payment_deadline = @payment_deadline, updated_at = @now
WHERE id = @id AND order_provisioning_state = 'CREATING_ORDER';

-- name: MarkOrderFailed :execrows
-- 建單的 River job 重試用盡時呼叫（A9-D19 S6 修訂）：同時轉成 ORDER_FAILED 並解除 live 鎖。
UPDATE procurement.buyer_quote_acceptances
SET state = 'ORDER_FAILED', order_provisioning_state = 'ORDER_FAILED', order_failure_reason = @reason, updated_at = @now
WHERE id = @id AND order_provisioning_state = 'CREATING_ORDER';

-- name: GetLatestPendingApprovalByRequest :one
-- 需求最新的、尚未過期的待執行核准（不論對應哪一版）；沒有回 ErrNoRows。planner 用它判斷詢價階段（A9-D35）。
SELECT * FROM procurement.rfq_publication_approvals
WHERE request_id = @request_id AND buyer_id = @buyer_id AND state = 'PENDING' AND expires_at > @now::timestamptz
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: GetLatestPublicationByRequest :one
-- 需求最新一次的發布，連同核准的 ID、凍結的受眾數與邀請數，以及發布的 schema_version（重算 hash 的原像版本）；讀取模型 GetRuntimeSnapshot.published_rfq 用。
SELECT p.id, p.version, p.published_at, p.schema_version, a.id AS approval_id,
    (SELECT count(*) FROM procurement.rfq_publication_approval_recipients r WHERE r.approval_id = a.id)::int AS recipient_count,
    (SELECT count(*) FROM procurement.rfq_publication_invites i WHERE i.publication_id = p.id)::int AS queued_count
FROM procurement.rfq_publications p
JOIN procurement.rfq_publication_approvals a ON a.id = p.approval_id
WHERE p.request_id = @request_id AND p.buyer_id = @buyer_id
ORDER BY p.published_at DESC, p.id DESC
LIMIT 1;
