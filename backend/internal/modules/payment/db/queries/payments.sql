-- name: InsertPayment :execrows
-- 建立付款的 transaction 寫「建立中」的列（沒有外部 ID）。order_no 唯一（D8：一張訂單一個 PaymentIntent）：同一張訂單已有列時不寫（ON CONFLICT DO NOTHING），
-- 呼叫端再讀既有的列決定怎麼處理。
INSERT INTO payment.payments (id, order_no, store_id, buyer_id, amount, provider, description, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(order_no), sqlc.arg(store_id), sqlc.arg(buyer_id), sqlc.arg(amount), sqlc.arg(provider), sqlc.arg(description), sqlc.arg(now), sqlc.arg(now))
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
-- PaymentIntent 的外部 ID 經歷程表對回付款列：舊嘗試（付款列已換新 ID）的 ID 也對得到。
SELECT p.*
FROM payment.payments p
JOIN payment.payment_attempts a ON a.payment_id = p.id
WHERE a.provider = sqlc.arg(provider) AND a.external_payment_id = sqlc.arg(external_payment_id);

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
-- 把有效嘗試的快照寫進付款列：外部 ID、冪等鍵、呼叫時刻與 provider 取自這次的嘗試（有效嘗試時與列上相同；舊嘗試晚到成功而換成有效時，列改成它的）。
-- 呼叫方已在同 transaction 鎖住列、以嘗試列的 fetched_at 擋下過期的讀取並算好下一個狀態。
UPDATE payment.payments
SET status = sqlc.arg(status),
    provider = sqlc.arg(provider),
    external_payment_id = sqlc.arg(external_payment_id),
    create_idempotency_key = sqlc.arg(create_idempotency_key),
    create_called_at = sqlc.arg(create_called_at),
    external_charge_id = sqlc.narg(external_charge_id),
    paid_at = sqlc.narg(paid_at),
    last_error = sqlc.narg(last_error),
    fetched_at = sqlc.arg(fetched_at),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: RotateCreateKey :one
-- provider 已知拒絕（可重試、沒有執行）：付款列換新的冪等鍵、清掉呼叫時刻與查回失敗次數，不計任何期限；呼叫方同 transaction 取代有效嘗試並寫新的一列。
-- 只有仍用 old_key 的建立中列換（並行的呼叫不重複換）。
UPDATE payment.payments
SET create_idempotency_key = gen_random_uuid(),
    create_called_at = NULL,
    create_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATING' AND external_payment_id IS NULL AND create_idempotency_key = sqlc.arg(old_key)
RETURNING *;

-- name: BumpCreateLookupFailures :one
UPDATE payment.payments
SET create_lookup_failures = create_lookup_failures + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATING' AND external_payment_id IS NULL
RETURNING create_lookup_failures;

-- name: ClearCreateLookupFailures :exec
UPDATE payment.payments
SET create_lookup_failures = 0
WHERE id = sqlc.arg(id) AND status = 'CREATING' AND create_lookup_failures > 0;

-- name: AnomalyPayment :one
-- provider 與本地紀錄不符或查回連續失敗：還在建立中、沒有外部 ID 的列進 ANOMALY（佔住這張訂單，補掃不再重試），原因記在 failure_reason。
UPDATE payment.payments
SET status = 'ANOMALY',
    failure_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATING' AND external_payment_id IS NULL
RETURNING *;

-- name: ResolvePaymentLinked :one
-- 後台人工結案：ANOMALY 的付款在 provider 確實有 PaymentIntent（external_payment_id 是證據）：連上它、暫記 AWAITING_PAYMENT，真正的狀態由隨後的同步重讀推導。
UPDATE payment.payments
SET status = 'AWAITING_PAYMENT',
    external_payment_id = sqlc.arg(external_payment_id),
    failure_reason = NULL,
    create_called_at = COALESCE(create_called_at, sqlc.arg(now)::timestamptz),
    create_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY' AND external_payment_id IS NULL
