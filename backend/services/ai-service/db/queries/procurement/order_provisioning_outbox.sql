-- procurement_order_provisioning_outbox 的可靠投遞持久層（doc 64 §6.2 選項 B、§6.4）。
-- Fetch／Claim／UpdateAttempt／MarkDone／MarkFailed／CountPending／OldestPending／
-- DeleteDone 八支照 pkg/outbox/query_template.sql.md 複製；claim 述詞鏡射候選述詞。

-- name: InsertProcurementOrderProvisioningOutbox :exec
-- ⛔ ON CONFLICT DO NOTHING 不是防禦性寫法，是契約的一部分：acceptance 的 insert
-- 走 `ON CONFLICT (buyer_id, client_request_id) DO UPDATE ... RETURNING *` 回放既有列，
-- 同一把冪等鑰匙重送時這一支會跟著再跑一次。業務鍵 uq_propo_acceptance_event 擋在 DB，
-- 這裡靜默略過即可——重送不得產生第二筆建單命令。
INSERT INTO procurement_order_provisioning_outbox (
    id, acceptance_id, event_type, payload, status, attempt_count, next_attempt_at, created_at
) VALUES ($1, $2, $3, $4, 'pending', 0, $5, $5)
ON CONFLICT (acceptance_id, event_type) DO NOTHING;

-- name: FindProcurementOrderProvisioningOutboxByAcceptance :one
SELECT * FROM procurement_order_provisioning_outbox
WHERE acceptance_id = $1 AND event_type = $2;

-- name: FetchPendingProcurementOrderProvisioningOutbox :many
-- 刻意沒有 FOR UPDATE SKIP LOCKED：這條路徑在 autocommit 下跑，SELECT 一回傳鎖就釋放。
-- 跨副本互斥來自 Claim 的 lease CAS（《多副本排程搶佔規範》§一 ②）。
SELECT * FROM procurement_order_provisioning_outbox
WHERE status = 'pending' AND next_attempt_at <= $1
ORDER BY next_attempt_at LIMIT $2;

-- name: ClaimProcurementOrderProvisioningOutbox :execrows
UPDATE procurement_order_provisioning_outbox
SET next_attempt_at = sqlc.arg(lease_until)
WHERE id = sqlc.arg(id)
  AND status = 'pending'
  AND next_attempt_at <= sqlc.arg(claimable_before);

-- name: UpdateProcurementOrderProvisioningOutboxAttempt :exec
UPDATE procurement_order_provisioning_outbox
SET attempt_count = $2, next_attempt_at = $3, last_error = $4 WHERE id = $1;

-- name: MarkProcurementOrderProvisioningOutboxDone :exec
UPDATE procurement_order_provisioning_outbox
SET status = 'done', completed_at = $2 WHERE id = $1;

-- name: MarkProcurementOrderProvisioningOutboxFailed :exec
-- last_error 在終態也要寫：只在非終態重試時寫 last_error 的表，會在真正落到
-- 'failed' 的那一刻剛好把維運要看的錯誤訊息丟掉。
UPDATE procurement_order_provisioning_outbox
SET status = 'failed', last_error = $2, completed_at = $3 WHERE id = $1;


-- name: CountPendingProcurementOrderProvisioningOutbox :one
SELECT COUNT(*) FROM procurement_order_provisioning_outbox WHERE status = 'pending';

-- name: OldestPendingProcurementOrderProvisioningOutboxCreatedAt :one
SELECT created_at FROM procurement_order_provisioning_outbox
WHERE status = 'pending' ORDER BY created_at ASC LIMIT 1;

-- name: DeleteDoneProcurementOrderProvisioningOutboxBefore :execrows
DELETE FROM procurement_order_provisioning_outbox
WHERE id IN (
    SELECT sub.id FROM procurement_order_provisioning_outbox AS sub
    WHERE sub.status = 'done' AND sub.completed_at < $1
    LIMIT $2
);
