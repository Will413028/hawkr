-- name: InsertProcurementRfqPublicationApproval :one
INSERT INTO procurement_rfq_publication_approvals (
    id, buyer_id, request_id, request_revision_id,
    confirmed_rfq_snapshot_id, confirmed_rfq_payload_hash,
    price_preview_id, price_preview_version,
    price_preview_payload_hash, price_preview_input_hash,
    buyer_requested_range_hash, response_deadline,
    audience_mode, audience_policy_id, audience_policy_version, audience_hash,
    candidate_set_id, candidate_set_policy_version,
    seller_facing_payload_json, seller_facing_payload_hash,
    state, schema_version,
    preview_client_request_id, generated_at, expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19,
    $20, $21, $22, $23, $24, $25
) -- 撞冪等鍵不拋錯（PostgreSQL 規範第十二章）：DO UPDATE 把冪等鍵寫回自己，不改內容
-- 但保證一定 RETURNING 一列，呼叫端用「回傳的 id 是不是這次產的」區分插入與既有列。
ON CONFLICT (buyer_id, preview_client_request_id)
DO UPDATE SET preview_client_request_id = EXCLUDED.preview_client_request_id
RETURNING *;

-- name: FindProcurementRfqPublicationApprovalByIdempotencyKey :one
SELECT * FROM procurement_rfq_publication_approvals
WHERE buyer_id = $1 AND preview_client_request_id = $2;

-- name: InsertProcurementRfqPublicationApprovalRecipient :exec
-- 呼叫端逐列插入（同 CreateProcurementCandidate 的既有慣例），position 即呼叫端切片的
-- 索引——順序是契約的一部分，見 schema 該表上方的註解。與 approval 本體同一個交易
-- （repository/publication_approval.go 的 InsertPublicationApproval），不單獨呼叫。
INSERT INTO procurement_rfq_publication_approval_recipients (
    approval_id, position, lane, store_id, prospect_id
) VALUES (
    $1, $2, $3, $4, $5
);

-- name: ListProcurementRfqPublicationApprovalRecipients :many
-- ORDER BY position 是這支查詢存在的理由：不排序就等於把「順序是契約」的保證在讀取端
-- 弄丟。沒有任何列（WISH_POOL 的 approval）回空切片而非錯誤。
SELECT * FROM procurement_rfq_publication_approval_recipients
WHERE approval_id = $1
ORDER BY position;

-- name: ClaimProcurementRfqPublicationApprovalForExecute :one
UPDATE procurement_rfq_publication_approvals
   SET state = 'EXECUTING', version = version + 1, updated_at = now()
 WHERE id = $1 AND version = $2 AND buyer_id = $3 AND state = 'PENDING'
RETURNING *;

-- name: MarkProcurementRfqPublicationApprovalSucceeded :one
UPDATE procurement_rfq_publication_approvals
   SET state = 'SUCCEEDED', version = version + 1, updated_at = now()
 WHERE id = $1 AND version = $2 AND state = 'EXECUTING'
RETURNING *;

