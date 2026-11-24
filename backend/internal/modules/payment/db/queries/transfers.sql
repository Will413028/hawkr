-- name: LockPaymentByOrderNo :one
-- 轉帳 use case 以訂單編號鎖付款列（付款列先於轉帳列與退款列）。
SELECT *
FROM payment.payments
WHERE order_no = $1
FOR UPDATE;

-- name: NonFailedRefundTotal :one
-- 付款已有的非失敗退款加總（進行中的也算）：轉帳金額是付款金額扣掉它。
SELECT COALESCE(SUM(amount), 0)::bigint AS total
FROM payment.refunds
WHERE payment_id = $1 AND status <> 'FAILED';

-- name: InFlightRefundTotal :one
-- 付款還沒終局的退款加總（PENDING、REFUNDING、ANOMALY）：它們失敗時賣家要收到這筆錢，轉帳不能先當成全額退款終結。
SELECT COALESCE(SUM(amount), 0)::bigint AS total
FROM payment.refunds
WHERE payment_id = $1 AND status IN ('PENDING', 'REFUNDING', 'ANOMALY');

-- name: InFlightRefundIDs :many
-- 付款還沒終局的退款 ID（與 InFlightRefundTotal 同一組狀態）：轉帳等它們終局時，log 指出擋住的是哪幾筆。
SELECT id
FROM payment.refunds
WHERE payment_id = $1 AND status IN ('PENDING', 'REFUNDING', 'ANOMALY')
ORDER BY id;

-- name: InsertTransfer :execrows
-- 一筆付款最多一筆有效（不是 FAILED 的）轉帳，只由唯一索引 transfers_active_payment 守（哪些狀態佔位只寫在索引的條件裡）：已有有效轉帳時不寫（ON CONFLICT DO NOTHING，不指定索引），呼叫端再讀既有的那筆。
INSERT INTO payment.transfers (id, payment_id, store_id, amount, destination_account_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(payment_id), sqlc.arg(store_id), sqlc.arg(amount), sqlc.arg(destination_account_id), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: ActiveTransferByPayment :one
SELECT *
FROM payment.transfers
WHERE payment_id = $1 AND status <> 'FAILED';

-- name: SucceededTransferByPayment :one
SELECT *
FROM payment.transfers
WHERE payment_id = $1 AND status = 'SUCCEEDED';

-- name: TransferByID :one
SELECT *
FROM payment.transfers
WHERE id = $1;

-- name: LockTransfer :one
-- 鎖轉帳列序列化 reversed_amount 的累加（付款列先於轉帳列，轉帳列先於退款列）。
SELECT *
FROM payment.transfers
WHERE id = $1
FOR UPDATE;

-- name: ClaimTransfer :one
-- 這把冪等鍵的第一次送出前（called_at 為空）把算定的金額與呼叫時刻一起寫入（呼叫端已鎖付款列與轉帳列、在同一個 transaction 算金額，D11）；called_at 已有值的列金額不再改（同一把鍵的金額不變）。
-- 只有 PENDING 的列寫。
UPDATE payment.transfers
SET amount = CASE WHEN called_at IS NULL THEN sqlc.arg(amount)::bigint ELSE amount END,
    called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: SucceedTransfer :one
UPDATE payment.transfers
SET status = 'SUCCEEDED',
    external_transfer_id = sqlc.arg(external_transfer_id),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: FailTransfer :one
UPDATE payment.transfers
SET status = 'FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: RotateTransferKey :one
-- provider 已知拒絕（可重試、沒有執行）：換新的冪等鍵（也是新的嘗試 ID）、清掉這把鍵的呼叫時刻與查回失敗次數，不計任何期限。只有仍用 old_key 的 PENDING 列換（並行的 job 不重複換）。
UPDATE payment.transfers
SET idempotency_key = gen_random_uuid(),
    called_at = NULL,
    lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING' AND idempotency_key = sqlc.arg(old_key)
RETURNING *;

-- name: BumpTransferLookupFailures :one
-- 查回失敗一次：累計連續失敗次數（成功的查回清零）。
UPDATE payment.transfers
SET lookup_failures = lookup_failures + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING lookup_failures;

-- name: ClearTransferLookupFailures :exec
UPDATE payment.transfers
SET lookup_failures = 0
WHERE id = sqlc.arg(id) AND status = 'PENDING' AND lookup_failures > 0;

-- name: AnomalyTransfer :one
-- provider 與本地紀錄不符、前置條件不成立或查回連續失敗：PENDING 進 ANOMALY（佔住唯一索引，補掃不再重試），原因記在 failure_reason。
UPDATE payment.transfers
SET status = 'ANOMALY',
    failure_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: ResolveTransferSucceeded :one
-- 後台人工結案：ANOMALY 的轉帳在 provider 確實存在（external_transfer_id 是證據），記成功並清掉原因；reversed_amount 記 provider 核實的累計已撤回額（本地還沒有撤回，列上是 0）。
UPDATE payment.transfers
SET status = 'SUCCEEDED',
    external_transfer_id = sqlc.arg(external_transfer_id),
    reversed_amount = sqlc.arg(verified_reversed)::bigint,
    failure_reason = NULL,
    called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz),
    lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: ResolveTransferFailed :one