RETURNING *;

-- name: ResolvePaymentFailed :one
-- 後台人工結案：確認 provider 沒有這筆付款，ANOMALY 進 CREATE_FAILED（買家可以再付，換新鍵）。
UPDATE payment.payments
SET status = 'CREATE_FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: EffectiveAttempt :one
SELECT *
FROM payment.payment_attempts
WHERE payment_id = sqlc.arg(payment_id) AND superseded_at IS NULL;

-- name: LinkEffectiveAttempt :one
-- 後台人工結案：有效嘗試連上 provider 的 PaymentIntent（還沒有外部 ID 的才連）。
UPDATE payment.payment_attempts
SET external_payment_id = sqlc.arg(external_payment_id),
    called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz)
WHERE payment_id = sqlc.arg(payment_id) AND superseded_at IS NULL AND external_payment_id IS NULL
RETURNING *;

-- name: FailPaymentCreate :one
-- 建立的終局：只有還在建立中、沒有外部 ID 的列能進 CREATE_FAILED。
UPDATE payment.payments
SET status = 'CREATE_FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATING' AND external_payment_id IS NULL
RETURNING *;

-- name: ResetPayment :one
-- 買家對「不會再成功」的付款再次付款（建立失敗的終局，或已取消而訂單仍可付）：同一列換新冪等鍵、清掉舊的外部 ID 與讀取結果、重設回建立中、provider 與說明以目前的為準
-- （一張訂單仍只有一列、一次只有一個有效 PaymentIntent）；訂單資料以這次讀到的為準。
UPDATE payment.payments
SET status = 'CREATING',
    failure_reason = NULL,
    last_error = NULL,
    external_payment_id = NULL,
    external_charge_id = NULL,
    paid_at = NULL,
    fetched_at = NULL,
    create_called_at = NULL,
    create_lookup_failures = 0,
    create_idempotency_key = gen_random_uuid(),
    provider = sqlc.arg(provider),
    description = sqlc.arg(description),
    store_id = sqlc.arg(store_id),
    buyer_id = sqlc.arg(buyer_id),
    amount = sqlc.arg(amount),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status IN ('CREATE_FAILED', 'CANCELED')
RETURNING *;

-- name: RebindCreatingPayment :one
-- 建立中、沒有外部 ID 的列屬於別的 provider（設定改了）：只在舊 provider 的呼叫從沒送出（create_called_at 為空）時改成目前的 provider，換新冪等鍵。
-- 呼叫過的列不改綁：舊 provider 已無法查回，無從證明它沒有建出 PaymentIntent，改綁會讓買家拿到兩個；那樣的列由呼叫方進 ANOMALY。
UPDATE payment.payments
SET provider = sqlc.arg(provider),
    create_idempotency_key = gen_random_uuid(),
    create_called_at = NULL,
    create_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATING' AND external_payment_id IS NULL AND provider <> sqlc.arg(provider)
  AND create_called_at IS NULL
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
SELECT DISTINCT a.payment_id
FROM payment.payment_attempts a
WHERE a.provider = sqlc.arg(provider) AND a.external_payment_id = ANY(sqlc.arg(external_payment_ids)::text[])
ORDER BY a.payment_id;

-- name: EnsureEffectiveAttempt :exec
-- 建立中的列沒有有效嘗試時寫一列（provider 與冪等鍵取自付款列）；已有就不寫（唯一索引 payment_attempts_effective）。
INSERT INTO payment.payment_attempts (id, payment_id, provider, idempotency_key, created_at)
SELECT sqlc.arg(id), p.id, p.provider, p.create_idempotency_key, sqlc.arg(now)
FROM payment.payments p
WHERE p.id = sqlc.arg(payment_id) AND p.status = 'CREATING'
ON CONFLICT (payment_id) WHERE superseded_at IS NULL DO NOTHING;

