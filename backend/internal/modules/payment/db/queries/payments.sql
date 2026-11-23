-- name: InsertPayment :execrows
-- 建立付款的 transaction 寫「建立中」的列（沒有外部 ID）。order_no 唯一（D8：一張訂單一個 PaymentIntent）：同一張訂單已有列時不寫（ON CONFLICT DO NOTHING），
-- 呼叫端再讀既有的列決定怎麼處理。
INSERT INTO payment.payments (id, order_no, store_id, buyer_id, amount, provider, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(order_no), sqlc.arg(store_id), sqlc.arg(buyer_id), sqlc.arg(amount), sqlc.arg(provider), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (order_no) DO NOTHING;

-- name: PaymentByID :one
SELECT *
FROM payment.payments
WHERE id = $1;

-- name: PaymentByOrderNo :one
SELECT *
FROM payment.payments
WHERE order_no = $1;

-- name: PaymentByExternal :one
SELECT *
FROM payment.payments
WHERE provider = $1 AND external_payment_id = $2;

-- name: LockPayment :one
-- 鎖定付款列序列化它的寫入（付款列先於退款列與轉帳列）。
SELECT *
FROM payment.payments
WHERE id = $1
FOR UPDATE;

-- name: MarkPaymentCreateCalled :one
-- 呼叫 provider 之前先記「已呼叫」的時刻（已記過的保留最早的）：之後列上就分得出「從沒呼叫」與「呼叫過、結果不明」。只有還在建立中的列寫。
UPDATE payment.payments
SET create_called_at = COALESCE(create_called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATING'
RETURNING *;

-- name: ApplyPaymentIntentSnapshot :one
-- 把 provider 讀到的快照寫進列（外部 ID 只在還沒有時寫）。只在這次讀取的開始時間不早於已存的 fetched_at 時寫入（較早開始的讀取不覆蓋較晚的）；
-- 沒有寫入時回 no rows。呼叫方已在同 transaction 鎖住列並算好下一個狀態。
UPDATE payment.payments
SET status = sqlc.arg(status),
    external_payment_id = COALESCE(external_payment_id, sqlc.narg(external_payment_id)),
    external_charge_id = sqlc.narg(external_charge_id),
    paid_at = sqlc.narg(paid_at),
    last_error = sqlc.narg(last_error),
    fetched_at = sqlc.arg(fetched_at),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND (fetched_at IS NULL OR fetched_at <= sqlc.arg(fetched_at))
RETURNING *;

-- name: FailPaymentCreate :one
-- 建立的終局：只有還在建立中、沒有外部 ID 的列能進 CREATE_FAILED。
UPDATE payment.payments
SET status = 'CREATE_FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATING' AND external_payment_id IS NULL
RETURNING *;

-- name: ResetFailedPayment :one
-- 買家對建立失敗的訂單再次付款：同一列換新冪等鍵、重設回建立中（一張訂單仍只有一列）；訂單資料以這次讀到的為準。
UPDATE payment.payments
SET status = 'CREATING',
    failure_reason = NULL,
    create_called_at = NULL,
    create_idempotency_key = gen_random_uuid(),
    store_id = sqlc.arg(store_id),
    buyer_id = sqlc.arg(buyer_id),
    amount = sqlc.arg(amount),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATE_FAILED'
RETURNING *;

-- name: CreatingPaymentIDs :many
-- 週期掃描：還在建立中、閒置超過門檻的列（job 掉了的保底；呼叫過的從第一次呼叫起算）。
SELECT id
FROM payment.payments
WHERE status = 'CREATING' AND COALESCE(create_called_at, created_at) <= sqlc.arg(idle_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: PollablePaymentIDs :many
-- 週期掃描：已建立、還沒結案的列，超過門檻沒有讀過就補排同步（漏掉 webhook 的保底）。
SELECT id
FROM payment.payments
WHERE status IN ('AWAITING_PAYMENT', 'PROCESSING') AND COALESCE(fetched_at, created_at) <= sqlc.arg(read_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: PaymentIDsByExternal :many
SELECT id
FROM payment.payments
WHERE provider = sqlc.arg(provider) AND external_payment_id = ANY(sqlc.arg(external_payment_ids)::text[])
ORDER BY id;