-- 後台人工結案：確認 provider 沒有這筆轉帳，ANOMALY 進 FAILED（讓出唯一索引）。
UPDATE payment.transfers
SET status = 'FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: AddTransferReversal :one
-- 累加這次撤回的額度（同 transaction 已鎖轉帳列；超過 amount 時 CHECK transfers_reversed_check 擋下）。
UPDATE payment.transfers
SET reversed_amount = reversed_amount + sqlc.arg(amount),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: PendingTransferIDs :many
-- 週期掃描：PENDING、閒置超過門檻的轉帳（job 掉了的保底；每次呼叫都會更新 updated_at，重試中的不會被重複排；ANOMALY 不在內，partial index transfers_pending）。
SELECT id
FROM payment.transfers
WHERE status = 'PENDING' AND updated_at <= sqlc.arg(idle_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: RefundByID :one
SELECT *
FROM payment.refunds
WHERE id = $1;

-- name: LockRefund :one
SELECT *
FROM payment.refunds
WHERE id = $1
FOR UPDATE;

-- name: ReleaseRefundReversal :one
-- 釋出還沒開始的預留、不撤回：WAITING（退款在撤回開始前失敗、轉帳少付或失敗、人工結案後沒有額度）進 RELEASED，原因記在 reversal_reason，退款狀態不動。
-- 已開始的撤回（有撤回列）由撤回列各自收斂，彙總狀態由 SettleRefundReversal 重算。
UPDATE payment.refunds
SET reversal_status = 'RELEASED',
    reversal_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND reversal_status = 'WAITING'
RETURNING *;

-- name: StartableReversals :many
-- 可以開始撤回的退款：預留在 WAITING、provider 已接受退款（REFUNDING 或 SUCCEEDED）。呼叫端已鎖付款列與轉帳列；退款列依 ID 升序鎖。
SELECT *
FROM payment.refunds
WHERE payment_id = $1 AND reversal_status = 'WAITING' AND status IN ('REFUNDING', 'SUCCEEDED')
ORDER BY id
FOR UPDATE;

-- name: StartRefundReversal :one
-- 開始撤回：WAITING → REVERSING（退款已被 provider 接受才行，schema 的 refunds_reversal_after_acceptance_check 也守）。
UPDATE payment.refunds
SET reversal_status = 'REVERSING',
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND reversal_status = 'WAITING' AND status IN ('REFUNDING', 'SUCCEEDED')
RETURNING *;

-- name: WaitingReversals :many
-- 付款底下預留在 WAITING 的退款，依建立順序（再依 ID）鎖：轉帳 claim 時整批釋出、轉帳人工結案時依核實的額度重新分配。呼叫端已鎖付款列與轉帳列。
SELECT *
FROM payment.refunds
WHERE payment_id = $1 AND reversal_status = 'WAITING'
ORDER BY created_at, id
FOR UPDATE;

-- name: SetRefundReversalAmount :one
-- 轉帳人工結案後依核實的額度重新分配 WAITING 的預留額（大於 0 才用；0 的改走 ReleaseRefundReversal）。
UPDATE payment.refunds
SET reversal_amount = sqlc.arg(reversal_amount),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND reversal_status = 'WAITING'
RETURNING *;

-- name: RefundByRequestKey :one
SELECT *
FROM payment.refunds
WHERE payment_id = sqlc.arg(payment_id) AND request_key = sqlc.arg(request_key);

-- name: PendingReversalTotal :one
-- 已預留、還沒成立的撤回額度，逐列算：WAITING 的退款還沒分成撤回列，以預留額計；其餘以撤回列計（REVERSING 與 ANOMALY 的撤回列佔住額度，REVERSED、RELEASED 的不佔），
-- 與撤回目標的剩餘額（OpenReversalTotalOnTransfer、ReversibleTopups）同一個口徑。
SELECT (
    COALESCE((SELECT SUM(w.reversal_amount) FROM payment.refunds AS w WHERE w.payment_id = $1 AND w.reversal_status = 'WAITING'), 0)
    + COALESCE((SELECT SUM(v.amount) FROM payment.refund_reversals AS v WHERE v.payment_id = $1 AND v.status IN ('REVERSING', 'ANOMALY')), 0)
)::bigint AS total;

-- name: InsertRefund :one
INSERT INTO payment.refunds (id, payment_id, request_key, amount, auto, reversal_status, reversal_amount, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(payment_id), sqlc.arg(request_key), sqlc.arg(amount), sqlc.arg(auto), sqlc.arg(reversal_status), sqlc.arg(reversal_amount), sqlc.arg(now), sqlc.arg(now))
RETURNING *;

-- name: MarkRefundCalled :one
-- 退款 provider 之前先記「已呼叫」的時刻（這把鍵已記過的保留最早的）。只有 PENDING 的列寫。
UPDATE payment.refunds
SET refund_called_at = COALESCE(refund_called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: ApplyRefundSnapshot :one
-- 退款呼叫的結果（呼叫的回應、查回找到的、或重讀 REFUNDING）：進 FAILED 時，已算進轉帳金額的退款（transfer_deducted）同一個 statement 進補差額的 WAITING；PENDING 與 REFUNDING 依 provider 的狀態進 REFUNDING、SUCCEEDED 或 FAILED，並記 provider 的退款 ID；
-- 已記過別的退款 ID 的列不寫（呼叫端當孤兒處理）。終局（SUCCEEDED、FAILED）與 ANOMALY 不在 WHERE 內，不會被倒退。
UPDATE payment.refunds
SET status = sqlc.arg(status),
    reversal_status = CASE WHEN sqlc.arg(status)::text = 'FAILED' AND reversal_status = 'WAITING' THEN 'RELEASED' ELSE reversal_status END,
    reversal_reason = CASE WHEN sqlc.arg(status)::text = 'FAILED' AND reversal_status = 'WAITING' THEN 'refund_failed' ELSE reversal_reason END,
    topup_status = CASE WHEN sqlc.arg(status)::text = 'FAILED' AND transfer_deducted THEN 'WAITING' ELSE topup_status END,
    external_refund_id = sqlc.arg(external_refund_id),
    refund_called_at = COALESCE(refund_called_at, sqlc.arg(now)::timestamptz),
    refund_lookup_failures = 0,
    failure_reason = sqlc.narg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status IN ('PENDING', 'REFUNDING')
  AND (external_refund_id IS NULL OR external_refund_id = sqlc.arg(external_refund_id))
RETURNING *;

-- name: FailRefundCall :one
-- 退款呼叫的終局失敗（provider 永久拒絕，或查回證明上一次不存在又過了最短等待）：進 FAILED，不佔額度；撤回預留在 WAITING 的同一個 statement 釋出（RELEASED，refund_failed）。
UPDATE payment.refunds
SET status = 'FAILED',
    reversal_status = CASE WHEN reversal_status = 'WAITING' THEN 'RELEASED' ELSE reversal_status END,
    reversal_reason = CASE WHEN reversal_status = 'WAITING' THEN 'refund_failed' ELSE reversal_reason END,
    topup_status = CASE WHEN transfer_deducted THEN 'WAITING' ELSE topup_status END,
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: RotateRefundKey :one
-- 退款被 provider 已知拒絕（可重試、沒有執行）：換新的退款冪等鍵（也是新的嘗試 ID），清掉呼叫時刻與查回失敗次數，不計任何期限。
UPDATE payment.refunds
SET idempotency_key = gen_random_uuid(),
    refund_called_at = NULL,
    refund_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING' AND idempotency_key = sqlc.arg(old_key)
RETURNING *;

-- name: BumpRefundLookupFailures :one
UPDATE payment.refunds
SET refund_lookup_failures = refund_lookup_failures + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING refund_lookup_failures;

-- name: ClearRefundLookupFailures :exec
UPDATE payment.refunds
SET refund_lookup_failures = 0
WHERE id = sqlc.arg(id) AND status = 'PENDING' AND refund_lookup_failures > 0;

-- name: AnomalyRefund :one
-- 退款與 provider 不符、前置條件不成立或查回連續失敗：進 ANOMALY（仍佔額度，補掃不再重試），原因記在 failure_reason；從 REFUNDING 進來時保留已知的 external_refund_id。
UPDATE payment.refunds
SET status = 'ANOMALY',
    failure_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status IN ('PENDING', 'REFUNDING')
RETURNING *;

-- name: ResolveRefundRefunding :one
-- 後台人工結案（退款的 ANOMALY）：provider 確實建立了退款（external_refund_id 是證據），暫記 REFUNDING，真正的狀態由重讀推導。
UPDATE payment.refunds
SET status = 'REFUNDING',
    external_refund_id = sqlc.arg(external_refund_id),
    refund_called_at = COALESCE(refund_called_at, sqlc.arg(now)::timestamptz),
    refund_lookup_failures = 0,
    failure_reason = NULL,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: ResolveRefundCallFailed :one
-- 後台人工結案（退款的 ANOMALY）：確認 provider 沒有建立退款，進 FAILED（不佔額度）。
UPDATE payment.refunds
SET status = 'FAILED',
    reversal_status = CASE WHEN reversal_status = 'WAITING' THEN 'RELEASED' ELSE reversal_status END,
    reversal_reason = CASE WHEN reversal_status = 'WAITING' THEN 'refund_failed' ELSE reversal_reason END,
    topup_status = CASE WHEN transfer_deducted THEN 'WAITING' ELSE topup_status END,
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: RefundByExternal :one
-- 事件的路由鍵對回退款列：只認付款屬於目前 provider 的退款（別的 provider 的退款 ID 不同源）。
SELECT r.*
FROM payment.refunds r
JOIN payment.payments p ON p.id = r.payment_id
WHERE p.provider = sqlc.arg(provider) AND r.external_refund_id = sqlc.arg(external_refund_id);

-- name: RefundIDsByExternal :many
-- 事件的路由鍵（provider 的退款 ID）對回退款列，給掃描補排有舊的未處理事件的退款同步。
SELECT r.id
FROM payment.refunds r
JOIN payment.payments p ON p.id = r.payment_id
WHERE p.provider = sqlc.arg(provider) AND r.external_refund_id = ANY(sqlc.arg(external_refund_ids)::text[]);

-- name: CallableRefundIDs :many
-- 週期掃描：PENDING 閒置超過門檻的退款（job 掉了的保底；ANOMALY 不在內，partial index refunds_open）。
SELECT id
FROM payment.refunds
WHERE status = 'PENDING' AND updated_at <= sqlc.arg(idle_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: RefundingRefundIDs :many
-- 週期掃描：REFUNDING 閒置超過門檻的退款（漏掉 webhook 的保底，補排重讀）。
SELECT id
FROM payment.refunds
WHERE status = 'REFUNDING' AND updated_at <= sqlc.arg(idle_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: RefundsByRequestKeys :many
-- 舊服務（legacy paymentserver）以呼叫端的冪等鍵（售後的案件 ID）讀退款：鍵只在同一筆付款內唯一，所以可能回多列，由呼叫方判斷。
SELECT sqlc.embed(r), p.order_no
FROM payment.refunds r
JOIN payment.payments p ON p.id = r.payment_id
WHERE r.request_key = ANY(sqlc.arg(request_keys)::text[]);

-- name: LatestRefundsByOrderNos :many
-- 每張訂單最新的一筆退款（建立時間，同時間依 ID）。
SELECT DISTINCT ON (p.order_no) sqlc.embed(r), p.order_no
FROM payment.refunds r
JOIN payment.payments p ON p.id = r.payment_id
WHERE p.order_no = ANY(sqlc.arg(order_nos)::text[])
ORDER BY p.order_no, r.created_at DESC, r.id DESC;
