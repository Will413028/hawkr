-- name: InsertRequest :exec
INSERT INTO procurement.requests (id, buyer_id, status, created_at, updated_at)
VALUES (@id, @buyer_id, 'CLARIFYING', @now, @now);

-- name: GetRequestForBuyer :one
-- 不屬於呼叫者的需求與不存在的回同一個沒有列（呼叫端回 NOT_FOUND）。
SELECT * FROM procurement.requests WHERE id = @id AND buyer_id = @buyer_id;

-- name: SetRequestStatus :execrows
-- 條件式轉移（S2：requests 沒有 version）：只有目前狀態是 from_status 時才改。
UPDATE procurement.requests SET status = @status, updated_at = @now
WHERE id = @id AND buyer_id = @buyer_id AND status = @from_status;

-- name: InsertRevision :exec
INSERT INTO procurement.request_revisions (
    id, request_id, revision_no, parent_revision_id, status,
    item, item_resolution_kind, item_resolution_label, quantity, unit, packaging,
    budget_per_unit, delivery_city, delivery_date, exact_match_required, created_at)
VALUES (
    @id, @request_id, @revision_no, @parent_revision_id, 'ACTIVE',
    @item, @item_resolution_kind, @item_resolution_label, @quantity, @unit, @packaging,
    @budget_per_unit, @delivery_city, @delivery_date, @exact_match_required, @now);

-- name: SupersedeActiveRevision :execrows
-- 新版本寫入前先把現行版本轉成 SUPERSEDED（partial unique 擋雙頭）。
UPDATE procurement.request_revisions v SET status = 'SUPERSEDED'
WHERE v.request_id = @request_id AND v.status = 'ACTIVE'
  AND EXISTS (SELECT 1 FROM procurement.requests r WHERE r.id = v.request_id AND r.buyer_id = @buyer_id);

-- name: GetActiveRevision :one
SELECT * FROM procurement.request_revisions v WHERE v.request_id = @request_id AND v.status = 'ACTIVE'
  AND EXISTS (SELECT 1 FROM procurement.requests r WHERE r.id = v.request_id AND r.buyer_id = @buyer_id);

-- name: GetRevision :one
SELECT * FROM procurement.request_revisions v WHERE v.id = @id AND v.request_id = @request_id
  AND EXISTS (SELECT 1 FROM procurement.requests r WHERE r.id = v.request_id AND r.buyer_id = @buyer_id);

-- name: NextRevisionNo :one
SELECT (COALESCE(MAX(revision_no), 0) + 1)::int AS next_no FROM procurement.request_revisions v WHERE v.request_id = @request_id
  AND EXISTS (SELECT 1 FROM procurement.requests r WHERE r.id = v.request_id AND r.buyer_id = @buyer_id);

-- name: ListRevisions :many
SELECT * FROM procurement.request_revisions v WHERE v.request_id = @request_id
  AND EXISTS (SELECT 1 FROM procurement.requests r WHERE r.id = v.request_id AND r.buyer_id = @buyer_id)
ORDER BY revision_no;

-- name: InsertConversation :exec
INSERT INTO procurement.conversations (id, buyer_id, request_id, created_at, updated_at)
VALUES (@id, @buyer_id, @request_id, @now, @now);

-- name: GetConversationForBuyer :one
SELECT * FROM procurement.conversations WHERE id = @id AND buyer_id = @buyer_id AND deleted_at IS NULL;

-- name: ListConversations :many
SELECT * FROM procurement.conversations
WHERE buyer_id = @buyer_id AND deleted_at IS NULL
ORDER BY updated_at DESC, id DESC
LIMIT @page_size;

-- name: RenameConversation :execrows
-- 目標狀態的方法（S4）：重送結果相同，不建收據。
UPDATE procurement.conversations SET title = @title, title_source = 'USER', updated_at = @now
WHERE id = @id AND buyer_id = @buyer_id AND deleted_at IS NULL;

-- name: SoftDeleteConversation :execrows
-- 目標狀態的方法（S4）：已刪除的不再更新時間，重送結果相同。
UPDATE procurement.conversations SET deleted_at = @now, updated_at = @now
WHERE id = @id AND buyer_id = @buyer_id AND deleted_at IS NULL;

-- name: SetConversationPreview :exec
UPDATE procurement.conversations SET last_message_preview = @preview, updated_at = @now
WHERE id = @id AND buyer_id = @buyer_id AND deleted_at IS NULL;

-- name: InsertTurn :execrows
-- 冪等鍵在實體上（S4）：同一位買家的同一個 client_request_id 已有輪次時回 0 列，呼叫端改讀既有的輪次。
INSERT INTO procurement.turns (
    id, request_id, conversation_id, buyer_id, result_revision_id, source,
    buyer_message, assistant_reply, steps, recorded_this_turn, client_request_id, created_at)
VALUES (
    @id, @request_id, @conversation_id, @buyer_id, @result_revision_id, @source,
    @buyer_message, @assistant_reply, @steps, @recorded_this_turn, @client_request_id, @now)
ON CONFLICT (buyer_id, client_request_id) DO NOTHING;

-- name: GetTurnByClientRequest :one
SELECT * FROM procurement.turns WHERE buyer_id = @buyer_id AND client_request_id = @client_request_id;

-- name: ListTurnsByRequest :many
SELECT * FROM procurement.turns WHERE request_id = @request_id AND buyer_id = @buyer_id ORDER BY created_at, id;
