-- name: InsertRefundReversal :one
-- 開始收回時為每個目標寫一列撤回（呼叫端已鎖付款列、轉帳列與相關退款列）。
INSERT INTO payment.refund_reversals (id, refund_id, payment_id, seq, target_refund_id, target_external_id, amount, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(refund_id), sqlc.arg(payment_id), sqlc.arg(seq), sqlc.narg(target_refund_id), sqlc.arg(target_external_id), sqlc.arg(amount), sqlc.arg(now), sqlc.arg(now))
RETURNING *;

-- name: RefundReversalByID :one
SELECT *
FROM payment.refund_reversals
WHERE id = $1;

-- name: LockRefundReversal :one
SELECT *
FROM payment.refund_reversals
WHERE id = $1
FOR UPDATE;

-- name: RefundReversalsByRefund :many
-- 一筆退款的全部撤回列（依 seq）。
SELECT *
FROM payment.refund_reversals
WHERE refund_id = $1
ORDER BY seq;

-- name: ReversingReversalsByRefund :many
-- 一筆退款進行中的撤回列（依 seq）：撤回 job 逐列收斂。
SELECT *
FROM payment.refund_reversals
WHERE refund_id = $1 AND status = 'REVERSING'
ORDER BY seq;

-- name: MarkRefundReversalCalled :one
-- 撤回 provider 之前先記「已呼叫」的時刻（這把鍵已記過的保留最早的）。只有 REVERSING 的列寫。
UPDATE payment.refund_reversals
SET called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING *;

-- name: ApplyRefundReversal :one
-- 撤回成功：REVERSING → REVERSED 並記 provider 的撤回 ID（與目標累計撤回額的累加、彙總狀態的重算在同一個 transaction）。
UPDATE payment.refund_reversals
SET status = 'REVERSED',
    external_id = sqlc.arg(external_id),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING *;

-- name: ReleaseReversal :one
-- 撤回的終局失敗（provider 永久拒絕，或查回證明上一次不存在又過了最短等待）：REVERSING 進 RELEASED，原因記在 reason。
UPDATE payment.refund_reversals
SET status = 'RELEASED',
    reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING *;

-- name: RotateReversalKey :one
-- 撤回被 provider 已知拒絕（可重試、沒有執行）：換新的撤回冪等鍵（也是新的嘗試 ID），清掉呼叫時刻與查回失敗次數，不計任何期限。
UPDATE payment.refund_reversals
SET idempotency_key = gen_random_uuid(),
    called_at = NULL,
    lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING' AND idempotency_key = sqlc.arg(old_key)
RETURNING *;

-- name: BumpReversalLookupFailures :one
UPDATE payment.refund_reversals
SET lookup_failures = lookup_failures + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING lookup_failures;

-- name: ClearReversalLookupFailures :exec
UPDATE payment.refund_reversals
SET lookup_failures = 0
WHERE id = sqlc.arg(id) AND status = 'REVERSING' AND lookup_failures > 0;

-- name: AnomalyReversal :one
-- 撤回與 provider 不符、前置條件不成立或查回連續失敗：REVERSING 進 ANOMALY（佔住目標的額度，補掃不再重試），原因記在 reason。
UPDATE payment.refund_reversals
SET status = 'ANOMALY',
    reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING *;

-- name: ResolveReversalDone :one
-- 後台人工結案（撤回的 ANOMALY）：provider 確實撤回了（external_id 是證據），進 REVERSED。
UPDATE payment.refund_reversals
SET status = 'REVERSED',
    external_id = sqlc.arg(external_id),
    called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz),
    reason = NULL,
    lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: ResolveReversalReleased :one
-- 後台人工結案（撤回的 ANOMALY）：確認 provider 沒有撤回，釋出。
UPDATE payment.refund_reversals
SET status = 'RELEASED',
    reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: SettleRefundReversal :exec
