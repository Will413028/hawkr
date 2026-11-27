
-- name: GetSellerPublicationView :one
SELECT sqlc.embed(p), sqlc.embed(a), sqlc.embed(rev)
FROM procurement.rfq_publication_invites i
JOIN procurement.rfq_publications p ON p.id = i.publication_id
JOIN procurement.rfq_publication_approvals a ON a.id = p.approval_id
JOIN procurement.request_revisions rev ON rev.id = a.request_revision_id
WHERE i.publication_id = @publication_id AND i.store_id = @store_id;

-- name: ListSellerPublicationViews :many
SELECT sqlc.embed(p), sqlc.embed(a), sqlc.embed(rev)
FROM procurement.rfq_publication_invites i
JOIN procurement.rfq_publications p ON p.id = i.publication_id
JOIN procurement.rfq_publication_approvals a ON a.id = p.approval_id
JOIN procurement.request_revisions rev ON rev.id = a.request_revision_id
WHERE i.store_id = @store_id
  AND (sqlc.narg('after_id')::uuid IS NULL
       OR p.published_at < sqlc.narg('after_published_at')::timestamptz
       OR (p.published_at = sqlc.narg('after_published_at')::timestamptz AND p.id < sqlc.narg('after_id')::uuid))
ORDER BY p.published_at DESC, p.id DESC
LIMIT @page_size;

-- name: ListSellerResponseStates :many
SELECT r.publication_id, r.latest_version, r.withdrawn_version, v.valid_until, (d.seller_response_id IS NOT NULL)::boolean AS has_draft
FROM procurement.seller_responses r
LEFT JOIN procurement.seller_response_versions v ON v.seller_response_id = r.id AND v.version = r.latest_version
LEFT JOIN procurement.seller_response_drafts d ON d.seller_response_id = r.id
WHERE r.store_id = @store_id AND r.publication_id = ANY(@publication_ids::uuid[]);

-- name: EnsureSellerResponse :exec
INSERT INTO procurement.seller_responses (id, publication_id, request_id, store_id, created_at, updated_at)
SELECT @id, i.publication_id, p.request_id, i.store_id, @now, @now
FROM procurement.rfq_publication_invites i
JOIN procurement.rfq_publications p ON p.id = i.publication_id
WHERE i.publication_id = @publication_id AND i.store_id = @store_id
ON CONFLICT (publication_id, store_id) DO NOTHING;

-- name: LockSellerResponseForStore :one
SELECT * FROM procurement.seller_responses WHERE publication_id = @publication_id AND store_id = @store_id FOR UPDATE;

-- name: LockPublicationBuyer :one
SELECT buyer_id FROM procurement.rfq_publications WHERE id = @id FOR SHARE;

-- name: HasActiveAcceptance :one
SELECT EXISTS (SELECT 1 FROM procurement.buyer_quote_acceptances WHERE request_id = @request_id AND state = 'ACTIVE') AS active;

-- name: GetSellerDraft :one
SELECT * FROM procurement.seller_response_drafts WHERE seller_response_id = @seller_response_id;

-- name: InsertSellerDraft :one
INSERT INTO procurement.seller_response_drafts (seller_response_id, payload, version, created_at, updated_at)
VALUES (@seller_response_id, @payload::jsonb, 1, @now, @now)
ON CONFLICT (seller_response_id) DO NOTHING
RETURNING *;

-- name: UpdateSellerDraft :one
UPDATE procurement.seller_response_drafts
SET payload = @payload::jsonb, version = version + 1, updated_at = @now
WHERE seller_response_id = @seller_response_id AND version = @expected_version
RETURNING *;

-- name: GetSellerResponseVersionByKey :one
SELECT * FROM procurement.seller_response_versions WHERE seller_response_id = @seller_response_id AND client_request_id = @client_request_id;

-- name: InsertSellerResponseVersion :one
INSERT INTO procurement.seller_response_versions (
    id, seller_response_id, version, response_type, spec_note, unit_basis_quantity, unit_basis_unit, offered_quantity, offered_quantity_unit,
    conversion_basis, currency, goods_amount, shipping_amount, tax_amount, tax_inclusion_basis, total_amount, lead_time_days,
    promised_delivery_at, valid_until, terms_note, deviation_disclosure, publication_version, client_request_id, submitted_at, created_at)
VALUES (
    @id, @seller_response_id, @version, @response_type, @spec_note, @unit_basis_quantity, @unit_basis_unit, @offered_quantity, @offered_quantity_unit,
    @conversion_basis, @currency, @goods_amount, @shipping_amount, @tax_amount, @tax_inclusion_basis, @total_amount, @lead_time_days,
    @promised_delivery_at, @valid_until, @terms_note, @deviation_disclosure, @publication_version, @client_request_id, @now, @now)
RETURNING *;

-- name: BumpSellerResponseVersion :execrows
UPDATE procurement.seller_responses SET latest_version = @version::int, updated_at = @now::timestamptz
WHERE id = @id AND latest_version = @expected_version::int;

-- name: WithdrawSellerResponse :execrows
UPDATE procurement.seller_responses
SET withdrawn_version = @version::int, withdrawn_at = @now::timestamptz, withdrawn_reason_code = sqlc.narg('reason_code')::text, updated_at = @now::timestamptz
WHERE id = @id AND latest_version = @version::int;

-- name: DeclineSellerResponse :execrows
UPDATE procurement.seller_responses
SET write_closed_at = @now::timestamptz, write_closed_reason = 'DECLINED', write_closed_reason_code = sqlc.narg('reason_code')::text, updated_at = @now::timestamptz
WHERE id = @id AND write_closed_at IS NULL;
