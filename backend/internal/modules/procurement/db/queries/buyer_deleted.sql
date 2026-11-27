
-- name: DeleteBuyerConversations :execrows
DELETE FROM procurement.conversations WHERE buyer_id = @buyer_id;

-- name: DeleteBuyerRuns :execrows
DELETE FROM procurement.agent_runs WHERE buyer_id = @buyer_id;

-- name: DeleteBuyerCandidateSets :execrows
DELETE FROM procurement.candidate_sets s
WHERE s.buyer_id = @buyer_id::bigint
  AND NOT EXISTS (SELECT 1 FROM procurement.rfq_publication_approvals a WHERE a.candidate_set_id = s.id);

-- name: AnonymizeBuyerCandidateSets :execrows
UPDATE procurement.candidate_sets SET buyer_id = NULL WHERE buyer_id = @buyer_id;

-- name: DeleteBuyerShortlists :execrows
DELETE FROM procurement.buyer_shortlist_entries WHERE request_id IN (SELECT id FROM procurement.requests WHERE buyer_id = @buyer_id);

-- name: DeleteUnpublishedBuyerRequests :execrows
DELETE FROM procurement.requests r
WHERE r.buyer_id = @buyer_id
  AND NOT EXISTS (SELECT 1 FROM procurement.rfq_publications p WHERE p.request_id = r.id);

-- name: AnonymizeBuyerApprovals :execrows
UPDATE procurement.rfq_publication_approvals SET buyer_id = NULL WHERE buyer_id = @buyer_id;

-- name: AnonymizeBuyerPublications :execrows
UPDATE procurement.rfq_publications SET buyer_id = NULL WHERE buyer_id = @buyer_id;

-- name: AnonymizeBuyerAcceptances :execrows
UPDATE procurement.buyer_quote_acceptances SET buyer_id = NULL WHERE buyer_id = @buyer_id;

-- name: AnonymizeBuyerRequests :execrows
UPDATE procurement.requests SET buyer_id = NULL WHERE buyer_id = @buyer_id;

-- name: DeleteBuyerUsage :execrows
DELETE FROM procurement.usage WHERE buyer_id = @buyer_id;
