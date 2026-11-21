-- name: InsertCandidateSet :execrows
-- 一次搜尋一列（S1）；同一個版本與 client_request_id 已有候選集時回 0 列，呼叫端改讀既有的。
INSERT INTO procurement.candidate_sets (id, request_id, request_revision_id, buyer_id, client_request_id, created_at)
VALUES (@id, @request_id, @request_revision_id, @buyer_id, @client_request_id, @now)
ON CONFLICT (request_revision_id, client_request_id) DO NOTHING;

-- name: GetCandidateSetForBuyer :one
SELECT * FROM procurement.candidate_sets WHERE id = @id AND request_id = @request_id AND buyer_id = @buyer_id;

-- name: GetCandidateSetByClientRequest :one
SELECT * FROM procurement.candidate_sets WHERE request_revision_id = @request_revision_id AND client_request_id = @client_request_id;

-- name: InsertCandidate :exec
-- 沒有店名欄位（S9）；呼叫端只給 OPEN 的店。
INSERT INTO procurement.candidates (
    id, candidate_set_id, store_id, source_id, score, rank, matched_facts, unverifiable_facts, facts_as_of, created_at)
VALUES (@id, @candidate_set_id, @store_id, @source_id, @score, @rank, @matched_facts, @unverifiable_facts, @facts_as_of, @now);

-- name: ListCandidates :many
SELECT * FROM procurement.candidates WHERE candidate_set_id = @candidate_set_id ORDER BY rank, id;

-- name: SetCandidateSelected :execrows
-- 目標狀態的方法（契約的 ToggleSelection 指明目標狀態）。
UPDATE procurement.candidates c
SET selected = @selected, selected_at = CASE WHEN @selected THEN @now ELSE NULL END
FROM procurement.candidate_sets s
WHERE c.id = @id AND c.candidate_set_id = s.id AND s.id = @candidate_set_id AND s.request_id = @request_id AND s.buyer_id = @buyer_id;
