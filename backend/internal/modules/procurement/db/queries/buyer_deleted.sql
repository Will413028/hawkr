-- 買家關閉（BuyerDeleted，S10）。買家資料刪除；賣家看得到的紀錄（已發布的詢價與其下游）保留並把 buyer_id 設 NULL（保留期跟詢價的生命週期）。
-- 順序：先清買家專屬的資料，再處理需求本身，最後清額度與 ADK session。

-- name: DeleteBuyerConversations :execrows
DELETE FROM procurement.conversations WHERE buyer_id = @buyer_id;

-- name: DeleteBuyerRuns :execrows
DELETE FROM procurement.agent_runs WHERE buyer_id = @buyer_id;

-- name: DeleteBuyerCandidateSets :execrows
DELETE FROM procurement.candidate_sets WHERE buyer_id = @buyer_id;

-- name: DeleteBuyerShortlists :execrows
DELETE FROM procurement.buyer_shortlist_entries WHERE request_id IN (SELECT id FROM procurement.requests WHERE buyer_id = @buyer_id);

-- name: DeleteUnpublishedBuyerRequests :execrows
-- 沒有發布過的需求整張刪除（連帶 revision、輪次、預覽、快照、核准，都是 ON DELETE CASCADE）。
DELETE FROM procurement.requests r
WHERE r.buyer_id = @buyer_id
  AND NOT EXISTS (SELECT 1 FROM procurement.rfq_publications p WHERE p.request_id = r.id);

-- name: AnonymizeBuyerPricePreviews :execrows
UPDATE procurement.price_previews SET buyer_id = NULL WHERE buyer_id = @buyer_id;

-- name: AnonymizeBuyerSnapshots :execrows
UPDATE procurement.confirmed_rfq_snapshots SET buyer_id = NULL WHERE buyer_id = @buyer_id;

-- name: AnonymizeBuyerApprovals :execrows
UPDATE procurement.rfq_publication_approvals SET buyer_id = NULL WHERE buyer_id = @buyer_id;

-- name: AnonymizeBuyerPublications :execrows
UPDATE procurement.rfq_publications SET buyer_id = NULL WHERE buyer_id = @buyer_id;

-- name: AnonymizeBuyerAcceptances :execrows
UPDATE procurement.buyer_quote_acceptances SET buyer_id = NULL WHERE buyer_id = @buyer_id;

-- name: AnonymizeBuyerRequests :execrows
-- 已發布的需求保留（賣家看得到的詢價依附在它上面），buyer_id 設 NULL。
UPDATE procurement.requests SET buyer_id = NULL WHERE buyer_id = @buyer_id;

-- name: DeleteBuyerUsage :execrows
DELETE FROM procurement.usage WHERE buyer_id = @buyer_id;

-- name: DeleteBuyerSessions :execrows
-- ADK session（graph 的 checkpoint，含買家的對話內容）；events 由 FK 連帶刪除，user_states 以 user_id 刪。
DELETE FROM procurement.sessions WHERE user_id = @user_id;

-- name: DeleteBuyerUserStates :execrows
DELETE FROM procurement.user_states WHERE user_id = @user_id;
