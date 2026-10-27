-- name: CreateProcurementConversation :one
INSERT INTO procurement_conversations (id, buyer_id, request_id, title, title_source)
VALUES ($1, $2, $3, '', 'AUTO')
RETURNING *;

-- name: GetOrCreateProcurementConversation :one
-- 容錯用：正常路徑一律靠 CreateRequest 在交易內建立 conversation row，這支
-- 查詢只在該保證被打破時（例如 rolling deploy 期間新舊 pod 交錯，舊 pod 仍在
-- 用沒有這段接線的 CreateRequest）自我修復——ON CONFLICT DO UPDATE 只是為了
-- 讓 RETURNING 在衝突時也能拿到既有列（DO NOTHING 拿不到 RETURNING），
-- 語意上是唯讀，不是真的想更新 updated_at。
INSERT INTO procurement_conversations (id, buyer_id, request_id, title, title_source)
VALUES ($1, $2, $3, '', 'AUTO')
ON CONFLICT (request_id) DO UPDATE SET updated_at = procurement_conversations.updated_at
RETURNING *;

-- name: GetProcurementConversationByRequestID :one
-- 純讀取版的 conversation 解析（Task 15 write-path 修復）：CreateRequest 一律
-- 在自己的交易內建立 conversation row（uq_procurement_conversations_request
-- 保證 1:1），所以呼叫端只要手上有一個真實存在的 request_id，這裡幾乎總是
-- 找得到列——例外是 GetOrCreateProcurementConversation 文件記載的那個窄
-- rolling-deploy race 視窗，那個視窗由下一次 AppendRevision／RewindTo 的
-- get-or-create 自我修復，這支查詢不做寫入、找不到就讓呼叫端把它當「這次還
-- 解析不出來」處理（repository.conversationIDForRequest 回 0），不是硬錯誤。
SELECT * FROM procurement_conversations
WHERE request_id = $1;

-- name: UpdateProcurementConversationPreview :exec
UPDATE procurement_conversations
SET last_message_preview = $2, updated_at = now()
WHERE request_id = $1;

-- name: UpdateProcurementConversationAutoTitle :exec
UPDATE procurement_conversations
SET title = $2, updated_at = now()
WHERE request_id = $1 AND title_source = 'AUTO';

-- name: ListProcurementConversations :many
-- 只回傳至少有一筆 procurement_answer_patches 的 conversation：沒有任何
-- completed turn 的 row（真孤兒 request，或 crash 在 CreateRequest 之後、
-- AppendRevision 之前）不該出現在列表——title/last_message_preview 對這種
-- row 沒有意義。用「有沒有 answer_patch」而不是「title/preview 是否為空
-- 字串」判斷：後者對 legacy backfill 的 row 是偽陽性，那些 row 有真實訊息
-- 歷史，只是 preview 欄位算不出來（該邏輯只存在於 AppendRevision）。
--
-- 依 updated_at DESC 排序（回覆過的舊對話會浮到最上面），不是純 id DESC（建立
-- 時間）。cursor 是 (updated_at, id) 複合值：has_cursor=false 代表第一頁，不套
-- cursor 條件；true 時用 row-wise comparison 取「排在游標之後」的列——這是
-- Postgres 對複合鍵 keyset pagination 的標準寫法，avoid 掉 updated_at 撞值時漏列
-- 或重複列的問題（單欄 cursor 做不到這件事）。
SELECT c.* FROM procurement_conversations c
WHERE c.buyer_id = $1 AND c.deleted_at IS NULL
  AND EXISTS (SELECT 1 FROM procurement_answer_patches p WHERE p.request_id = c.request_id)
  AND (
    NOT sqlc.arg(has_cursor)::bool
    OR (c.updated_at, c.id) < (sqlc.arg(cursor_updated_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  )
ORDER BY c.updated_at DESC, c.id DESC
LIMIT sqlc.arg(limit_plus_one)::int;

-- name: GetProcurementConversationForMessages :one
SELECT c.id, c.request_id, c.title, r.status
FROM procurement_conversations c
JOIN procurement_requests r ON r.id = c.request_id
WHERE c.id = $1 AND c.buyer_id = $2 AND c.deleted_at IS NULL;

-- name: ListProcurementAnswerPatchesByRequestID :many
SELECT * FROM procurement_answer_patches
WHERE request_id = $1
ORDER BY created_at ASC;

-- name: RenameProcurementConversation :execrows
UPDATE procurement_conversations
SET title = $3, title_source = 'USER', updated_at = now()
WHERE id = $1 AND buyer_id = $2 AND deleted_at IS NULL;

-- name: GetProcurementConversationForDelete :one
SELECT deleted_at FROM procurement_conversations
WHERE id = $1 AND buyer_id = $2;

-- name: SoftDeleteProcurementConversation :exec
UPDATE procurement_conversations
SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND buyer_id = $2 AND deleted_at IS NULL;

-- name: FindProcurementConversationCommandByIdempotencyKey :one
SELECT * FROM procurement_conversation_commands
WHERE buyer_id = $1 AND client_request_id = $2;

-- name: CreateProcurementConversationCommand :one
INSERT INTO procurement_conversation_commands (
  id, buyer_id, client_request_id, conversation_id, result_json
) VALUES (
  $1, $2, $3, $4, $5
)
RETURNING *;
