-- name: PaymentAccountByStore :one
SELECT *
FROM payment.accounts
WHERE store_id = $1;

-- name: PaymentAccountByExternal :one
SELECT *
FROM payment.accounts
WHERE provider = $1 AND external_account_id = $2;

-- name: LockPaymentAccount :one
-- 鎖定帳戶列序列化它的寫入（全域鎖順序 payment 先於 seller：同步的 transaction 先鎖這裡，再經 PayoutAccountChanged 讓 seller 鎖它的列）。
SELECT *
FROM payment.accounts
WHERE store_id = $1
FOR UPDATE;

-- name: InsertPaymentAccount :execrows
-- seller 核准時在它的 transaction 內寫「建立中」的列（沒有外部 ID），contact_email 是 seller 傳入的聯絡信箱快照。一個擁有者一列，關店再開沿用既有的列；
-- 既有的列不改（ON CONFLICT DO NOTHING）：核准的 transaction 已鎖住 seller 的列，再鎖下層 payment 的既有列會違反全域鎖順序（platform/lockorder），
-- 既有的列只由本模組的 job 與管理員的指令寫。
INSERT INTO payment.accounts (store_id, provider, contact_email, requirement_collection, created_at, updated_at)
VALUES (sqlc.arg(store_id), sqlc.arg(provider), sqlc.arg(contact_email), sqlc.arg(requirement_collection), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (store_id) DO NOTHING;

-- name: SetPaymentAccountExternal :one
-- 外部帳戶建立後填上外部 ID，狀態從 NOT_CREATED 進 ACTION_REQUIRED（新帳戶要賣家補資料，同步 job 重讀後覆寫）；只在還沒有外部 ID 時寫（重送安全）。
UPDATE payment.accounts
SET external_account_id = sqlc.arg(external_account_id),
    status = 'ACTION_REQUIRED',
    create_failed_at = NULL,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NULL
RETURNING *;

-- name: MarkPaymentAccountCreateFailed :execrows
-- 建立重試用完：記下失敗的時刻（進後台待辦）；已經有外部 ID 的列不寫。
UPDATE payment.accounts
SET create_failed_at = sqlc.arg(now),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NULL AND create_failed_at IS NULL;

-- name: ClearPaymentAccountCreateFailed :execrows
-- 管理員重試建立：只有建立失敗的列清空失敗時刻，由呼叫方同一個 transaction 重新排入建立 job。
UPDATE payment.accounts
SET create_failed_at = NULL,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NULL AND create_failed_at IS NOT NULL;

-- name: ApplyPaymentSnapshot :one
-- 同步重讀外部帳戶之後整列覆寫快取；沒有外部 ID 的列不寫。讀取開始時間早於已存的 fetched_at 的不寫（較晚開始的讀取已經先寫入，較早開始的不能蓋掉
-- 較新的快取，D32）：沒有回傳列時呼叫方不標記 inbox 事件、不發布 PayoutAccountChanged。fetched_at 是 NULL（還沒有同步過）一律寫。
UPDATE payment.accounts
SET status = sqlc.arg(status),
    transfers_active = sqlc.arg(transfers_active),
    payouts_active = sqlc.arg(payouts_active),
    requirements = sqlc.arg(requirements),
    status_reason = sqlc.narg(status_reason),
    fetched_at = sqlc.arg(fetched_at),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NOT NULL
  AND (fetched_at IS NULL OR fetched_at <= sqlc.arg(fetched_at))
RETURNING *;

-- name: ClaimPaymentAccountSellerRead :one
-- 賣家觸發的重讀先搶讀取名額再呼叫 provider（D33）：已經有外部 ID、上次賣家觸發的讀取（seller_read_claimed_at，含失敗的與並行的）和上次寫入的讀取
-- （fetched_at）都早於 stale_before 才寫入 seller_read_claimed_at 並回傳列；沒有回傳列就是名額被占走（呼叫方不讀 provider、直接成功）。單一條件式 UPDATE，
-- 並行的呼叫由列鎖序列化，後到的重新檢查條件而拿不到。同步 job 不經過這裡。
UPDATE payment.accounts
SET seller_read_claimed_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NOT NULL
  AND (seller_read_claimed_at IS NULL OR seller_read_claimed_at <= sqlc.arg(stale_before))
  AND (fetched_at IS NULL OR fetched_at <= sqlc.arg(stale_before))
RETURNING *;

-- name: PollablePaymentAccountStoreIDs :many
-- 輪詢補漏：UNDER_REVIEW、ACTION_REQUIRED 的帳戶，DISABLED 但原因碼不是終局（rejected*、unsupported*）的帳戶（restricted 加 contact_stripe 或 no_resolution
-- 可以被 Stripe 解除，掉了 webhook 時只有輪詢補得到；rejected 與 unsupported 不會再啟用，不輪詢），依 store_id 分批。inbox 有舊的未處理事件的帳戶由
-- Sweep 經 platform inbox 的 StaleRoutes 另外找。
SELECT a.store_id
FROM payment.accounts a
WHERE (a.status IN ('UNDER_REVIEW', 'ACTION_REQUIRED')
       OR (a.status = 'DISABLED'
           AND (a.status_reason IS NULL
                OR (a.status_reason NOT LIKE 'rejected%' AND a.status_reason NOT LIKE 'unsupported%'))))
  AND a.store_id > sqlc.arg(after_store_id)
ORDER BY a.store_id
LIMIT sqlc.arg(batch);

-- name: PaymentAccountStoreIDsByExternal :many
-- 外部帳戶 ID 對應的擁有者（inbox 的路由鍵是外部帳戶 ID）。
SELECT store_id
FROM payment.accounts
WHERE provider = sqlc.arg(provider) AND external_account_id = ANY(sqlc.arg(external_account_ids)::text[])
ORDER BY store_id;

-- name: StalePaymentAccountCreateStoreIDs :many
-- 保底：核准後建立 job 沒有跑成（例如擁有者關閉期間被略過）而且沒有進入失敗的帳戶；擁有者是否營運中由呼叫方經 Owners 確認後才補排建立。
SELECT store_id
FROM payment.accounts
WHERE external_account_id IS NULL AND create_failed_at IS NULL
  AND updated_at < sqlc.arg(idle_before) AND store_id > sqlc.arg(after_store_id)
ORDER BY store_id
LIMIT sqlc.arg(batch);

-- name: DeletePaymentAccounts :many
-- 擁有者的個資清理：刪除這些擁有者的帳戶列（含聯絡信箱），回傳有外部帳戶的列的 provider 與外部 ID（呼叫方把它們在 inbox 的未處理事件標為已處理）。
-- 外部帳戶本身不在這裡處理。
DELETE FROM payment.accounts
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[])
RETURNING provider, external_account_id;
