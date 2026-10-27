-- name: PayUniRegistrationByStore :one
SELECT *
FROM seller.payuni_registrations
WHERE store_id = $1;

-- name: PayUniRegistrationsByStores :many
-- 批次讀取店家的 PayUni 註冊；沒有註冊的店家沒有列。
SELECT *
FROM seller.payuni_registrations
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]);


-- name: PayUniRegistrationByMerID :one
SELECT *
FROM seller.payuni_registrations
WHERE mer_id = $1;

-- name: LockPayUniRegistration :one
-- 鎖定註冊列序列化它的寫入（鎖順序：店家列、申請列先於註冊列；job 只鎖註冊列）。
SELECT *
FROM seller.payuni_registrations
WHERE store_id = $1
FOR UPDATE;

-- name: UpsertPayUniIntent :one
-- 核准時寫入或刷新綁定意圖：沒有列就建立；已有列時 intent_seq 加一讓 job 重新送出，PayUni 的狀態與觀測紀錄保留（那是外部
-- 事實）。還沒綁定過的列（bound_at 為 NULL：PENDING，或失敗的）回到 PENDING 重新綁定；已綁定的列（PayUni 端已有這個商店，
-- PayUni 拒絕之後重新送出再核准）維持 BOUND，由 job 以 update_merchant 送新的資料（再 bind 只會得到 AGENT04017），並記下
-- resubmitted_at 讓輪詢在一段期間內繼續問新的結果。
INSERT INTO seller.payuni_registrations (store_id, mer_id, application_id, created_at, updated_at)
VALUES (sqlc.arg(store_id), sqlc.arg(mer_id), sqlc.arg(application_id), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (store_id) DO UPDATE
SET application_id = EXCLUDED.application_id,
    bind_status = CASE WHEN seller.payuni_registrations.bound_at IS NULL THEN 'PENDING' ELSE 'BOUND' END,
    bind_error_code = NULL,
    bind_failed_at = NULL,
    bind_attempts = CASE WHEN seller.payuni_registrations.bound_at IS NULL THEN 0 ELSE seller.payuni_registrations.bind_attempts END,
    resubmitted_at = CASE WHEN seller.payuni_registrations.bound_at IS NULL THEN NULL ELSE EXCLUDED.updated_at END,
    sync_error_code = NULL,
    intent_seq = seller.payuni_registrations.intent_seq + 1,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: RebindPayUni :one
-- 管理員重新綁定：只有 FAILED 的列回到 PENDING（歸零重試次數），intent_seq 加一；其他狀態 0 列。
UPDATE seller.payuni_registrations
SET bind_status = 'PENDING',
    bind_error_code = NULL,
    bind_failed_at = NULL,
    bind_attempts = 0,
    intent_seq = intent_seq + 1,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND bind_status = 'FAILED'
RETURNING *;

-- name: BumpPayUniIntent :one
-- 需要送給 PayUni 的資料改變了（例如店家改地址）：intent_seq 加一；job 讀的永遠是目前的值，連續的修改自然合併。
UPDATE seller.payuni_registrations
SET intent_seq = intent_seq + 1,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id)
RETURNING *;

-- name: CountPayUniBindAttempt :one
-- job 開始時為還在綁定的列（PENDING）計一次嘗試；呼叫方已鎖住這一列。
UPDATE seller.payuni_registrations
SET bind_attempts = bind_attempts + 1,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND bind_status = 'PENDING'
RETURNING *;

-- name: MarkPayUniBound :one
UPDATE seller.payuni_registrations
SET bind_status = 'BOUND',
    bind_error_code = NULL,
    bind_failed_at = NULL,
    bound_at = sqlc.arg(now),
    synced_seq = sqlc.arg(synced_seq),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id)
RETURNING *;

-- name: MarkPayUniBindFailed :one
UPDATE seller.payuni_registrations
SET bind_status = 'FAILED',
    bind_error_code = sqlc.arg(error_code),
    bind_failed_at = sqlc.arg(now)::timestamptz,
    synced_seq = sqlc.arg(synced_seq),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id)
RETURNING *;

-- name: MarkPayUniSynced :one
-- 聯絡欄位與地址的同步結果：error_code 為 NULL 表示成功；被永久拒絕時記下代碼，synced_seq 照樣前進（賣家再改一次才重送）。
UPDATE seller.payuni_registrations
SET synced_seq = sqlc.arg(synced_seq),
    sync_error_code = sqlc.narg(error_code),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id)
RETURNING *;

