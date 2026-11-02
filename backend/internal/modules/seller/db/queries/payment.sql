-- name: PaymentAccountByStore :one
SELECT *
FROM seller.payment_accounts
WHERE store_id = $1;

-- name: PaymentAccountsByStores :many
-- 批次讀取店家的收款帳戶；沒有帳戶的店家沒有列。
SELECT *
FROM seller.payment_accounts
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]);

-- name: PaymentAccountByExternal :one
SELECT *
FROM seller.payment_accounts
WHERE provider = $1 AND external_account_id = $2;

-- name: LockPaymentAccount :one
-- 鎖定帳戶列序列化它的寫入（鎖順序：店家列先於帳戶列，帳戶列先於公告列）。
SELECT *
FROM seller.payment_accounts
WHERE store_id = $1
FOR UPDATE;

-- name: InsertPaymentAccount :execrows
-- 核准時寫「建立中」的列（沒有外部 ID），contact_email 是申請上的收款聯絡信箱；一店一列，關店再開沿用既有的列；已有列且還沒有外部 ID 時（建立失敗或 job 還沒跑）以這次申請的信箱覆寫，建立才會用最新的信箱；已有外部 ID 的列保留原信箱（建立之後改信箱由 D23 延後處理）。
INSERT INTO seller.payment_accounts (store_id, provider, contact_email, requirement_collection, created_at, updated_at)
VALUES (sqlc.arg(store_id), sqlc.arg(provider), sqlc.arg(contact_email), sqlc.arg(requirement_collection), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (store_id) DO UPDATE
SET contact_email = EXCLUDED.contact_email,
    updated_at = EXCLUDED.updated_at
WHERE seller.payment_accounts.external_account_id IS NULL;

-- name: SetPaymentAccountExternal :one
-- 外部帳戶建立後填上外部 ID，狀態從 NOT_CREATED 進 ACTION_REQUIRED（新帳戶要賣家補資料，同步 job 重讀後覆寫）；只在還沒有外部 ID 時寫（重送安全）。
UPDATE seller.payment_accounts
SET external_account_id = sqlc.arg(external_account_id),
    status = 'ACTION_REQUIRED',
    create_failed_at = NULL,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NULL
RETURNING *;

-- name: MarkPaymentAccountCreateFailed :execrows
-- 建立重試用完：記下失敗的時刻（進後台待辦）；已經有外部 ID 的列不寫。
UPDATE seller.payment_accounts
SET create_failed_at = sqlc.arg(now),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NULL AND create_failed_at IS NULL;

-- name: ClearPaymentAccountCreateFailed :execrows
-- 管理員重試建立：只有建立失敗的列清空失敗時刻，由呼叫方同一個 transaction 重新排入建立 job。
UPDATE seller.payment_accounts
SET create_failed_at = NULL,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NULL AND create_failed_at IS NOT NULL;

-- name: ApplyPaymentSnapshot :one
-- 同步 job 重讀外部帳戶之後整列覆寫快取（沒有事件時間排序，重讀即最新）；沒有外部 ID 的列不寫。
UPDATE seller.payment_accounts
SET status = sqlc.arg(status),
    transfers_active = sqlc.arg(transfers_active),
    payouts_active = sqlc.arg(payouts_active),
    requirements = sqlc.arg(requirements),
    status_reason = sqlc.narg(status_reason),
    fetched_at = sqlc.arg(fetched_at),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NOT NULL
RETURNING *;

-- name: InsertPaymentProviderEvent :execrows
-- webhook inbox：(provider, event_id) 唯一，重送的事件 0 列。
INSERT INTO seller.payment_provider_events (provider, event_id, external_account_id, received_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (provider, event_id) DO NOTHING;

-- name: MarkPaymentEventsProcessed :execrows
-- 同步 job 讀外部帳戶之前收到的事件，已經被這次的重讀涵蓋。
UPDATE seller.payment_provider_events
SET processed_at = sqlc.arg(now)
WHERE provider = sqlc.arg(provider) AND external_account_id = sqlc.arg(external_account_id)
  AND processed_at IS NULL AND received_at <= sqlc.arg(received_before);

-- name: PaymentEventsPending :one
-- 這個外部帳戶還有沒被重讀涵蓋的事件（job 執行期間到的）：同步 job 稍後再跑一輪。
SELECT EXISTS (
    SELECT 1
    FROM seller.payment_provider_events
    WHERE provider = $1 AND external_account_id = $2 AND processed_at IS NULL
) AS pending;

-- name: PollablePaymentAccountStoreIDs :many
-- 輪詢補漏：UNDER_REVIEW、ACTION_REQUIRED 的帳戶，DISABLED 但原因碼不是終局（rejected*、unsupported*）的帳戶（restricted 加 contact_stripe 或 no_resolution
-- 可以被 Stripe 解除，掉了 webhook 時只有輪詢補得到；rejected 與 unsupported 不會再啟用，不輪詢），加上任何狀態中 inbox 有 event_before
-- 之前收到卻還沒處理的事件的帳戶（事件在同步寫入快照之後、job 標記完成之前到達時，排入被執行中的 job 合併掉，inbox 是唯一不會漏的來源），依 store_id 分批。
SELECT a.store_id
FROM seller.payment_accounts a
WHERE (a.status IN ('UNDER_REVIEW', 'ACTION_REQUIRED')
       OR (a.status = 'DISABLED'
           AND (a.status_reason IS NULL
                OR (a.status_reason NOT LIKE 'rejected%' AND a.status_reason NOT LIKE 'unsupported%')))
       OR EXISTS (
           SELECT 1 FROM seller.payment_provider_events e
           WHERE e.provider = a.provider AND e.external_account_id = a.external_account_id
             AND e.processed_at IS NULL AND e.received_at < sqlc.arg(event_before)
       ))
  AND a.store_id > sqlc.arg(after_store_id)
ORDER BY a.store_id
LIMIT sqlc.arg(batch);

-- name: StalePaymentAccountCreateStoreIDs :many
-- 保底：核准後建立 job 沒有跑成（例如關店期間被略過）而且沒有進入失敗的帳戶，店家營運中時補排建立。
SELECT p.store_id
FROM seller.payment_accounts p
JOIN seller.stores s ON s.store_id = p.store_id
WHERE p.external_account_id IS NULL AND p.create_failed_at IS NULL AND s.status = 'ACTIVE'
  AND p.updated_at < sqlc.arg(idle_before) AND p.store_id > sqlc.arg(after_store_id)
ORDER BY p.store_id
LIMIT sqlc.arg(batch);
