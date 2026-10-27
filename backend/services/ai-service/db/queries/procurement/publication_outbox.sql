-- procurement_rfq_publication_outbox 的可靠投遞持久層（#2450 Slice 2b）。
-- 前六支照 pkg/outbox/query_template.sql.md 複製；claim 述詞必須鏡射候選述詞。

-- name: InsertProcurementRfqPublicationOutbox :exec
INSERT INTO procurement_rfq_publication_outbox (
    id, invite_id, event_type, payload, status, attempt_count, next_attempt_at, created_at
) VALUES ($1, $2, $3, $4, 'pending', 0, $5, $5);

-- name: FetchPendingProcurementRfqPublicationOutbox :many
-- 刻意沒有 FOR UPDATE SKIP LOCKED：這條路徑在 autocommit 下跑，SELECT 一回傳鎖就釋放。
-- 跨副本互斥來自 Claim 的 lease CAS（《多副本排程搶佔規範》§一 ②）。
SELECT * FROM procurement_rfq_publication_outbox
WHERE status = 'pending' AND next_attempt_at <= $1
ORDER BY next_attempt_at LIMIT $2;

-- name: ClaimProcurementRfqPublicationOutbox :execrows
UPDATE procurement_rfq_publication_outbox
SET next_attempt_at = sqlc.arg(lease_until)
WHERE id = sqlc.arg(id)
  AND status = 'pending'
  AND next_attempt_at <= sqlc.arg(claimable_before);

-- name: UpdateProcurementRfqPublicationOutboxAttempt :exec
UPDATE procurement_rfq_publication_outbox
SET attempt_count = $2, next_attempt_at = $3, last_error = $4 WHERE id = $1;

-- name: MarkProcurementRfqPublicationOutboxDone :exec
UPDATE procurement_rfq_publication_outbox SET status = 'done', completed_at = $2 WHERE id = $1;

-- name: MarkProcurementRfqPublicationOutboxFailed :exec
UPDATE procurement_rfq_publication_outbox
SET status = 'failed', last_error = $2, completed_at = $3 WHERE id = $1;

-- name: CountPendingProcurementRfqPublicationOutbox :one
SELECT COUNT(*) FROM procurement_rfq_publication_outbox WHERE status = 'pending';

-- name: OldestPendingProcurementRfqPublicationOutboxCreatedAt :one
SELECT created_at FROM procurement_rfq_publication_outbox
WHERE status = 'pending' ORDER BY created_at ASC LIMIT 1;

-- name: DeleteDoneProcurementRfqPublicationOutboxBefore :execrows
DELETE FROM procurement_rfq_publication_outbox
WHERE id IN (
    SELECT sub.id FROM procurement_rfq_publication_outbox AS sub
    WHERE sub.status = 'done' AND sub.completed_at < $1
    LIMIT $2
);

-- name: MarkProcurementRfqPublicationInviteSent :exec
-- CAS：只有還在 QUEUED 的才轉 SENT。影響 0 列代表前一輪已轉過（重試路徑），
-- 呼叫端視為成功（設計書 §4.1）。
UPDATE procurement_rfq_publication_invites
SET delivery_state = 'SENT', delivery_state_at = $2, updated_at = $2
WHERE id = $1 AND delivery_state = 'QUEUED';

-- name: MarkProcurementRfqPublicationInviteFailedByOutboxID :exec
-- 終態失敗：outbox 列轉 failed 的**同一個交易**內把對應 invite 轉 FAILED。
-- ⛔ 不可改用 pkg/outbox.Hooks：OnAttemptFailed 每次嘗試失敗都觸發（第一次暫時性失敗
-- 就會把 invite 打成 FAILED），OnTerminalFailed 雖只在終態觸發但在 MarkFailed 回來之後
-- 才跑、不可能與它原子。設計書 §4.4。
UPDATE procurement_rfq_publication_invites
SET delivery_state = 'FAILED', delivery_state_at = $2, updated_at = $2
WHERE id = (SELECT sub.invite_id FROM procurement_rfq_publication_outbox AS sub WHERE sub.id = $1)
  AND delivery_state = 'QUEUED';
