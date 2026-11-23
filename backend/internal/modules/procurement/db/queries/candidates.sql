-- name: InsertCandidateSet :exec
-- 一次搜尋一列（S1）；冪等鍵在 run（A9-D29），重送由呼叫端以 run 的冪等鍵擋下，不會走到這裡。
INSERT INTO procurement.candidate_sets (id, request_id, request_revision_id, buyer_id, created_at)
VALUES (@id, @request_id, @request_revision_id, @buyer_id, @now);

-- name: GetCandidateSetForBuyer :one
SELECT * FROM procurement.candidate_sets WHERE id = @id AND request_id = @request_id AND buyer_id = @buyer_id;

-- name: InsertCandidate :exec
-- 沒有店名欄位（S9）；呼叫端只給 OPEN 的店。
INSERT INTO procurement.candidates (id, candidate_set_id, store_id, source_id, rank, created_at)
VALUES (@id, @candidate_set_id, @store_id, @source_id, @rank, @now);

-- name: ListCandidates :many
SELECT c.* FROM procurement.candidates c
JOIN procurement.candidate_sets s ON s.id = c.candidate_set_id
WHERE c.candidate_set_id = @candidate_set_id AND s.buyer_id = @buyer_id
ORDER BY c.rank, c.id;

-- name: SetCandidateSelected :execrows
-- 目標狀態的方法（契約的 ToggleSelection 指明目標狀態）。
UPDATE procurement.candidates c
SET selected = @selected::bool, selected_at = CASE WHEN @selected::bool THEN @now::timestamptz ELSE NULL END
FROM procurement.candidate_sets s
WHERE c.id = @id AND c.candidate_set_id = s.id AND s.id = @candidate_set_id AND s.request_id = @request_id AND s.buyer_id = @buyer_id;

-- name: GetLatestSucceededSearchSet :one
-- 需求最新一筆成功的候選搜尋所產生的候選集 ID（有效的候選集＝需求在 MATCHING 時的這一個，A9-D29(4)）；沒有成功過的搜尋沒有列。
SELECT candidate_set_id FROM procurement.agent_runs
WHERE request_id = @request_id AND buyer_id = @buyer_id AND run_type = 'CANDIDATE_SEARCH' AND status = 'SUCCEEDED' AND candidate_set_id IS NOT NULL
ORDER BY created_at DESC, id DESC
LIMIT 1;
