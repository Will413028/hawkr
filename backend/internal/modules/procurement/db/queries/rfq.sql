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
SELECT * FROM procurement.rfq_publication_approvals WHERE id = @id AND buyer_id = @buyer_id;

-- name: GetApprovalByClientRequest :one
SELECT * FROM procurement.rfq_publication_approvals WHERE buyer_id = @buyer_id AND client_request_id = @client_request_id;

-- name: ClaimApprovalForExecute :one
UPDATE procurement.rfq_publication_approvals
SET state = 'EXECUTING', version = version + 1, updated_at = @now
WHERE id = @id AND version = @version AND buyer_id = @buyer_id AND state = 'PENDING' AND expires_at > @now::timestamptz
RETURNING *;

-- name: MarkApprovalSucceeded :execrows
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
INSERT INTO procurement.rfq_publication_invites (id, publication_id, position, store_id, created_at, updated_at)
VALUES (@id, @publication_id, @position, @store_id, @now, @now);

-- name: ListInvites :many
SELECT i.* FROM procurement.rfq_publication_invites i
JOIN procurement.rfq_publications p ON p.id = i.publication_id
WHERE i.publication_id = @publication_id AND p.buyer_id = @buyer_id
ORDER BY i.position;

-- name: InsertSellerResponse :exec
INSERT INTO procurement.seller_responses (id, publication_id, request_id, store_id, created_at, updated_at)
VALUES (@id, @publication_id, @request_id, @store_id, @now, @now);

-- name: GetSellerResponse :one
SELECT * FROM procurement.seller_responses WHERE publication_id = @publication_id AND store_id = @store_id;

-- name: InsertAcceptance :execrows
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
UPDATE procurement.buyer_quote_acceptances
SET order_provisioning_state = 'ORDER_CREATED', order_no = @order_no, payment_deadline = @payment_deadline, updated_at = @now
WHERE id = @id AND order_provisioning_state = 'CREATING_ORDER';

-- name: MarkOrderFailed :execrows
UPDATE procurement.buyer_quote_acceptances
SET state = 'ORDER_FAILED', order_provisioning_state = 'ORDER_FAILED', order_failure_reason = @reason, updated_at = @now
WHERE id = @id AND order_provisioning_state = 'CREATING_ORDER';

-- name: GetLatestPendingApprovalByRequest :one
SELECT * FROM procurement.rfq_publication_approvals
WHERE request_id = @request_id AND buyer_id = @buyer_id AND state = 'PENDING' AND expires_at > @now::timestamptz
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: GetLatestPublicationByRequest :one
SELECT p.id, p.version, p.published_at, p.schema_version, a.id AS approval_id,
    (SELECT count(*) FROM procurement.rfq_publication_approval_recipients r WHERE r.approval_id = a.id)::int AS recipient_count,
    (SELECT count(*) FROM procurement.rfq_publication_invites i WHERE i.publication_id = p.id)::int AS queued_count
FROM procurement.rfq_publications p
JOIN procurement.rfq_publication_approvals a ON a.id = p.approval_id
WHERE p.request_id = @request_id AND p.buyer_id = @buyer_id
ORDER BY p.published_at DESC, p.id DESC
LIMIT 1;

-- name: SetAcceptanceOrderJob :exec
UPDATE procurement.buyer_quote_acceptances SET order_job_id = @job_id WHERE id = @id;

-- name: GetOrderBasis :one
SELECT a.id, a.buyer_id, a.store_id, a.goods_amount, a.shipping_amount, a.tax_amount, a.accepted_deal_price,
    a.order_provisioning_state, a.order_ref,
    rev.item, rev.item_resolution_label,
    trim_scale(v.offered_quantity)::text AS offered_quantity, v.offered_quantity_unit
FROM procurement.buyer_quote_acceptances a
JOIN procurement.rfq_publications p ON p.id = a.publication_id
JOIN procurement.rfq_publication_approvals ap ON ap.id = p.approval_id
JOIN procurement.request_revisions rev ON rev.id = ap.request_revision_id
JOIN procurement.seller_response_versions v ON v.seller_response_id = a.seller_response_id AND v.version = a.seller_response_version
WHERE a.id = @id;

-- name: ListCreatingOrderIDsAfter :many
SELECT id FROM procurement.buyer_quote_acceptances
WHERE order_provisioning_state = 'CREATING_ORDER' AND id > @after
ORDER BY id LIMIT @batch;

-- name: LockCreatingAcceptance :one
SELECT id, order_job_id FROM procurement.buyer_quote_acceptances
WHERE id = @id AND order_provisioning_state = 'CREATING_ORDER' FOR UPDATE;

-- name: TryLockOrderProvisioning :one
SELECT pg_try_advisory_xact_lock(hashtextextended('procurement.order_provisioning:' || sqlc.arg(id)::uuid::text, 0))::boolean AS locked;