-- 重算退款列上向賣家收回的彙總（呼叫端已鎖付款列與該退款列，撤回列剛改完）：有 ANOMALY 的列是 ANOMALY，其次有 REVERSING 的是 REVERSING，其次有 RELEASED 的是 RELEASED，其餘 REVERSED；
-- reversal_recovered 是 REVERSED 列的加總，reversal_reason 取 ANOMALY 或 RELEASED 列的原因。只改 REVERSING 或 ANOMALY 的彙總（已 RELEASED 或 REVERSED 的不再變）。
UPDATE payment.refunds AS r
SET reversal_status = CASE WHEN agg.anomalies > 0 THEN 'ANOMALY' WHEN agg.reversing > 0 THEN 'REVERSING' WHEN agg.released > 0 THEN 'RELEASED' ELSE 'REVERSED' END,
    reversal_recovered = agg.recovered,
    reversal_reason = CASE WHEN agg.anomalies > 0 THEN agg.anomaly_reason WHEN agg.reversing > 0 THEN NULL WHEN agg.released > 0 THEN agg.release_reason ELSE NULL END,
    updated_at = sqlc.arg(now)
FROM (
    SELECT COALESCE(SUM(amount) FILTER (WHERE status = 'REVERSED'), 0)::bigint AS recovered,
           COUNT(*) FILTER (WHERE status = 'ANOMALY') AS anomalies,
           COUNT(*) FILTER (WHERE status = 'REVERSING') AS reversing,
           COUNT(*) FILTER (WHERE status = 'RELEASED') AS released,
           MIN(reason) FILTER (WHERE status = 'ANOMALY') AS anomaly_reason,
           MIN(reason) FILTER (WHERE status = 'RELEASED') AS release_reason
    FROM payment.refund_reversals
    WHERE refund_id = sqlc.arg(id)
) AS agg
WHERE r.id = sqlc.arg(id) AND r.reversal_status IN ('REVERSING', 'ANOMALY');

-- name: ReversingRefundIDs :many
-- 週期掃描：有 REVERSING 的撤回列、閒置超過門檻的退款（ANOMALY 不在內，partial index refund_reversals_reversing）。
SELECT refund_id
FROM payment.refund_reversals
WHERE status = 'REVERSING' AND updated_at <= sqlc.arg(idle_before) AND refund_id > sqlc.arg(after_id)
GROUP BY refund_id
ORDER BY refund_id
LIMIT sqlc.arg(batch);

-- name: OpenReversalTotalOnTransfer :one
-- 原轉帳上進行中與 ANOMALY 的撤回列額度（還沒累加到 transfers.reversed_amount，佔住剩餘可撤額）。
SELECT COALESCE(SUM(amount), 0)::bigint AS total
FROM payment.refund_reversals
WHERE payment_id = $1 AND target_refund_id IS NULL AND status IN ('REVERSING', 'ANOMALY');

-- name: ReversibleTopups :many
-- 付款底下成功的補差額轉帳（DONE）與各自的剩餘可撤額度（退款額 - 已撤回 - 進行中與 ANOMALY 的撤回列），依退款 ID 升序；已沒有額度的不列出。呼叫端已鎖付款列與轉帳列。
SELECT r.id, r.topup_external_id AS external_id,
       (r.amount - r.topup_reversed_amount - COALESCE((SELECT SUM(v.amount) FROM payment.refund_reversals AS v WHERE v.target_refund_id = r.id AND v.status IN ('REVERSING', 'ANOMALY')), 0))::bigint AS free
FROM payment.refunds AS r
WHERE r.payment_id = $1 AND r.topup_status = 'DONE'
  AND r.amount - r.topup_reversed_amount - COALESCE((SELECT SUM(v.amount) FROM payment.refund_reversals AS v WHERE v.target_refund_id = r.id AND v.status IN ('REVERSING', 'ANOMALY')), 0) > 0
ORDER BY r.id;

-- name: DoneTopupCapacity :one
-- 付款底下成功的補差額轉帳的剩餘可撤額度加總（退款額 - 已撤回，不扣預留；預留由 PendingReversalTotal 扣）。
SELECT COALESCE(SUM(amount - topup_reversed_amount), 0)::bigint AS total
FROM payment.refunds
WHERE payment_id = $1 AND topup_status = 'DONE';

-- name: AddTopupReversal :one
-- 累加補差額轉帳這次撤回的額度（同 transaction 已鎖擁有它的退款列；超過退款額時 CHECK refunds_topup_reversed_check 擋下）。
UPDATE payment.refunds
SET topup_reversed_amount = topup_reversed_amount + sqlc.arg(amount),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: LockRefundsByID :many
-- 依 ID 升序鎖一組退款列（撤回落地時鎖擁有者退款與補差額轉帳的擁有者）。
SELECT *
FROM payment.refunds
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id
FOR UPDATE;
