-- 發布的讀取介面 seller.pub_review_item／pub_review_items(at)（step10e-3d-design.md §2.1）：11 支 review_<種類>(at) 的 UNION ALL。Atlas 社群版略過函式與複合型別，
-- 這支 migration 是手寫的（`atlas migrate new`，單體規則 6）；定義與 objects.sql 相同（TestSchemaFileMatchesMigrations），註解見 objects.sql。
-- 發布的讀取介面（step10e-3d-design.md §2.1）：at 這一刻營運要處理的事項，後台待辦（reviewqueue 模組）的 seller 部分。每一列是一件待辦：種類、對象的鍵、
-- 店家、開始時間與期限，欄位的意義同 review_item。11 種都有期限，due_at 一律有值（E3）。是 11 支 review_<種類>(at) 的 UNION ALL：成員的條件與時長只寫在那些
-- 函式（規則只有一個家，E10），這裡不重寫；成員與 Go 判定式一致由資料集測試鎖住。時間由呼叫方以請求的 clock 傳入，不用 now()。
-- 消費端排序與分頁（reviewqueue 的 keyset：due_at、start_at、kind COLLATE "C"、item_id COLLATE "C"）；kind 是 UPPER_SNAKE，item_id 的位元組順序穩定。
CREATE TYPE seller.pub_review_item AS (
    kind TEXT,
    item_id TEXT,
    report_id UUID,
    case_id UUID,
    inquiry_id UUID,
    authority_order_id UUID,
    target_position SMALLINT,
    account_id BIGINT,
    measure_id UUID,
    request_id UUID,
    food_registration_id UUID,
    kyc_application_id UUID,
    store_id BIGINT,
    store_name TEXT,
    start_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ
);

CREATE FUNCTION seller.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF seller.pub_review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT * FROM seller.review_report_triage(at)
    UNION ALL
    SELECT * FROM seller.review_case_decision(at)
    UNION ALL
    SELECT * FROM seller.review_case_inquiry_overdue(at)
    UNION ALL
    SELECT * FROM seller.review_case_fda_referral(at)
    UNION ALL
    SELECT * FROM seller.review_authority_order(at)
    UNION ALL
    SELECT * FROM seller.review_authority_target_notice(at)
    UNION ALL
    SELECT * FROM seller.review_measure_appeal(at)
    UNION ALL
    SELECT * FROM seller.review_measure_lift_request(at)
    UNION ALL
    SELECT * FROM seller.review_measure_review(at)
    UNION ALL
    SELECT * FROM seller.review_food_registration(at)
    UNION ALL
    SELECT * FROM seller.review_kyc_application(at);
END;
