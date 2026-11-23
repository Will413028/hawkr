-- name: ListCurrentSellerResponses :many
-- 一次發布收到的賣家回價，每位賣家只列目前版本（latest_version > 0），依回價身分列的建立時間由新到舊、同時間依回價 ID 由大到小（keyset (created_at DESC, id DESC)；身分列不可變，賣家改版不會讓列換位置）。
-- 回價身分列沒有 buyer_id，擁有者由發布推出（p.buyer_id）：不是這位買家的發布、或發布不屬於這張需求，一列都不回。
-- 游標是上一頁最後一列的 (created_at, id)；沒有游標時 after_id 為 NULL。shortlisted 只在入圍清單掛的就是這個版本時為 true。
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
-- 不鎖的讀取，給入圍清單用：回價屬於這位買家的這張需求（經發布的 buyer_id）才有列。
SELECT r.* FROM procurement.seller_responses r
JOIN procurement.rfq_publications p ON p.id = r.publication_id
WHERE r.id = @id AND r.request_id = @request_id AND p.buyer_id = @buyer_id;

-- name: LockSellerResponseForBuyer :one
-- 接受報價用：鎖回價身分列（與賣家改版、撤回、婉拒串行），不屬於這位買家的這張需求沒有列。
SELECT r.* FROM procurement.seller_responses r
JOIN procurement.rfq_publications p ON p.id = r.publication_id
WHERE r.id = @id AND r.request_id = @request_id AND p.buyer_id = @buyer_id
FOR UPDATE OF r;

-- name: GetSellerResponseVersion :one
SELECT * FROM procurement.seller_response_versions WHERE seller_response_id = @seller_response_id AND version = @version;

-- name: PutShortlistEntry :one
-- 同一則回價只會有一個版本在清單（唯一鍵 (request_id, seller_response_id)）：已在清單時換成新的版本（A9-D31 Q12）。
INSERT INTO procurement.buyer_shortlist_entries (id, request_id, seller_response_id, seller_response_version, created_at, updated_at)
VALUES (@id, @request_id, @seller_response_id, @seller_response_version, @now, @now)
ON CONFLICT (request_id, seller_response_id) DO UPDATE SET seller_response_version = EXCLUDED.seller_response_version, updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: DeleteShortlistEntry :one
-- 目標狀態的方法：不在清單時沒有列回來，結果相同。擁有者由呼叫端先以 GetRequestForBuyer 確認。
DELETE FROM procurement.buyer_shortlist_entries
WHERE request_id = @request_id AND seller_response_id = @seller_response_id
RETURNING seller_response_version;
