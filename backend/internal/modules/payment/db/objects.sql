-- payment 的 view、函式與複合型別（單體規則 6）：改這裡要以 `atlas migrate new <名稱> --env dev` 手寫 migration 再 `atlas migrate hash --env dev`。
-- 函式一律 BEGIN ATOMIC、只參照自己的表。

-- 發布的讀取介面：後台待辦的一列（reviewqueue 讀，形狀在 db/pub.sql）。只放 payment 的種類需要的鍵：store_id 是擁有者，店名由後台的入口組裝。
CREATE TYPE payment.pub_review_item AS (
    kind TEXT,
    item_id TEXT,
    store_id BIGINT,
    start_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ
);

-- 發布的讀取介面：at 這一刻營運要處理的 payment 待辦（時間由呼叫方以請求的 clock 傳入，不用 now()），沒有排序，消費端排序與分頁。
-- PAYMENT_ACCOUNT_CREATE_FAILED 建立收款帳戶失敗：建立 job 的重試用完（create_failed_at 有值，還沒有外部 ID）。管理員只能「重試建立」（清空
-- create_failed_at 並重新排入），所以處理完待辦自然消失。item_id 是店家 ID 的文字。開始是失敗的時刻，期限 24 小時（上線前的起始值）。
CREATE FUNCTION payment.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF payment.pub_review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'PAYMENT_ACCOUNT_CREATE_FAILED'::text, a.store_id::text, a.store_id, a.create_failed_at, a.create_failed_at + INTERVAL '24 hours'
    FROM payment.accounts a
    WHERE a.create_failed_at IS NOT NULL;
END;