-- name: ApplyPayUniStatus :one
-- 套用一次觀測的結果。唯一的順序保護：只有事件時間比目前狀態新的才寫入（亂序到達的舊通知 0 列，不倒退）；
-- 狀態碼與 outcome 同一個 UPDATE 寫入，合併（部分觀測保留沒回報的欄位）在呼叫方、鎖住這一列之後做。
-- 個資清理過的註冊列不再存 reason（PayUni 的自由文字）：關店之後才到的通知只更新狀態碼。
UPDATE seller.payuni_registrations
SET mer_status = sqlc.narg(mer_status),
    member_status = sqlc.narg(member_status),
    credit_status = sqlc.narg(credit_status),
    applepay_status = sqlc.narg(applepay_status),
    googlepay_status = sqlc.narg(googlepay_status),
    atm_status = sqlc.narg(atm_status),
    reason = CASE WHEN pii_purged_at IS NULL THEN sqlc.narg(reason)::text END,
    outcome = sqlc.arg(outcome),
    status_observed_at = sqlc.arg(executed_at),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND (status_observed_at IS NULL OR status_observed_at < sqlc.arg(executed_at))
RETURNING *;

-- name: TouchPayUniChecked :exec
-- 輪詢與手動刷新問過 PayUni 的時間（不論結果有沒有比目前新），排輪詢用。
UPDATE seller.payuni_registrations
SET status_checked_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id);

-- name: InsertPayUniObservation :one
-- 觀測紀錄只有這一支寫入（append-only，個資清理除外）；(store_id, event_key) 重複時不寫入、回 0 列。店家的個資已經清理
-- 的話，reason 不存（關店之後才到的通知）。
INSERT INTO seller.payuni_observations (
    observation_id, store_id, source, event_key, mer_status, member_status, credit_status, applepay_status, googlepay_status,
    atm_status, reason, executed_at, received_at
)
VALUES (
    sqlc.arg(observation_id), sqlc.arg(store_id), sqlc.arg(source), sqlc.narg(event_key), sqlc.narg(mer_status),
    sqlc.narg(member_status), sqlc.narg(credit_status), sqlc.narg(applepay_status), sqlc.narg(googlepay_status),
    sqlc.narg(atm_status),
    CASE WHEN EXISTS (SELECT 1 FROM seller.stores s WHERE s.store_id = sqlc.arg(store_id) AND s.pii_purged_at IS NOT NULL)
         THEN NULL ELSE sqlc.narg(reason)::text END,
    sqlc.arg(executed_at), sqlc.arg(now)
)
ON CONFLICT (store_id, event_key) DO NOTHING
RETURNING *;

-- name: PayUniObservations :many
-- 店家的觀測紀錄，新到舊的 keyset（executed_at, observation_id）；索引 payuni_observations_store 涵蓋這個排序。
SELECT *
FROM seller.payuni_observations
WHERE store_id = sqlc.arg(store_id)
  AND (
    sqlc.narg(after_executed_at)::timestamptz IS NULL OR
    (executed_at, observation_id) < (sqlc.narg(after_executed_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY executed_at DESC, observation_id DESC
LIMIT sqlc.arg(page_size);

-- name: PayUniRegistrationsToPoll :many
-- 已綁定、PayUni 還沒有定論（沒有觀測、審核中、待補件、不認得的狀態）、超過間隔沒問過的列；關閉的店家不問。PayUni 拒絕之後
-- 重新送出再核准的列，在 resubmitted_after 之後也算還沒有定論（舊的拒絕不是新的結果）。
SELECT r.store_id
FROM seller.payuni_registrations r
JOIN seller.stores s ON s.store_id = r.store_id
WHERE r.bind_status = 'BOUND'
  AND (r.outcome IN ('NONE', 'UNDER_REVIEW', 'PENDING_INFO', 'UNRECOGNIZED') OR (r.outcome = 'REJECTED' AND r.resubmitted_at > sqlc.arg(resubmitted_after)::timestamptz))
  AND (r.status_checked_at IS NULL OR r.status_checked_at < sqlc.arg(checked_before))
  AND s.status <> 'CLOSED'
ORDER BY r.status_checked_at NULLS FIRST, r.store_id
LIMIT sqlc.arg(batch_size);

-- name: PayUniRegistrationsToResync :many
-- 有資料還沒送出、閒置超過一段時間的列（job 遺失、當掉或被丟棄時的保底）；FAILED 要管理員重新綁定，關閉的店家不送。
SELECT r.store_id
FROM seller.payuni_registrations r
JOIN seller.stores s ON s.store_id = r.store_id
WHERE r.bind_status <> 'FAILED'
  AND r.synced_seq < r.intent_seq
  AND r.updated_at < sqlc.arg(idle_before)
  AND s.status <> 'CLOSED'
ORDER BY r.updated_at, r.store_id
LIMIT sqlc.arg(batch_size);
