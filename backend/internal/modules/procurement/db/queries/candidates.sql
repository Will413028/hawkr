-- name: InsertCandidateSet :exec
INSERT INTO procurement.candidate_sets (id, request_id, request_revision_id, buyer_id, created_at)
VALUES (@id, @request_id, @request_revision_id, @buyer_id::bigint, @now);

-- name: GetCandidateSetForBuyer :one
SELECT * FROM procurement.candidate_sets WHERE id = @id AND request_id = @request_id AND buyer_id = @buyer_id::bigint;

-- name: InsertCandidate :exec
INSERT INTO procurement.candidates (id, candidate_set_id, store_id, source_id, rank, created_at)
VALUES (@id, @candidate_set_id, @store_id, @source_id, @rank, @now);

-- name: ListCandidates :many
SELECT c.* FROM procurement.candidates c
JOIN procurement.candidate_sets s ON s.id = c.candidate_set_id
WHERE c.candidate_set_id = @candidate_set_id AND s.buyer_id = @buyer_id::bigint
ORDER BY c.rank, c.id;

-- name: SetCandidateSelected :execrows
UPDATE procurement.candidates c
SET selected = @selected::bool, selected_at = CASE WHEN @selected::bool THEN @now::timestamptz ELSE NULL END
FROM procurement.candidate_sets s
WHERE c.id = @id AND c.candidate_set_id = s.id AND s.id = @candidate_set_id AND s.request_id = @request_id AND s.buyer_id = @buyer_id::bigint;

-- name: GetLatestSucceededSearchSet :one
SELECT candidate_set_id FROM procurement.agent_runs
WHERE request_id = @request_id AND buyer_id = @buyer_id AND run_type = 'CANDIDATE_SEARCH' AND status = 'SUCCEEDED' AND candidate_set_id IS NOT NULL
ORDER BY created_at DESC, id DESC
LIMIT 1;