-- name: InsertProcurementRfqPublicationCommand :one
-- execute 的冪等 claim（#2450 Scope 6 步驟 1、Scope 7）。撞冪等鍵不拋錯
-- （PostgreSQL 規範第十二章）：DO UPDATE 把冪等鍵寫回自己，不改內容但保證一定
-- RETURNING 一列，呼叫端用「回傳的 id 是不是這次產的」區分 claim 成功與既有列。
INSERT INTO procurement_rfq_publication_commands (
    id, buyer_id, request_id, approval_id, approval_version,
    client_request_id, canonical_request_hash
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (buyer_id, client_request_id)
DO UPDATE SET client_request_id = EXCLUDED.client_request_id
RETURNING *;

-- name: FindProcurementRfqPublicationCommandByIdempotencyKey :one
SELECT * FROM procurement_rfq_publication_commands
WHERE buyer_id = $1 AND client_request_id = $2;

-- name: InsertProcurementRfqPublication :one
-- 一個 command 只能有一張 publication（Scope 6「exactly-one」）的機械保證是
-- uq_procurement_rfq_publications_command；這裡本身不做冪等判斷，撞鍵直接讓
-- 呼叫端拿到錯誤。
INSERT INTO procurement_rfq_publications (
    id, buyer_id, request_id, request_revision_id,
    approval_id, approval_version, command_id, confirmed_rfq_snapshot_id,
    seller_facing_payload_hash, audience_mode, audience_hash,
    state, schema_version, published_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
) RETURNING *;

-- name: FindProcurementRfqPublicationByCommandID :one
SELECT * FROM procurement_rfq_publications
WHERE command_id = $1;

-- name: SetProcurementRfqPublicationCommandPublicationID :exec
-- execute 成功後把 command 指回它建出來的 publication（Scope 6 步驟 4 之後的回放錨點；
-- 見 schema 該欄的註解：失敗或仍在執行時為 NULL）。
--
-- `publication_id IS NULL` 這個 guard 讓重複呼叫在一個已經有值的 command 上安全地變成
-- no-op，同 SetProcurementAgentRunConversationID 的既有寫法——這是純粹的 metadata 回填，
-- 不是狀態轉移，不碰任何 version，也不需要 CAS：真正的 exactly-one 保證是
-- uq_procurement_rfq_publications_command 那條唯一索引。
UPDATE procurement_rfq_publication_commands
   SET publication_id = $2, updated_at = now()
 WHERE id = $1 AND publication_id IS NULL;

-- name: FindProcurementRfqPublicationByID :one
SELECT * FROM procurement_rfq_publications WHERE id = $1;

-- name: FindProcurementRfqPublicationInviteByID :one
SELECT * FROM procurement_rfq_publication_invites WHERE id = $1;

-- name: FindProcurementRfqPublicationInviteByPublicationAndStore :one
-- PLATFORM 賣家對某個 publication 的可見性判準。WISH_POOL 拉模式沒有 invite 列
-- （#2539 Slice 2c 未落地），所以目前只有 DIRECT lane 的賣家看得到 publication。
SELECT * FROM procurement_rfq_publication_invites
WHERE publication_id = sqlc.arg(publication_id) AND store_id = sqlc.arg(store_id);

-- name: ListProcurementRfqPublicationsForStore :many
-- seller_facing_payload_json 一併撈出來：一串不帶品名的 publication id 對賣家沒有
-- 用處，而它跟 GetPublicationInviteSellerView 是同一個 join，不是額外一輪查詢。
SELECT sqlc.embed(p), sqlc.embed(i), a.seller_facing_payload_json, a.response_deadline,
       sr.latest_version, sr.withdrawn_version,
       lv.valid_until AS latest_version_valid_until,
       (d.seller_response_id IS NOT NULL)::boolean AS has_draft
FROM procurement_rfq_publications p
JOIN procurement_rfq_publication_invites i ON i.publication_id = p.id
JOIN procurement_rfq_publication_approvals a ON a.id = p.approval_id
LEFT JOIN procurement_seller_responses sr
       ON sr.publication_id = p.id AND sr.store_id = i.store_id
LEFT JOIN procurement_seller_response_drafts d
       ON d.seller_response_id = sr.id
LEFT JOIN procurement_seller_response_versions lv
       ON lv.seller_response_id = sr.id AND lv.version = sr.latest_version
WHERE i.store_id = sqlc.arg(store_id)
ORDER BY p.published_at DESC, p.id DESC
LIMIT sqlc.arg(row_limit);

-- name: FindLatestProcurementRfqPublicationByRequest :one
SELECT id, version, published_at
FROM procurement_rfq_publications
WHERE request_id = sqlc.arg(request_id) AND buyer_id = sqlc.arg(buyer_id)
ORDER BY created_at DESC, id DESC
LIMIT 1;