-- name: SupersedeEffectiveAttempt :one
-- 把有效的嘗試標成已被取代（買家重付、改綁 provider、或舊嘗試晚到成功而換成它）；沒有有效嘗試時回 no rows。
UPDATE payment.payment_attempts
SET superseded_at = sqlc.arg(now)
WHERE payment_id = sqlc.arg(payment_id) AND superseded_at IS NULL
RETURNING *;

-- name: ReviveAttempt :one
-- 舊嘗試晚到成功、付款列改以它入帳：它成為有效的嘗試（呼叫方先取代現有的有效嘗試）。
UPDATE payment.payment_attempts
SET superseded_at = NULL
WHERE id = sqlc.arg(id) AND superseded_at IS NOT NULL
RETURNING *;

-- name: AttemptByID :one
SELECT *
FROM payment.payment_attempts
WHERE id = sqlc.arg(id) AND payment_id = sqlc.arg(payment_id);

-- name: AttemptByIDOnly :one
SELECT *
FROM payment.payment_attempts
WHERE id = sqlc.arg(id);

-- name: AttemptsOfPayment :many
-- 付款的嘗試，依建立順序；同步要重讀的 PaymentIntent 都在這裡。
SELECT *
FROM payment.payment_attempts
WHERE payment_id = sqlc.arg(payment_id) AND provider = sqlc.arg(provider) AND external_payment_id IS NOT NULL
ORDER BY created_at, id;

-- name: MarkAttemptCalled :one
-- 呼叫 provider 之前記下有效嘗試的呼叫時刻（保留最早的）。
UPDATE payment.payment_attempts
SET called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz)
WHERE payment_id = sqlc.arg(payment_id) AND idempotency_key = sqlc.arg(idempotency_key) AND superseded_at IS NULL
RETURNING *;

-- name: ApplyAttemptSnapshot :one
-- 把讀到的狀態記進嘗試列（外部 ID 只在還沒有時寫；入帳的狀態不倒退）。只在這次讀取的開始時間不早於已存的 fetched_at 時寫入；沒有寫入時回 no rows。
UPDATE payment.payment_attempts
SET external_payment_id = COALESCE(external_payment_id, sqlc.arg(external_payment_id)::text),
    state = CASE WHEN state = 'SUCCEEDED' THEN state ELSE sqlc.arg(state)::text END,
    fetched_at = sqlc.arg(fetched_at)
WHERE id = sqlc.arg(id) AND (fetched_at IS NULL OR fetched_at <= sqlc.arg(fetched_at)::timestamptz)
RETURNING *;

-- name: MarkAttemptExcessPaid :exec
-- 付款列已經以另一個 PaymentIntent 入帳，這個舊嘗試也入帳了：記下重複入帳（保留最早的時刻），給後台查。
UPDATE payment.payment_attempts
SET excess_paid_at = COALESCE(excess_paid_at, sqlc.arg(now)::timestamptz)
WHERE id = sqlc.arg(id) AND state = 'SUCCEEDED' AND superseded_at IS NOT NULL;

-- name: ExcessPaidAttempts :many
-- 後台查詢：重複入帳的嘗試（同一張訂單兩個 PaymentIntent 都扣到款）。
SELECT *
FROM payment.payment_attempts
WHERE excess_paid_at IS NOT NULL AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: StaleAttemptPaymentIDs :many
-- 週期掃描：被取代、沒有結案（狀態不是入帳或取消）又超過門檻沒讀過的嘗試所屬的付款，補排同步（舊嘗試的成功或取消沒有留在本地時的保底）。
SELECT DISTINCT payment_id
FROM payment.payment_attempts
WHERE superseded_at IS NOT NULL AND external_payment_id IS NOT NULL AND (state IS NULL OR state NOT IN ('SUCCEEDED', 'CANCELED'))
  AND COALESCE(fetched_at, created_at) <= sqlc.arg(read_before)::timestamptz AND payment_id > sqlc.arg(after_id)
ORDER BY payment_id
LIMIT sqlc.arg(batch);
