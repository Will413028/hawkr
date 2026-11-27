-- name: ListCurrentSellerResponses :many
SELECT sqlc.embed(r), sqlc.embed(v), (s.id IS NOT NULL)::boolean AS shortlisted
FROM procurement.seller_responses r
JOIN procurement.rfq_publications p ON p.id = r.publication_id
JOIN procurement.seller_response_versions v ON v.seller_response_id = r.id AND v.version = r.latest_version
LEFT JOIN procurement.buyer_shortlist_entries s ON s.seller_response_id = r.id AND s.request_id = r.request_id AND s.seller_response_version = v.version
WHERE r.request_id = @request_id AND r.publication_id = @publication_id AND p.buyer_id = @buyer_id AND r.latest_version > 0
  AND (sqlc.narg('after_id')::uuid IS NULL
       OR r.created_at < sqlc.narg('after_created_at')::timestamptz
       OR (r.created_at = sqlc.narg('after_created_at')::timestamptz AND r.id < sqlc.narg('after_id')::uuid))
ORDER BY r.created_at DESC, r.id DESC
LIMIT @page_size;

-- name: GetSellerResponseForBuyer :one
SELECT r.* FROM procurement.seller_responses r
JOIN procurement.rfq_publications p ON p.id = r.publication_id
WHERE r.id = @id AND r.request_id = @request_id AND p.buyer_id = @buyer_id;

-- name: LockSellerResponseForBuyer :one
SELECT r.* FROM procurement.seller_responses r
JOIN procurement.rfq_publications p ON p.id = r.publication_id
WHERE r.id = @id AND r.request_id = @request_id AND p.buyer_id = @buyer_id
FOR UPDATE OF r;

-- name: GetSellerResponseVersion :one
SELECT * FROM procurement.seller_response_versions WHERE seller_response_id = @seller_response_id AND version = @version;

-- name: PutShortlistEntry :one
INSERT INTO procurement.buyer_shortlist_entries (id, request_id, seller_response_id, seller_response_version, created_at, updated_at)
VALUES (@id, @request_id, @seller_response_id, @seller_response_version, @now, @now)
ON CONFLICT (request_id, seller_response_id) DO UPDATE SET seller_response_version = EXCLUDED.seller_response_version, updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: DeleteShortlistEntry :one
DELETE FROM procurement.buyer_shortlist_entries
WHERE request_id = @request_id AND seller_response_id = @seller_response_id
RETURNING seller_response_version;
