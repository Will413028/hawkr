-- name: InsertRequest :exec
INSERT INTO procurement.requests (id, buyer_id, status, created_at, updated_at)
VALUES (@id, @buyer_id, 'CLARIFYING', @now, @now);

-- name: LockRequestForBuyer :one
SELECT * FROM procurement.requests WHERE id = @id AND buyer_id = @buyer_id FOR UPDATE;

-- name: GetRequestForBuyer :one
SELECT * FROM procurement.requests WHERE id = @id AND buyer_id = @buyer_id;

-- name: SetRequestStatus :execrows
UPDATE procurement.requests SET status = @status, updated_at = @now
WHERE id = @id AND buyer_id = @buyer_id AND status = @from_status;

-- name: SetRequestStatusFromAny :execrows
UPDATE procurement.requests SET status = @to_status, updated_at = @now
WHERE id = @id AND buyer_id = @buyer_id AND status = ANY(@from_statuses::text[]);

-- name: LockActiveRevision :one
SELECT v.* FROM procurement.request_revisions v WHERE v.request_id = @request_id AND v.status = 'ACTIVE'
  AND EXISTS (SELECT 1 FROM procurement.requests r WHERE r.id = v.request_id AND r.buyer_id = @buyer_id)
FOR UPDATE OF v;

-- name: InsertRevision :execrows
INSERT INTO procurement.request_revisions (
    id, request_id, revision_no, parent_revision_id, status,
    item, item_resolution_kind, item_resolution_label, item_resolution_category_id, quantity, unit, packaging,
    budget_per_unit, delivery_city, delivery_date, exact_match_required, created_at)
SELECT
    @id, r.id, @revision_no, @parent_revision_id, 'ACTIVE',
    @item, @item_resolution_kind, @item_resolution_label, @item_resolution_category_id, @quantity, @unit, @packaging,
    @budget_per_unit, @delivery_city, @delivery_date, @exact_match_required, @now
FROM procurement.requests r
WHERE r.id = @request_id AND r.buyer_id = @buyer_id;

-- name: SupersedeActiveRevision :execrows
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
SELECT (COALESCE(MAX(v.revision_no), 0) + 1)::int AS next_no
FROM procurement.requests r
LEFT JOIN procurement.request_revisions v ON v.request_id = r.id
WHERE r.id = @request_id AND r.buyer_id = @buyer_id
GROUP BY r.id;

-- name: ListRevisions :many
SELECT * FROM procurement.request_revisions v WHERE v.request_id = @request_id
  AND EXISTS (SELECT 1 FROM procurement.requests r WHERE r.id = v.request_id AND r.buyer_id = @buyer_id)
ORDER BY revision_no;

-- name: InsertConversation :exec
INSERT INTO procurement.conversations (id, buyer_id, request_id, created_at, updated_at)
VALUES (@id, @buyer_id, @request_id, @now, @now);

-- name: GetConversationForBuyer :one
SELECT * FROM procurement.conversations WHERE id = @id AND buyer_id = @buyer_id AND deleted_at IS NULL;

-- name: GetConversationByRequest :one
SELECT * FROM procurement.conversations WHERE request_id = @request_id AND buyer_id = @buyer_id AND deleted_at IS NULL;

-- name: ListConversations :many
SELECT * FROM procurement.conversations
WHERE buyer_id = @buyer_id AND deleted_at IS NULL
ORDER BY updated_at DESC, id DESC
LIMIT @page_size;

-- name: ListConversationsAfter :many
SELECT * FROM procurement.conversations
WHERE buyer_id = @buyer_id AND deleted_at IS NULL AND (updated_at, id) < (@after_updated_at::timestamptz, @after_id::uuid)
ORDER BY updated_at DESC, id DESC
LIMIT @page_size;

-- name: GetConversationIncludingDeleted :one
SELECT * FROM procurement.conversations WHERE id = @id AND buyer_id = @buyer_id;

-- name: RenameConversation :execrows
UPDATE procurement.conversations SET title = @title, title_source = 'USER', updated_at = @now
WHERE id = @id AND buyer_id = @buyer_id AND deleted_at IS NULL;

-- name: SoftDeleteConversation :execrows
UPDATE procurement.conversations SET deleted_at = @now, updated_at = @now
WHERE id = @id AND buyer_id = @buyer_id AND deleted_at IS NULL;

-- name: SetConversationPreview :exec
UPDATE procurement.conversations SET last_message_preview = @preview, updated_at = @now
WHERE id = @id AND buyer_id = @buyer_id AND deleted_at IS NULL;

-- name: InsertTurn :execrows
INSERT INTO procurement.turns (
    id, request_id, conversation_id, buyer_id, result_revision_id, source,
    buyer_message, assistant_reply, steps, recorded_this_turn, optional_streak, skipped_slots, turn_result, command_slot, client_request_id, created_at)
VALUES (
    @id, @request_id, @conversation_id, @buyer_id, @result_revision_id, @source,
    @buyer_message, @assistant_reply, @steps, @recorded_this_turn, @optional_streak, COALESCE(@skipped_slots::text[], '{}'), @turn_result, sqlc.narg('command_slot'), @client_request_id, @now)
ON CONFLICT (buyer_id, client_request_id) DO NOTHING;

-- name: GetTurnByClientRequest :one
SELECT * FROM procurement.turns WHERE buyer_id = @buyer_id AND client_request_id = @client_request_id;

-- name: ListTurnsByRequest :many
SELECT * FROM procurement.turns WHERE request_id = @request_id AND buyer_id = @buyer_id ORDER BY created_at, id;

-- name: GetLatestTurnByRequest :one
SELECT * FROM procurement.turns WHERE request_id = @request_id AND buyer_id = @buyer_id ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: SetConversationAutoTitle :exec
UPDATE procurement.conversations SET title = @title, updated_at = @now
WHERE id = @id AND buyer_id = @buyer_id AND deleted_at IS NULL AND title_source = 'AUTO';
