-- 賣家端的讀寫。賣家可見性的唯一依據是邀請列的 (publication_id, store_id)（A9-D44(2)）：下面的讀取都從邀請列出發，
-- 不帶 buyer_id，買家刪除後（buyer_id 為 NULL）被邀請的店照常讀得到；沒有邀請列的店一列都查不到。

-- name: GetSellerPublicationView :one
-- 一筆發給這家店的詢價，連同組出賣家可見內容要用的核准與需求版本（組法在 rfq.SellerFacingOf）。沒有列就是不存在或不是發給這家店。
SELECT sqlc.embed(p), sqlc.embed(a), sqlc.embed(rev)
FROM procurement.rfq_publication_invites i
JOIN procurement.rfq_publications p ON p.id = i.publication_id
JOIN procurement.rfq_publication_approvals a ON a.id = p.approval_id
JOIN procurement.request_revisions rev ON rev.id = a.request_revision_id
WHERE i.publication_id = @publication_id AND i.store_id = @store_id;

-- name: ListSellerPublicationViews :many
-- 發給這家店的詢價，依 (published_at DESC, id DESC) keyset；游標是上一頁最後一列的 (published_at, id)，沒有游標時 after_id 為 NULL。
-- 一次 join 取核准與需求版本，不逐列再查。不過濾已成交或買家已刪的詢價（賣家讀得到自己收過的詢價）。
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
-- 這家店對一批詢價的回價狀態：目前版本的有效期限（推導 latest_response_status 用）與有沒有草稿。只回已經有身分列的詢價。
SELECT r.publication_id, r.latest_version, r.withdrawn_version, v.valid_until, (d.seller_response_id IS NOT NULL)::boolean AS has_draft
FROM procurement.seller_responses r
LEFT JOIN procurement.seller_response_versions v ON v.seller_response_id = r.id AND v.version = r.latest_version
LEFT JOIN procurement.seller_response_drafts d ON d.seller_response_id = r.id
WHERE r.store_id = @store_id AND r.publication_id = ANY(@publication_ids::uuid[]);

-- name: EnsureSellerResponse :exec
-- 第一次寫入時建立身分列；只有被邀請的店會插入（SELECT 從邀請列出發），併發的第二個寫入者撞唯一鍵時什麼都不做。
INSERT INTO procurement.seller_responses (id, publication_id, request_id, store_id, created_at, updated_at)
SELECT @id, i.publication_id, p.request_id, i.store_id, @now, @now
FROM procurement.rfq_publication_invites i
JOIN procurement.rfq_publications p ON p.id = i.publication_id
WHERE i.publication_id = @publication_id AND i.store_id = @store_id
ON CONFLICT (publication_id, store_id) DO NOTHING;

-- name: LockSellerResponseForStore :one
-- 賣家寫入用：鎖這家店對這筆詢價的身分列（與買家的 AcceptSellerQuote 鎖同一列，兩邊串行）。沒有列回 ErrNoRows。
SELECT * FROM procurement.seller_responses WHERE publication_id = @publication_id AND store_id = @store_id FOR UPDATE;

-- name: LockPublicationBuyer :one
-- 賣家寫入用：以 FOR SHARE 鎖發布列並回目前的 buyer_id（NULL 是買家已刪除）。BuyerDeleted 的 UPDATE 要等寫入提交，所以檢查與寫入之間不會插進買家刪除。鎖順序：店家列、回價身分列、發布列。
SELECT buyer_id FROM procurement.rfq_publications WHERE id = @id FOR SHARE;

-- name: HasActiveAcceptance :one
-- 這張需求有沒有進行中的成交（ACTIVE 的接受單；建單失敗解除鎖後不算）。不帶 buyer_id：買家刪除後接受單的 buyer_id 是 NULL。
SELECT EXISTS (SELECT 1 FROM procurement.buyer_quote_acceptances WHERE request_id = @request_id AND state = 'ACTIVE') AS active;

-- name: GetSellerDraft :one
SELECT * FROM procurement.seller_response_drafts WHERE seller_response_id = @seller_response_id;

-- name: InsertSellerDraft :one
-- 建立第一版；已有草稿時沒有列回來（呼叫端回版本衝突）。
INSERT INTO procurement.seller_response_drafts (seller_response_id, payload, version, created_at, updated_at)
VALUES (@seller_response_id, @payload::jsonb, 1, @now, @now)
ON CONFLICT (seller_response_id) DO NOTHING
RETURNING *;

-- name: UpdateSellerDraft :one
-- 整份覆寫；版本不符沒有列回來（呼叫端回版本衝突）。
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
-- 已鎖住身分列；latest_version 當 CAS，沒更新到列代表鎖沒有守住（不應發生）。
UPDATE procurement.seller_responses SET latest_version = @version::int, updated_at = @now::timestamptz
WHERE id = @id AND latest_version = @expected_version::int;

-- name: WithdrawSellerResponse :execrows
UPDATE procurement.seller_responses
SET withdrawn_version = @version::int, withdrawn_at = @now::timestamptz, withdrawn_reason_code = sqlc.narg('reason_code')::text, updated_at = @now::timestamptz
WHERE id = @id AND latest_version = @version::int;

-- name: DeclineSellerResponse :execrows
-- 目標狀態：已婉拒的列不動（保留第一次的時間與原因）。
UPDATE procurement.seller_responses
SET write_closed_at = @now::timestamptz, write_closed_reason = 'DECLINED', write_closed_reason_code = sqlc.narg('reason_code')::text, updated_at = @now::timestamptz
WHERE id = @id AND write_closed_at IS NULL;
